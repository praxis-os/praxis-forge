# Design — `AgentSpec` v0

Initial shape of the declarative agent definition. Phase 0 proposal;
Phase 1 ships the Go types and the YAML parser.

This document freezes the **top-level shape** and the **invariants**.
Fields marked `(Phase N)` are part of the target surface but are
deferred to a later phase — the schema rejects them until their phase
ships.

## Top-level shape

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.support-triage         # stable, dotted, lowercase
  version: 1.4.0                  # semver, required
  displayName: "Support Triage"
  description: "Triages inbound support tickets and routes them."
  owners:
    - team: platform-ai
      contact: platform-ai@acme.example
  labels:
    domain: support
    tier: production

extends:                          # optional; acyclic; depth ≤ 8; Phase 2a
  - acme.base-agent@2.0.0         # resolved via SpecStore passed to Build

provider:                         # required
  ref: provider.anthropic         # factory id in the registry
  config:
    model: claude-sonnet-4-5
    maxOutputTokens: 2048
    temperature: 0.2

prompt:                           # required
  system:
    ref: prompt.acme.triage-system@1
  user: null                      # user messages are caller-supplied per invoke

tools:                            # zero or more tool packs
  - ref: toolpack.jira-read@3
    config:
      projectKey: SUP
  - ref: toolpack.kb-search@2

mcpImports: []                    # Phase 4

skills: []                        # Phase 3

policies:                         # composed into one praxis PolicyHook
  - ref: policypack.pii-redaction@1
  - ref: policypack.jailbreak-guard@2
    config:
      strictness: high

filters:                          # composed per-stage
  preLLM:
    - ref: filter.secret-scrubber@1
  preTool:
    - ref: filter.path-escape@1
  postTool:
    - ref: filter.output-truncate@1
      config:
        maxBytes: 16384

budget:                           # selects a registered budget profile
  ref: budgetprofile.default-tier1@1
  overrides:
    maxWallClock: 45s
    maxToolCalls: 24

telemetry:
  ref: telemetryprofile.acme-otel@1

credentials:
  ref: credresolver.acme-vault@1
  scopes:
    - jira:read
    - kb:search

identity:
  ref: identitysigner.acme-ed25519@1

outputContract:                   # Phase 3 (skills-driven); optional
  null
```

## Overlays

Overlays are typed YAML documents whose body mirrors `AgentSpec` with
every field optional. They are applied **after** `extends:` resolution
and **before** validation of the merged result. Overlays are sibling
files passed to `forge.Build` via `forge.WithOverlays(...)`; they are
not embedded inside an `AgentSpec`.

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: prod-override            # attribution label, surfaced in errors + manifest
spec:
  provider:                      # any subset of AgentSpec fields
    ref: provider.anthropic@1.0.0
    config:
      model: claude-opus-4-7
  policies:                      # replace by default
    - ref: policypack.staging-observer@1.0.0
  tools: []                      # explicit empty clears the base list
```

Phase 2a merge semantics:

- **Lists** (`tools`, `policies`, `filters.preLLM`, etc.) replace the
  base list when set. The `RefList` wrapper distinguishes "absent"
  (preserve base), "explicit null/empty" (clear), and "populated"
  (replace).
- **`Config map[string]any`** blocks replace the entire map (no deep
  merge across opaque schemas).
- **`metadata.labels`** replaces, not merges.
- **Scalars** that are zero/empty in the overlay preserve the base.

Locked fields (`apiVersion`, `kind`, `metadata.id`, `metadata.version`)
cannot be touched by an overlay; attempting to do so produces
`spec.ErrLockedFieldOverride` with attribution to the source overlay.

Phase-gated AgentSpec fields (`extends`, `skills`, `mcpImports`,
`outputContract`) are deliberately absent from the overlay body, so
the strict YAML decoder rejects them at parse time.

Overlay count is bounded by `spec.MaxOverlayCount` (16); exceeding the
bound returns `spec.ErrCompositionLimit`.

## Referenced kinds (registry factory IDs)

Every `ref:` resolves to a factory registered in the
`ComponentRegistry`. The factory kind is inferred from the schema slot,
not from the ID itself (the ID is an opaque stable identifier). The
registry rejects the build if a referenced id + version is not
registered, or if it is registered under a different kind.

Kinds in v0:

| Slot             | Kind                    | Resolves to praxis seam                       |
|------------------|-------------------------|-----------------------------------------------|
| `provider`       | `provider`              | `llm.Provider`                                |
| `prompt.system`  | `prompt_asset`          | `string` injected per-invoke as `SystemPrompt`|
| `tools[]`        | `tool_pack`             | `tools.Invoker` (merged by build layer)       |
| `policies[]`     | `policy_pack`           | `hooks.PolicyHook` (merged)                   |
| `filters.*`      | `pre_llm_filter` etc.   | `hooks.PreLLMFilter` / etc. (merged)          |
| `budget`         | `budget_profile`        | `budget.Guard` + `budget.Config` defaults     |
| `telemetry`      | `telemetry_profile`     | `telemetry.LifecycleEventEmitter` + enricher  |
| `credentials`    | `credential_resolver`   | `credentials.Resolver`                        |
| `identity`       | `identity_signer`       | `identity.Signer`                             |

Deferred kinds: `skill` (Phase 3), `mcp_binding` (Phase 4), `output_contract`
(Phase 3, driven by skills).

## Invariants

The parser/validator must enforce every invariant before the build step
runs. No warnings — each failure aborts the build.

1. **Strict schema.** Unknown top-level keys, unknown sub-keys, and
   unknown enum values are rejected. No silent drops.
2. **Typed values only.** Every `config:` map is validated against the
   target factory's JSON Schema at build time. No free-form `any`
   reaches downstream code.
3. **Declarative only.** No embedded Go, Starlark, expressions,
   templates, or references to host env vars. Credentials come from the
   resolver; model params are literal.
4. **Every `ref:` resolves.** Missing factory → build fails. Wrong kind
   → build fails. Version mismatch with declared compatibility range →
   build fails.
5. **Acyclic `extends:` chain.** Cycle detection runs before
   normalization. Depth is capped at `spec.MaxExtendsDepth` (8) to
   catch pathological inheritance. Both violations surface as
   `spec.ErrExtendsInvalid` with a typed `*spec.ExtendsError` carrying
   the resolution chain and a `Reason` of `"cycle"` or `"depth"`.
   Parents resolve through an injectable `spec.SpecStore`; if the
   spec declares `Extends` and `Build` was not given a SpecStore via
   `forge.WithSpecStore(...)`, it returns `spec.ErrNoSpecStore`.
6. **Stable metadata.** `metadata.id` is immutable for the life of an
   agent; `metadata.version` is strictly semver and required on every
   spec.
7. **No orchestration leakage.** v0 rejects any key implying routing,
   delegation, planning, team membership, multi-agent state, or
   session persistence. Keys under consideration are blocked by
   schema, not by convention.
8. **No executable side effects in config.** Factories may not be
   passed closures, file handles, or network descriptors through the
   spec. Runtime material flows through the registry programmatically
   (e.g. credential resolvers receive their secrets store at
   registration time, not via the spec).
9. **Deterministic merge order.** Phase 2a fixes the merge field-iteration
   order to match `AgentSpec`'s declaration order in `spec/types.go`.
   Phase 2b layers a canonical JSON serialization plus a stable hash
   on top of this — two semantically equivalent specs must hash
   identically. Reordering the `AgentSpec` struct fields changes the
   hash by design (the hash is bound to the struct shape).
10. **Budget ceilings are contracts.** `budget.overrides` may only
    *tighten* the referenced profile, never loosen it. The validator
    rejects loosening overrides.

## Explicit deferrals

- **Skills (Phase 3).** `skills:` is present in the schema but must be
  empty in v0. A non-empty list fails validation with a
  `skills_not_yet_supported` error.
- **MCP imports (Phase 4).** `mcpImports:` same rule.
- **Bundles / lockfiles (Phase 5).** No `bundle:` or `lockfile:` field
  in v0. These are repository-level artifacts, not spec fields.
- **Expose-as-MCP.** Not a spec concern. It is a build-output concern
  that depends on a future praxis seam (see
  [`mismatches.md`](mismatches.md) Mismatch 4).
- **Output contracts.** Declared only when a skill contributes one;
  v0 without skills has no output contract field.

## File layout convention (proposed)

Not enforced in v0, but the loader will look for:

```
agents/
  acme.support-triage/
    agent.yaml          # the AgentSpec
    overlays/
      staging.yaml
      prod.yaml
    README.md
```

Loader API takes a spec path + a list of overlay paths; the directory
convention is a convenience, not a mandate.

## Open questions (raised for Phase 1 review)

- **Resolved in Phase 1.** Prompts flow through registered
  `prompt_asset` factories. The simplest factory (`prompt.literal@1`)
  returns its configured `text` verbatim. Inline literal prompts in the
  spec remain prohibited for tighter governance.
- Versioning scheme for factory IDs: attach to the ID (`@1.4.0`) or to
  the spec's compatibility block? v0 uses `@version` in the ref for
  locality.
- Where does a "model fallback list" live — provider config, or a
  separate field? v0 keeps it inside provider config to avoid growing
  new top-level keys.
