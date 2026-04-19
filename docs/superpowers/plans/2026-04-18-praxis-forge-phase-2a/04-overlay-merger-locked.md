# Task group 4 — overlay merger + locked-field validation + `Normalize` entry

> Part of [praxis-forge Phase 2a Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md).

**Commit (atomic):** `feat(spec): overlay merger + locked-field validation`

**Scope:** the rest of `Normalize`. Three pieces:

1. `mergeChain(parents..., base)` — applies child-wins merge over the resolved extends chain in field-declaration order. Uniform rule for ALL fields including locked ones; base wins last.
2. `applyOverlays(merged, overlays)` — applies overlays first-to-last, last-wins, with the `RefList.Set` flag driving "clear vs preserve".
3. `scanLockedDrift(base, parents, overlays)` — pure function that walks parents and overlays directly (not the merged result) and emits `ErrLockedFieldOverride` for any layer that supplies a locked-field value differing from `base`.

Plus the public `Normalize(ctx, s, overlays, store)` entry point that wires resolver → merge → apply → scan → existing `Validate` together. Bounded to 16 overlays via `ErrCompositionLimit`.

> **Spec amendment note (two deviations from sections 5 + 6 of the design spec):**
>
> 1. The spec's `validateLocked(base, final, prov, errs)` diffs base vs the merged result. Under our base-wins-last merge rule, `base == final` always, so drift is never visible in `final`. The plan implements the design *intent* — "the agent's identity stays fixed; overlays cannot rebrand it" — through `scanLockedDrift(base, parents, overlays, errs)` that inspects inputs directly.
>
> 2. The spec lists "Locked-field violation in extends → ErrLockedFieldOverride" as a test case (Section 9). On reflection this is wrong: extends parents are separate agents with their own metadata.id/version (e.g. `acme.support-triage@1.0.0` extends `acme.base@1.0.0`). Their identity differs from the child by design. Since base wins under child-wins merge, the parent's locked fields are silently overridden — no harm. **scanLockedDrift checks overlays only**, not parents. The "extends override locked" assertion in the spec is dropped as not implementable without breaking normal extends usage.
>
> Sentinel (`ErrLockedFieldOverride`) and call-site contract (`errors.Is(err, ErrLockedFieldOverride)`) are unchanged. The spec sections 5 + 6 should be re-amended as part of task group 8 to reflect both points; this plan is authoritative for implementation.

This is the largest task group: ~3 production files, ~2 test files, ~6 fixtures.

---

## Task 4.1: `applyComponentRef` and other field-shape helpers

**Files:**
- Create: `spec/merge.go`

- [ ] **Step 1: Write the helpers**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

// merge.go centralizes the field-shape helpers used by both mergeChain
// (parents → base) and applyOverlays (overlays → merged). The two
// callers differ only in how they detect "set" — mergeChain checks
// nil/zero on the AgentSpec field; applyOverlays checks the
// AgentOverlayBody's pointer field plus, for replaceable lists, the
// RefList.Set flag.

// scalarString picks the child value if non-empty; otherwise the parent.
func scalarString(parent, child string) string {
	if child != "" {
		return child
	}
	return parent
}

// pointerStruct returns child if non-nil; otherwise parent. Generic over
// any pointer type the spec uses (*ComponentRef, *BudgetRef, *CredRef).
func pointerStruct[T any](parent, child *T) *T {
	if child != nil {
		return child
	}
	return parent
}

// sliceReplaceIfSet returns child if it is non-nil; otherwise parent.
// Used for typed slices on AgentSpec where "no value" is nil.
func sliceReplaceIfSet[T any](parent, child []T) []T {
	if child != nil {
		return child
	}
	return parent
}

// mapStringStringReplace returns child if non-nil; otherwise parent.
// Per design, Metadata.Labels uses replace semantics rather than merge.
func mapStringStringReplace(parent, child map[string]string) map[string]string {
	if child != nil {
		return child
	}
	return parent
}
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 4.2: `mergeChain` — parents → base, field by field, in declaration order

**Files:**
- Modify: `spec/normalize.go`

- [ ] **Step 1: Append `mergeChain` and the per-parent merge helper**

Append to `spec/normalize.go`:

```go
// mergeChain folds parents into base in root-first order with
// child-wins semantics. Returns a new AgentSpec value (no mutation of
// inputs). Field iteration order matches AgentSpec's declaration order
// in spec/types.go — that order is the canonical merge order Phase 2b
// will hash against, so it is fixed here even though the hash itself
// has not landed.
//
// parents: root-first slice from resolveExtendsChain (parents[0] is
// the deepest ancestor).
// base:    the leaf spec passed to Normalize.
//
// Per-field provenance is recorded into prov: a parent contribution
// flips Role to RoleParent and Step to chain depth from the leaf
// (1 = deepest parent merged first; len(parents) = direct parent of
// base).
func mergeChain(parents []*AgentSpec, base *AgentSpec, prov *provenanceFields) *AgentSpec {
	// Initialize provenance to "base" for every field; per-layer merges
	// overwrite when the layer contributes a non-zero value. Final
	// provenance reflects the layer that won the field.
	*prov = provenanceFields{
		APIVersion:  Provenance{Role: RoleBase},
		Kind:        Provenance{Role: RoleBase},
		Metadata:    Provenance{Role: RoleBase},
		Provider:    Provenance{Role: RoleBase},
		Prompt:      Provenance{Role: RoleBase},
		Tools:       Provenance{Role: RoleBase},
		Policies:    Provenance{Role: RoleBase},
		Filters:     Provenance{Role: RoleBase},
		Budget:      Provenance{Role: RoleBase},
		Telemetry:   Provenance{Role: RoleBase},
		Credentials: Provenance{Role: RoleBase},
		Identity:    Provenance{Role: RoleBase},
	}

	// Walk parents in root-first order, then fold base last. Each
	// mergeOne call lets the "child" layer win on every non-zero field.
	// Step values: parents are 1-based from the leaf (parent #1 is
	// closest to base, parent #len is deepest ancestor); base records
	// as RoleBase with no step.
	var merged AgentSpec
	for i, parent := range parents {
		parentProv := Provenance{Role: RoleParent, Step: len(parents) - i}
		mergeOne(&merged, parent, parentProv, prov)
	}
	baseProv := Provenance{Role: RoleBase}
	mergeOne(&merged, base, baseProv, prov)

	// Extends is always cleared on the merged result: the chain has
	// been flattened into the merged value.
	merged.Extends = nil
	return &merged
}

// mergeOne folds child into merged with child-wins semantics, updating
// per-field provenance whenever the child contributes a non-zero
// value. The provenance value to record on contribution is supplied
// by the caller (parent step number or RoleBase).
//
// nolint:gocyclo // linear walk over AgentSpec's top-level fields.
func mergeOne(merged *AgentSpec, child *AgentSpec, p Provenance, prov *provenanceFields) {
	// Uniform child-wins rule for every field, including the locked
	// ones (apiVersion, kind, metadata.id, metadata.version). Locked
	// drift is detected separately by scanLockedDrift, which inspects
	// the *inputs* (parents, overlays) against base — not the merged
	// result. That keeps mergeOne simple and avoids the "base wins
	// last but we still need drift attribution" tangle.
	if child.APIVersion != "" {
		merged.APIVersion = child.APIVersion
		prov.APIVersion = p
	}
	if child.Kind != "" {
		merged.Kind = child.Kind
		prov.Kind = p
	}

	// Metadata: replace whole struct if any field present (cheap heuristic;
	// locked-field check enforces ID/Version constancy).
	if !isZeroMetadata(child.Metadata) {
		merged.Metadata = mergeMetadata(merged.Metadata, child.Metadata)
		prov.Metadata = p
	}

	// Provider: replace if non-empty ref.
	if child.Provider.Ref != "" {
		merged.Provider = child.Provider
		prov.Provider = p
	}

	// Prompt: child wins if either subfield is set.
	if child.Prompt.System != nil || child.Prompt.User != nil {
		merged.Prompt = child.Prompt
		prov.Prompt = p
	}

	// Lists: replace if non-nil.
	if child.Tools != nil {
		merged.Tools = child.Tools
		prov.Tools = p
	}
	if child.Policies != nil {
		merged.Policies = child.Policies
		prov.Policies = p
	}

	// FilterBlock: any sub-slice non-nil → child wins on the whole block.
	if child.Filters.PreLLM != nil || child.Filters.PreTool != nil || child.Filters.PostTool != nil {
		merged.Filters = child.Filters
		prov.Filters = p
	}

	// Pointer-typed structs.
	if child.Budget != nil {
		merged.Budget = child.Budget
		prov.Budget = p
	}
	if child.Telemetry != nil {
		merged.Telemetry = child.Telemetry
		prov.Telemetry = p
	}
	if child.Credentials != nil {
		merged.Credentials = child.Credentials
		prov.Credentials = p
	}
	if child.Identity != nil {
		merged.Identity = child.Identity
		prov.Identity = p
	}

	// Skills / MCPImports / OutputContract are phase-gated for Phase
	// 2a; mergeOne deliberately ignores them so a future-phase parent
	// fragment cannot smuggle them through extends.
}

// mergeMetadata folds child fields onto merged with child-wins per
// scalar field. Labels uses replace semantics (per design).
func mergeMetadata(merged, child Metadata) Metadata {
	out := merged
	out.ID = scalarString(merged.ID, child.ID)
	out.Version = scalarString(merged.Version, child.Version)
	out.DisplayName = scalarString(merged.DisplayName, child.DisplayName)
	out.Description = scalarString(merged.Description, child.Description)
	if child.Owners != nil {
		out.Owners = child.Owners
	}
	out.Labels = mapStringStringReplace(merged.Labels, child.Labels)
	return out
}

func isZeroMetadata(m Metadata) bool {
	return m.ID == "" && m.Version == "" && m.DisplayName == "" && m.Description == "" &&
		len(m.Owners) == 0 && len(m.Labels) == 0
}
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 4.3: `applyOverlays` — overlay → merged, last wins

**Files:**
- Modify: `spec/normalize.go`

- [ ] **Step 1: Append `applyOverlays`**

Append to `spec/normalize.go`:

```go
// MaxOverlayCount bounds how many overlays Normalize will apply. Picked
// at design time; tune in a later phase if real deployments hit it.
const MaxOverlayCount = 16

// applyOverlays folds overlays into merged in slice order, last-wins.
// Reads RefList.Set to distinguish "preserve base" from "explicit
// clear-or-replace". Updates per-field provenance as each overlay
// contributes.
//
// Returns ErrCompositionLimit if len(overlays) > MaxOverlayCount.
//
// Pre-condition: validateLocked is the responsibility of the caller
// (Normalize), not this helper. applyOverlays does not enforce
// locked-field protection — it freely writes whatever the overlay
// supplies and lets validateLocked surface drift after the fact.
func applyOverlays(merged *AgentSpec, overlays []*AgentOverlay, prov *provenanceFields) error {
	if len(overlays) > MaxOverlayCount {
		return fmt.Errorf("applyOverlays: %d overlays exceed %d: %w",
			len(overlays), MaxOverlayCount, ErrCompositionLimit)
	}

	for idx, ov := range overlays {
		if ov == nil {
			continue
		}
		stepProv := Provenance{Role: RoleOverlay, Step: idx, File: ov.File}
		applyOne(merged, &ov.Spec, stepProv, prov)
	}
	return nil
}

// applyOne folds a single overlay body into merged with overlay-wins
// semantics, updating per-field provenance whenever the overlay sets a
// field. RefList.Set is the test for "did the overlay touch this list?".
//
// nolint:gocyclo // linear walk over AgentOverlayBody's fields.
func applyOne(merged *AgentSpec, body *AgentOverlayBody, p Provenance, prov *provenanceFields) {
	if body.Metadata != nil {
		merged.Metadata = applyOverlayMetadata(merged.Metadata, *body.Metadata)
		prov.Metadata = p
	}
	if body.Provider != nil {
		merged.Provider = *body.Provider
		prov.Provider = p
	}
	if body.Prompt != nil {
		merged.Prompt = *body.Prompt
		prov.Prompt = p
	}
	if body.Tools != nil && body.Tools.Set {
		merged.Tools = body.Tools.Items
		// Capture the overlay's source line on the field provenance.
		stepProv := p
		stepProv.Line = body.Tools.Line
		prov.Tools = stepProv
	}
	if body.Policies != nil && body.Policies.Set {
		merged.Policies = body.Policies.Items
		stepProv := p
		stepProv.Line = body.Policies.Line
		prov.Policies = stepProv
	}
	if body.Filters != nil {
		applyOverlayFilters(merged, body.Filters, p, prov)
	}
	if body.Budget != nil {
		merged.Budget = body.Budget
		prov.Budget = p
	}
	if body.Telemetry != nil {
		merged.Telemetry = body.Telemetry
		prov.Telemetry = p
	}
	if body.Credentials != nil {
		merged.Credentials = body.Credentials
		prov.Credentials = p
	}
	if body.Identity != nil {
		merged.Identity = body.Identity
		prov.Identity = p
	}
}

// applyOverlayMetadata folds an overlay's metadata onto merged. Locked
// fields (ID, Version) are written through; validateLocked is what
// rejects drift afterwards, so the error message can still attribute
// the change to the overlay.
func applyOverlayMetadata(merged Metadata, ov OverlayMetadata) Metadata {
	out := merged
	if ov.ID != "" {
		out.ID = ov.ID
	}
	if ov.Version != "" {
		out.Version = ov.Version
	}
	if ov.DisplayName != "" {
		out.DisplayName = ov.DisplayName
	}
	if ov.Description != "" {
		out.Description = ov.Description
	}
	if ov.Owners != nil {
		out.Owners = ov.Owners
	}
	if ov.Labels != nil {
		out.Labels = ov.Labels
	}
	return out
}

// applyOverlayFilters folds the three filter stages individually so an
// overlay can clear preLLM but leave preTool/postTool untouched.
func applyOverlayFilters(merged *AgentSpec, ov *FilterOverlay, p Provenance, prov *provenanceFields) {
	any := false
	if ov.PreLLM != nil && ov.PreLLM.Set {
		merged.Filters.PreLLM = ov.PreLLM.Items
		any = true
	}
	if ov.PreTool != nil && ov.PreTool.Set {
		merged.Filters.PreTool = ov.PreTool.Items
		any = true
	}
	if ov.PostTool != nil && ov.PostTool.Set {
		merged.Filters.PostTool = ov.PostTool.Items
		any = true
	}
	if any {
		prov.Filters = p
	}
}
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 4.4: `scanLockedDrift` pure function

**Files:**
- Create: `spec/locked.go`

- [ ] **Step 1: Write the function**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import "fmt"

// scanLockedDrift inspects every overlay supplied to Normalize for
// any locked-field value that differs from base. Each drift produces a
// wrapped ErrLockedFieldOverride entry on the aggregator carrying the
// offending overlay's name and index.
//
// Locked fields:
//   - APIVersion (overlay envelope; should match base)
//   - Metadata.ID
//   - Metadata.Version
//
// (Kind on AgentOverlay is "AgentOverlay", not "AgentSpec"; it is not
// a base-comparable field, so it is intentionally skipped.)
//
// Parents are not scanned: extends parents are separate agents with
// their own identities. The child-wins merge rule already ensures
// base.metadata.id/version win over any parent's, so a parent's
// differing locked fields are silently overridden by base — no
// rebrand of the child agent occurs.
//
// scanLockedDrift is a pure function over the inputs; reusable from
// any future composition path. It deliberately does not diff base
// against the merged result (that diff is always zero under the
// base-wins-last merge rule we use). It also does not live as a
// method on *AgentSpec — that would couple AgentSpec to the merge
// concern.
func scanLockedDrift(base *AgentSpec, overlays []*AgentOverlay, errs *Errors) {
	for i, ov := range overlays {
		if ov == nil {
			continue
		}
		// AgentOverlay.APIVersion drift would have been rejected at
		// LoadOverlay time (envelope check); programmatic overlays
		// constructed in Go bypass that envelope, so re-check here.
		if ov.APIVersion != "" && ov.APIVersion != base.APIVersion {
			errs.Wrap(ErrLockedFieldOverride,
				lockedMsg("apiVersion", i, ov.Metadata.Name, ov.APIVersion, base.APIVersion))
		}
		if ov.Spec.Metadata == nil {
			continue
		}
		if ov.Spec.Metadata.ID != "" && ov.Spec.Metadata.ID != base.Metadata.ID {
			errs.Wrap(ErrLockedFieldOverride,
				lockedMsg("metadata.id", i, ov.Metadata.Name, ov.Spec.Metadata.ID, base.Metadata.ID))
		}
		if ov.Spec.Metadata.Version != "" && ov.Spec.Metadata.Version != base.Metadata.Version {
			errs.Wrap(ErrLockedFieldOverride,
				lockedMsg("metadata.version", i, ov.Metadata.Name, ov.Spec.Metadata.Version, base.Metadata.Version))
		}
	}
}

// lockedMsg formats a uniform error string. Examples:
//
//	"metadata.id: locked, overlay #0 (prod-override) set \"acme.other\" (base = \"acme.demo\")"
//	"metadata.id: locked, overlay #1 set \"acme.other\" (base = \"acme.demo\")"  (no name)
func lockedMsg(field string, idx int, name, got, base string) string {
	if name != "" {
		return fmt.Sprintf("%s: locked, overlay #%d (%s) set %q (base = %q)", field, idx, name, got, base)
	}
	return fmt.Sprintf("%s: locked, overlay #%d set %q (base = %q)", field, idx, got, base)
}
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 4.5: `scanLockedDrift` unit tests

**Files:**
- Create: `spec/locked_test.go`

- [ ] **Step 1: Write the tests**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"errors"
	"strings"
	"testing"
)

func lockedTestSpec(apiv, kind, id, ver string) *AgentSpec {
	return &AgentSpec{
		APIVersion: apiv,
		Kind:       kind,
		Metadata:   Metadata{ID: id, Version: ver},
	}
}

func TestScanLockedDrift_NoOverlaysIsClean(t *testing.T) {
	base := lockedTestSpec("forge.praxis-os.dev/v0", "AgentSpec", "acme.demo", "0.1.0")
	var errs Errors
	scanLockedDrift(base, nil, &errs)
	if errs.Len() != 0 {
		t.Fatalf("unexpected violations: %v", errs)
	}
}

func TestScanLockedDrift_OverlayMatchesBaseIsClean(t *testing.T) {
	base := lockedTestSpec("forge.praxis-os.dev/v0", "AgentSpec", "acme.demo", "0.1.0")
	overlay := &AgentOverlay{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentOverlay",
		Metadata:   OverlayMeta{Name: "matching"},
		Spec: AgentOverlayBody{
			Metadata: &OverlayMetadata{ID: "acme.demo", Version: "0.1.0"},
		},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if errs.Len() != 0 {
		t.Fatalf("unexpected violations: %v", errs)
	}
}

func TestScanLockedDrift_OverlayMetadataIDDriftFlagged(t *testing.T) {
	base := lockedTestSpec("forge.praxis-os.dev/v0", "AgentSpec", "acme.demo", "0.1.0")
	overlay := &AgentOverlay{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentOverlay",
		Metadata:   OverlayMeta{Name: "bad-rebrand"},
		Spec: AgentOverlayBody{
			Metadata: &OverlayMetadata{ID: "acme.other"},
		},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if !errors.Is(errs, ErrLockedFieldOverride) {
		t.Fatalf("expected wrap of ErrLockedFieldOverride, got %v", errs)
	}
	if !strings.Contains(errs.Error(), "metadata.id") {
		t.Fatalf("error string missing field name: %v", errs)
	}
	if !strings.Contains(errs.Error(), "overlay #0 (bad-rebrand)") {
		t.Fatalf("error string missing overlay attribution: %v", errs)
	}
	if !strings.Contains(errs.Error(), `"acme.other"`) {
		t.Fatalf("error string missing offending value: %v", errs)
	}
	if !strings.Contains(errs.Error(), `base = "acme.demo"`) {
		t.Fatalf("error string missing base value: %v", errs)
	}
}

func TestScanLockedDrift_OverlayWithoutNameAttributesByIndex(t *testing.T) {
	base := lockedTestSpec("forge.praxis-os.dev/v0", "AgentSpec", "acme.demo", "0.1.0")
	overlay := &AgentOverlay{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentOverlay",
		// No Metadata.Name
		Spec: AgentOverlayBody{
			Metadata: &OverlayMetadata{Version: "9.9.9"},
		},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if !errors.Is(errs, ErrLockedFieldOverride) {
		t.Fatalf("expected wrap of ErrLockedFieldOverride, got %v", errs)
	}
	if !strings.Contains(errs.Error(), "overlay #0 set") {
		t.Fatalf("error must omit name when overlay has no name: %v", errs)
	}
}

func TestScanLockedDrift_AllThreeOverlayFieldsTogether(t *testing.T) {
	base := lockedTestSpec("forge.praxis-os.dev/v0", "AgentSpec", "acme.demo", "0.1.0")
	overlay := &AgentOverlay{
		APIVersion: "nope/v0", // apiVersion drift
		Kind:       "AgentOverlay",
		Metadata:   OverlayMeta{Name: "ov"},
		Spec: AgentOverlayBody{
			Metadata: &OverlayMetadata{ID: "acme.other", Version: "9.9.9"}, // id + version drift
		},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if errs.Len() != 3 {
		t.Fatalf("want 3 violations, got %d (%v)", errs.Len(), errs)
	}
	for _, want := range []string{"apiVersion", "metadata.id", "metadata.version"} {
		if !strings.Contains(errs.Error(), want) {
			t.Errorf("missing %q in %v", want, errs)
		}
	}
}

func TestScanLockedDrift_NilOverlaySkipped(t *testing.T) {
	base := lockedTestSpec("forge.praxis-os.dev/v0", "AgentSpec", "acme.demo", "0.1.0")
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{nil}, &errs)
	if errs.Len() != 0 {
		t.Fatalf("nil overlay should be skipped: %v", errs)
	}
}

func TestScanLockedDrift_OverlayWithoutMetadataBlockSkipsLockedChecks(t *testing.T) {
	// An overlay that doesn't touch metadata at all is fine — only
	// metadata.id and metadata.version need locked-field protection
	// among the overlay's mutable fields.
	base := lockedTestSpec("forge.praxis-os.dev/v0", "AgentSpec", "acme.demo", "0.1.0")
	overlay := &AgentOverlay{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentOverlay",
		Metadata:   OverlayMeta{Name: "no-metadata"},
		Spec: AgentOverlayBody{
			Provider: &ComponentRef{Ref: "provider.fake@1.0.0"},
		},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if errs.Len() != 0 {
		t.Fatalf("overlay with no metadata block should pass: %v", errs)
	}
}
```

- [ ] **Step 2: Run (expect pass)**

Run: `go test ./spec/... -run TestScanLockedDrift -v`
Expected: every subtest PASS.

---

## Task 4.6: Public `Normalize` entry point

**Files:**
- Modify: `spec/normalize.go`

- [ ] **Step 1: Append the public entry point**

Append to `spec/normalize.go`:

```go
// Normalize resolves the extends chain, applies overlays in order,
// validates locked fields, runs the existing Phase 1 validator on the
// merged result, and returns a NormalizedSpec carrying the merged
// AgentSpec, the resolved chain, the overlay attribution list, and
// per-field provenance.
//
// Steps, in order:
//  1. resolveExtendsChain(ctx, s, store) → []*AgentSpec, root-first
//  2. mergeChain(parents..., s)          → merged AgentSpec, child wins
//  3. applyOverlays(merged, overlays)    → final AgentSpec
//  4. scanLockedDrift(s, parents, overlays) → wraps drifts as ErrLockedFieldOverride
//  5. final.Validate()                   → existing Phase 1 invariants
//
// All errors (chain, overlay limit, locked, validate) are aggregated
// into a single returned error so a single Normalize call surfaces
// every violation. Use errors.Is on the result to discriminate.
func Normalize(
	ctx context.Context,
	s *AgentSpec,
	overlays []*AgentOverlay,
	store SpecStore,
) (*NormalizedSpec, error) {
	parents, chain, err := resolveExtendsChain(ctx, s, store)
	if err != nil {
		return nil, err
	}

	prov := &provenanceFields{}
	merged := mergeChain(parents, s, prov)

	if err := applyOverlays(merged, overlays, prov); err != nil {
		return nil, err
	}

	var errs Errors
	scanLockedDrift(s, overlays, &errs)
	_ = parents // parents are only needed by mergeChain; scan ignores them

	if vErr := merged.Validate(); vErr != nil {
		// merged.Validate returns Errors directly. Append its messages
		// to ours so the aggregator carries every violation.
		var inner Errors
		if errors.As(vErr, &inner) {
			for _, m := range inner.msgs {
				errs.Addf("%s", m)
			}
			errs.sentinels = append(errs.sentinels, inner.sentinels...)
		} else {
			errs.Addf("%s", vErr.Error())
		}
	}

	if err := errs.OrNil(); err != nil {
		return nil, err
	}

	return &NormalizedSpec{
		Spec:         *merged,
		ExtendsChain: chain,
		Overlays:     buildOverlayAttribution(overlays),
		fields:       *prov,
	}, nil
}

func buildOverlayAttribution(overlays []*AgentOverlay) []OverlayAttribution {
	if len(overlays) == 0 {
		return nil
	}
	out := make([]OverlayAttribution, 0, len(overlays))
	for _, ov := range overlays {
		if ov == nil {
			continue
		}
		out = append(out, OverlayAttribution{
			Name: ov.Metadata.Name,
			File: ov.File,
		})
	}
	return out
}
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 4.7: End-to-end normalize tests — overlay-only path

**Files:**
- Create: `spec/testdata/normalize/overlay_replace_provider/base.yaml`
- Create: `spec/testdata/normalize/overlay_replace_provider/overlay-1.yaml`
- Create: `spec/testdata/normalize/overlay_replace_provider/want_provider_ref.txt`
- Modify: `spec/normalize_test.go`

- [ ] **Step 1: Write fixtures**

```yaml
# spec/testdata/normalize/overlay_replace_provider/base.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.demo
  version: 0.1.0
provider:
  ref: provider.fake@1.0.0
  config:
    model: claude-sonnet-4-5
prompt:
  system:
    ref: prompt.demo-system@1.0.0
```

```yaml
# spec/testdata/normalize/overlay_replace_provider/overlay-1.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: prod-override
spec:
  provider:
    ref: provider.anthropic@1.0.0
    config:
      model: claude-opus-4-7
```

`spec/testdata/normalize/overlay_replace_provider/want_provider_ref.txt`: `provider.anthropic@1.0.0`

- [ ] **Step 2: Append the test**

Append to `spec/normalize_test.go`:

```go
import (
	"os"
	"path/filepath"
)

func TestNormalize_OverlayReplacesProvider(t *testing.T) {
	dir := "testdata/normalize/overlay_replace_provider"
	s, err := LoadSpec(filepath.Join(dir, "base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ov, err := LoadOverlay(filepath.Join(dir, "overlay-1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Normalize(context.Background(), s, []*AgentOverlay{ov}, nil)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	wantBytes, _ := os.ReadFile(filepath.Join(dir, "want_provider_ref.txt"))
	want := strings.TrimSpace(string(wantBytes))
	if got.Spec.Provider.Ref != want {
		t.Fatalf("provider.ref=%q, want %q", got.Spec.Provider.Ref, want)
	}
	if got.Spec.Provider.Config["model"] != "claude-opus-4-7" {
		t.Fatalf("provider.config.model=%v", got.Spec.Provider.Config["model"])
	}

	// Provenance for provider should now point at the overlay.
	prov, ok := got.Provenance("provider")
	if !ok || prov.Role != RoleOverlay || prov.Step != 0 {
		t.Fatalf("provider provenance = %+v (ok=%v)", prov, ok)
	}
	if len(got.Overlays) != 1 || got.Overlays[0].Name != "prod-override" {
		t.Fatalf("overlay attribution = %+v", got.Overlays)
	}
}
```

- [ ] **Step 3: Run (expect pass)**

Run: `go test ./spec/... -run TestNormalize_OverlayReplacesProvider -v`
Expected: PASS.

---

## Task 4.8: End-to-end normalize tests — extends-only path

**Files:**
- Create: `spec/testdata/normalize/extends_two_deep/base.yaml`
- Create: `spec/testdata/normalize/extends_two_deep/parent-direct.yaml`
- Create: `spec/testdata/normalize/extends_two_deep/parent-grand.yaml`
- Modify: `spec/normalize_test.go`

- [ ] **Step 1: Write fixtures**

```yaml
# spec/testdata/normalize/extends_two_deep/base.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.child
  version: 0.1.0
provider:
  ref: provider.fake@1.0.0
prompt:
  system:
    ref: prompt.child@1.0.0
extends:
  - acme.parent@1.0.0
```

```yaml
# spec/testdata/normalize/extends_two_deep/parent-direct.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.parent
  version: 0.1.0
provider:
  ref: provider.parent@1.0.0
tools:
  - ref: toolpack.from-parent@1.0.0
extends:
  - acme.grand@1.0.0
```

```yaml
# spec/testdata/normalize/extends_two_deep/parent-grand.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.grand
  version: 0.1.0
policies:
  - ref: policypack.from-grand@1.0.0
prompt:
  system:
    ref: prompt.grand@1.0.0
provider:
  ref: provider.grand@1.0.0
```

- [ ] **Step 2: Append the test**

Append to `spec/normalize_test.go`:

```go
func TestNormalize_TwoDeepExtendsRespectsChildWins(t *testing.T) {
	dir := "testdata/normalize/extends_two_deep"
	base, err := LoadSpec(filepath.Join(dir, "base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	parentDirect, err := LoadSpec(filepath.Join(dir, "parent-direct.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	parentGrand, err := LoadSpec(filepath.Join(dir, "parent-grand.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	store := MapSpecStore{
		"acme.parent@1.0.0": parentDirect,
		"acme.grand@1.0.0":  parentGrand,
	}
	got, err := Normalize(context.Background(), base, nil, store)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	// provider.ref: child wins over both parents.
	if got.Spec.Provider.Ref != "provider.fake@1.0.0" {
		t.Fatalf("provider.ref=%q", got.Spec.Provider.Ref)
	}
	// tools: contributed by direct parent (base did not set them).
	if len(got.Spec.Tools) != 1 || got.Spec.Tools[0].Ref != "toolpack.from-parent@1.0.0" {
		t.Fatalf("tools=%+v", got.Spec.Tools)
	}
	// policies: contributed by grandparent only.
	if len(got.Spec.Policies) != 1 || got.Spec.Policies[0].Ref != "policypack.from-grand@1.0.0" {
		t.Fatalf("policies=%+v", got.Spec.Policies)
	}
	// metadata.id/version come from base (locked).
	if got.Spec.Metadata.ID != "acme.child" || got.Spec.Metadata.Version != "0.1.0" {
		t.Fatalf("metadata=%+v", got.Spec.Metadata)
	}
	// Extends has been flattened.
	if len(got.Spec.Extends) != 0 {
		t.Fatalf("Extends should be flattened, got %v", got.Spec.Extends)
	}
	// ExtendsChain captures both parents root-first.
	wantChain := []string{"acme.grand@1.0.0", "acme.parent@1.0.0"}
	if len(got.ExtendsChain) != len(wantChain) {
		t.Fatalf("chain=%v", got.ExtendsChain)
	}
	for i, w := range wantChain {
		if got.ExtendsChain[i] != w {
			t.Errorf("chain[%d]=%q, want %q", i, got.ExtendsChain[i], w)
		}
	}
	// Provenance for tools should point at parent #1 (direct parent),
	// because base does not set tools and the direct parent does.
	prov, _ := got.Provenance("tools")
	if prov.Role != RoleParent || prov.Step != 1 {
		t.Errorf("tools provenance = %+v, want parent #1", prov)
	}
	// provider provenance should be base (child wins).
	prov, _ = got.Provenance("provider")
	if prov.Role != RoleBase {
		t.Errorf("provider provenance = %+v, want base", prov)
	}
}
```

- [ ] **Step 3: Run (expect pass)**

Run: `go test ./spec/... -run TestNormalize_TwoDeepExtends -v`
Expected: PASS.

---

## Task 4.9: Normalize negative tests — locked, count, list semantics

**Files:**
- Create: `spec/testdata/normalize/overlay_overrides_id/base.yaml` + `overlay-1.yaml`
- Create: `spec/testdata/normalize/overlay_clears_tools/base.yaml` + `overlay-1.yaml`
- Modify: `spec/normalize_test.go`

- [ ] **Step 1: Write the locked-violation fixture**

```yaml
# spec/testdata/normalize/overlay_overrides_id/base.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.demo
  version: 0.1.0
provider:
  ref: provider.fake@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
```

```yaml
# spec/testdata/normalize/overlay_overrides_id/overlay-1.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: bad-rebrand
spec:
  metadata:
    id: acme.other
```

- [ ] **Step 2: Write the explicit-clear fixture**

```yaml
# spec/testdata/normalize/overlay_clears_tools/base.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.demo
  version: 0.1.0
provider:
  ref: provider.fake@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
tools:
  - ref: toolpack.http-get@1.0.0
```

```yaml
# spec/testdata/normalize/overlay_clears_tools/overlay-1.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: clear-tools
spec:
  tools: []
```

- [ ] **Step 3: Append the negative tests**

Append to `spec/normalize_test.go`:

```go
func TestNormalize_OverlayOverridesIDIsRejected(t *testing.T) {
	dir := "testdata/normalize/overlay_overrides_id"
	s, _ := LoadSpec(filepath.Join(dir, "base.yaml"))
	ov, _ := LoadOverlay(filepath.Join(dir, "overlay-1.yaml"))
	_, err := Normalize(context.Background(), s, []*AgentOverlay{ov}, nil)
	if err == nil {
		t.Fatal("expected error for locked-field override, got nil")
	}
	if !errors.Is(err, ErrLockedFieldOverride) {
		t.Fatalf("err=%v, want ErrLockedFieldOverride", err)
	}
	if !strings.Contains(err.Error(), "metadata.id") {
		t.Fatalf("error must name field: %v", err)
	}
	if !strings.Contains(err.Error(), "overlay #0") {
		t.Fatalf("error must attribute to overlay: %v", err)
	}
}

func TestNormalize_OverlayClearsTools(t *testing.T) {
	dir := "testdata/normalize/overlay_clears_tools"
	s, _ := LoadSpec(filepath.Join(dir, "base.yaml"))
	ov, _ := LoadOverlay(filepath.Join(dir, "overlay-1.yaml"))
	got, err := Normalize(context.Background(), s, []*AgentOverlay{ov}, nil)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if len(got.Spec.Tools) != 0 {
		t.Fatalf("tools should be cleared, got %+v", got.Spec.Tools)
	}
	prov, _ := got.Provenance("tools")
	if prov.Role != RoleOverlay {
		t.Errorf("tools provenance = %+v, want overlay", prov)
	}
}

func TestNormalize_OverlayCountExceedsLimit(t *testing.T) {
	s := stubSpec()
	overlays := make([]*AgentOverlay, MaxOverlayCount+1)
	for i := range overlays {
		overlays[i] = &AgentOverlay{
			APIVersion: expectedAPIVersion,
			Kind:       "AgentOverlay",
			Metadata:   OverlayMeta{Name: "ov"},
		}
	}
	_, err := Normalize(context.Background(), s, overlays, nil)
	if !errors.Is(err, ErrCompositionLimit) {
		t.Fatalf("err=%v, want ErrCompositionLimit", err)
	}
}

func TestNormalize_NoStoreWithExtendsErrors(t *testing.T) {
	s := stubSpec("acme.parent@1.0.0")
	_, err := Normalize(context.Background(), s, nil, nil)
	if !errors.Is(err, ErrNoSpecStore) {
		t.Fatalf("err=%v, want ErrNoSpecStore", err)
	}
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./spec/... -run TestNormalize -v`
Expected: every subtest PASS.

---

## Task 4.10: Lint + commit task group 4

- [ ] **Step 1: Race + full spec suite**

Run: `go test -race ./spec/... -count=1`
Expected: PASS, no race warnings.

- [ ] **Step 2: Lint**

Run: `make lint`
Expected: zero reports.

- [ ] **Step 3: Commit**

```bash
git add spec/merge.go spec/normalize.go spec/normalize_test.go spec/locked.go spec/locked_test.go spec/testdata/normalize
git commit -m "feat(spec): overlay merger + locked-field validation

Completes the Normalize pipeline: mergeChain folds parents into base
in field-declaration order with child-wins semantics; applyOverlays
folds overlays last-wins, reading RefList.Set to distinguish 'preserve
base' from 'explicit clear or replace'; validateLocked emits a
wrapped ErrLockedFieldOverride for any drift on apiVersion, kind,
metadata.id, or metadata.version.

Public Normalize(ctx, s, overlays, store) wires resolver → merge →
apply → locked → existing Validate together and aggregates all
violations into a single returned error so callers see every issue.

MaxOverlayCount = 16; exceeding → ErrCompositionLimit.
Phase-gated AgentSpec fields (skills, mcpImports, outputContract) are
not propagated through merge so a future-phase parent fragment cannot
smuggle them in via extends.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```
