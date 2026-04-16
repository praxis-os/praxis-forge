# Design — Memory and state: what forge does not own, and how users own it

This document accompanies [`adr/0003-memory-strategy-across-three-levels.md`](../adr/0003-memory-strategy-across-three-levels.md).
The ADR states the decision; this document shows the patterns users follow
because of it.

## Three levels at a glance

| Level | Horizon | Owner | Mechanism | Storage |
|-------|---------|-------|-----------|---------|
| Short-term | Within a single `Invoke` | praxis | `[]llm.Message` grown inside `Orchestrator.Invoke` | In-memory; discarded when the call returns |
| Medium-term | Across `Invoke` calls (session, approval resume) | Caller (today) / `praxis-os` (tomorrow) | Serialize `[]llm.Message` and `ApprovalSnapshot`; re-present on next call | Caller-chosen (file, DB, cache) |
| Long-term | Durable knowledge outside any session (RAG, preferences) | Caller | Retrieval at call site, or `PreLLMFilter`; result enters `req.Messages` | Caller-chosen (vector store, KB) |

Forge owns **nothing** in the right-hand three columns. Everything forge
exposes — `AgentSpec`, `ComponentRegistry`, `BuiltAgent`, `Manifest` — is
about composing the agent, not persisting its state.

## Level 1 — Short-term (within invocation)

Praxis appends messages to `[]llm.Message` turn by turn inside
`Orchestrator.Invoke`. It does not truncate, compress, or summarize.

### When the default is fine

Small conversations, short tool chains, models with generous context
windows. `BuiltAgent.Invoke` is a direct pass-through; nothing to do.

### When you need context-window management

Implement `hooks.PreLLMFilter` (the only seam where the message list may
be mutated before the LLM call), wrap it in a forge
`PreLLMFilterFactory`, register it, and reference it from your spec.

**Interface (praxis):**

```go
type PreLLMFilter interface {
    Filter(ctx context.Context, messages []llm.Message) (
        filtered []llm.Message,
        decisions []FilterDecision,
        err error,
    )
}
```

**Example: sliding-window keep-last-N (sketch).**

```go
package slidingwindow

import (
    "context"

    "github.com/praxis-os/praxis/hooks"
    "github.com/praxis-os/praxis/llm"
)

type Filter struct{ MaxTurns int }

func (f *Filter) Filter(
    ctx context.Context,
    messages []llm.Message,
) ([]llm.Message, []hooks.FilterDecision, error) {
    if len(messages) <= f.MaxTurns {
        return messages, nil, nil
    }
    kept := messages[len(messages)-f.MaxTurns:]
    return kept, []hooks.FilterDecision{{
        Action: hooks.FilterActionRedact,
        Field:  "messages",
        Reason: "sliding-window truncation",
    }}, nil
}
```

**Register with forge:**

```go
type Factory struct{}

func (Factory) ID() registry.ID       { return registry.ID{Name: "sliding-window", Version: "1"} }
func (Factory) Description() string   { return "Keeps the last N messages" }
func (Factory) Build(ctx context.Context, cfg map[string]any) (hooks.PreLLMFilter, error) {
    n, _ := cfg["max_turns"].(int)
    if n <= 0 { n = 20 }
    return &Filter{MaxTurns: n}, nil
}

// At program start:
_ = reg.RegisterPreLLMFilter(Factory{})
```

**Reference from `AgentSpec`:** add under `pre_llm_filters:` with the
factory ID and config. Forge's `preLLMFilterChain` (see
`build/filter_chains.go:15-65`) fans out multiple filters to the single
praxis seam.

### Anti-patterns (short-term)

- Do not use a `PostToolFilter` to trim history — it sees the tool
  result, not the message list, and cannot drop earlier turns.
- Do not try to "compress between invocations" — there is no such point;
  invocations end with `InvocationResult`, and the next call starts fresh
  with whatever `Messages` the caller chose to pass.

## Level 2 — Medium-term (session continuity, approval resume)

Praxis is terminal on approval (D07). Resuming is the caller's job:
persist the snapshot, collect the human decision, rebuild the request,
invoke again.

### Sequence: approval request → resume

```
   caller                       BuiltAgent                praxis.Orchestrator
     │                                │                              │
     │ Invoke(req{Messages: h0})      │                              │
     ├───────────────────────────────▶│──── pass-through ────────────▶│
     │                                │                              │
     │                                │                     policy emits
     │                                │                  RequireApproval
     │                                │                              │
     │                                │◀──── ApprovalRequiredError ──┤
     │◀───── error w/ Snapshot ───────┤       {Messages, Model,      │
     │                                │        SystemPrompt,         │
     │                                │        BudgetAtApproval,     │
     │                                │        ApprovalMetadata,     │
     │                                │        RequestMetadata}      │
     │                                │                              │
  persist snapshot                    │                              │
  to caller store                     │                              │
     │                                │                              │
  collect human                       │                              │
  decision (offline,                  │                              │
  webhook, CLI, …)                    │                              │
     │                                │                              │
  rebuild req: Messages = snapshot.Messages                          │
               SystemPrompt = snapshot.SystemPrompt                  │
               Model = snapshot.Model                                │
               Metadata += approval result                           │
     │                                │                              │
     │ Invoke(req{resumed})           │                              │
     ├───────────────────────────────▶│──── pass-through ────────────▶│
     │                                │                              │
     │◀────── InvocationResult ───────┤◀──────── continue loop ──────┤
```

### Pattern: caller-owned session store

The store is an interface **the caller defines for themselves**. Forge
does not prescribe its shape. A minimal sketch:

```go
// Caller's own package — not forge.

type SessionID string

type Session struct {
    ID            SessionID
    Messages      []llm.Message
    PendingApproval *errors.ApprovalSnapshot // nil unless paused
    Updated       time.Time
}

type Store interface {
    Load(ctx context.Context, id SessionID) (*Session, error)
    Save(ctx context.Context, s *Session) error
}
```

Wire it around `BuiltAgent.Invoke`:

```go
func (a *App) Turn(ctx context.Context, id SessionID, userInput string) (*llm.Message, error) {
    sess, err := a.store.Load(ctx, id)
    if err != nil { return nil, err }

    sess.Messages = append(sess.Messages, llm.Message{
        Role:  llm.RoleUser,
        Parts: []llm.MessagePart{llm.TextPart(userInput)},
    })

    res, err := a.agent.Invoke(ctx, praxis.InvocationRequest{
        Model:        a.defaultModel,
        SystemPrompt: a.systemPrompt,
        Messages:     sess.Messages,
    })

    var approval *errors.ApprovalRequiredError
    if errors.As(err, &approval) {
        sess.PendingApproval = &approval.Snapshot
        _ = a.store.Save(ctx, sess)
        return nil, err // caller decides how to surface this
    }
    if err != nil { return nil, err }

    sess.Messages = append(sess.Messages, *res.Response)
    sess.PendingApproval = nil
    _ = a.store.Save(ctx, sess)
    return res.Response, nil
}

func (a *App) Resume(ctx context.Context, id SessionID, approved bool) error {
    sess, err := a.store.Load(ctx, id)
    if err != nil { return err }
    if sess.PendingApproval == nil {
        return fmt.Errorf("no pending approval for %s", id)
    }

    snap := sess.PendingApproval
    meta := map[string]string{"approval_decision": "approved"}
    if !approved { meta["approval_decision"] = "denied" }

    res, err := a.agent.Invoke(ctx, praxis.InvocationRequest{
        Model:        snap.Model,
        SystemPrompt: snap.SystemPrompt,
        Messages:     snap.Messages,
        Metadata:     meta,
    })
    if err != nil { return err } // could re-pause; handle recursively

    sess.Messages = snap.Messages
    sess.Messages = append(sess.Messages, *res.Response)
    sess.PendingApproval = nil
    return a.store.Save(ctx, sess)
}
```

### Notes

- `BudgetAtApproval` is immutable. The caller decides whether to reset,
  carry forward, or tighten budget on resume by constructing a fresh
  `BudgetConfig` (or omitting it to accept the `BuiltAgent` default).
- Use `AttributeEnricher` for `session_id` telemetry tagging. That seam
  accepts caller-provided attributes (`praxis/telemetry/null.go:37-41`)
  and keeps session identifiers out of the spec/manifest surface.
- A reference implementation of an in-memory and a JSON-file-backed
  `Store` may appear under `examples/session-pattern/` in a future
  phase. It would be example code, not imported by any package under
  `build/`, `registry/`, `spec/`, or `factories/`.

### Anti-patterns (medium-term)

- Do not add `SessionStore`, `Conversation`, or equivalent to any
  `AgentSpec` block. Specs describe agents, not sessions.
- Do not embed a session ID in a factory config. Identifiers flow
  through `InvocationRequest.Metadata` and the enricher, never through
  the registry.
- Do not ask forge to "auto-resume" paused invocations. The caller's
  session logic is where human approval and retry policy live.

## Level 3 — Long-term (RAG, knowledge base, preferences)

Forge does not ship vector stores, embedding adapters, or retrievers.
Two patterns cover the space; pick whichever fits the call site.

### Path A — retrieval at the call site (preferred)

The caller queries its own store, builds the system prompt or prepends a
message, and invokes. Forge is entirely transparent to the retrieval
machinery.

```go
// Caller code.

docs, err := a.kb.Search(ctx, userInput, 5)
if err != nil { return nil, err }

context := renderContext(docs) // caller's own formatting

res, err := a.agent.Invoke(ctx, praxis.InvocationRequest{
    Model:        a.defaultModel,
    SystemPrompt: a.systemPrompt,
    Messages: []llm.Message{
        {Role: llm.RoleSystem, Parts: []llm.MessagePart{llm.TextPart(context)}},
        {Role: llm.RoleUser,   Parts: []llm.MessagePart{llm.TextPart(userInput)}},
    },
})
```

Choose this when:
- retrieval logic needs to live alongside other business logic,
- the store is itself a governable component in the caller's system
  (access control, audit),
- you want retrieval to be visible in caller telemetry rather than forge
  telemetry.

### Path B — retrieval as a `PreLLMFilter`

The user implements a filter that inspects the last user message,
queries a store, and injects context messages. Registered under
`KindPreLLMFilter`, it appears in the manifest like any other component.

```go
type RetrievalFilter struct {
    Store KnowledgeStore
    K     int
}

func (f *RetrievalFilter) Filter(
    ctx context.Context,
    messages []llm.Message,
) ([]llm.Message, []hooks.FilterDecision, error) {
    if len(messages) == 0 { return messages, nil, nil }

    query := lastUserText(messages)
    if query == "" { return messages, nil, nil }

    docs, err := f.Store.Search(ctx, query, f.K)
    if err != nil { return nil, nil, err }

    contextMsg := llm.Message{
        Role:  llm.RoleSystem,
        Parts: []llm.MessagePart{llm.TextPart(render(docs))},
    }
    out := append([]llm.Message{contextMsg}, messages...)
    return out, []hooks.FilterDecision{{
        Action: hooks.FilterActionPass,
        Field:  "messages",
        Reason: "rag-injection",
    }}, nil
}
```

Choose this when:
- retrieval should be governed the same way other components are
  (appears in `Manifest`, bound to a factory ID and version),
- you want retrieval to compose with other filters via the forge filter
  chain (e.g., run retrieval, then apply a sliding-window filter),
- the agent definition should look the same regardless of which caller
  runs it (portability across call sites).

### Anti-patterns (long-term)

- Do not propose a forge-native `KindVectorStore` / `KindRetriever` /
  `KindMemoryStore`. ADR 0003 rules this out.
- Do not cache retrievals inside forge. Caching policy depends on the
  store; the caller or the filter's implementation owns it.
- Do not encode store endpoints or API keys in `AgentSpec`. Secrets
  flow through registered `CredentialResolver` factories, set up at
  program start (ADR 0002 / Mismatch 7).

## Cross-references

- **ADR**: [`adr/0003-memory-strategy-across-three-levels.md`](../adr/0003-memory-strategy-across-three-levels.md)
- **Governing ADR**: [`adr/0001-praxis-forge-layering.md`](../adr/0001-praxis-forge-layering.md) (lines 103-106)
- **Mismatches**: [`design/mismatches.md`](mismatches.md) Mismatch 1 (BuiltAgent stateless) and Mismatch 5 (approval surfaced verbatim)
- **Forge non-goals**: [`design/forge-overview.md`](forge-overview.md) (lines 22-26)
- **Kernel short-term loop**: `praxis/orchestrator/loop.go:142-270`
- **Kernel message-mutation seam**: `praxis/hooks/interfaces.go:34-36`
- **Kernel telemetry enricher**: `praxis/telemetry/null.go:37-41`
- **Kernel D07**: `praxis/docs/phase-1-api-scope/01-decisions-log.md:243-305`
- **Kernel non-goal 3**: `praxis/docs/phase-1-api-scope/03-non-goals.md:52-66`
- **Kernel approval snapshot**: `praxis/errors/concrete.go:212-256`
- **Forge filter chain composition**: `build/filter_chains.go:15-65`
- **Forge registry kinds**: `registry/kind.go:10-22`
- **Forge PreLLMFilter factory interface**: `registry/factories.go:42-47`
