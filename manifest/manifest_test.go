// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestManifest_MarshalStableOrder(t *testing.T) {
	m := Manifest{
		SpecID:      "acme.demo",
		SpecVersion: "0.1.0",
		BuiltAt:     time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		Resolved: []ResolvedComponent{
			{Kind: "provider", ID: "provider.anthropic@1.0.0", Config: map[string]any{"model": "x"}},
			{Kind: "tool_pack", ID: "toolpack.http-get@1.0.0", Config: map[string]any{"timeoutMs": 5000}},
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"specId":"acme.demo"`) {
		t.Fatalf("specId missing: %s", s)
	}
	if strings.Index(s, `"provider.anthropic@1.0.0"`) > strings.Index(s, `"toolpack.http-get@1.0.0"`) {
		t.Fatal("resolved order not preserved")
	}
}

func TestManifest_EmptyChainAndOverlaysOmitted(t *testing.T) {
	m := Manifest{
		SpecID:      "acme.demo",
		SpecVersion: "0.1.0",
		BuiltAt:     time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		Resolved: []ResolvedComponent{
			{Kind: "provider", ID: "provider.fake@1.0.0"},
		},
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "extendsChain") {
		t.Errorf("extendsChain should be omitted when empty: %s", got)
	}
	if strings.Contains(got, "overlays") {
		t.Errorf("overlays should be omitted when empty: %s", got)
	}
}

func TestManifest_PopulatedRoundTrip(t *testing.T) {
	m := Manifest{
		SpecID:      "acme.demo",
		SpecVersion: "0.1.0",
		BuiltAt:     time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		ExtendsChain: []string{
			"acme.grand@1.0.0",
			"acme.parent@1.0.0",
		},
		Overlays: []OverlayAttribution{
			{Name: "prod-override", File: "overlays/prod.yaml"},
			{Name: "ad-hoc"}, // File omitted
		},
		Resolved: []ResolvedComponent{
			{Kind: "provider", ID: "provider.fake@1.0.0"},
		},
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var got Manifest
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.ExtendsChain) != 2 || got.ExtendsChain[1] != "acme.parent@1.0.0" {
		t.Fatalf("ExtendsChain=%+v", got.ExtendsChain)
	}
	if len(got.Overlays) != 2 {
		t.Fatalf("Overlays=%+v", got.Overlays)
	}
	if got.Overlays[1].File != "" {
		t.Errorf("File should round-trip empty (omitempty): %+v", got.Overlays[1])
	}
}
