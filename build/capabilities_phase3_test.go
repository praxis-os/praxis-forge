// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"sort"
	"testing"

	"github.com/praxis-os/praxis-forge/manifest"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

func TestCapabilities_PresentIncludesSkillAndContract(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{{Ref: "skill.a@1.0.0"}}
	s.OutputContract = &spec.ComponentRef{Ref: "outputcontract.json@1.0.0"}

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterSkill(fakeSkill{id: "skill.a@1.0.0", s: registry.Skill{PromptFragment: "x"}})
	_ = r.RegisterOutputContract(fakeOutputContract{
		id: "outputcontract.json@1.0.0",
		oc: registry.OutputContract{Schema: map[string]any{"type": "object"}},
	})

	ns, _ := spec.Normalize(context.Background(), s, nil, nil)
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	present := built.Manifest.Capabilities.Present
	sort.Strings(present)
	// Present should include both.
	if !contains(present, "skill") {
		t.Errorf("Present missing skill: %v", present)
	}
	if !contains(present, "output_contract") {
		t.Errorf("Present missing output_contract: %v", present)
	}
}

func TestCapabilities_SkippedWhenAbsent(t *testing.T) {
	s := baseSpec()

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})

	ns, _ := spec.Normalize(context.Background(), s, nil, nil)
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	skipped := built.Manifest.Capabilities.Skipped

	var gotSkill, gotContract bool
	for _, s := range skipped {
		if s.Kind == "skill" && s.Reason == "not_specified" {
			gotSkill = true
		}
		if s.Kind == "output_contract" && s.Reason == "not_specified" {
			gotContract = true
		}
	}
	if !gotSkill {
		t.Errorf("Skipped missing skill/not_specified: %+v", skipped)
	}
	if !gotContract {
		t.Errorf("Skipped missing output_contract/not_specified: %+v", skipped)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

var _ manifest.Manifest // keep import live for future expansion
