# praxis-forge — Phase 3 design spec

**Date:** 2026-04-21
**Milestone:** Phase 3 — Skills + Output Contracts
**Status:** Approved (ready for planning)
**Scope chosen:** MVP "A" (prompt fragments + required tools + required policies + output contract; auto-inject expansion; flat skill graph)

## Context

Phase 2b (shipped 2026-04-20) closed the deterministic-build story: canonical
JSON, stable `NormalizedHash`, and `Capabilities` flags on `Manifest`.
Phase 3 activates two kinds anticipated since Phase 0 but phase-gated
to date: **skills** and **output contracts**.

Today the spec accepts both fields at the parser but fails validation
when they are non-empty ([`spec/validate.go:81-83`](../../../spec/validate.go#L81-L83)
and [`spec/types.go:22-27`](../../../spec/types.go#L22-L27)). Both kinds
are listed as deferred in [`docs/design/registry-interfaces.md:44-46`](../../design/registry-interfaces.md#L44-L46)
and [`docs/design/agent-spec-v0.md:158-159`](../../design/agent-spec-v0.md#L158-L159).
Phase 3 unlocks them.

A skill is a **composable higher-level package**
([`docs/CONTEXT_SEED.md:102-118`](../../CONTEXT_SEED.md#L102-L118)): it
expands an agent definition at build time with prompt fragments,
required components (tools / policies / output contract), and governance
metadata. Skills participate in dependency/conflict validation and fail
the build on violation rather than silently dropping contributions.

## Scope decisions

### In scope (MVP)

A skill may contribute any subset of:

1. **Prompt fragment** — literal string appended after the base system
   prompt.
2. **Required tools** — list of `toolpack.<name>@<semver>` component
   references, auto-injected into the effective tool set.
3. **Required policies** — list of `policypack.<name>@<semver>`
   component references, auto-injected into the effective policy set.
4. **Required output contract** — at most one
   `outputcontract.<name>@<semver>` reference, auto-injected into the
   effective output-contract slot.

### Out of scope (deferred)

- **Few-shots as a separate primitive.** Express them inside
  `PromptFragment`; a dedicated slot adds surface without semantics.
- **Model preferences** (model, temperature, etc.). Cross-cuts the
  provider factory; relitigates Phase 1 provider semantics.
- **Dedicated safety-constraint primitive.** Safety lives in policy
  packs; skills contribute policies, not a parallel notion of safety.
- **Validation rules beyond output contract.** The output contract
  covers schematic validation; richer assertions are orchestrator-side.
- **Transitive skill-requires-skill.** v0 skills are flat: a skill
  declares required tools / policies / output contract, not required
  skills. If composition is needed, the consumer declares multiple
  skills at the spec level.
- **Runtime validation of LLM outputs against the output contract.**
  Orchestrator-side concern. Forge only resolves + stamps the schema
  into the manifest.
- **Required MCP bindings.** Phase 4 territory.
- **Lockfile integration for output-contract versioning.** Phase 5.

## Data model

New types in [`registry/types.go`](../../../registry/types.go):

```go
// Skill is the value produced by a skill factory.
type Skill struct {
    PromptFragment         string                 // may be empty
    RequiredTools          []RequiredComponent
    RequiredPolicies       []RequiredComponent
    RequiredOutputContract *RequiredComponent     // at most one
    Descriptor             SkillDescriptor
}

// RequiredComponent is a skill's declaration that a specific registered
// factory must be part of the effective composition. Config is
// canonical-compared against any pre-existing ref of the same id:
// identical = idempotent, different = conflict.
type RequiredComponent struct {
    ID     ID                     // e.g. "toolpack.http-fetch@1"
    Config map[string]any         // may be nil
}

type SkillDescriptor struct {
    Name    string
    Owner   string
    Summary string
    Tags    []string
}

// OutputContract is the value produced by an output-contract factory.
type OutputContract struct {
    Schema     map[string]any     // valid JSON Schema; structural check only
    Descriptor OutputContractDescriptor
}

type OutputContractDescriptor struct {
    Name    string
    Owner   string
    Summary string
}
```

`RequiredComponent.Config` uses the same canonical comparison used by
`spec.canonicalEncode` (Phase 2b) — zero duplication.

## Registry additions

Move `KindSkill` and `KindOutputContract` from the deferred block to the
active block in [`registry/kind.go`](../../../registry/kind.go).

Add factory interfaces in [`registry/factories.go`](../../../registry/factories.go):

```go
type SkillFactory interface {
    ID() ID
    Description() string
    Build(ctx context.Context, cfg map[string]any) (Skill, error)
}

type OutputContractFactory interface {
    ID() ID
    Description() string
    Build(ctx context.Context, cfg map[string]any) (OutputContract, error)
}
```

Extend [`registry/registry.go`](../../../registry/registry.go):

- maps: `skills map[ID]SkillFactory`, `outputContracts map[ID]OutputContractFactory`
- registration: `RegisterSkill(f SkillFactory) error`, `RegisterOutputContract(f OutputContractFactory) error`
- lookup: `Skill(id ID) (SkillFactory, error)`, `OutputContract(id ID) (OutputContractFactory, error)`
- `Each` enumerator extended to visit both new kinds.

## Spec changes

In [`spec/validate.go`](../../../spec/validate.go):

- Remove the `skills` phase-gate block ([`spec/validate.go:81-83`](../../../spec/validate.go#L81-L83))
  and the `outputContract` phase-gate block
  ([`spec/validate.go:87-89`](../../../spec/validate.go#L87-L89)). Leave
  the `mcpImports` phase-gate in place — still Phase 4.
- Add referential validation: `spec.skills[].ref` must match the
  pattern `^skill\.[a-z0-9-]+@\d+(\.\d+){0,2}$` (mirrors existing
  toolpack/policypack conventions in
  [`registry/id.go`](../../../registry/id.go)). Similarly
  `spec.outputContract.ref` must match
  `^outputcontract\.[a-z0-9-]+@\d+(\.\d+){0,2}$`.
- Add a well-formedness check: empty `ref:` in either collection fails
  `skill_missing_ref` / `outputcontract_missing_ref`.

In [`spec/testdata/overlay/invalid/`](../../../spec/testdata/overlay/invalid/):

- Remove `phase_gated_skills.yaml` + `.err.txt` (no longer invalid).
- Add valid skill overlay fixtures under `spec/testdata/overlay/valid/skills_*`.

Update [`docs/design/agent-spec-v0.md`](../../design/agent-spec-v0.md) §"Explicit deferrals"
to drop the `Skills (Phase 3)` and `Output contracts` entries.

No changes to `AgentSpec` struct itself — the fields already exist
([`spec/types.go:25-27`](../../../spec/types.go#L25-L27)).

## Build pipeline — new expansion stage

Current pipeline (post-Phase 2b):

```
parse → validate → normalize (Phase 2a)
      → canonical/hash (Phase 2b)
      → resolve components → compose → materialize → manifest
```

Phase 3 pipeline:

```
parse → validate → normalize
      → canonical/hash (pre-expansion: NormalizedHash)
      → resolve skill + output-contract factories
      → EXPAND — auto-inject skill contributions into effective
                 Tools / Policies / OutputContract sets; detect conflicts
      → canonical/hash (post-expansion: ExpandedHash)
      → resolve remaining expanded components
      → compose → materialize → manifest (both hashes + attribution)
```

`NormalizedSpec.NormalizedHash()` remains unchanged — Phase 2b contract
holds. The new hash is a Manifest field, not a `NormalizedSpec` method;
this keeps `NormalizedSpec` immutable and concerns cleanly separated.

New file: [`build/expand.go`](../../../build/expand.go) with

```go
// ExpandedSpec is the post-skill-expansion composition. Embeds AgentSpec
// with the effective Tools / Policies / OutputContract rewritten, plus
// attribution telling the manifest which skill injected what.
type ExpandedSpec struct {
    Spec       spec.AgentSpec
    InjectedBy map[string]registry.ID // key: "kind:id", value: skill ID
}

func expandSkills(
    ctx context.Context,
    s *spec.AgentSpec,
    r *registry.ComponentRegistry,
) (*ExpandedSpec, error)
```

`ExpandedSpec` is produced by `expandSkills` and consumed by the rest of
`Build`. The unchanged parts of `Build` (`resolve`, chain assembly,
`orchestrator.New`) now operate on `ExpandedSpec.Spec` instead of the
raw `AgentSpec`.

## Expansion semantics

For each `RequiredComponent` contributed by a skill, compared against
every ref already present in the spec or contributed by an earlier
skill (in `spec.skills[]` declaration order):

| State | Result | Error code |
|-------|--------|-----------|
| Same `id` (kind + name + version), canonical-identical `Config` | idempotent no-op (still recorded in attribution) | — |
| Same `kind.<name>`, different semver | fail | `skill_conflict_version_divergence` |
| Same `id`, `Config` differs canonically | fail | `skill_conflict_config_divergence` |
| Two skills require output contracts with different `id` | fail | `skill_conflict_output_contract_multiple` |
| Skill requires output contract + user declared one with different `id` or different `Config` | fail | `skill_conflict_output_contract_user_override` |
| Skill references unregistered component | fail | `skill_unresolved_required_component` |
| Skill with empty contribution (no fragment, no requirements) | fail at build-time | `skill_empty_contribution` |

Canonical-equality of `Config` reuses `spec.canonicalEncode` from
Phase 2b.

Declaration order: after extends-chain resolution
([`spec/normalize.go`](../../../spec/normalize.go)) and overlay application
(Phase 2a provenance). Deterministic.

## Prompt fragment merge

- Base system prompt comes from `spec.prompt.system` resolved by
  `PromptAssetFactory` (unchanged from Phase 1).
- Each skill with `PromptFragment != ""` contributes by append.
- Order: `spec.skills[]` declaration order after normalize.
- Separator: `"\n\n"` (fixed).
- Deduplication: byte-identical fragments across multiple skills
  collapse silently (realistic case: two skills asserting the same
  safety reminder). The manifest `Resolved` entries for both skills
  remain — dedupe affects string assembly only, not audit.

The final concatenated prompt flows through the existing
`BuiltAgent.SystemPrompt` field. No change to the praxis pass-through.

## Manifest additions

In [`manifest/manifest.go`](../../../manifest/manifest.go):

```go
type Manifest struct {
    SpecID         string               `json:"specId"`
    SpecVersion    string               `json:"specVersion"`
    BuiltAt        time.Time            `json:"builtAt"`
    NormalizedHash string               `json:"normalizedHash"`
    ExpandedHash   string               `json:"expandedHash,omitempty"` // NEW
    Capabilities   Capabilities         `json:"capabilities"`
    ExtendsChain   []string             `json:"extendsChain,omitempty"`
    Overlays       []OverlayAttribution `json:"overlays,omitempty"`
    Resolved       []ResolvedComponent  `json:"resolved"`
}

type ResolvedComponent struct {
    Kind            string         `json:"kind"`
    ID              string         `json:"id"`
    Config          map[string]any `json:"config,omitempty"`
    Descriptors     any            `json:"descriptors,omitempty"`
    InjectedBySkill string         `json:"injectedBySkill,omitempty"` // NEW
}
```

`ExpandedHash` is emitted whenever `spec.skills[]` is non-empty —
even when all contributions turn out to be idempotent and the
post-expansion canonical form happens to equal the pre-expansion form.
An audit reader sees "expansion ran and produced this hash" regardless
of whether it netted a new component. When `spec.skills[]` is empty,
`ExpandedHash` is omitted from the JSON (no expansion occurred).

`Capabilities.Present` gains `"skill"` when `spec.skills[]` is non-empty
and `"output_contract"` when an output-contract slot resolved.
`Capabilities.Skipped` adds both kinds with reason `"not_specified"` when
absent. This reuses the Phase 2b `CapabilitySkip` shape and registry
declaration order: budget → telemetry → credential_resolver →
identity_signer → **skill → output_contract**.

Attribution rule for `InjectedBySkill`:

- User-declared only → field empty.
- Skill-injected only → field holds the injecting skill's id.
- Both declared identically (user + one or more skills) → field empty;
  user-explicit wins attribution. The contributing skills remain
  discoverable via their own `ResolvedComponent` entries
  (`Kind: skill`), whose descriptors list `RequiredTools` /
  `RequiredPolicies` / `RequiredOutputContract`. No information loss.
- Multiple skills inject identically, no user declaration → field holds
  the id of the first skill (in `spec.skills[]` declaration order) to
  require it. The other skills' requirements are still audited via
  their skill descriptors.

## Vertical slice factories

Analogous to `toolpack.http-get@1` in Phase 1, Phase 3 ships two
concrete factories to prove the skill → output-contract path:

### `factories/skillstructuredoutput/` → `skill.structured-output@1`

- **Prompt fragment:** literal
  `"Respond with JSON matching the required schema. Do not include prose outside the JSON."`
- **Required tools:** none
- **Required policies:** `policypack.pii-redaction@1` (existing Phase 1
  factory in [`factories/policypackpiiredact`](../../../factories/policypackpiiredact/))
- **Required output contract:** `outputcontract.json-schema@1`
- **Config:** none required

### `factories/outputcontractjsonschema/` → `outputcontract.json-schema@1`

- **Config:** `{schema: <object>}` where `schema` is a JSON Schema
  document
- **Build:** returns `OutputContract{Schema: cfg.schema, Descriptor: ...}`
- **Validation (structural, zero-dep):**
  - `schema` non-nil
  - root has at least one of `$schema`, `type`, `properties`, `$ref`
  - no runtime semantic validation (would require a JSON Schema library;
    v0 stays zero-dep consistent with Phase 2b)

### Demo updates

[`examples/demo`](../../../examples/demo/) gains a `-structured` flag:
when set, it registers both new factories and builds an agent with
`skills: [skill.structured-output@1]` and a minimal user schema.

## Test coverage

### Unit tests

- `registry/registry_test.go` — register/lookup for skill and
  output-contract factories; duplicate detection; frozen-after-Build.
- `build/expand_test.go` — 12 cases covering every row of the
  expansion-semantics table plus declaration-order determinism.
- `spec/validate_test.go` — new skills/outputContract referential
  validation.

### Integration tests

- `build/build_skills_test.go` — end-to-end YAML → BuiltAgent + manifest
  with skill attribution, both hashes stable, canonical forms locked.

### Fixtures

Under `spec/testdata/skills/`:

- `basic-skill/` — one skill, prompt + one already-declared tool
- `auto-inject-tool/` — skill requires an unlisted tool
- `auto-inject-policy/` — skill requires an unlisted policy
- `conflict-version/` — two skills, divergent versions
- `conflict-config/` — two skills, same id, divergent configs
- `idempotent-overlap/` — two skills require the same component, same config
- `empty-contribution/` — skill contributes nothing → build error
- `output-contract-auto-inject/` — skill contract, user has none
- `output-contract-user-match/` — user and skill declare identical contract
- `output-contract-user-override/` — divergent user contract → error
- `expanded-hash-stable/` — two logically-equivalent specs → same `ExpandedHash`
- `fragment-dedup/` — two skills with byte-identical fragments → deduped prompt
- `fragment-order/` — prompt fragment order reflects `spec.skills[]`

Each fixture has `spec.yaml` + (for success) `want.expanded.json` and
`want.expanded.hash` golden files, matching the Phase 2b fixture pattern.

## Commit sequence

1. `feat(registry): activate KindSkill and KindOutputContract`
2. `feat(registry): SkillFactory, OutputContractFactory interfaces + Skill / OutputContract types`
3. `feat(registry): register + lookup for skills and output contracts`
4. `feat(spec): unlock skills and outputContract validation gates; remove phase-gated fixtures`
5. `feat(build): expand.go — skill resolution + auto-inject + conflict detection`
6. `feat(build): ExpandedSpec canonical form + ExpandedHash`
7. `feat(manifest): ExpandedHash field + InjectedBySkill attribution + capabilities extension`
8. `feat(build): wire expansion into Build pipeline; extend buildManifest for skills/output contracts`
9. `feat(factories): skill.structured-output@1`
10. `feat(factories): outputcontract.json-schema@1`
11. `test(spec+build): skills fixtures + expansion integration tests`
12. `examples(demo): -structured flag exercising skill + output-contract path`
13. `docs: Phase 3 design doc (this file) + agent-spec-v0 update + forge-overview roadmap update`

## Risks and mitigations

| Risk | Mitigation |
|------|-----------|
| `ExpandedHash` vs. `NormalizedHash` semantics confuse consumers | Godoc: "normalized = author wrote; expanded = what ran after skill expansion". Manifest round-trip test asserts both fields. |
| Empty-contribution skill creates dead weight in manifest | Build-time error `skill_empty_contribution`; Descriptor-only registration without any active contribution rejected. |
| Structural-only JSON Schema validation accepts malformed schemas | Godoc is explicit: "structural check only; semantic validation deferred to orchestrator or a future dep-bearing phase". Users needing semantic validation today can register their own `OutputContractFactory` that imports a JSON Schema library. |
| Prompt fragment dedup hides duplication bugs | Manifest `Resolved` lists every contributing skill even when fragments dedupe — full audit preserved. |
| Two skills legitimately need different configs for the same tool | Not supported in v0. Build fails with `skill_conflict_config_divergence`. User resolves by splitting into two agents or by one skill winning via explicit user-level tool declaration. Documented in the error message. |
| Overlay adding a skill that injects a policy whose config conflicts with a locked user-declared policy | Detected by expansion after overlays are applied (pipeline order matters). Locked-field protection from Phase 2a still runs before expansion; skill-driven conflicts are a separate error class. |

## Out of scope (Phase 3.1 and beyond)

- Few-shots as a separate primitive
- Model preferences (model, temperature, etc.)
- Transitive skill-requires-skill (depth > 1)
- Runtime validation of LLM outputs against the output contract
- MCP required bindings (Phase 4)
- Lockfile integration for output-contract versioning (Phase 5)
- `skill` kind introspection / expansion-tree export on `NormalizedSpec`

## References

- [`docs/CONTEXT_SEED.md:102-118`](../../CONTEXT_SEED.md#L102-L118) — skill responsibilities
- [`docs/CONTEXT_SEED.md:221-230`](../../CONTEXT_SEED.md#L221-L230) — Phase 3 charter
- [`docs/design/agent-spec-v0.md:158-159`](../../design/agent-spec-v0.md#L158-L159) — deferred kinds
- [`docs/design/forge-overview.md:178-182`](../../design/forge-overview.md#L178-L182) — phase roadmap
- [`docs/design/registry-interfaces.md:44-46`](../../design/registry-interfaces.md#L44-L46) — reserved kinds
- [`docs/superpowers/specs/2026-04-20-praxis-forge-phase-2b-design.md`](2026-04-20-praxis-forge-phase-2b-design.md) — prior phase
- [`spec/types.go:22-27`](../../../spec/types.go#L22-L27) — current phase-gated fields
- [`spec/validate.go:81-83`](../../../spec/validate.go#L81-L83) — current gating enforcement
