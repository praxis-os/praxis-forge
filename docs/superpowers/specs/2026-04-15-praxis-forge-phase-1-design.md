# praxis-forge — Phase 1 design spec

**Date:** 2026-04-15
**Milestone:** Phase 1 — minimum vertical slice
**Status:** Approved for implementation planning
**Companion plan:** `/Users/francescofiore/.claude/plans/immutable-frolicking-deer.md`

## Context

`praxis-forge` is the declarative agent definition/composition/materialization layer between [`praxis`](../../../) (invocation kernel, stateless per turn) and future `praxis-os` (multi-agent orchestration). Phase 0 shipped five architecture documents — [ADR 0001](../../adr/0001-praxis-forge-layering.md), [forge-overview](../../design/forge-overview.md), [agent-spec-v0](../../design/agent-spec-v0.md), [registry-interfaces](../../design/registry-interfaces.md), [mismatches](../../design/mismatches.md) — and zero runtime code. The module holds only [doc.go](../../../doc.go), [README.md](../../../README.md), and [go.mod](../../../go.mod) (Go 1.26, local `replace` directive to `../praxis`).

Phase 1 produces the first vertical slice: an `AgentSpec` YAML loads, strictly validates, resolves every referenced component through a typed registry, and materializes a stateless `BuiltAgent` backed by `*orchestrator.Orchestrator`. One concrete factory per kernel seam ships alongside a realistic demo agent. Goal: prove the declarative → runtime path end-to-end before layering on overlays, skills, MCP consume, or packaging.

## Brainstorm decisions

Six locked decisions shape the scope:

1. **Full kernel surface.** One factory per seam (11 Kinds including `prompt_asset`).
2. **Typed Go structs + custom validator.** No JSON Schema library in Phase 1. `ConfigSchema() json.RawMessage` drops from the factory interface — amendment to [registry-interfaces.md](../../design/registry-interfaces.md).
3. **Real Anthropic provider** wrapping praxis's `anthropic` package.
4. **Realistic demo agent**: PII-redaction policy + HTTP-GET tool. Not a smoke-test.
5. **Strict Phase 1 composition depth.** No overlays, no `extends`, no lockfile, no canonical hashing. Simple Manifest only. `forge.Build` signature has no `overlays` parameter.
6. **Module-cohesive package layout** (Approach A). `spec/`, `registry/`, `build/`, `manifest/`, `factories/<kind>/`, `examples/demo/`.

## Section 1 — Public API + layering

### Package map

```
praxis-forge/
  forge.go                 # facade: LoadSpec, Build, Option, BuiltAgent
  spec/                    # AgentSpec types, YAML loader, strict validator
  registry/                # ComponentRegistry + 11 Factory interfaces
  build/                   # resolver, chain adapters, composer, materializer
  manifest/                # Manifest + stable JSON marshal
  factories/<kind>/        # one leaf pkg per concrete factory
  examples/demo/           # realistic end-to-end demo
  internal/testutil/       # fakeprovider, clock, canned messages
```

### Public surface

```go
package forge

func LoadSpec(path string) (*spec.AgentSpec, error)
func Build(ctx context.Context, s *spec.AgentSpec, r *registry.ComponentRegistry, opts ...Option) (*BuiltAgent, error)

type BuiltAgent struct { /* unexported; orch + manifest */ }
func (b *BuiltAgent) Invoke(ctx context.Context, req praxis.InvocationRequest) (*praxis.InvocationResult, error)
func (b *BuiltAgent) Manifest() manifest.Manifest

type Option func(*options)
func WithLogger(*slog.Logger) Option
```

### Dependency rule

`spec` depends on nothing internal. `registry` depends on `praxis` kernel types only. `build` imports `spec`, `registry`, `manifest`, `praxis`. Enforced by layout and `depguard` (lint).

### ADR amendments from Phase 0 docs

- [registry-interfaces.md:67](../../design/registry-interfaces.md) — drop `ConfigSchema() json.RawMessage` from factory interfaces (typed-only validator). Add `KindPromptAsset` and `PromptAssetFactory`.
- [forge-overview.md](../../design/forge-overview.md) — `forge.Build` in Phase 1 has no `overlays` parameter; added in Phase 2.
- [agent-spec-v0.md](../../design/agent-spec-v0.md) — resolve open question on inline prompts: Phase 1 requires registered prompt assets. Add `prompt_asset` to the referenced-kinds table.

## Section 2 — Spec, Registry, Build contracts

### spec/

```go
type AgentSpec struct {
    APIVersion string         `yaml:"apiVersion"`
    Kind       string         `yaml:"kind"`
    Metadata   Metadata       `yaml:"metadata"`
    Provider   ComponentRef   `yaml:"provider"`
    Prompt     PromptBlock    `yaml:"prompt"`
    Tools      []ComponentRef `yaml:"tools"`
    Policies   []ComponentRef `yaml:"policies"`
    Filters    FilterBlock    `yaml:"filters"`
    Budget     *BudgetRef     `yaml:"budget"`
    Telemetry  *ComponentRef  `yaml:"telemetry"`
    Credentials *CredRef      `yaml:"credentials"`
    Identity   *ComponentRef  `yaml:"identity"`
    // Phase-gated: must be empty in v0
    Extends        []string      `yaml:"extends"`
    Skills         []ComponentRef `yaml:"skills"`
    MCPImports     []ComponentRef `yaml:"mcpImports"`
    OutputContract *ComponentRef  `yaml:"outputContract"`
}

type ComponentRef struct {
    Ref    string         `yaml:"ref"`     // "<dotted>@<semver>"
    Config map[string]any `yaml:"config"`  // passed raw to factory
}
```

**Loader**: `yaml.v3` with `decoder.KnownFields(true)` — strict unknown-key rejection.

**Validator invariants** (Phase 1):
1. `apiVersion == "forge.praxis-os.dev/v0"` and `kind == "AgentSpec"`.
2. `metadata.id` matches dotted-lowercase regex; `metadata.version` is valid semver.
3. Every `ref` matches `<dotted>@<semver>` format.
4. `extends`, `skills`, `mcpImports`, `outputContract` empty → phase-gated error otherwise.
5. No duplicate tool-pack `ref` within `tools[]`.
6. `provider` and `prompt.system` required; everything else optional.
7. `budget.overrides` loosening rejected (enforced in `build/budget.go` after factory default is known).

### registry/

```go
type Kind string

const (
    KindProvider           Kind = "provider"
    KindPromptAsset        Kind = "prompt_asset"
    KindToolPack           Kind = "tool_pack"
    KindPolicyPack         Kind = "policy_pack"
    KindPreLLMFilter       Kind = "pre_llm_filter"
    KindPreToolFilter      Kind = "pre_tool_filter"
    KindPostToolFilter     Kind = "post_tool_filter"
    KindBudgetProfile      Kind = "budget_profile"
    KindTelemetryProfile   Kind = "telemetry_profile"
    KindCredentialResolver Kind = "credential_resolver"
    KindIdentitySigner     Kind = "identity_signer"
)

type ID string // "<dotted>@<semver>"

type ProviderFactory interface {
    ID() ID
    Description() string
    Build(ctx context.Context, cfg map[string]any) (llm.Provider, error)
}
// Analogous interface per Kind — 11 total. PromptAssetFactory returns (string, error).

type ComponentRegistry struct { /* unexported map[Kind]map[ID]any */ }
func NewComponentRegistry() *ComponentRegistry
func (r *ComponentRegistry) RegisterProvider(f ProviderFactory) error // +10 siblings
func (r *ComponentRegistry) Provider(id ID) (ProviderFactory, error)  // +10 siblings
func (r *ComponentRegistry) Freeze()                                  // called by forge.Build
```

**Invariants**:
- Duplicate `(kind, id)` registration → error at register time.
- After `Freeze`, all `Register*` return `ErrRegistryFrozen`.
- Wrong-kind lookup → typed error.
- No `any` leaks to the build layer.

### build/

Pipeline stages:

```
parse → validate → freeze registry → resolve refs → build factories
      → compose (chains + router) → apply budget overrides
      → orchestrator.New(provider, opts…) → wrap in BuiltAgent + Manifest
```

**Chain adapters (the load-bearing piece)** per [mismatch #2](../../design/mismatches.md):

- `policyChain []hooks.PolicyHook` — short-circuit: Deny / RequireApproval returns immediately; Log records and continues; Allow / Continue continues. Empty chain → `hooks.AllowAllPolicyHook`.
- `preLLMFilterChain`, `preToolFilterChain`, `postToolFilterChain` — per-stage action cascade. `FilterActionBlock` aborts; `FilterActionRedact` mutates the flowing value; `FilterActionLog` records; `FilterActionPass` no-ops. Empty chain → praxis passthrough.
- `toolRouter` — map of tool name → `tools.Invoker`. Duplicate name across packs is a build error. Implements `tools.Invoker`.

**Materialize**: `orchestrator.New(provider, withOpts…)` wrapped in `BuiltAgent{orch, manifest}`. `BuiltAgent.Invoke` is pass-through per [mismatch #1](../../design/mismatches.md).

## Section 3 — Factories + tests + delivery

### 11 concrete factories

| Kind | Factory ID | Notes |
|------|-----------|-------|
| provider | `provider.anthropic@1` | Wraps praxis `anthropic.New`. API key injected at factory construction (not in spec, per [mismatch #7](../../design/mismatches.md)). |
| prompt_asset | `prompt.literal@1` | Config: `text string`. Returns verbatim. Consumers register many literals under distinct IDs. |
| tool_pack | `toolpack.http-get@1` | One tool `http_get(url)`. Config: `allowedHosts []string`, `timeoutMs int`. Blocks non-allowlisted hosts. Returns full body (truncation is filter's job). |
| policy_pack | `policypack.pii-redaction@1` | Regex bank. Config: `strictness low\|medium\|high`. `high` denies on SSN/CC. |
| pre_llm_filter | `filter.secret-scrubber@1` | Redacts `sk-*`, `ghp_*`, AWS key patterns. `FilterActionRedact`. |
| pre_tool_filter | `filter.path-escape@1` | Blocks `../` traversal in tool args. `FilterActionBlock`. |
| post_tool_filter | `filter.output-truncate@1` | Config: `maxBytes int`. Truncate + `FilterActionLog`. |
| budget_profile | `budgetprofile.default-tier1@1` | 30s / 50k in / 10k out / 24 calls / 500k µ$. |
| telemetry_profile | `telemetryprofile.slog@1` | One slog line per event. Enricher reads tenant/user from context. |
| credential_resolver | `credresolver.env@1` | `os.Getenv` by scope-to-envvar transform. `Close` zeroes backing slice. |
| identity_signer | `identitysigner.ed25519@1` | Wraps `identity.NewEd25519Signer`. Key injected at factory construction. |

Each factory: ~100–300 LOC, own leaf package, table-driven unit tests covering valid/invalid config and one happy-path + one edge case behavior.

### Testing strategy

- **Unit**: every factory, spec parser, each validator invariant, registry semantics, every chain adapter.
- **Integration (offline)**: `forge_test.go` runs full parse → build → invoke against `internal/testutil/fakeprovider` (canned `LLMResponse`). Exercises every factory via a fixture spec.
- **Live**: `examples/demo` tagged `//go:build integration`. Requires `ANTHROPIC_API_KEY`. CI runs unit only by default.
- **Golden fixtures**: `spec/testdata/valid/*.yaml`, `spec/testdata/invalid/*.yaml` + paired `*.err.txt` (exact substring).

### Dev + CI

- Go 1.26; `go test ./... -race`; `golangci-lint run ./...` (reuse praxis config).
- No CLI.

### Delivery shape

Single track, five atomic commits for review:
1. `spec/` — types, loader, validator, fixtures.
2. `registry/` — Kind enum, 11 Factory interfaces, freeze semantics.
3. `build/` + `manifest/` — resolver, chain adapters, materializer, manifest.
4. `factories/` — 11 leaf packages + unit tests.
5. `forge.go` facade + `examples/demo` + integration test harness + README update.

A sixth doc-only commit lands the design-doc amendments listed in Section 1.

## Verification

1. `go test ./... -race` green (offline; fakeprovider).
2. `golangci-lint run ./...` clean.
3. `ANTHROPIC_API_KEY=… go test -tags=integration ./examples/demo/...` round-trips Anthropic + `http_get`.
4. `built.Manifest()` lists all 11 resolved factory IDs with configs.
5. Every `spec/testdata/invalid/*.yaml` fails `spec.Validate()` with the paired `.err.txt` substring.
6. Post-`Build` registry registration returns `ErrRegistryFrozen`.

## Out of scope (explicit deferrals)

- Overlays, `extends`, canonical hashing, lockfile → Phase 2
- Skills → Phase 3
- MCP consume → Phase 4
- Bundle / packaging → Phase 5
- `praxis-os` handoff contract freeze → Phase 6
- CLI — not in this milestone
- Expose-as-MCP — parked until praxis adds a server-side seam ([mismatch #4](../../design/mismatches.md))
