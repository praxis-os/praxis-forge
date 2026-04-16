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

**Phase 1 — minimum vertical slice.** Spec loader, typed
`ComponentRegistry`, 11 factory kinds, composition adapters, and
materialization into a real `*orchestrator.Orchestrator`. 11 concrete
factories ship alongside the kernel wiring; see
[`docs/superpowers/specs/`](docs/superpowers/specs/) for the design
spec and [`docs/superpowers/plans/`](docs/superpowers/plans/) for the
task-by-task implementation plan.

Phase 0 architecture docs remain authoritative:

- [ADR 0001 — praxis-forge layering](docs/adr/0001-praxis-forge-layering.md)
- [ADR 0002 — external registries at dev time only](docs/adr/0002-external-registries-at-devtime-only.md)
- [Design overview](docs/design/forge-overview.md)
- [AgentSpec v0](docs/design/agent-spec-v0.md)
- [Registry interfaces](docs/design/registry-interfaces.md)
- [Default ToolPacks](docs/design/default-toolpacks.md)
- [Default Policy-packs](docs/design/default-policypacks.md)
- [External registries](docs/design/external-registries.md)
- [Mismatches vs. praxis runtime](docs/design/mismatches.md)
- [Source brief (CONTEXT_SEED)](docs/CONTEXT_SEED.md)

## Quickstart

```go
r := registry.NewComponentRegistry()
// Register one factory per kind referenced by your spec.
must(r.RegisterProvider(provideranthropic.NewFactory("provider.anthropic@1.0.0", apiKey)))
must(r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.sys@1.0.0")))
// ... remaining kinds ...

s, err := forge.LoadSpec("agent.yaml")
if err != nil { log.Fatal(err) }

b, err := forge.Build(ctx, s, r)
if err != nil { log.Fatal(err) }

res, err := b.Invoke(ctx, praxis.InvocationRequest{
    Model:        "claude-sonnet-4-5",
    SystemPrompt: b.SystemPrompt(),
    Messages: []llm.Message{{
        Role:  llm.RoleUser,
        Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: "hello"}},
    }},
})
```

See [`examples/demo`](examples/demo/) for a full realistic example
(Anthropic provider + http_get tool + full filter chain). Run with
`ANTHROPIC_API_KEY=sk-... go run -tags=integration ./examples/demo`.

## Module

```
module github.com/praxis-os/praxis-forge
go    1.26
```

Depends on `github.com/praxis-os/praxis` v0.9.0 (remote).
