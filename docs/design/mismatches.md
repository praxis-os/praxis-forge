# Design — Mismatches between the proposed forge model and `praxis`

Explicit list of places where the seed brief's vocabulary does not line
up one-to-one with the current praxis runtime surface. Each item states
what the gap is, what forge will do about it, and what (if anything)
might need to change in praxis later.

## Mismatch 1 — `BuiltAgent` wants to feel session-like; praxis is stateless

**Gap.** The seed calls the build output a `BuiltAgent`, which reads as
"an agent you can talk to". Praxis is stateless per turn
(`Orchestrator.Invoke` at `praxis/orchestrator/orchestrator.go:115`
consumes `Messages` and returns `FinalState`, owning no cross-call
history). If `BuiltAgent` held a conversation buffer, forge would be
quietly running a session — which is orchestration.

**Forge resolution.** `BuiltAgent` is a **stateless wiring + metadata
bundle**. Its `Invoke` method is a thin pass-through:

```go
func (b *BuiltAgent) Invoke(ctx context.Context, req praxis.InvocationRequest) (*praxis.InvocationResult, error) {
    return b.orch.Invoke(ctx, req)
}
```

The caller (or `praxis-os`) owns the message buffer across turns.
Forge exposes metadata (manifest, identity, governance profile) but no
session state.

**Praxis implication.** None. Praxis stays as-is.

**See also.** [`adr/0003-memory-strategy-across-three-levels.md`](../adr/0003-memory-strategy-across-three-levels.md)
codifies this cross-level; [`design/memory-and-state.md`](memory-and-state.md)
shows how users own the message buffer at Levels 1 and 2.

## Mismatch 2 — Praxis accepts one policy/filter per stage; forge lists many

**Gap.** The AgentSpec lists arrays of policies and filters. Praxis
accepts exactly one `hooks.PolicyHook` and one `PreLLMFilter` /
`PreToolFilter` / `PostToolFilter` per stage
(`praxis/hooks/interfaces.go:18-62`). There is no built-in chain.

**Forge resolution.** Forge's build layer constructs **composition
adapters** at build time — `policyChain`, `preLLMFilterChain`,
`preToolFilterChain`, `postToolFilterChain` — each implementing the
corresponding single-instance praxis interface and fanning out to the
registered components. Verdict short-circuit rules follow
`praxis/hooks/types.go:29-135` (Deny / RequireApproval stop the chain;
Log continues; Block from any filter aborts the stage).

See [`registry-interfaces.md`](registry-interfaces.md#composition-adapters--the-load-bearing-piece).

**Praxis implication.** Praxis does not need to grow a chain concept.
Forge is the correct altitude for composition.

## Mismatch 3 — No registries in praxis; forge is the first registry layer

**Gap.** The seed talks about a `ComponentRegistry`. Praxis has none:
providers, tools, policies, filters, budgets, resolvers, signers, and
telemetry sinks are passed directly to `orchestrator.New` via functional
options. A declarative layer cannot function without a lookup surface
from string IDs to Go values.

**Forge resolution.** `forge/registry` introduces the first registry in
the stack. Registrations happen at program start; lookups are typed per
kind; runtime plugin loading is explicitly out of scope. The registry
is frozen once a `forge.Build` call uses it.

**Praxis implication.** None. Praxis stays registry-free and
embeddable; forge never pushes registry concepts down into the kernel.

## Mismatch 4 — Seed mentions "expose as MCP"; praxis has no server seam

**Gap.** The seed says forge should eventually support both "consume
MCP" and "expose as MCP". The current praxis MCP surface
(`praxis/mcp/invoker.go:54`) is a **client**: it embeds `tools.Invoker`
and `io.Closer`, exposes `Definitions()` for registered tool schemas,
and namespaces MCP tools as `{Logical}__{mcpName}`. There is no seam
today that turns a configured praxis agent into an MCP server.

**Forge resolution.**

- *Consume MCP* path maps cleanly to Phase 4. A `mcp_binding` factory
  kind builds an `mcp.Invoker`, normalizes the remote tool metadata
  into `ToolDescriptor` entries (risk tier, auth scopes, owner), and
  merges into the main tool router through the same mechanism as
  local tool packs.
- *Expose as MCP* is **designed for** (see
  [`forge-overview.md`](forge-overview.md#phase-roadmap-from-seed-with-module-internal-notes)
  Phase 6 handoff surface) but **parked** until praxis grows a
  server-side seam. Forge will not grow its own MCP server runtime; that
  would duplicate transport concerns and blur the kernel boundary.

**Praxis implication.** A future enhancement to praxis will need to
offer an MCP-server-side wrapper that accepts a configured orchestrator
and exposes it as an MCP server. Track as an upstream request when
Phase 6 begins.

## Mismatch 5 — Approval resumption crosses the session boundary

**Gap.** Praxis emits `errors.ApprovalRequiredError` with an
`ApprovalSnapshot` (`praxis/errors/concrete.go:238`) when a policy
returns `RequireApproval`. Resumption means the caller persists the
snapshot and submits a fresh `InvocationRequest` later. The seed lists
`credentials/auth requirements` and `output contracts` in the spec but
does not describe session persistence.

**Forge resolution.** Forge surfaces the `ApprovalSnapshot` verbatim in
the invocation result (no copying, no flattening). Persistence and
replay are **caller-side concerns** today, and `praxis-os` concerns
tomorrow. Forge does not gain a session/approval store.

**Praxis implication.** None today. If `praxis-os` needs an approval
persistence helper, it will live there, not here.

**See also.** [`adr/0003-memory-strategy-across-three-levels.md`](../adr/0003-memory-strategy-across-three-levels.md)
§ Medium-term; [`design/memory-and-state.md`](memory-and-state.md) shows
the full request → pause → persist → resume sequence with example code.

## Mismatch 6 — Budget ceilings are per-request; spec declares profile + overrides

**Gap.** `budget.Config` (`praxis/budget/types.go:49`) is a flat set of
ceilings attached per `InvocationRequest`. The spec describes a named
`budgetprofile` plus `overrides`. There is no praxis-side notion of
"profile".

**Forge resolution.** A `budget_profile` factory produces both a
`budget.Guard` and a default `budget.Config`. The build layer applies
overlay/spec overrides *at build time* to the default config, then
either (a) stashes the resulting config inside `BuiltAgent` and copies
it onto each outgoing `InvocationRequest`, or (b) exposes the config so
callers can merge with their own per-call values. Phase 1 picks (a) for
simplicity; Phase 2 may expose (b) once per-call overrides become
interesting. Overrides may only **tighten** the profile ceilings, per
spec invariant #10.

**Praxis implication.** None. The default-config application happens
inside forge; praxis continues to accept per-request configs.

## Mismatch 7 — Credentials/identity flow through the registry, not the spec

**Gap.** The spec has `credentials:` and `identity:` blocks, but
credentials and identity keys are sensitive. They must not live in the
spec file.

**Forge resolution.** The spec only names a **registered
resolver/signer factory** and declares the scopes or claim set it needs.
Secret material (API keys, private keys, vault paths) is handed to the
factory at *registration time* in Go, not via the spec. A resolver
factory that requires a secret it was not given fails at registration
time, long before any spec is loaded.

**Praxis implication.** None; this is already how
`praxis/credentials` and `praxis/identity` expect to be wired
(`credentials.Resolver`, `identity.Signer`).

## Mismatch 8 — Deterministic build vs. Go map iteration

**Gap.** Go map iteration order is non-deterministic. Any build step
that serializes, hashes, or diffs a composed spec must impose a
canonical ordering explicitly.

**Forge resolution.** The normalizer produces canonical ordering before
any hashing: alphabetical keys, list ordering normalized for
commutative fields, version numbers fully qualified. Hash inputs are
stable byte sequences. The manifest hash is a reproducibility signal
for Phase 5 lockfiles.

**Praxis implication.** None.

## Non-mismatches worth noting

A few seed concepts map cleanly to praxis and need no workaround:

- **Provider selection** → `llm.Provider` interface
  (`praxis/llm/provider.go:25`). Clean 1:1.
- **Tool invocation** → `tools.Invoker`
  (`praxis/tools/interfaces.go:17`). Clean 1:1.
- **Telemetry sinks** → `telemetry.LifecycleEventEmitter` +
  `AttributeEnricher` (`praxis/telemetry/null.go:22`). Clean 1:1.
- **MCP client normalization** → `mcp.Invoker` embeds `tools.Invoker`
  and namespaces tools already; forge extends with governance metadata
  rather than rebuilding.
- **Null-default safety** → praxis's null implementations
  (`hooks.AllowAllPolicyHook`, `PassThroughPreLLMFilter`, etc.) let
  forge omit optional blocks without branching during `Build`.
