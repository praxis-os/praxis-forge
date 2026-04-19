# Task group 2 — provenance type + `NormalizedSpec` wrapper

> Part of [praxis-forge Phase 2a Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md).

**Commit (atomic):** `feat(spec): provenance type + NormalizedSpec wrapper`

**Scope:** declare the provenance value (`File`, `Line`, `Role`, `Step`), the `Role` enum, the `provenanceFields` mirror struct (one `Provenance` per top-level `AgentSpec` field), the `OverlayAttribution` carrier, and the `NormalizedSpec` wrapper type with its `Provenance(fieldPath)` accessor. **No** merge logic yet — that lands in task groups 3 + 4. This commit is the type scaffolding plus a focused unit test that the accessor returns the right per-field values when populated by hand.

---

## Task 2.1: `Provenance` value + `Role` enum + `OverlayAttribution`

**Files:**
- Create: `spec/provenance.go`

- [ ] **Step 1: Write the types**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import "fmt"

// Provenance attributes a single field in the merged spec back to its
// source: which file (or in-memory ref) it came from, what role that
// source played in composition, and the YAML line where it was set.
//
// Provenance is data, not a cross-cutting concern: it travels alongside
// the merged spec on NormalizedSpec, not through context.
type Provenance struct {
	File string // file path or ref id (empty for struct-literal sources)
	Line int    // 1-based line in the source YAML; 0 if unknown
	Role Role
	Step int // 0 for base; chain depth from leaf for parent; overlay index for overlay
}

// Role tags where a Provenance value originated within the composition
// pipeline.
type Role uint8

const (
	// RoleBase marks a value sourced from the raw AgentSpec passed to
	// Build (i.e. the leaf of the extends chain).
	RoleBase Role = iota
	// RoleParent marks a value sourced from a spec reached through
	// AgentSpec.Extends. Provenance.Step records the chain depth from
	// the leaf (1 = direct parent, 2 = grandparent, ...).
	RoleParent
	// RoleOverlay marks a value sourced from an overlay applied after
	// merge. Provenance.Step is the overlay's position in the
	// overlays slice (0 = first overlay applied).
	RoleOverlay
)

// String returns a short stable label for the role, used in error
// messages.
func (r Role) String() string {
	switch r {
	case RoleBase:
		return "base"
	case RoleParent:
		return "parent"
	case RoleOverlay:
		return "overlay"
	default:
		return fmt.Sprintf("role(%d)", r)
	}
}

// describe formats the provenance for inclusion in error messages.
// Examples:
//
//	overlay "prod-override" (overlay #1) at testdata/overlay/prod.yaml:7
//	parent "acme.base@1.0.0" (parent #2) at acme/base.yaml:3
//	base
func (p Provenance) describe() string {
	switch p.Role {
	case RoleBase:
		return "base"
	case RoleParent:
		if p.File != "" {
			return fmt.Sprintf("parent #%d at %s:%d", p.Step, p.File, p.Line)
		}
		return fmt.Sprintf("parent #%d", p.Step)
	case RoleOverlay:
		if p.File != "" {
			return fmt.Sprintf("overlay #%d at %s:%d", p.Step, p.File, p.Line)
		}
		return fmt.Sprintf("overlay #%d", p.Step)
	default:
		return p.Role.String()
	}
}

// OverlayAttribution surfaces in Manifest.Overlays and on
// NormalizedSpec to identify each overlay that contributed to the
// merged result.
type OverlayAttribution struct {
	// Name is the overlay's metadata.name.
	Name string `json:"name"`
	// File is the path passed to LoadOverlay; empty for struct-literal
	// overlays. Reproduced in the manifest with omitempty.
	File string `json:"file,omitempty"`
}
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 2.2: `provenanceFields` mirror + `NormalizedSpec` + accessor

**Files:**
- Modify: `spec/provenance.go`

- [ ] **Step 1: Append the wrapper types**

Append to `spec/provenance.go`:

```go
// provenanceFields mirrors the top-level shape of AgentSpec, one
// Provenance per field. Nested collections share their parent field's
// provenance (per-element provenance is out of scope for Phase 2a).
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

// NormalizedSpec is the canonical merge result of a base AgentSpec, its
// resolved extends chain, and any overlays applied on top.
//
// After Normalize completes:
//   - Spec.Extends is always nil/empty (the chain has been flattened).
//   - ExtendsChain lists the resolved parent refs root-first (empty if
//     no extends).
//   - Overlays carries one OverlayAttribution per overlay applied, in
//     the order they were applied.
//   - The (unexported) provenance mirror records, for each top-level
//     field, where the final value came from.
type NormalizedSpec struct {
	Spec         AgentSpec
	ExtendsChain []string
	Overlays     []OverlayAttribution

	fields provenanceFields // unexported; access via NormalizedSpec.Provenance
}

// Provenance returns the source attribution for a top-level spec
// field. fieldPath uses the spec's lowercase YAML names: "apiVersion",
// "kind", "metadata", "provider", "prompt", "tools", "policies",
// "filters", "budget", "telemetry", "credentials", "identity".
//
// The boolean reports whether the path is recognized. Nested paths
// (e.g. "filters.preLLM") return the parent field's provenance and ok
// = true; this is intentional — Phase 2a does not track per-element
// provenance.
func (n *NormalizedSpec) Provenance(fieldPath string) (Provenance, bool) {
	// Strip subpath after the first '.': nested fields share the
	// top-level field's provenance.
	top := fieldPath
	for i, c := range fieldPath {
		if c == '.' {
			top = fieldPath[:i]
			break
		}
	}
	switch top {
	case "apiVersion":
		return n.fields.APIVersion, true
	case "kind":
		return n.fields.Kind, true
	case "metadata":
		return n.fields.Metadata, true
	case "provider":
		return n.fields.Provider, true
	case "prompt":
		return n.fields.Prompt, true
	case "tools":
		return n.fields.Tools, true
	case "policies":
		return n.fields.Policies, true
	case "filters":
		return n.fields.Filters, true
	case "budget":
		return n.fields.Budget, true
	case "telemetry":
		return n.fields.Telemetry, true
	case "credentials":
		return n.fields.Credentials, true
	case "identity":
		return n.fields.Identity, true
	}
	return Provenance{}, false
}

// setFieldProvenance is the internal hook merge/apply functions use to
// record where a field's value came from. Kept package-private to
// preserve the "data flows out through accessors only" invariant.
//
// Caller passes the same top-level field name set the accessor accepts.
// Unrecognized names are a programming error and panic; the caller is
// always inside spec/.
func (n *NormalizedSpec) setFieldProvenance(field string, p Provenance) {
	switch field {
	case "apiVersion":
		n.fields.APIVersion = p
	case "kind":
		n.fields.Kind = p
	case "metadata":
		n.fields.Metadata = p
	case "provider":
		n.fields.Provider = p
	case "prompt":
		n.fields.Prompt = p
	case "tools":
		n.fields.Tools = p
	case "policies":
		n.fields.Policies = p
	case "filters":
		n.fields.Filters = p
	case "budget":
		n.fields.Budget = p
	case "telemetry":
		n.fields.Telemetry = p
	case "credentials":
		n.fields.Credentials = p
	case "identity":
		n.fields.Identity = p
	default:
		panic(fmt.Sprintf("setFieldProvenance: unknown field %q", field))
	}
}
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 2.3: `NormalizedSpec.Provenance` unit test

**Files:**
- Create: `spec/provenance_test.go`

- [ ] **Step 1: Write the test**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import "testing"

func TestProvenance_AllTopLevelFieldsRoundTrip(t *testing.T) {
	n := &NormalizedSpec{}
	cases := map[string]Provenance{
		"apiVersion":  {Role: RoleBase},
		"kind":        {Role: RoleBase},
		"metadata":    {Role: RoleBase, File: "base.yaml", Line: 3},
		"provider":    {Role: RoleParent, Step: 1, File: "acme/base.yaml", Line: 7},
		"prompt":      {Role: RoleBase, File: "base.yaml", Line: 9},
		"tools":       {Role: RoleOverlay, Step: 0, File: "overlay-1.yaml", Line: 4},
		"policies":    {Role: RoleParent, Step: 2, File: "acme/grand.yaml", Line: 2},
		"filters":     {Role: RoleOverlay, Step: 1, File: "overlay-2.yaml", Line: 5},
		"budget":      {Role: RoleBase, File: "base.yaml", Line: 14},
		"telemetry":   {Role: RoleBase, File: "base.yaml", Line: 17},
		"credentials": {Role: RoleBase, File: "base.yaml", Line: 19},
		"identity":    {Role: RoleBase, File: "base.yaml", Line: 21},
	}
	for f, p := range cases {
		n.setFieldProvenance(f, p)
	}
	for f, want := range cases {
		got, ok := n.Provenance(f)
		if !ok {
			t.Fatalf("%s: not recognized", f)
		}
		if got != want {
			t.Fatalf("%s: got %+v, want %+v", f, got, want)
		}
	}
}

func TestProvenance_NestedPathReturnsParentFieldProvenance(t *testing.T) {
	n := &NormalizedSpec{}
	n.setFieldProvenance("filters", Provenance{Role: RoleOverlay, Step: 0, File: "ov.yaml", Line: 2})
	got, ok := n.Provenance("filters.preLLM")
	if !ok {
		t.Fatal("filters.preLLM should resolve to filters' provenance")
	}
	if got.Role != RoleOverlay || got.Step != 0 || got.File != "ov.yaml" {
		t.Fatalf("got %+v", got)
	}
}

func TestProvenance_UnknownPathFalse(t *testing.T) {
	n := &NormalizedSpec{}
	if _, ok := n.Provenance("nope"); ok {
		t.Fatal("unknown field should return ok=false")
	}
}

func TestProvenance_DescribeFormat(t *testing.T) {
	cases := []struct {
		p    Provenance
		want string
	}{
		{Provenance{Role: RoleBase}, "base"},
		{Provenance{Role: RoleParent, Step: 2, File: "acme/base.yaml", Line: 7},
			"parent #2 at acme/base.yaml:7"},
		{Provenance{Role: RoleParent, Step: 1}, "parent #1"},
		{Provenance{Role: RoleOverlay, Step: 0, File: "ov.yaml", Line: 4},
			"overlay #0 at ov.yaml:4"},
		{Provenance{Role: RoleOverlay, Step: 1}, "overlay #1"},
	}
	for _, c := range cases {
		if got := c.p.describe(); got != c.want {
			t.Errorf("describe(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run (expect pass)**

Run: `go test ./spec/... -run TestProvenance -v`
Expected: every subtest PASS.

---

## Task 2.4: Lint + commit task group 2

- [ ] **Step 1: Race + full spec suite**

Run: `go test -race ./spec/... -count=1`
Expected: PASS, no race warnings.

- [ ] **Step 2: Lint**

Run: `make lint`
Expected: zero reports.

- [ ] **Step 3: Commit**

```bash
git add spec/provenance.go spec/provenance_test.go
git commit -m "feat(spec): provenance type + NormalizedSpec wrapper

Adds Provenance{File, Line, Role, Step}, the Role enum (base | parent
| overlay), the unexported provenanceFields mirror struct (one
Provenance per top-level AgentSpec field), the OverlayAttribution
carrier, and the NormalizedSpec wrapper with its Provenance(fieldPath)
accessor.

Nested paths (filters.preLLM, metadata.id) intentionally return the
parent field's provenance: Phase 2a does not track per-element
provenance — too expensive for the inspection benefit. Per-element
attribution can land in a later phase without breaking the accessor
contract.

No merge logic in this commit; that lands in the next two task groups.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```
