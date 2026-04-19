# praxis-forge — Phase 2a design spec

**Date:** 2026-04-18
**Milestone:** Phase 2a — composition depth (overlays + extends + provenance)
**Status:** Approved for implementation planning
**Companion plan:** `docs/superpowers/plans/2026-04-18-praxis-forge-phase-2a/` (TBD next step)

## Context

Phase 1 shipped the first vertical slice: strict YAML loading, typed
registry of 11 factory kinds, build pipeline that materializes a
stateless `*orchestrator.Orchestrator` from `github.com/praxis-os/praxis`,
plus a runnable demo. The `Build` signature deliberately omitted overlays;
[`spec.AgentSpec.Extends`](../../../spec/types.go) is parsed but
phase-gated to empty by [`(*AgentSpec).Validate`](../../../spec/validate.go).

[`docs/design/forge-overview.md`](../../design/forge-overview.md) groups
five concerns under "Phase 2 — composition depth":

1. `overlays []AgentOverlay` enters the `Build` call.
2. `extends:` chain resolution.
3. Canonical manifest with stable field ordering.
4. Deterministic build (stable hashing of normalized form).
5. Richer inspection.

Phase 2a covers concerns 1–2 plus the structural groundwork (provenance
tracking, field ordering discipline) needed for 3–5 to land cleanly in
Phase 2b. This split keeps the merge-semantics work in one
shippable cut and defers hashing/manifest expansion to a later commit
once overlay/extends behavior has settled.

## Brainstorm decisions

Locked decisions shaping scope:

1. **Phase 2 split.** This spec covers Phase 2a (overlays + extends +
   provenance + normalization). Phase 2b will add canonical JSON
   ordering, stable hashing, and richer manifest inspection.
2. **Overlay form.** Typed `AgentOverlay` Go struct with the same shape
   as `AgentSpec` but every field optional. Deep-merge rules: lists
   replace by default, `Config map[string]any` blocks replace whole.
3. **Extends-first ordering.** Resolve `extends:` chain → flat base
   spec → apply overlays. Overlays never reach the chain resolver.
4. **`SpecStore` interface for parents.** Parents loaded via injectable
   `spec.SpecStore`. `FilesystemSpecStore` (root-rooted) and
   `MapSpecStore` (in-memory) ship with Phase 2a. If `Extends` is
   non-empty and no `SpecStore` is configured, `Build` returns a hard
   error.
5. **Merge parity.** Same merge rule used for child-over-parent
   (extends) and overlay-over-merged (overlay). One mental model.
6. **Functional-options entry path.** `Build` signature does **not**
   gain a positional overlay slot. Overlays enter via
   `forge.WithOverlays(...)` to preserve the existing `Build(ctx, s, r,
   opts...)` shape. Amends [`docs/design/forge-overview.md:119`](../../design/forge-overview.md).
7. **Tri-state via wrapper, not `*[]T`.** Replaceable collections in
   `AgentOverlayBody` use a small wrapper type (`Set bool` + items)
   to disambiguate "absent" vs "explicit null/empty" vs "populated"
   without footgun semantics across `yaml.v3` strict-decode.
8. **Provenance is data, embedded.** Per-field source attribution lives
   on `NormalizedSpec` as a typed mirror struct (one level deep),
   exposed via `Provenance(fieldPath string)`. No stringly-typed map,
   no context propagation.
9. **Locked-field set.** `apiVersion`, `kind`, `metadata.id`,
   `metadata.version` cannot be touched by extends or overlays.
10. **Out of Phase 2a:** canonical hash, lockfile, bundle, skill
    resolution, MCP imports, output contract, capability flags.

## Section 1 — Public API + layering

### Package map (additive, no Phase 1 file moves)

```
praxis-forge/
  forge.go                 # gains LoadOverlay, LoadOverlays, WithOverlays, WithSpecStore
  spec/
    types.go               # unchanged
    overlay.go             # NEW: AgentOverlay + AgentOverlayBody + tri-state wrappers
    load_overlay.go        # NEW: strict YAML decoder for overlay files
    store.go               # NEW: SpecStore interface + FilesystemSpecStore + MapSpecStore
    normalize.go           # NEW: Normalize(ctx, s, overlays, store) → *NormalizedSpec
    provenance.go          # NEW: Provenance + AttributedSpec wrapper
    locked.go              # NEW: validateLocked pure function
    errors.go              # NEW sentinels added; existing Errors aggregator extended
  build/build.go           # updated to call spec.Normalize before resolver
  manifest/manifest.go     # gains ExtendsChain []string, Overlays []OverlayAttribution
```

No file under `factories/`, `registry/`, or `internal/testutil/` is
touched in Phase 2a.

### Public surface (additions only)

```go
package forge

func LoadOverlay(path string) (*spec.AgentOverlay, error)
func LoadOverlays(paths ...string) ([]*spec.AgentOverlay, error) // convenience

func WithOverlays(ov ...*spec.AgentOverlay) Option
func WithSpecStore(store spec.SpecStore) Option

// Build signature unchanged at the call site:
func Build(
    ctx context.Context,
    s *spec.AgentSpec,
    r *registry.ComponentRegistry,
    opts ...Option,
) (*BuiltAgent, error)

func (*BuiltAgent) NormalizedSpec() *spec.NormalizedSpec
```

`BuiltAgent.Manifest()` keeps its existing signature; the returned
`Manifest` value gains two new JSON fields (see Section 8).

### Dependency rule (unchanged)

`spec` depends on nothing internal. `registry` depends on `praxis`
kernel types only. `build` imports `spec`, `registry`, `manifest`,
`praxis`. `forge` re-exports.

### ADR amendments from Phase 0/1

- [`docs/design/forge-overview.md:119`](../../design/forge-overview.md) —
  remove the planned positional `overlays []AgentOverlay` slot from the
  Phase 2 `Build` signature. Replace with:
  > "Overlays enter `Build` via `forge.WithOverlays(...)`. The positional
  > signature `Build(ctx, s, r, opts...)` is preserved across phases."
- [`docs/design/agent-spec-v0.md`](../../design/agent-spec-v0.md) —
  document Phase 2a behavior of `extends:`: chain resolution semantics,
  cycle/depth limits, locked fields, parent-fragment validation rules.

## Section 2 — `AgentOverlay` shape

### Top-level overlay file

```yaml
apiVersion: forge.praxis-os.dev/v0       # must match AgentSpec.APIVersion
kind: AgentOverlay                       # must == "AgentOverlay"
metadata:
  name: prod-override                    # attribution label, surfaced in errors + manifest
spec:
  # AgentOverlayBody — same shape as AgentSpec but every field optional.
  provider:
    ref: provider.anthropic@1.0.0
    config: { model: claude-opus-4-7 }
  policies:
    - { ref: policy.pii.strict@1.0.0 }
```

### Go types

```go
package spec

type AgentOverlay struct {
    APIVersion string           `yaml:"apiVersion"`
    Kind       string           `yaml:"kind"`
    Metadata   OverlayMeta      `yaml:"metadata"`
    Spec       AgentOverlayBody `yaml:"spec"`
}

type OverlayMeta struct {
    Name string `yaml:"name"`
}

type AgentOverlayBody struct {
    Metadata    *OverlayMetadata `yaml:"metadata,omitempty"`     // ID/Version locked at apply
    Provider    *ComponentRef    `yaml:"provider,omitempty"`
    Prompt      *PromptBlock     `yaml:"prompt,omitempty"`
    Tools       *RefList         `yaml:"tools,omitempty"`
    Policies    *RefList         `yaml:"policies,omitempty"`
    Filters     *FilterOverlay   `yaml:"filters,omitempty"`
    Budget      *BudgetRef       `yaml:"budget,omitempty"`
    Telemetry   *ComponentRef    `yaml:"telemetry,omitempty"`
    Credentials *CredRef         `yaml:"credentials,omitempty"`
    Identity    *ComponentRef    `yaml:"identity,omitempty"`
}

// OverlayMetadata mirrors Metadata but every field is optional. ID and
// Version are validated as locked at apply time and rejected if touched.
type OverlayMetadata struct {
    ID          string            `yaml:"id,omitempty"`          // locked
    Version     string            `yaml:"version,omitempty"`     // locked
    DisplayName string            `yaml:"displayName,omitempty"`
    Description string            `yaml:"description,omitempty"`
    Owners      []Owner           `yaml:"owners,omitempty"`
    Labels      map[string]string `yaml:"labels,omitempty"`      // replace, not merge
}

type FilterOverlay struct {
    PreLLM   *RefList `yaml:"preLLM,omitempty"`
    PreTool  *RefList `yaml:"preTool,omitempty"`
    PostTool *RefList `yaml:"postTool,omitempty"`
}
```

### Tri-state wrapper

```go
// RefList wraps []ComponentRef so the YAML decoder can distinguish
// "field absent" from "field present (set to anything, including null
// or empty)".
//
//   tools:                              # field absent           → Set=false
//   tools:                              # explicit null          → Set=true,  Items=nil
//   tools: []                           # explicit empty list    → Set=true,  Items=[]
//   tools: [{ref: ...}]                 # populated              → Set=true,  Items=[...]
//
// Merge semantics on the apply path: if Set is false the base list is
// preserved; if Set is true the wrapper replaces the base list verbatim
// (including nil and empty cases).
type RefList struct {
    Set   bool
    Items []ComponentRef
}

func (rl *RefList) UnmarshalYAML(node *yaml.Node) error { /* ... */ }
```

### Strict decoding

`LoadOverlay` and `LoadOverlays` use a `yaml.v3` decoder with
`KnownFields(true)` set at the **top level** (not only on the body).
Unknown keys at any depth fail parse.

Phase-gated AgentSpec fields (`extends`, `skills`, `mcpImports`,
`outputContract`) are deliberately **absent** from `AgentOverlayBody`,
so the strict decoder rejects them at parse time with a clear error
naming the field and source line. The `ErrPhaseGatedInOverlay` sentinel
in Section 7 is reserved for the parallel in-Go construction path: a
caller building an `AgentOverlay` value programmatically cannot place
phase-gated content in fields that do not exist, but the sentinel
disambiguates future-proofing checks (e.g. when an overlay carries a
struct field added for Phase 3 that should still be rejected before
Phase 3 ships).

## Section 3 — `SpecStore`

### Interface

```go
package spec

// SpecStore loads parent specs referenced by AgentSpec.Extends. Forge
// invokes Load serially during chain resolution; implementations need
// not be safe for concurrent use.
type SpecStore interface {
    Load(ctx context.Context, ref string) (*AgentSpec, error)
}

// ErrSpecNotFound is the sentinel SpecStore implementations must return
// when a ref is not resolvable. Wrapping is allowed; errors.Is must hold.
var ErrSpecNotFound = errors.New("spec not found")
```

### Implementations

```go
// FilesystemSpecStore resolves refs as filesystem paths relative to
// Root. A ref containing ".." outside Root returns ErrSpecNotFound.
type FilesystemSpecStore struct {
    Root string
}
func (s *FilesystemSpecStore) Load(ctx context.Context, ref string) (*AgentSpec, error)

// MapSpecStore is an in-memory store suitable for tests and for
// callers that load specs from non-filesystem sources (HTTP, embedded
// FS, etc.) and pre-decode them.
type MapSpecStore map[string]*AgentSpec
func (m MapSpecStore) Load(ctx context.Context, ref string) (*AgentSpec, error)
```

`FilesystemSpecStore.Load` reads the file, runs the same strict YAML
decoder used by `LoadSpec`, and returns the decoded `*AgentSpec`. It
does **not** run `Validate()` — parent fragments are validated only as
part of the merged result (see Section 5).

### Configuration in `forge.Build`

If `s.Extends` is non-empty and no `WithSpecStore(...)` was passed,
`Build` returns `ErrNoSpecStore` immediately. There is no implicit
filesystem default; the caller must opt in.

## Section 4 — Provenance

### Type

```go
package spec

// Provenance attributes a single field in the merged spec back to its
// source: which file (or in-memory ref) it came from, what role that
// source played, and the YAML line where it was set.
type Provenance struct {
    File string // file path or ref id
    Line int    // 1-based; 0 if unknown (e.g. struct-literal overlay)
    Role Role   // base | parent | overlay
    Step int    // 0 for base/initial parent; chain depth for parent; overlay index for overlay
}

type Role uint8

const (
    RoleBase    Role = iota // raw AgentSpec passed to Build
    RoleParent              // sourced from extends chain (Step = depth from leaf)
    RoleOverlay             // sourced from overlay (Step = position in overlays slice)
)
```

### Storage shape

`NormalizedSpec` carries provenance in a sibling struct that mirrors
`AgentSpec` one level down. Each top-level field has a
`Provenance` value; nested collections share the provenance of their
parent field (per-element provenance is not tracked in Phase 2a — too
expensive for the inspection benefit).

```go
type NormalizedSpec struct {
    Spec        AgentSpec       // the merged result; Extends is always nil/empty
    ExtendsChain []string       // refs resolved root-first (empty if no extends)
    Overlays    []OverlayAttribution
    fields      provenanceFields // unexported mirror struct
}

type OverlayAttribution struct {
    Name string // overlay metadata.name
    File string // path passed to LoadOverlay; empty for struct-literal overlays
}

type provenanceFields struct {
    APIVersion  Provenance
    Kind        Provenance
    Metadata    Provenance
    Provider    Provenance
    Prompt      Provenance
    Tools       Provenance
    Policies    Provenance
    Filters     Provenance
    Budget      Provenance
    Telemetry   Provenance
    Credentials Provenance
    Identity    Provenance
}

// Provenance returns the source attribution for a top-level spec field.
// fieldPath uses dotted form: "provider", "filters.preLLM", etc.
// The boolean reports whether the path is recognized.
func (n *NormalizedSpec) Provenance(fieldPath string) (Provenance, bool)
```

The mirror struct is hand-written; it stays small (12 fields) for the
foreseeable future. Codegen revisited only if the spec grows past ~50
top-level fields.

## Section 5 — Normalize pipeline

### Entry point

```go
package spec

// Normalize resolves the extends chain, applies overlays in order, and
// returns the canonical NormalizedSpec. The returned value is suitable
// to drive the build pipeline.
//
// Steps, in order:
//   1. resolveExtendsChain(ctx, s, store) → []*AgentSpec, root-first
//   2. mergeChain(parents..., s)          → merged AgentSpec, child wins
//   3. applyOverlays(merged, overlays)    → final AgentSpec
//   4. validateLocked(s, final)           → errors if locked fields drifted
//   5. final.Validate()                   → Phase 1 validator on the result
//
// All errors are aggregated; a single Normalize call surfaces every
// violation rather than failing on the first.
func Normalize(
    ctx context.Context,
    s *AgentSpec,
    overlays []*AgentOverlay,
    store SpecStore,
) (*NormalizedSpec, error)
```

### Chain resolution

```go
// resolveExtendsChain walks s.Extends and every parent's Extends in
// turn, accumulating loaded parents in root-first order.
//
// Limits:
//   - max depth 8                       (ErrExtendsInvalid, Reason: "depth")
//   - cycle detected via visited set    (ErrExtendsInvalid, Reason: "cycle")
//   - parent missing from store         (ErrSpecNotFound wrapped)
//
// Context cancellation aborts the walk and returns ctx.Err().
func resolveExtendsChain(
    ctx context.Context,
    s *AgentSpec,
    store SpecStore,
) ([]*AgentSpec, error)
```

`s.Extends` is treated as an ordered list. Parents earlier in the list
are merged in earlier (i.e. later parents in the list win over earlier
ones, before the child wins over all of them). Each parent's own
`Extends` is resolved depth-first before the parent itself is added to
the chain.

### Merge rules

| Field shape | Rule |
|-------------|------|
| Scalar (string, int, time.Duration) | child wins over parent (only if child non-zero); zero values do **not** override |
| Pointer to struct (`*ComponentRef`, etc.) | child wins if non-nil |
| Slice of typed elements (`[]ComponentRef`) | child replaces parent if non-nil; nil preserves parent |
| `map[string]any` (`ComponentRef.Config`) | child replaces parent's whole map; no deep merge |
| `map[string]string` (`Metadata.Labels`) | replace, not merge |
| `[]string` (`Owners`, etc.) | replace if non-nil |

Overlay apply uses the same rules but reads the `RefList.Set` flag
instead of nil-checking the slice, so explicit "clear to empty" works:

```yaml
spec:
  tools: []        # RefList{Set: true, Items: nil} → result tools is empty
```

vs. omitting `tools:` entirely (`RefList.Set == false` → base preserved).

### Field iteration order

Merge functions iterate `AgentSpec` fields in **declaration order** of
the struct (the order they appear in `spec/types.go`). This guarantees
deterministic merge regardless of YAML map iteration. The order is also
the canonical order Phase 2b will use to compute the stable hash, so
fixing it now avoids re-engineering the merge step later.

If the struct definition is reordered, the behavior of merge changes;
this is acceptable because the hash itself is bound to the struct
shape (same constraint already applies to the manifest's `Resolved`
list ordering in Phase 1).

### Validation in the pipeline

- **Per-fragment:** parents and overlays are not validated in
  isolation. They may legitimately be partial.
- **`validateLocked(base, final)`:** runs after merge+overlay,
  comparing the locked fields in `final` against `base`. Any drift
  produces `ErrLockedFieldOverride` with provenance.
- **`final.Validate()`:** the existing Phase 1 validator runs on the
  flattened result. After `Normalize` the result has empty `Extends`
  by construction, so the existing phase-gate check naturally passes.
  Other Phase 1 invariants (header, ref shape, duplicates) apply to
  the merged whole.

Errors from all three steps are aggregated through the existing
`spec.Errors` aggregator; the returned error satisfies
`errors.Is(err, ErrValidation)` and any matching new sentinel.

## Section 6 — Locked-field validation

```go
// validateLocked compares the four locked fields between the original
// base spec and the post-merge result. Any drift produces a
// LockedFieldViolation entry on the aggregator.
//
// Locked fields:
//   - APIVersion
//   - Kind
//   - Metadata.ID
//   - Metadata.Version
func validateLocked(base, final *AgentSpec, prov *provenanceFields, errs *Errors) {
    if base.APIVersion != final.APIVersion {
        errs.Wrap(ErrLockedFieldOverride,
            "apiVersion: locked field overridden by %s", prov.APIVersion.describe())
    }
    if base.Kind != final.Kind {
        errs.Wrap(ErrLockedFieldOverride,
            "kind: locked field overridden by %s", prov.Kind.describe())
    }
    if base.Metadata.ID != final.Metadata.ID {
        errs.Wrap(ErrLockedFieldOverride,
            "metadata.id: locked field overridden by %s", prov.Metadata.describe())
    }
    if base.Metadata.Version != final.Metadata.Version {
        errs.Wrap(ErrLockedFieldOverride,
            "metadata.version: locked field overridden by %s", prov.Metadata.describe())
    }
}
```

`validateLocked` is a pure function over both spec values plus the
provenance mirror; reusable from any future composition path. It does
**not** live as a method on `AgentSpec` — that would couple the
`spec` type to the merge concern.

## Section 7 — Error model

### New sentinels

```go
package spec

var (
    ErrNoSpecStore         = errors.New("forge: extends present but no SpecStore configured")
    ErrSpecNotFound        = errors.New("spec store: ref not found")
    ErrExtendsInvalid      = errors.New("extends: invalid")        // cycle or depth
    ErrLockedFieldOverride = errors.New("locked field overridden")
    ErrPhaseGatedInOverlay = errors.New("phase-gated field in overlay")
    ErrCompositionLimit    = errors.New("composition limit exceeded")
)
```

### Typed error carrier

```go
// ExtendsError carries the chain that triggered an extends violation
// so callers can log the exact path. Reason is one of "cycle" or
// "depth".
type ExtendsError struct {
    Chain  []string // refs in order, starting from the spec that triggered the failure
    Reason string
}

func (e *ExtendsError) Error() string
func (e *ExtendsError) Unwrap() error            // returns ErrExtendsInvalid
func (e *ExtendsError) Is(target error) bool     // matches ErrExtendsInvalid
```

`ErrExtendsInvalid` collapses the cycle/depth distinction at the
sentinel level. Callers needing to discriminate inspect
`*ExtendsError.Reason`.

### Bounds

| Limit | Value | Sentinel |
|-------|-------|----------|
| Extends depth | 8 | `ErrExtendsInvalid` (Reason: `"depth"`) |
| Overlay count | 16 | `ErrCompositionLimit` |
| Combined size of merged spec | none | n/a (governance concern) |

### Aggregation

Existing `spec.Errors` aggregator is extended with one helper:

```go
func (e *Errors) Wrap(sentinel error, format string, args ...any)
```

which records the formatted message **and** keeps the sentinel
discoverable through `errors.Is` on the aggregated error. Implementation:
`Errors` gains a sibling `[]error` of sentinels alongside the existing
`[]string` of messages; `Errors.Is(target)` returns true if any recorded
sentinel matches via `errors.Is`, in addition to the existing
`ErrValidation` arm.

Phase 2a engineer alternative: if `errors.Join` (Go 1.20+) and a
typed-violation slice produce a cleaner aggregator without a string
hop, prefer that and wire it through equivalently — the public contract
is `errors.Is(err, ErrLockedFieldOverride)` etc. holding for the
returned error, regardless of internal shape. See open item #2 in the
final section.

## Section 8 — Manifest extensions

```go
package manifest

type Manifest struct {
    SpecID       string                `json:"specId"`
    SpecVersion  string                `json:"specVersion"`
    BuiltAt      time.Time             `json:"builtAt"`
    ExtendsChain []string              `json:"extendsChain,omitempty"` // NEW; root-first
    Overlays     []OverlayAttribution  `json:"overlays,omitempty"`     // NEW
    Resolved     []ResolvedComponent   `json:"resolved"`
}

type OverlayAttribution struct {
    Name string `json:"name"`
    File string `json:"file,omitempty"`
}
```

`Resolved` ordering already follows declaration order of the spec; no
change. `ExtendsChain` and `Overlays` are appended after the existing
fields and use `omitempty` so Phase 1 manifests round-trip identically.

The full canonical-form hash arrives in Phase 2b; for Phase 2a the
manifest gains attribution metadata only.

## Section 9 — Testing strategy

### Unit tests

- `spec/store_test.go`
  - `FilesystemSpecStore`: hits, misses, `..` rejection.
  - `MapSpecStore`: hits, misses returning `ErrSpecNotFound`.

- `spec/overlay_test.go`
  - YAML decode table for every `RefList`-typed field × the four
    states (absent, null, `[]`, populated).
  - Strict-decode rejection for unknown top-level keys.
  - Strict-decode rejection for phase-gated fields (`extends`,
    `skills`, `mcpImports`, `outputContract`) at any depth.

- `spec/normalize_test.go` — table-driven, embedded YAML fixtures.
  - Extends depth 0 / 1 / 3 / 8 (pass) / 9 (`ErrExtendsInvalid`).
  - Cycles: A→B→A, A→B→C→A, self-cycle A→A.
  - Missing parent → `ErrSpecNotFound` (wrapped).
  - Overlay order: 0 / 1 / 3 overlays; later wins.
  - Overlay count > 16 → `ErrCompositionLimit`.
  - Locked-field violation in extends → `ErrLockedFieldOverride`.
  - Locked-field violation in overlay → `ErrLockedFieldOverride`.
  - `Config` replace semantics (overlay's config wholly replaces base).
  - Explicit empty list (`tools: []`) clears base list.
  - Field absence preserves base list.
  - `ctx` cancellation during chain walk → `ctx.Err()`.
  - No `WithSpecStore` configured + `Extends` non-empty →
    `ErrNoSpecStore`.

- `spec/locked_test.go` — direct unit test on `validateLocked` with
  hand-built provenance fields.

- `spec/provenance_test.go` — `NormalizedSpec.Provenance(path)`
  returns expected attribution for every top-level field across base /
  parent / overlay sources.

### Integration tests

- `forge_test.go` (extension):
  - Build a 2-deep extends chain + 2 overlays via `MapSpecStore`,
    assert `Manifest.ExtendsChain` carries the two parent refs in
    root-first order, `Manifest.Overlays` carries both overlay names,
    and the resolved provider is the overlay's, not the base's.
  - Negative path: same scenario without `WithSpecStore` →
    `errors.Is(err, ErrNoSpecStore)`.

### Fixtures

Layout under `spec/testdata/normalize/<scenario>/`:

```
<scenario>/
  base.yaml
  parent-1.yaml      # if scenario uses extends
  parent-2.yaml
  overlay-1.yaml     # if scenario uses overlays
  overlay-2.yaml
  want.json          # expected normalized AgentSpec serialization (positive)
  want.err           # expected error sentinel + substring (negative)
```

Test loader walks every subdirectory; each becomes its own subtest via
`t.Run(scenario, ...)`.

## Section 10 — Out of scope (Phase 2b and beyond)

- Canonical JSON ordering of `NormalizedSpec`.
- Stable hash field on `Manifest` (`NormalizedHash`).
- Capability flags surfaced in manifest (which kinds participated,
  which factories were skipped).
- Dependency graph export (`spec.Graph()`).
- Lockfile, bundle, signing.
- Skills resolution / expansion rules (Phase 3).
- MCP imports (Phase 4).
- Output contract resolution (Phase 3).
- Per-element provenance for collection items.

Each of the above ships in a later phase; none is blocked by Phase 2a
choices.

## Delivery shape (atomic commits)

| # | Commit | Scope |
|---|--------|-------|
| 1 | `feat(spec): SpecStore interface + filesystem and map impls` | `spec/store.go`, `spec/store_test.go`, `spec/errors.go` (`ErrSpecNotFound`) |
| 2 | `feat(spec): AgentOverlay + RefList tri-state + LoadOverlay` | `spec/overlay.go`, `spec/load_overlay.go`, decoder tests, fixtures |
| 3 | `feat(spec): provenance type + NormalizedSpec wrapper` | `spec/provenance.go`, unit test |
| 4 | `feat(spec): extends chain resolver` | `spec/normalize.go` (chain only), depth/cycle tests |
| 5 | `feat(spec): overlay merger + locked-field validation` | extend `spec/normalize.go`, add `spec/locked.go`, full normalize tests |
| 6 | `feat(forge): WithOverlays + WithSpecStore options` | `forge.go`, build wiring, `forge_test.go` migration |
| 7 | `feat(manifest): ExtendsChain + Overlays attribution` | `manifest/manifest.go`, build pipeline writes attribution |
| 8 | `feat(forge): NormalizedSpec accessor + LoadOverlays helper + integration test` | `forge.go`, end-to-end scenario, demo update if relevant |
| 9 | `docs: amend Phase 0/1 design docs for Phase 2a` | `forge-overview.md` (Build signature note), `agent-spec-v0.md` (extends semantics) |

Each commit lands on a green test suite (unit + offline integration)
and passes `make lint` / `go vet`.

## Verification (after every commit lands)

- `make test-race` — clean.
- `make lint` — zero reports.
- `go vet ./...` — clean.
- `go test ./... -count=1` — all packages green.
- Round-trip a 2-deep extends + 2-overlay scenario through `forge.Build`
  via the integration test fixture; assert `Manifest.ExtendsChain`,
  `Manifest.Overlays`, and `BuiltAgent.NormalizedSpec()` shapes.
- Inspect a violation case via the integration test:
  `errors.Is(err, spec.ErrLockedFieldOverride)` is true and the error
  string carries the source overlay file and field path.

## Open items the engineer must verify against the live tree

These cannot be resolved in this spec because they depend on details
the implementer will see when writing code:

1. The exact `yaml.v3` `*yaml.Node` API used for line-number capture in
   `RefList.UnmarshalYAML` (`Line` field is documented but version-
   sensitive).
2. Whether `errors.Join` (Go 1.20+) suits aggregating heterogeneous
   sentinels better than the bespoke `Errors.Wrap` proposed in
   Section 7. Pick whichever is cleaner once the aggregator's call
   sites are concrete.
3. Compatibility of `FilesystemSpecStore` on Windows (path separators,
   symlink semantics). Out of scope to design here; engineer adds a
   `runtime.GOOS == "windows"` guard or a path-cleaning helper as
   needed.
