# praxis-forge

The agent **definition, composition, and materialization** layer of the
praxis stack.

```
praxis          invocation kernel  (single-agent loop, stateless per turn)
praxis-forge    declarative agent definition + composition + build  ← you are here
praxis-os       multi-agent orchestration + control plane (future)
```

## What this is

`praxis-forge` takes a declarative `AgentSpec`, resolves registered
components through typed factories, validates the composition strictly,
and materializes a reproducible `BuiltAgent` backed by a configured
[`praxis`](../praxis) `Orchestrator`.

## What this is not

- Not a YAML loader. Specs are typed, validated, and composable.
- Not a prompt wrapper. It governs tools, policies, filters, budgets,
  credentials, identity, and MCP bindings in a single deterministic build.
- **Not** a mini-orchestrator. No routing, delegation, planners,
  supervisors, workflow graphs, team semantics, or long-lived sessions.
  Those live in `praxis-os`.

## Status

**Phase 0 — architecture and contracts.** No runtime code yet.
See [`docs/`](docs/):

- [ADR 0001 — praxis-forge layering](docs/adr/0001-praxis-forge-layering.md)
- [Design overview](docs/design/forge-overview.md)
- [AgentSpec v0](docs/design/agent-spec-v0.md)
- [Registry interfaces](docs/design/registry-interfaces.md)
- [Mismatches vs. praxis runtime](docs/design/mismatches.md)
- [Source brief (CONTEXT_SEED)](docs/CONTEXT_SEED.md)

Phase 1 will add the first vertical slice: spec loader, typed registry with
one provider / one policy pack / one tool pack, and materialization into a
real `*orchestrator.Orchestrator`.

## Module

```
module github.com/praxis-os/praxis-forge
go    1.26
```

Depends on `github.com/praxis-os/praxis` via a local `replace` directive
pointing at `../praxis` during Phase 0. A published tag will replace the
directive once the kernel stabilizes.
