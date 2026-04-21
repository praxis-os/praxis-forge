// SPDX-License-Identifier: Apache-2.0

// Package build resolves components, composes praxis hooks, and
// materializes the BuiltAgent. This file holds the Phase-3 skill
// expansion stage.
package build

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

// ExpandedSpec is the post-skill-expansion composition. Spec carries
// the effective AgentSpec with Tools / Policies / OutputContract
// rewritten to include skill contributions. Skills carries the raw
// skill ids + configs + resolved values so the manifest can emit them.
// InjectedBy maps "kind:id" entries to the skill id that drove inclusion
// (attribution rules: see design doc §"Manifest additions").
type ExpandedSpec struct {
	Spec       spec.AgentSpec
	Skills     []ResolvedSkill
	InjectedBy map[string]registry.ID
}

// ResolvedSkill records a resolved skill factory's contribution for the
// manifest. Config is the authoring-site config from spec.skills[].config
// (kept verbatim for audit); Value is the built Skill.
type ResolvedSkill struct {
	ID     registry.ID
	Config map[string]any
	Value  registry.Skill
}

// ResolvedOutputContract mirrors ResolvedSkill for the single
// optional output-contract slot.
type ResolvedOutputContract struct {
	ID     registry.ID
	Config map[string]any
	Value  registry.OutputContract
}

// Error codes emitted by expandSkills. Each is wrapped into an error
// string that includes the skill id and the conflicting target.
const (
	errCodeEmptyContribution      = "skill_empty_contribution"
	errCodeUnresolvedRequired     = "skill_unresolved_required_component"
	errCodeVersionDivergence      = "skill_conflict_version_divergence"
	errCodeConfigDivergence       = "skill_conflict_config_divergence"
	errCodeOutputMultiple         = "skill_conflict_output_contract_multiple"
	errCodeOutputUserOverride     = "skill_conflict_output_contract_user_override"
)

// expandSkills resolves every spec.skills[] factory, auto-injects each
// skill's RequiredTools / RequiredPolicies / RequiredOutputContract
// into the effective composition, and returns an ExpandedSpec with the
// rewritten AgentSpec + attribution.
func expandSkills(
	ctx context.Context,
	s *spec.AgentSpec,
	r *registry.ComponentRegistry,
) (*ExpandedSpec, error) {
	// Start with a value-copy of spec; take fresh slices so mutation
	// does not leak back to the caller's AgentSpec.
	out := &ExpandedSpec{
		Spec:       *s,
		InjectedBy: map[string]registry.ID{},
	}
	out.Spec.Tools = append([]spec.ComponentRef(nil), s.Tools...)
	out.Spec.Policies = append([]spec.ComponentRef(nil), s.Policies...)
	if s.OutputContract != nil {
		oc := *s.OutputContract
		out.Spec.OutputContract = &oc
	}

	if len(s.Skills) == 0 {
		return out, nil
	}

	// Resolve each skill factory in declaration order. Build the Skill
	// values first so all validation runs before any injection.
	for i, skillRef := range s.Skills {
		skillID := registry.ID(skillRef.Ref)
		fac, err := r.Skill(skillID)
		if err != nil {
			return nil, fmt.Errorf("resolve skills[%d] %s: %w", i, skillRef.Ref, err)
		}
		skVal, err := fac.Build(ctx, skillRef.Config)
		if err != nil {
			return nil, fmt.Errorf("build skills[%d] %s: %w", i, skillRef.Ref, err)
		}
		if isEmptyContribution(skVal) {
			return nil, fmt.Errorf(
				"skills[%d] %s: %s: skill contributes no prompt fragment, no required components",
				i, skillRef.Ref, errCodeEmptyContribution,
			)
		}
		out.Skills = append(out.Skills, ResolvedSkill{
			ID:     skillID,
			Config: skillRef.Config,
			Value:  skVal,
		})

		// Auto-inject RequiredTools.
		for _, rc := range skVal.RequiredTools {
			if err := injectRequired(&out.Spec.Tools, out.InjectedBy, "tool_pack", skillID, rc); err != nil {
				return nil, err
			}
		}
		// Auto-inject RequiredPolicies.
		for _, rc := range skVal.RequiredPolicies {
			if err := injectRequired(&out.Spec.Policies, out.InjectedBy, "policy_pack", skillID, rc); err != nil {
				return nil, err
			}
		}
		// Auto-inject RequiredOutputContract (singular).
		if skVal.RequiredOutputContract != nil {
			if err := injectOutputContract(out, skillID, *skVal.RequiredOutputContract); err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}

// isEmptyContribution reports whether a Skill has no meaningful output:
// no prompt fragment, no required tools/policies, no output contract.
func isEmptyContribution(sk registry.Skill) bool {
	return sk.PromptFragment == "" &&
		len(sk.RequiredTools) == 0 &&
		len(sk.RequiredPolicies) == 0 &&
		sk.RequiredOutputContract == nil
}

// injectRequired merges one RequiredComponent into the target slice
// with strict conflict detection. kindLabel is the short kind slug used
// in the InjectedBy map key (e.g. "tool_pack", "policy_pack") and in
// error messages.
func injectRequired(
	target *[]spec.ComponentRef,
	injectedBy map[string]registry.ID,
	kindLabel string,
	skillID registry.ID,
	rc registry.RequiredComponent,
) error {
	if rc.ID == "" {
		return fmt.Errorf("%s %s: RequiredComponent missing ID", kindLabel, skillID)
	}
	wantName, wantVersion, err := spec.ParseID(string(rc.ID))
	if err != nil {
		return fmt.Errorf("%s %s: %w", kindLabel, skillID, err)
	}

	for _, existing := range *target {
		gotName, gotVersion, err := spec.ParseID(existing.Ref)
		if err != nil {
			return fmt.Errorf("%s %s: existing ref %q: %w", kindLabel, skillID, existing.Ref, err)
		}
		if gotName != wantName {
			continue
		}
		if gotVersion != wantVersion {
			return fmt.Errorf(
				"%s: skill %s wants %s at version %s but composition already has %s (existing ref %s)",
				errCodeVersionDivergence, skillID, wantName, wantVersion, gotVersion, existing.Ref,
			)
		}
		// Same id. Compare configs.
		eq, err := spec.CanonicalConfigsEqual(existing.Config, rc.Config)
		if err != nil {
			return fmt.Errorf("%s: %w", errCodeConfigDivergence, err)
		}
		if !eq {
			return fmt.Errorf(
				"%s: skill %s wants %s with a different config than the one already in the composition",
				errCodeConfigDivergence, skillID, rc.ID,
			)
		}
		return nil // idempotent — user or earlier skill already declared this; preserve existing attribution.
	}

	// Not present; append and attribute to this skill.
	*target = append(*target, spec.ComponentRef{Ref: string(rc.ID), Config: rc.Config})
	injectedBy[kindLabel+":"+string(rc.ID)] = skillID
	return nil
}

// injectOutputContract applies the singular output contract slot.
func injectOutputContract(out *ExpandedSpec, skillID registry.ID, rc registry.RequiredComponent) error {
	if rc.ID == "" {
		return fmt.Errorf("output_contract skill %s: RequiredOutputContract missing ID", skillID)
	}
	if _, _, err := spec.ParseID(string(rc.ID)); err != nil {
		return fmt.Errorf("output_contract skill %s: %w", skillID, err)
	}

	if out.Spec.OutputContract == nil {
		out.Spec.OutputContract = &spec.ComponentRef{Ref: string(rc.ID), Config: rc.Config}
		out.InjectedBy["output_contract:"+string(rc.ID)] = skillID
		return nil
	}

	// An output contract is already present — from the user or a previous skill.
	if out.Spec.OutputContract.Ref != string(rc.ID) {
		// Tell the author which collision path applied.
		if _, skillInjected := out.InjectedBy["output_contract:"+out.Spec.OutputContract.Ref]; skillInjected {
			return fmt.Errorf(
				"%s: skills require different output contracts (%s vs %s)",
				errCodeOutputMultiple, out.Spec.OutputContract.Ref, rc.ID,
			)
		}
		return fmt.Errorf(
			"%s: skill %s requires output contract %s but user declared %s",
			errCodeOutputUserOverride, skillID, rc.ID, out.Spec.OutputContract.Ref,
		)
	}
	// Same id. Compare configs.
	eq, err := spec.CanonicalConfigsEqual(out.Spec.OutputContract.Config, rc.Config)
	if err != nil {
		return fmt.Errorf("%s: %w", errCodeOutputUserOverride, err)
	}
	if !eq {
		// Disambiguate based on whether the existing entry came from user or another skill.
		if _, skillInjected := out.InjectedBy["output_contract:"+out.Spec.OutputContract.Ref]; skillInjected {
			return fmt.Errorf(
				"%s: two skills require %s with divergent configs",
				errCodeOutputMultiple, rc.ID,
			)
		}
		return fmt.Errorf(
			"%s: skill %s requires %s but user config differs",
			errCodeOutputUserOverride, skillID, rc.ID,
		)
	}
	return nil // idempotent.
}
