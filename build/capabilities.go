// SPDX-License-Identifier: Apache-2.0

package build

import (
	"sort"

	"github.com/praxis-os/praxis-forge/manifest"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

// computeCapabilities builds the manifest.Capabilities summary from the
// resolved components and the normalized spec.
//
// Phase 3: Skill and OutputContract kinds are included. A skill kind
// is "present" when spec.skills[] is non-empty; output_contract is
// "present" when the effective OutputContract (user-declared or
// skill-injected) is set on the expanded spec. Both kinds are "skipped"
// with reason "not_specified" when absent.
func computeCapabilities(s *spec.AgentSpec, res *resolved, expanded *ExpandedSpec) manifest.Capabilities {
	var present []string
	var skipped []manifest.CapabilitySkip

	// Required kinds: always present.
	present = append(present, string(registry.KindProvider))
	present = append(present, string(registry.KindPromptAsset))

	if len(res.toolPackIDs) > 0 {
		present = append(present, string(registry.KindToolPack))
	}
	if len(res.policyHookIDs) > 0 {
		present = append(present, string(registry.KindPolicyPack))
	}
	if len(res.preLLMIDs) > 0 {
		present = append(present, string(registry.KindPreLLMFilter))
	}
	if len(res.preToolIDs) > 0 {
		present = append(present, string(registry.KindPreToolFilter))
	}
	if len(res.postToolIDs) > 0 {
		present = append(present, string(registry.KindPostToolFilter))
	}

	// Singular optional kinds: present when built, skipped when spec field nil.
	if s.Budget != nil {
		present = append(present, string(registry.KindBudgetProfile))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindBudgetProfile), Reason: "not_specified"})
	}
	if s.Telemetry != nil {
		present = append(present, string(registry.KindTelemetryProfile))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindTelemetryProfile), Reason: "not_specified"})
	}
	if s.Credentials != nil {
		present = append(present, string(registry.KindCredentialResolver))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindCredentialResolver), Reason: "not_specified"})
	}
	if s.Identity != nil {
		present = append(present, string(registry.KindIdentitySigner))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindIdentitySigner), Reason: "not_specified"})
	}

	// Phase 3: skills and output contract.
	if len(s.Skills) > 0 {
		present = append(present, string(registry.KindSkill))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindSkill), Reason: "not_specified"})
	}
	// Effective output contract = user-declared OR skill-injected. The
	// expanded spec holds the resolved truth.
	hasContract := expanded != nil && expanded.Spec.OutputContract != nil
	if hasContract {
		present = append(present, string(registry.KindOutputContract))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindOutputContract), Reason: "not_specified"})
	}

	sort.Strings(present)
	return manifest.Capabilities{
		Present: present,
		Skipped: skipped,
	}
}
