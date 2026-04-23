// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// MaxExtendsDepth bounds how deep an extends chain may go before
// resolveExtendsChain returns ErrExtendsInvalid (Reason: "depth").
// Picked at design time; tune in a later phase if real specs hit it.
const MaxExtendsDepth = 8

// ExtendsError carries the chain that triggered an extends violation
// so callers can log the exact path. Reason is one of "cycle" or
// "depth".
//
// Wraps ErrExtendsInvalid: callers may match on the sentinel via
// errors.Is, then type-assert to *ExtendsError to inspect Chain and
// Reason.
type ExtendsError struct {
	// Chain lists refs in resolution order, starting from the spec
	// that triggered the failure. For cycles it ends with the ref
	// that closed the loop (also present earlier in the slice).
	Chain []string
	// Reason is "cycle" or "depth".
	Reason string
}

func (e *ExtendsError) Error() string {
	return fmt.Sprintf("extends: %s detected through chain [%s]",
		e.Reason, strings.Join(e.Chain, " -> "))
}

// Unwrap exposes ErrExtendsInvalid so errors.Is(err, ErrExtendsInvalid)
// holds for any *ExtendsError.
func (e *ExtendsError) Unwrap() error { return ErrExtendsInvalid }

// Is matches ErrExtendsInvalid (in addition to the default identity
// match against the same *ExtendsError pointer).
func (e *ExtendsError) Is(target error) bool {
	return target == ErrExtendsInvalid
}

// resolveExtendsChain walks s.Extends and every parent's Extends in
// turn, accumulating loaded parents in root-first order. Each parent's
// own extends is resolved depth-first before the parent itself is
// added to the chain.
//
// Limits:
//   - max depth MaxExtendsDepth (8); deeper → *ExtendsError, Reason "depth"
//   - cycle detected via visited set;       → *ExtendsError, Reason "cycle"
//   - parent missing from store              → wrapped ErrSpecNotFound
//
// Context cancellation aborts the walk and returns ctx.Err().
//
// The returned slice is root-first: parents[0] is the deepest ancestor;
// parents[len-1] is the direct parent of s. The merge step in
// mergeChain consumes them in this order with child-wins semantics.
//
// Returned []string is the resolved chain in root-first order; used as
// NormalizedSpec.ExtendsChain by Normalize.
func resolveExtendsChain(
	ctx context.Context,
	s *AgentSpec,
	store SpecStore,
) (parents []*AgentSpec, chain []string, err error) {
	if len(s.Extends) == 0 {
		return nil, nil, nil
	}
	if store == nil {
		return nil, nil, ErrNoSpecStore
	}

	visited := map[string]bool{}
	var walk func(node *AgentSpec, _ string, depth int, path []string) error
	walk = func(node *AgentSpec, _ string, depth int, path []string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > MaxExtendsDepth {
			return &ExtendsError{Chain: append([]string(nil), path...), Reason: "depth"}
		}
		for _, parentRef := range node.Extends {
			cycledPath := append(path, parentRef) //nolint:gocritic // each iteration builds its own slice for the error path
			if visited[parentRef] {
				return &ExtendsError{Chain: append([]string(nil), cycledPath...), Reason: "cycle"}
			}
			visited[parentRef] = true

			parent, err := store.Load(ctx, parentRef)
			if err != nil {
				if errors.Is(err, ErrSpecNotFound) {
					return fmt.Errorf("resolve %q: %w", parentRef, err)
				}
				return fmt.Errorf("resolve %q: %w", parentRef, err)
			}
			if err := walk(parent, parentRef, depth+1, cycledPath); err != nil {
				return err
			}
			parents = append(parents, parent)
			chain = append(chain, parentRef)
		}
		return nil
	}

	if err := walk(s, "", 0, nil); err != nil {
		return nil, nil, err
	}
	return parents, chain, nil
}

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
		APIVersion:    Provenance{Role: RoleBase},
		Kind:          Provenance{Role: RoleBase},
		Metadata:      Provenance{Role: RoleBase},
		Provider:      Provenance{Role: RoleBase},
		Prompt:        Provenance{Role: RoleBase},
		Tools:         Provenance{Role: RoleBase},
		Policies:      Provenance{Role: RoleBase},
		Filters:       Provenance{Role: RoleBase},
		Budget:        Provenance{Role: RoleBase},
		Telemetry:     Provenance{Role: RoleBase},
		Credentials:   Provenance{Role: RoleBase},
		Identity:      Provenance{Role: RoleBase},
		Skills:        Provenance{Role: RoleBase},
		OutputContract: Provenance{Role: RoleBase},
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

	// Phase 3: Skills and OutputContract; Phase 4: MCPImports. All propagate child-wins per replaceable-list semantics.
	if child.Skills != nil {
		merged.Skills = child.Skills
		prov.Skills = p
	}
	if child.OutputContract != nil {
		merged.OutputContract = child.OutputContract
		prov.OutputContract = p
	}
	if child.MCPImports != nil {
		merged.MCPImports = child.MCPImports
		prov.MCPImports = p
	}
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
	if body.Tools.Set {
		merged.Tools = body.Tools.Items
		// Capture the overlay's source line on the field provenance.
		stepProv := p
		stepProv.Line = body.Tools.Line
		prov.Tools = stepProv
	}
	if body.Policies.Set {
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
	modified := false
	if ov.PreLLM.Set {
		merged.Filters.PreLLM = ov.PreLLM.Items
		modified = true
	}
	if ov.PreTool.Set {
		merged.Filters.PreTool = ov.PreTool.Items
		modified = true
	}
	if ov.PostTool.Set {
		merged.Filters.PostTool = ov.PostTool.Items
		modified = true
	}
	if modified {
		prov.Filters = p
	}
}

// Normalize merges the resolved extends chain and overlays into a single
// canonical AgentSpec, updating provenance for each field. Returns
// *NormalizedSpec on success; any error (cycle, depth, missing parent,
// locked violations, composition limit) returns nil with the error set.
//
// All three steps (resolve → merge → apply → validate → scan) are
// composed here and executed in sequence.
func Normalize(
	ctx context.Context,
	s *AgentSpec,
	overlays []*AgentOverlay,
	store SpecStore,
) (*NormalizedSpec, error) {
	// Resolve extends chain.
	parents, chain, err := resolveExtendsChain(ctx, s, store)
	if err != nil {
		return nil, err
	}

	// Create the normalized result container.
	result := &NormalizedSpec{
		ExtendsChain: chain,
	}

	// Merge: fold parents (root-first) into base with child-wins semantics.
	result.Spec = *mergeChain(parents, s, &result.fields)

	// Apply overlays: fold each overlay onto the merged result.
	if err := applyOverlays(&result.Spec, overlays, &result.fields); err != nil {
		return nil, err
	}

	// Validate the merged+overlaid spec against Phase 1 invariants.
	if err := result.Spec.Validate(); err != nil {
		return nil, err
	}

	// Scan for locked-field drift in overlays.
	var errs Errors
	scanLockedDrift(s, overlays, &errs)
	if err := errs.OrNil(); err != nil {
		return nil, err
	}

	// Record overlay attribution.
	for _, ov := range overlays {
		if ov == nil {
			continue
		}
		result.Overlays = append(result.Overlays, OverlayAttribution{
			Name: ov.Metadata.Name,
			File: ov.File,
		})
	}

	return result, nil
}
