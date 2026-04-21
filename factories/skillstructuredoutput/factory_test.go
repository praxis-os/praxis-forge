// SPDX-License-Identifier: Apache-2.0

package skillstructuredoutput

import (
	"context"
	"strings"
	"testing"
)

func TestFactory_ProducesPromptFragment(t *testing.T) {
	sk, err := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if sk.PromptFragment == "" {
		t.Error("PromptFragment should be non-empty")
	}
	if !strings.Contains(sk.PromptFragment, "JSON") {
		t.Errorf("PromptFragment should mention JSON; got %q", sk.PromptFragment)
	}
}

func TestFactory_RequiresPIIRedactionPolicy(t *testing.T) {
	sk, err := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(sk.RequiredPolicies) != 1 {
		t.Fatalf("want 1 policy, got %d", len(sk.RequiredPolicies))
	}
	if sk.RequiredPolicies[0].ID != "policypack.pii-redaction@1.0.0" {
		t.Errorf("policy id: %s", sk.RequiredPolicies[0].ID)
	}
}

func TestFactory_RequiresOutputContract(t *testing.T) {
	sk, err := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if sk.RequiredOutputContract == nil {
		t.Fatal("RequiredOutputContract should be non-nil")
	}
	if sk.RequiredOutputContract.ID != "outputcontract.json-schema@1.0.0" {
		t.Errorf("contract id: %s", sk.RequiredOutputContract.ID)
	}
}

func TestFactory_AcceptsEmptyConfig(t *testing.T) {
	_, err := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("nil cfg should work: %v", err)
	}
	_, err = NewFactory("skill.structured-output@1.0.0").Build(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("empty cfg should work: %v", err)
	}
}

func TestFactory_Descriptor(t *testing.T) {
	sk, _ := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if sk.Descriptor.Name != "structured-output" {
		t.Errorf("Descriptor.Name: %s", sk.Descriptor.Name)
	}
	if sk.Descriptor.Owner != "core" {
		t.Errorf("Descriptor.Owner: %s", sk.Descriptor.Owner)
	}
}
