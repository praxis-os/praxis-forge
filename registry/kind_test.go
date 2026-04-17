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
