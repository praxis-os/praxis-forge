// SPDX-License-Identifier: Apache-2.0

package registry

import "testing"

func TestSkill_ZeroValueAcceptable(t *testing.T) {
	var s Skill
	if s.PromptFragment != "" {
		t.Fatalf("zero Skill should have empty PromptFragment")
	}
	if len(s.RequiredTools) != 0 || len(s.RequiredPolicies) != 0 {
		t.Fatalf("zero Skill should have empty slices")
	}
	if s.RequiredOutputContract != nil {
		t.Errorf("zero RequiredOutputContract: want nil, got %v", s.RequiredOutputContract)
	}
}

func TestSkill_FullyPopulated(t *testing.T) {
	s := Skill{
		PromptFragment: "You are a helpful assistant",
		RequiredTools: []RequiredComponent{
			{ID: "tool.foo@1.0.0", Config: map[string]interface{}{"key": "value"}},
		},
		RequiredPolicies: []RequiredComponent{
			{ID: "policy.bar@1.0.0", Config: nil},
		},
		RequiredOutputContract: &RequiredComponent{
			ID:     "contract.baz@1.0.0",
			Config: map[string]interface{}{"timeout": 30},
		},
		Descriptor: SkillDescriptor{
			Name:    "TestSkill",
			Owner:   "test-owner",
			Summary: "A test skill",
			Tags:    []string{"test", "demo"},
		},
	}
	if s.PromptFragment == "" {
		t.Fatal("PromptFragment should not be empty")
	}
	if len(s.RequiredTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(s.RequiredTools))
	}
	if len(s.RequiredPolicies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(s.RequiredPolicies))
	}
	if s.RequiredOutputContract.ID == "" {
		t.Fatal("RequiredOutputContract.ID should not be empty")
	}
	if s.Descriptor.Name == "" {
		t.Fatal("Descriptor.Name should not be empty")
	}
}

func TestOutputContract_FullyPopulated(t *testing.T) {
	c := OutputContract{
		Schema: map[string]interface{}{
			"type":  "object",
			"props": map[string]interface{}{},
		},
		Descriptor: OutputContractDescriptor{
			Name:    "TestContract",
			Owner:   "test-owner",
			Summary: "A test output contract",
		},
	}
	if len(c.Schema) == 0 {
		t.Fatal("Schema should not be empty")
	}
	if c.Descriptor.Name == "" {
		t.Fatal("Descriptor.Name should not be empty")
	}
}

func TestRequiredComponent_NilConfigAllowed(t *testing.T) {
	rc := RequiredComponent{
		ID:     "component.test@1.0.0",
		Config: nil,
	}
	if rc.ID == "" {
		t.Fatal("ID should not be empty")
	}
	if rc.Config != nil {
		t.Fatal("Config should be nil")
	}
}
