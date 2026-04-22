// SPDX-License-Identifier: Apache-2.0

package registry

import "testing"

func TestKindString(t *testing.T) {
	if string(KindProvider) != "provider" {
		t.Fatalf("KindProvider=%q", KindProvider)
	}
}

func TestParseID_PropagatesSpecRules(t *testing.T) {
	if _, err := ParseID("bad"); err == nil {
		t.Fatal("expected error")
	}
	id, err := ParseID("provider.foo@1.0.0")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if id != "provider.foo@1.0.0" {
		t.Fatalf("id=%s", id)
	}
}

func TestKind_Phase3ActiveKinds(t *testing.T) {
	if string(KindSkill) != "skill" {
		t.Fatalf("KindSkill=%q, expected \"skill\"", KindSkill)
	}
	if string(KindOutputContract) != "output_contract" {
		t.Fatalf("KindOutputContract=%q, expected \"output_contract\"", KindOutputContract)
	}
}

func TestKind_Phase4ActiveKinds(t *testing.T) {
	if string(KindMCPBinding) != "mcp_binding" {
		t.Fatalf("KindMCPBinding=%q, expected \"mcp_binding\"", KindMCPBinding)
	}
}
