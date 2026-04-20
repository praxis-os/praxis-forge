// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"testing"

	"github.com/praxis-os/praxis-forge/manifest"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

var hexRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// newMinRegistry builds a registry with provider, prompt, and budget factories
// matching the IDs used in minSpec and buildMinimal tests.
func newMinRegistry(t *testing.T) *registry.ComponentRegistry {
	t.Helper()
	r := registry.NewComponentRegistry()
	if err := r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"}); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}
	if err := r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"}); err != nil {
		t.Fatalf("RegisterPromptAsset: %v", err)
	}
	if err := r.RegisterBudgetProfile(minBudgetFac{id: "budgetprofile.default@1.0.0"}); err != nil {
		t.Fatalf("RegisterBudgetProfile: %v", err)
	}
	return r
}

// buildMinimal builds the minimal spec used across Phase 2b hash tests.
// No budget/telemetry/credentials/identity unless overridden.
func buildMinimal(t *testing.T, s *spec.AgentSpec) *BuiltAgent {
	t.Helper()
	r := newMinRegistry(t)
	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return built
}

// TestBuild_NormalizedHashFormat verifies the hash is a 64-char lowercase hex string.
func TestBuild_NormalizedHashFormat(t *testing.T) {
	s := minSpec()
	built := buildMinimal(t, s)
	h := built.Manifest.NormalizedHash
	if !hexRE.MatchString(h) {
		t.Errorf("NormalizedHash not 64-char lowercase hex: %q", h)
	}
}

// TestBuild_NormalizedHashStable verifies the same spec produces the same hash
// across two independent Build calls.
func TestBuild_NormalizedHashStable(t *testing.T) {
	s1 := minSpec()
	s2 := minSpec()

	built1 := buildMinimal(t, s1)
	built2 := buildMinimal(t, s2)

	h1 := built1.Manifest.NormalizedHash
	h2 := built2.Manifest.NormalizedHash
	if h1 != h2 {
		t.Errorf("hash not stable across builds:\n  build1: %s\n  build2: %s", h1, h2)
	}
}

// TestBuild_NormalizedHashChangesOnSpecChange verifies different specs hash differently.
func TestBuild_NormalizedHashChangesOnSpecChange(t *testing.T) {
	s1 := minSpec()
	s2 := minSpec()
	s2.Metadata.Version = "9.9.9"

	built1 := buildMinimal(t, s1)
	built2 := buildMinimal(t, s2)

	if built1.Manifest.NormalizedHash == built2.Manifest.NormalizedHash {
		t.Error("different specs should produce different hashes")
	}
}

// TestBuild_CapabilitiesMinimalSpec verifies Capabilities for a spec with no
// optional kinds set (budget/telemetry/credentials/identity all nil).
func TestBuild_CapabilitiesMinimalSpec(t *testing.T) {
	s := &spec.AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "test.agent", Version: "1.0.0"},
		Provider:   spec.ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: "prompt.sys@1.0.0"}},
		// No Budget, Telemetry, Credentials, Identity
	}
	built := buildMinimal(t, s)
	caps := built.Manifest.Capabilities

	// Present must be sorted.
	if !sort.StringsAreSorted(caps.Present) {
		t.Errorf("Capabilities.Present not sorted: %v", caps.Present)
	}
	// Must contain provider and prompt_asset.
	if !containsKind(caps.Present, "provider") {
		t.Errorf("provider missing from Present: %v", caps.Present)
	}
	if !containsKind(caps.Present, "prompt_asset") {
		t.Errorf("prompt_asset missing from Present: %v", caps.Present)
	}

	// All four optional kinds must be skipped.
	skippedKinds := make(map[string]string)
	for _, sk := range caps.Skipped {
		skippedKinds[sk.Kind] = sk.Reason
	}
	for _, optional := range []string{"budget_profile", "telemetry_profile", "credential_resolver", "identity_signer"} {
		reason, ok := skippedKinds[optional]
		if !ok {
			t.Errorf("expected %q in Skipped", optional)
			continue
		}
		if reason != "not_specified" {
			t.Errorf("Skipped[%q].Reason = %q, want %q", optional, reason, "not_specified")
		}
	}
}

// TestBuild_CapabilitiesWithBudget verifies that budget_profile moves from
// Skipped to Present when the spec has a Budget field.
func TestBuild_CapabilitiesWithBudget(t *testing.T) {
	s := minSpec() // includes Budget
	built := buildMinimal(t, s)
	caps := built.Manifest.Capabilities

	if !containsKind(caps.Present, "budget_profile") {
		t.Errorf("budget_profile should be in Present when spec has Budget: %v", caps.Present)
	}
	for _, sk := range caps.Skipped {
		if sk.Kind == "budget_profile" {
			t.Errorf("budget_profile should not be in Skipped when spec has Budget")
		}
	}
}

// TestBuild_ManifestJSONRoundTrip verifies the manifest with new fields
// survives a json.Marshal → json.Unmarshal round-trip.
func TestBuild_ManifestJSONRoundTrip(t *testing.T) {
	built := buildMinimal(t, minSpec())
	m := built.Manifest

	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got manifest.Manifest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.NormalizedHash != m.NormalizedHash {
		t.Errorf("NormalizedHash: want %q, got %q", m.NormalizedHash, got.NormalizedHash)
	}
	if len(got.Capabilities.Present) != len(m.Capabilities.Present) {
		t.Errorf("Capabilities.Present len: want %d, got %d", len(m.Capabilities.Present), len(got.Capabilities.Present))
	}
}

// minSpec returns the minimal valid AgentSpec for integration tests (with budget,
// matching TestBuild_MinimalSpec conventions).
func minSpec() *spec.AgentSpec {
	return &spec.AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "a.b", Version: "0.1.0"},
		Provider:   spec.ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: "prompt.sys@1.0.0"}},
		Budget:     &spec.BudgetRef{Ref: "budgetprofile.default@1.0.0"},
	}
}

func containsKind(kinds []string, k string) bool {
	for _, v := range kinds {
		if v == k {
			return true
		}
	}
	return false
}
