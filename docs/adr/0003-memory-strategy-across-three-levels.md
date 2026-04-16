# ADR 0003 — Memory strategy: forge owns zero of short/medium/long-term memory

- **Status:** Accepted (Phase 1)
- **Date:** 2026-04-16
- **Deciders:** praxis-os maintainers
- **Supersedes:** —
- **Related:** [`adr/0001-praxis-forge-layering.md`](0001-praxis-forge-layering.md), [`design/memory-and-state.md`](../design/memory-and-state.md), [`design/mismatches.md`](../design/mismatches.md) (Mismatch 1 and 5), [`design/forge-overview.md`](../design/forge-overview.md)

## Context

Agent systems have three distinct memory horizons, and each has to live
somewhere:

- **Short-term** — the message history inside a single LLM invocation
  (turn + tool call + tool result, repeated until a final answer).
- **Medium-term** — continuity across invocations: resuming a paused
  conversation after human approval, threading multi-turn dialog, carrying
  budget state forward.
- **Long-term** — retrieval-augmented context (RAG), durable user
  preferences, knowledge-base entries that outlive any single session.

The `praxis` kernel has already made explicit, load-bearing decisions
about these horizons. They are not assumptions we can ignore:

- **Short-term**: praxis grows `[]llm.Message` turn by turn inside
  `Orchestrator.Invoke` (`praxis/orchestrator/loop.go:142-270`). It does
  **not** truncate, compress, or summarize. The only mutation seam before
  the LLM call is `hooks.PreLLMFilter` (`praxis/hooks/interfaces.go:34-36`,
  applied in `loop.go:211-215`), which praxis exposes precisely so higher
  layers can own that policy.
- **Medium-term**: D07 (`praxis/docs/phase-1-api-scope/01-decisions-log.md:243-305`)
  codifies that approval resumption is terminal on the kernel side. The
  caller persists the `ApprovalSnapshot`
  (`praxis/errors/concrete.go:238`), collects the human decision, and
  issues a fresh `Invoke` with the snapshot's messages rehydrated.
- **Long-term**: Phase-1 non-goal 3
  (`praxis/docs/phase-1-api-scope/03-non-goals.md:52-66`) states that
  praxis will **not** ship a vector store, embedding adapter, or memory
  abstraction. RAG users integrate a store independently and pass
  retrieved context into `Invoke` as part of the message list.

The kernel thus publishes a clear contract: **it owns the within-call
message buffer only; everything else is caller-owned**, and it exposes
`PreLLMFilter`, `PostToolFilter`, `AttributeEnricher`, and
`ApprovalSnapshot` as the hook surface for higher layers to build on.

Forge is the layer directly above praxis. Without an explicit decision,
forge could drift into owning memory by convenience — a `Session` type
next to `BuiltAgent`, a `SessionStore` factory kind, an in-process cache
of conversations by session ID. Each of these would:

1. Duplicate choices praxis deliberately delegated upward.
2. Force consumers into forge-specific shapes (storage, scoping,
   identity), violating the portability the kernel's decoupling contract
   preserves.
3. Contradict the "Your Harness, Your Memory" principle (LangChain,
   2026-04): whoever owns the harness owns the memory. Forge is not the
   harness; forge is the builder. Memory belongs to the user (today) and
   to `praxis-os` (tomorrow), not to the definition-and-composition
   layer.

The decision codified here is partially latent in ADR 0001
(`adr/0001-praxis-forge-layering.md:103-106`: "Forge does **not** persist
session state") and in `design/mismatches.md` (Mismatch 1, Mismatch 5),
but it is spread across documents and keyed to specific artifacts
(`BuiltAgent`, `ApprovalSnapshot`) rather than stated cross-level as a
single invariant. A dedicated ADR pins it down so future phases cannot
erode it by accident.

## Decision

Forge owns **zero of the three memory horizons**. For each one, forge
either passes through to praxis unchanged or documents a caller-side
pattern without implementing it.

### Short-term (within invocation): delegate 1:1 to praxis

- `BuiltAgent.Invoke` (`build/build.go:19-24`) is a thin pass-through to
  the embedded `*orchestrator.Orchestrator`. Forge does not wrap, cache,
  or transform the message buffer.
- Forge adds **no** built-in context-window management, sliding-window
  policy, token counter, summarization, or compression. That choice is
  inherently model-, provider-, and use-case-specific, and praxis has
  already published the mutation seam (`PreLLMFilter`) where those
  policies belong.
- Users who need such policies implement `hooks.PreLLMFilter`, register
  it under `KindPreLLMFilter` in the `ComponentRegistry`, and reference
  it from their `AgentSpec`. The existing composition adapter
  (`build/filter_chains.go:15-65`) fans out multi-filter specs into the
  single praxis interface. No new forge machinery is required.

### Medium-term (session continuity, approval resume): caller-owned

- Forge does **not** introduce a `Session`, `Conversation`,
  `SessionStore`, or any equivalent abstraction. `BuiltAgent` stays a
  stateless wiring + metadata bundle as established by Mismatch 1.
- `ApprovalSnapshot` is surfaced verbatim through the invocation result,
  never copied, flattened, or persisted by forge (Mismatch 5). The
  approval-resume loop (obtain `ApprovalRequiredError` → persist snapshot
  → collect human decision → rebuild `InvocationRequest` → re-invoke)
  is caller-owned today and a `praxis-os` concern tomorrow.
- `BudgetSnapshot` inside `ApprovalSnapshot` is immutable at pause time;
  forge will not offer roll-forward semantics. The caller decides budget
  policy on resume.
- Forge ships a documented pattern (see
  [`design/memory-and-state.md`](../design/memory-and-state.md)) but no
  runtime code and no importable helper from `build/`, `registry/`,
  `spec/`, or `factories/`.

### Long-term (RAG, knowledge base, durable preferences): caller-owned

- Forge does **not** define `KindMemoryStore`, `KindVectorStore`,
  `KindRetriever`, or any kindred factory kind. A forge-owned adapter
  would duplicate praxis non-goal 3 at a higher altitude.
- Two caller-side integration paths are documented, neither
  implemented by forge:

  **Path A — retrieval at call site (preferred).** The caller runs
  retrieval against its own store before `Invoke`, builds the system
  prompt and/or prepends context messages in `req.Messages`. Forge
  stays transparent to the retrieval machinery entirely.

  **Path B — retrieval as `PreLLMFilter`.** The user implements a
  filter that inspects the pending messages, queries an external
  store, and injects retrieved chunks. Registered via
  `KindPreLLMFilter`, it appears in the manifest and is governable
  like any other component. Choose this when retrieval needs to sit
  inside the forge-governed composition rather than outside it.

- Neither path requires changes to forge. Both are documented in
  `design/memory-and-state.md` with runnable examples.

### Invariant: no consumer-specific identifiers on the forge API

No `session_id`, `user_id`, `tenant_id`, `conversation_id`, or equivalent
identifier becomes part of the forge public API — neither in
`AgentSpec`, `BuiltAgent.Invoke`, `Manifest`, nor any registered factory
config schema. Such identifiers belong to:

- **Telemetry correlation**: via a caller-provided `AttributeEnricher`
  (`praxis/telemetry/null.go:37-41`), which is exactly what the kernel
  built that seam for.
- **Caller-side metadata**: attached to the caller's own session store
  and indexed there; forge does not index or scope by them.

This invariant is what preserves portability. A user who wants to
migrate from one session backend to another, or export their full
conversation state to a different system, must be able to do so without
rewriting anything forge produced.

## Consequences

### Positive

- **Decoupling contract preserved.** Praxis's D07 + non-goal 3 are
  propagated faithfully through forge. No new consumer-specific concepts
  enter the kernel path.
- **Portability of user state.** Users can serialize and export
  `[]llm.Message`, `ApprovalSnapshot`, and any retrieval context they
  build — none of that material is routed through a forge proprietary
  type.
- **Harness-owner control intact.** The "Your Harness, Your Memory"
  property: memory lives with whoever runs the agent, not with the
  library that composed it. Forge is explicitly not the harness.
- **Guardrail against drift.** Future phases (2-6) cannot introduce
  cross-call cache, session stores, or persistent runtime state without
  superseding this ADR. Reviewers have a concrete rule to point to.
- **Smaller, focused surface.** Forge retains its "definition +
  composition + materialization" scope; it does not grow an
  orchestration or persistence dimension.

### Negative / accepted costs

- **Users write their own session glue.** There is no turnkey
  `forge.Session` abstraction. Mitigated by a pattern document
  ([`design/memory-and-state.md`](../design/memory-and-state.md)) with
  sequence diagram and runnable examples, and optionally a reference
  implementation under `examples/` in a future phase.
- **Context-window policy is caller responsibility.** Users running long
  contexts with models that have tight limits must register their own
  `PreLLMFilter`. Mitigated by the documented pattern and, optionally, a
  future reference filter under `examples/sliding-window-filter/`.
- **No forge-native RAG.** Users who want retrieval must bring their own
  store. This is intentional (non-goal 3 propagation), and matches how
  praxis already framed the problem.

### Non-consequences (explicitly ruled out)

- **No future `KindSessionStore` or `KindMemoryStore`.** A Phase-2+
  proposal to add such a kind must supersede this ADR explicitly.
- **No stateful methods on `BuiltAgent`.** No `StartSession`,
  `ContinueSession`, `ResumeFromSnapshot`, or equivalent. Any helper
  with that shape belongs to the caller or `praxis-os`.
- **No runtime cache for retrieved context.** Phase-5 bundle/lockfile
  covers build-time reproducibility, not runtime memoization. A cache
  proposal at runtime must supersede this ADR.
- **No consumer identifiers in `AgentSpec` or `Manifest`.** Specs describe
  agents; they do not name sessions, users, or tenants.

## Alternatives considered

1. **Introduce a `SessionStore` interface in forge with adapters for
   file / Redis / Postgres.** Rejected: forces consumers into a
   forge-specific storage shape, breaks portability (migrating backends
   means migrating a forge type), and duplicates medium-term memory
   ownership that the kernel already delegated upward. The price of
   "convenient" session helpers is exactly the decoupling we are
   protecting.
2. **Extend `BuiltAgent` with stateful methods (`StartSession`,
   `ContinueSession`).** Rejected: blurs forge into an orchestration
   layer. `BuiltAgent` is a stateless wiring + metadata bundle (Mismatch
   1); adding session-lifecycle methods fundamentally changes its shape
   and contradicts ADR 0001 invariant 2 ("praxis-forge stops before
   orchestration semantics").
3. **Add a `KindMemoryStore` / `KindVectorStore` factory kind for RAG
   integrations.** Rejected: duplicates praxis non-goal 3 at a higher
   altitude. A forge-owned vector-store interface would bind users to
   our shape and evolve with our release cadence, losing the portability
   that made the kernel's non-goal valuable in the first place. Users
   who want RAG already have two paths (call-site injection or
   `PreLLMFilter`) that use existing seams.
4. **Silently treat this as "obvious" and leave the decision scattered
   across ADR 0001 and mismatches.md.** Rejected: the latent decision is
   keyed to specific artifacts (`BuiltAgent`, `ApprovalSnapshot`) and
   does not name the three-level framing. Without an explicit cross-level
   ADR, Phase-2+ contributors can inadvertently introduce state (runtime
   lockfile cache, skill persistent memory, per-call retrieval
   memoization) without recognizing they are crossing a line.
5. **Push memory concerns up to `praxis-os` only, and document nothing
   in forge.** Rejected: forge has users today (Phase-1 ships a real
   `forge.Build` API). Those users need guidance *now* on how to own
   their memory; they cannot wait for `praxis-os`. The pattern
   document is small and does not expand forge's runtime surface.

## References

- Governing ADR: [`adr/0001-praxis-forge-layering.md`](0001-praxis-forge-layering.md) (lines 103-106)
- Session-related mismatches: [`design/mismatches.md`](../design/mismatches.md) (Mismatch 1 lines 8-30, Mismatch 5 lines 95-110)
- Non-goals: [`design/forge-overview.md`](../design/forge-overview.md) (lines 22-26)
- Pattern guide for users: [`design/memory-and-state.md`](../design/memory-and-state.md)
- Stateless pass-through: `build/build.go:19-24`, `build/build.go:93`
- Composition adapter for filters: `build/filter_chains.go:15-65`
- Factory kinds (unchanged): `registry/kind.go:10-22`
- Kernel short-term loop: `praxis/orchestrator/loop.go:142-270`
- Kernel message-mutation seam: `praxis/hooks/interfaces.go:34-36`
- Kernel telemetry enricher: `praxis/telemetry/null.go:37-41`
- Kernel D07 (approval semantics): `praxis/docs/phase-1-api-scope/01-decisions-log.md:243-305`
- Kernel non-goal 3 (no memory/vector abstractions): `praxis/docs/phase-1-api-scope/03-non-goals.md:52-66`
- Kernel approval snapshot: `praxis/errors/concrete.go:238`, populated at `praxis/orchestrator/loop.go:368-374`
