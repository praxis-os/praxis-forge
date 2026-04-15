# ADR 0001 — Layering: `praxis` → `praxis-forge` → `praxis-os`

- **Status:** Accepted (Phase 0)
- **Date:** 2026-04-15
- **Deciders:** praxis-os maintainers
- **Supersedes:** —
- **Related:** [`docs/CONTEXT_SEED.md`](../CONTEXT_SEED.md), [`design/forge-overview.md`](../design/forge-overview.md), [`design/registry-interfaces.md`](../design/registry-interfaces.md), [`design/agent-spec-v0.md`](../design/agent-spec-v0.md), [`design/default-toolpacks.md`](../design/default-toolpacks.md), [`design/default-policypacks.md`](../design/default-policypacks.md), [`design/external-registries.md`](../design/external-registries.md), [`design/mismatches.md`](../design/mismatches.md)

## Context

The `praxis` module (`github.com/praxis-os/praxis`) is a single-agent
invocation kernel: given a provider, optional tool invoker, policy hook,
filters, budget guard, pricing, credential resolver, identity signer, and
telemetry sinks, it runs one turn of the 14-state FSM and returns an
`InvocationResult`. It is deliberately small and stateless per call
(`orchestrator.New` at `praxis/orchestrator/orchestrator.go:74`,
`Orchestrator.Invoke` at `praxis/orchestrator/orchestrator.go:115`).

Praxis intentionally does **not** provide:

- an `Agent` type that names a specific configuration of the kernel
- registries for providers, tools, policies, filters, MCP bindings, skills
- a declarative description of an agent that can be versioned, diffed, or packaged
- composition helpers that chain multiple policies or filters into the single-instance stage hooks
- deterministic build output with inspectable metadata
- any notion of orchestration across multiple agents

A second layer, `praxis-os`, is planned as the multi-agent
orchestration and control-plane layer. It will need a **stable
unit-of-agent** to schedule, route to, and observe. Today that unit
does not exist: callers wire ad-hoc orchestrators directly, which is
neither versionable nor inspectable.

We need a layer that turns declarative intent into reproducible,
governable, invocable units without leaking orchestration concerns
downward into the kernel or upward into `praxis-os`.

## Decision

Introduce **`praxis-forge`** as an intermediate layer with three
invariant boundaries:

1. `praxis-forge` **depends on** `praxis`; the reverse is forbidden.
   `praxis` must remain unaware of `praxis-forge` and be usable
   standalone.
2. `praxis-forge` **stops before orchestration semantics**. It produces
   stateless `BuiltAgent` units. Conversation history, approval
   resumption, delegation, routing, and workflow graphs are the
   responsibility of the caller or `praxis-os`.
3. `praxis-os` (future) **consumes `BuiltAgent` units**, not raw kernel
   wiring. The handoff contract (invoke surface, capability metadata,
   governance metadata, runtime identity, manifest) is defined by
   forge and frozen before `praxis-os` begins.

Within forge:

- The authored artifact is an `AgentSpec` (declarative, typed, strictly
  validated). Environment-specific adjustments come from `AgentOverlay`.
- All referenced components resolve through a typed `ComponentRegistry`
  populated at program start with factories. Config selects registered
  behavior; it never defines arbitrary runtime behavior.
- `praxis` single-instance stage hooks (one `PolicyHook`, one
  `PreLLMFilter`, one `PreToolFilter`, one `PostToolFilter`) are fed by
  forge-managed composition adapters that fan out to multiple registered
  policies or filters. Praxis sees one instance per stage; forge is the
  layer that composes.
- MCP is adopted as the existing `mcp.Invoker` seam
  (`praxis/mcp/invoker.go:54`). Forge normalizes MCP tool metadata into
  the generic tool descriptor model so downstream governance applies
  uniformly.
- Builds are deterministic and inspectable. The Phase-5 bundle and
  lockfile formalize this; the Phase-1 `BuiltAgent` already carries a
  manifest describing what was resolved and from where.

## Consequences

### Positive

- A named, versioned unit of agent now exists. `praxis-os` can schedule
  and observe it without touching `praxis` directly.
- The first registry layer in the stack lives in forge, which is the
  correct altitude. Praxis stays registry-free and embeddable.
- Governance (policy, filters, budget, identity, credentials) is
  composed declaratively and validated at build time, not scattered
  across call sites.
- `praxis-os` begins life with a stable contract to integrate against,
  not a moving kernel surface.

### Negative / accepted costs

- Forge must implement **composition adapters** for policy and filter
  chains since `praxis` accepts only one instance per stage. This is a
  one-time engineering cost; see [`design/registry-interfaces.md`](../design/registry-interfaces.md).
- Forge introduces a second module to version and release. Contributors
  must decide correctly between kernel seams (praxis) and composition
  concerns (forge).
- Until `praxis` grows a "serve as MCP server" seam, forge can only
  *consume* MCP. The *expose-as-MCP* direction is designed for but
  parked (see Mismatch 4 in [`design/mismatches.md`](../design/mismatches.md)).

### Non-consequences (explicitly ruled out)

- Forge does **not** persist session state. Conversation buffers and
  approval-resume snapshots belong to the caller (today) and to
  `praxis-os` (tomorrow).
- Forge does **not** ship a CLI. The surface is a Go API and a spec
  format. A CLI may appear later but is not a Phase-0 concern.
- Forge does **not** introduce runtime plugin loading. Registries are
  populated programmatically at program start; factories are Go code
  linked into the binary.

## Alternatives considered

1. **Extend `praxis` directly with registries and an `Agent` type.**
   Rejected: couples kernel to declarative/packaging concerns, inflates
   the embedding story, and blocks praxis from staying usable as a
   minimal standalone dependency.
2. **Skip forge; let `praxis-os` consume raw orchestrators.**
   Rejected: duplicates declaration/composition/validation logic inside
   the orchestration layer, fuses two separable concerns, and loses a
   versionable unit-of-agent for non-orchestrated embeddings (scripts,
   tools, tests) that do not want `praxis-os` overhead.
3. **Make forge a thin YAML loader that emits orchestrator options.**
   Rejected: no typed validation, no deterministic build, no metadata
   for downstream governance or orchestration, and the seed explicitly
   forbids this shape.

## References

- Seed brief: [`docs/CONTEXT_SEED.md`](../CONTEXT_SEED.md)
- Kernel entry: `praxis/orchestrator/orchestrator.go:74,115`
- Kernel options: `praxis/orchestrator/options.go:18`
- Kernel request/result: `praxis/request.go:14`, `praxis/result.go:14`
- Policy/filter seams: `praxis/hooks/interfaces.go:18-62`
- MCP seam: `praxis/mcp/invoker.go:54`
- Reference wiring example: `praxis/examples/policy/main.go`
