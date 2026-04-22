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

func TestManifest_NormalizedHashAndCapabilities_RoundTrip(t *testing.T) {
	m := Manifest{
		SpecID:         "acme.demo",
		SpecVersion:    "0.1.0",
		BuiltAt:        time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		NormalizedHash: "84eb0a2e820c7a9676ead6611dde13493a0806d7f2def0594501441a4a3a5e8c",
		Capabilities: Capabilities{
			Present: []string{"prompt_asset", "provider"},
			Skipped: []CapabilitySkip{
				{Kind: "budget_profile", Reason: "not_specified"},
				{Kind: "credential_resolver", Reason: "not_specified"},
			},
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
	if got.NormalizedHash != m.NormalizedHash {
		t.Errorf("NormalizedHash: want %q, got %q", m.NormalizedHash, got.NormalizedHash)
	}
	if len(got.Capabilities.Present) != 2 || got.Capabilities.Present[0] != "prompt_asset" {
		t.Errorf("Capabilities.Present: %+v", got.Capabilities.Present)
	}
	if len(got.Capabilities.Skipped) != 2 || got.Capabilities.Skipped[0].Kind != "budget_profile" {
		t.Errorf("Capabilities.Skipped: %+v", got.Capabilities.Skipped)
	}
}

func TestManifest_CapabilitiesSkippedOmittedWhenEmpty(t *testing.T) {
	m := Manifest{
		SpecID:      "acme.demo",
		SpecVersion: "0.1.0",
		BuiltAt:     time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		Capabilities: Capabilities{
			Present: []string{"prompt_asset", "provider"},
		},
		Resolved: []ResolvedComponent{{Kind: "provider", ID: "provider.fake@1.0.0"}},
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"skipped"`) {
		t.Errorf("skipped should be omitted when empty: %s", out)
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

func TestManifest_ExpandedHashRoundTrip(t *testing.T) {
	m := Manifest{
		SpecID:         "acme.demo",
		SpecVersion:    "0.1.0",
		BuiltAt:        time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		NormalizedHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpandedHash:   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Resolved: []ResolvedComponent{
			{Kind: "provider", ID: "provider.fake@1.0.0"},
		},
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"expandedHash":"bbbb`) {
		t.Errorf("expandedHash not present in JSON: %s", out)
	}

	var got Manifest
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.ExpandedHash != m.ExpandedHash {
		t.Errorf("ExpandedHash: want %q, got %q", m.ExpandedHash, got.ExpandedHash)
	}
}

func TestManifest_ExpandedHashOmittedWhenEmpty(t *testing.T) {
	m := Manifest{
		SpecID:         "acme.demo",
		SpecVersion:    "0.1.0",
		BuiltAt:        time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		NormalizedHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		// ExpandedHash intentionally empty
		Resolved: []ResolvedComponent{{Kind: "provider", ID: "provider.fake@1.0.0"}},
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "expandedHash") {
		t.Errorf("expandedHash should be omitted when empty: %s", out)
	}
}

func TestResolvedComponent_InjectedBySkillRoundTrip(t *testing.T) {
	rc := ResolvedComponent{
		Kind:            "tool_pack",
		ID:              "toolpack.http-get@1.0.0",
		InjectedBySkill: "skill.structured-output@1.0.0",
	}
	out, err := json.Marshal(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"injectedBySkill":"skill.structured-output@1.0.0"`) {
		t.Errorf("injectedBySkill missing: %s", out)
	}

	var got ResolvedComponent
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.InjectedBySkill != rc.InjectedBySkill {
		t.Errorf("InjectedBySkill: want %q, got %q", rc.InjectedBySkill, got.InjectedBySkill)
	}
}

func TestResolvedComponent_InjectedBySkillOmittedWhenEmpty(t *testing.T) {
	rc := ResolvedComponent{
		Kind: "provider",
		ID:   "provider.fake@1.0.0",
	}
	out, err := json.Marshal(rc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "injectedBySkill") {
		t.Errorf("injectedBySkill should be omitted when empty: %s", out)
	}
}
