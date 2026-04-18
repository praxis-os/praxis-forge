// SPDX-License-Identifier: Apache-2.0

package spec

import "testing"

func TestProvenance_AllTopLevelFieldsRoundTrip(t *testing.T) {
	n := &NormalizedSpec{}
	cases := map[string]Provenance{
		"apiVersion":  {Role: RoleBase},
		"kind":        {Role: RoleBase},
		"metadata":    {Role: RoleBase, File: "base.yaml", Line: 3},
		"provider":    {Role: RoleParent, Step: 1, File: "acme/base.yaml", Line: 7},
		"prompt":      {Role: RoleBase, File: "base.yaml", Line: 9},
		"tools":       {Role: RoleOverlay, Step: 0, File: "overlay-1.yaml", Line: 4},
		"policies":    {Role: RoleParent, Step: 2, File: "acme/grand.yaml", Line: 2},
		"filters":     {Role: RoleOverlay, Step: 1, File: "overlay-2.yaml", Line: 5},
		"budget":      {Role: RoleBase, File: "base.yaml", Line: 14},
		"telemetry":   {Role: RoleBase, File: "base.yaml", Line: 17},
		"credentials": {Role: RoleBase, File: "base.yaml", Line: 19},
		"identity":    {Role: RoleBase, File: "base.yaml", Line: 21},
	}
	for f, p := range cases {
		n.setFieldProvenance(f, p)
	}
	for f, want := range cases {
		got, ok := n.Provenance(f)
		if !ok {
			t.Fatalf("%s: not recognized", f)
		}
		if got != want {
			t.Fatalf("%s: got %+v, want %+v", f, got, want)
		}
	}
}

func TestProvenance_NestedPathReturnsParentFieldProvenance(t *testing.T) {
	n := &NormalizedSpec{}
	n.setFieldProvenance("filters", Provenance{Role: RoleOverlay, Step: 0, File: "ov.yaml", Line: 2})
	got, ok := n.Provenance("filters.preLLM")
	if !ok {
		t.Fatal("filters.preLLM should resolve to filters' provenance")
	}
	if got.Role != RoleOverlay || got.Step != 0 || got.File != "ov.yaml" {
		t.Fatalf("got %+v", got)
	}
}

func TestProvenance_UnknownPathFalse(t *testing.T) {
	n := &NormalizedSpec{}
	if _, ok := n.Provenance("nope"); ok {
		t.Fatal("unknown field should return ok=false")
	}
}

func TestProvenance_DescribeFormat(t *testing.T) {
	cases := []struct {
		p    Provenance
		want string
	}{
		{Provenance{Role: RoleBase}, "base"},
		{Provenance{Role: RoleParent, Step: 2, File: "acme/base.yaml", Line: 7},
			"parent #2 at acme/base.yaml:7"},
		{Provenance{Role: RoleParent, Step: 1}, "parent #1"},
		{Provenance{Role: RoleOverlay, Step: 0, File: "ov.yaml", Line: 4},
			"overlay #0 at ov.yaml:4"},
		{Provenance{Role: RoleOverlay, Step: 1}, "overlay #1"},
	}
	for _, c := range cases {
		if got := c.p.describe(); got != c.want {
			t.Errorf("describe(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}
