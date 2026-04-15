# Design — Registry interfaces

Typed component resolution for `praxis-forge`. Phase 0 proposal; the Go
code lands in Phase 1.

## Why forge needs a registry at all

`praxis` has no registries — every seam is wired as a single instance
passed to `orchestrator.New` via functional options
(`praxis/orchestrator/options.go:18`). That is correct for a minimal
kernel, but a declarative layer needs to:

- resolve `ref: provider.anthropic@1` (a string) to an actual
  `llm.Provider` (a Go value);
- validate the user's `config:` map against the target's expected
  shape *before* anything is constructed;
- enumerate what is registered so tooling can inspect/diff an agent;
- fail closed when a spec references something that does not exist.

The registry is therefore a forge invention — the first registry layer
in the whole stack — sitting between declarative spec and kernel
instantiation.

## Core concepts

```go
package registry

// Kind is a closed enum of component categories forge knows about.
type Kind string

const (
    KindProvider           Kind = "provider"
    KindToolPack           Kind = "tool_pack"
    KindPolicyPack         Kind = "policy_pack"
    KindPreLLMFilter       Kind = "pre_llm_filter"
    KindPreToolFilter      Kind = "pre_tool_filter"
    KindPostToolFilter     Kind = "post_tool_filter"
    KindBudgetProfile      Kind = "budget_profile"
    KindTelemetryProfile   Kind = "telemetry_profile"
    KindCredentialResolver Kind = "credential_resolver"
    KindIdentitySigner     Kind = "identity_signer"
    // Deferred:
    KindSkill      Kind = "skill"       // Phase 3
    KindMCPBinding Kind = "mcp_binding" // Phase 4
)

// ID uniquely addresses a registered factory for a given Kind.
// Format: `<dotted-name>@<semver>`. IDs are stable; a new version is a
// new registration, not an in-place update.
type ID string
```

### `Factory` — one interface, one Build method per kind

The registry stores typed factories. Each factory carries its own input
schema (JSON Schema or a Go-typed decoder contract) and produces the
corresponding praxis seam.

Illustrative sketch — actual Go types will be split per kind to avoid
generic-map inputs:

```go
// ProviderFactory constructs an llm.Provider from typed config.
type ProviderFactory interface {
    ID() ID
    Kind() Kind                          // always KindProvider
    Description() string
    ConfigSchema() json.RawMessage       // JSON Schema describing config
    Build(ctx context.Context, raw json.RawMessage) (llm.Provider, error)
}

// ToolPackFactory constructs a tools.Invoker plus declared metadata.
type ToolPackFactory interface {
    ID() ID
    Kind() Kind                          // KindToolPack
    Description() string
    ConfigSchema() json.RawMessage
    Build(ctx context.Context, raw json.RawMessage) (ToolPack, error)
}

// ToolPack is the value produced by a tool-pack factory. It is what the
// build layer merges into the final tools.Invoker plus the declared tool
// descriptor metadata that feeds the manifest.
type ToolPack struct {
    Invoker     tools.Invoker
    Definitions []llm.ToolDefinition  // schemas exposed to the model
    Descriptors []ToolDescriptor      // forge-managed governance metadata
}

// ToolDescriptor adds governance-grade metadata on top of the raw
// llm.ToolDefinition: risk tier, policy tags, auth scopes, owner, SLO,
// timeout/retry hints, audit references. The manifest carries these.
type ToolDescriptor struct {
    Name        string
    Owner       string
    RiskTier    RiskTier   // low | moderate | high | destructive
    PolicyTags  []string   // matched against policy-pack tag predicates
    AuthScopes  []string
    TimeoutHint time.Duration
    RetryHint   RetryHint
    Source      string     // e.g. "toolpack.jira-read@3"
}
```

Analogous factory interfaces exist for the other kinds:

- `PolicyPackFactory.Build(ctx, raw) (PolicyPack, error)` —
  `PolicyPack.Hook hooks.PolicyHook` plus a list of `PolicyDescriptor`
  entries for manifest/audit.
- `PreLLMFilterFactory.Build(...) (hooks.PreLLMFilter, error)`
- `PreToolFilterFactory.Build(...) (hooks.PreToolFilter, error)`
- `PostToolFilterFactory.Build(...) (hooks.PostToolFilter, error)`
- `BudgetProfileFactory.Build(...) (BudgetProfile, error)` —
  produces `budget.Guard` + default `budget.Config`.
- `TelemetryProfileFactory.Build(...) (TelemetryProfile, error)` —
  produces `telemetry.LifecycleEventEmitter` + `telemetry.AttributeEnricher`.
- `CredentialResolverFactory.Build(...) (credentials.Resolver, error)`
- `IdentitySignerFactory.Build(...) (identity.Signer, error)`

### `ComponentRegistry`

```go
type ComponentRegistry struct { /* unexported */ }

func NewComponentRegistry() *ComponentRegistry

// Registration — one method per kind to preserve static typing.
func (r *ComponentRegistry) RegisterProvider(f ProviderFactory) error
func (r *ComponentRegistry) RegisterToolPack(f ToolPackFactory) error
func (r *ComponentRegistry) RegisterPolicyPack(f PolicyPackFactory) error
// ... and so on per kind

// Lookup — one method per kind, again for static typing.
func (r *ComponentRegistry) Provider(id ID) (ProviderFactory, error)
func (r *ComponentRegistry) ToolPack(id ID) (ToolPackFactory, error)
// ... and so on

// Introspection — generic enumeration for tooling and manifests.
func (r *ComponentRegistry) Each(fn func(Kind, ID, Factory))
```

Invariants:

- Registration is frozen after a registry is passed to `forge.Build`.
  Each registry is effectively immutable once first used.
- Duplicate `(kind, id)` registration returns an error at registration
  time.
- Lookups are O(1) and return a typed factory; no `any` leaks into the
  build layer.

## Composition adapters — the load-bearing piece

`praxis` accepts only one `hooks.PolicyHook` and one filter per stage.
A declarative spec typically lists several. Forge bridges this gap
with **chain adapters** built at build time, not at registration time:

```go
// Owned by forge/build. Illustrative.
type policyChain []hooks.PolicyHook

func (c policyChain) Evaluate(
    ctx context.Context,
    phase hooks.Phase,
    in hooks.PolicyInput,
) (hooks.Decision, error) {
    for _, h := range c {
        d, err := h.Evaluate(ctx, phase, in)
        if err != nil {
            return hooks.Decision{}, err
        }
        switch d.Verdict {
        case hooks.VerdictDeny, hooks.VerdictRequireApproval:
            return d, nil                  // short-circuit
        case hooks.VerdictLog:
            // record, continue
        case hooks.VerdictAllow, hooks.VerdictContinue:
            continue
        }
    }
    return hooks.Allow(), nil
}
```

Filter chains follow the same pattern, respecting the stage's
action semantics (Pass / Redact / Log / Block from
`praxis/hooks/types.go:109-135`). A `Block` from any filter in a chain
aborts the stage.

Tool-pack merging is a router keyed on tool name with a conflict policy
(default: reject on duplicate name; overlays may set
`onConflict: prefer-last`). The merged router implements `tools.Invoker`
and delegates by name.

## Factory registration shape (consumer side)

Registration happens in Go, at program start. A typical main:

```go
func newRegistry() *registry.ComponentRegistry {
    r := registry.NewComponentRegistry()

    must(r.RegisterProvider(anthropicfactory.New("anthropic@1")))
    must(r.RegisterProvider(openaifactory.New("openai@1")))

    must(r.RegisterToolPack(jiraread.NewFactory("toolpack.jira-read@3")))
    must(r.RegisterPolicyPack(piiredaction.NewFactory("policypack.pii-redaction@1")))

    must(r.RegisterBudgetProfile(defaulttier1.NewFactory("budgetprofile.default-tier1@1")))
    must(r.RegisterTelemetryProfile(otelprofile.NewFactory("telemetryprofile.acme-otel@1")))
    must(r.RegisterCredentialResolver(vault.NewFactory("credresolver.acme-vault@1")))
    must(r.RegisterIdentitySigner(ed25519factory.New("identitysigner.acme-ed25519@1")))

    return r
}
```

No runtime plugin loading. No reflection. Extending the registry means
linking new Go code.

## Open design questions

- **JSON Schema vs. Go-typed decoders.** JSON Schema is universal and
  usable by tooling; typed Go decoders produce better compile-time
  guarantees but require a shadow schema export for introspection.
  Phase 1 will probably use both: the factory embeds a `*jsonschema.Schema`
  and the `Build` method unmarshals into its own typed struct.
- **Versioning at lookup time.** Should `ref: foo@1` match
  `foo@1.2.3` via semver compatibility, or require exact match? v0
  leans exact-match for determinism; semver range resolution can be
  added in Phase 2 with a lockfile.
- **Multi-provider agents.** Provider-fallback is handled inside a
  single `provider` factory for now. A multi-provider top-level field
  is deferred until we have a use case that cannot be expressed with a
  composite factory.
