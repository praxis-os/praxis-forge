// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"testing"
)

// --- Test factories ---

type testSkillFactory struct{ id ID }

func (f testSkillFactory) ID() ID              { return f.id }
func (f testSkillFactory) Description() string { return "test skill factory" }
func (f testSkillFactory) Build(context.Context, map[string]any) (Skill, error) {
	return Skill{}, nil
}

type testOutputContractFactory struct{ id ID }

func (f testOutputContractFactory) ID() ID              { return f.id }
func (f testOutputContractFactory) Description() string { return "test output contract factory" }
func (f testOutputContractFactory) Build(context.Context, map[string]any) (OutputContract, error) {
	return OutputContract{}, nil
}

// --- Compile-time assertions ---

var _ SkillFactory = testSkillFactory{}
var _ OutputContractFactory = testOutputContractFactory{}

// --- Tests ---

func TestSkillFactory_BuildReturnsSkill(t *testing.T) {
	f := testSkillFactory{id: "skill.test@1.0.0"}
	skill, err := f.Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := any(skill).(Skill); !ok {
		t.Fatalf("Build did not return Skill")
	}
}

func TestOutputContractFactory_BuildReturnsContract(t *testing.T) {
	f := testOutputContractFactory{id: "contract.test@1.0.0"}
	contract, err := f.Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := any(contract).(OutputContract); !ok {
		t.Fatalf("Build did not return OutputContract")
	}
}
