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

	// Role string constants for error messages.
	roleBaseStr    = "base"
	roleParentStr  = "parent"
	roleOverlayStr = "overlay"
)

// String returns a short stable label for the role, used in error
// messages.
func (r Role) String() string {
	switch r {
	case RoleBase:
		return roleBaseStr
	case RoleParent:
		return roleParentStr
	case RoleOverlay:
		return roleOverlayStr
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
		return roleBaseStr
	case RoleParent:
		if p.File != "" {
			return fmt.Sprintf("%s #%d at %s:%d", roleParentStr, p.Step, p.File, p.Line)
		}
		return fmt.Sprintf("%s #%d", roleParentStr, p.Step)
	case RoleOverlay:
		if p.File != "" {
			return fmt.Sprintf("%s #%d at %s:%d", roleOverlayStr, p.Step, p.File, p.Line)
		}
		return fmt.Sprintf("%s #%d", roleOverlayStr, p.Step)
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

// Field name constants for provenance accessors.
const (
	fieldAPIVersion  = "apiVersion"
	fieldKind        = "kind"
	fieldMetadata    = "metadata"
	fieldProvider    = "provider"
	fieldPrompt      = "prompt"
	fieldTools       = "tools"
	fieldPolicies    = "policies"
	fieldFilters     = "filters"
	fieldBudget      = "budget"
	fieldTelemetry   = "telemetry"
	fieldCredentials = "credentials"
	fieldIdentity    = "identity"
)

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

	fields        provenanceFields // unexported; access via NormalizedSpec.Provenance
	canonicalMemo memoCanonical    // memoized canonical JSON; access via CanonicalJSON/NormalizedHash
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
	case fieldAPIVersion:
		return n.fields.APIVersion, true
	case fieldKind:
		return n.fields.Kind, true
	case fieldMetadata:
		return n.fields.Metadata, true
	case fieldProvider:
		return n.fields.Provider, true
	case fieldPrompt:
		return n.fields.Prompt, true
	case fieldTools:
		return n.fields.Tools, true
	case fieldPolicies:
		return n.fields.Policies, true
	case fieldFilters:
		return n.fields.Filters, true
	case fieldBudget:
		return n.fields.Budget, true
	case fieldTelemetry:
		return n.fields.Telemetry, true
	case fieldCredentials:
		return n.fields.Credentials, true
	case fieldIdentity:
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
	case fieldAPIVersion:
		n.fields.APIVersion = p
	case fieldKind:
		n.fields.Kind = p
	case fieldMetadata:
		n.fields.Metadata = p
	case fieldProvider:
		n.fields.Provider = p
	case fieldPrompt:
		n.fields.Prompt = p
	case fieldTools:
		n.fields.Tools = p
	case fieldPolicies:
		n.fields.Policies = p
	case fieldFilters:
		n.fields.Filters = p
	case fieldBudget:
		n.fields.Budget = p
	case fieldTelemetry:
		n.fields.Telemetry = p
	case fieldCredentials:
		n.fields.Credentials = p
	case fieldIdentity:
		n.fields.Identity = p
	default:
		panic(fmt.Sprintf("setFieldProvenance: unknown field %q", field))
	}
}
