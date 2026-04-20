// SPDX-License-Identifier: Apache-2.0

package spec_test

import (
	"encoding/hex"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/praxis-os/praxis-forge/spec"
)

const (
	testAPIVersion   = "forge.praxis-os.dev/v0"
	testProviderRef  = "provider@1.0.0"
	testSystemRef    = "prompt.system@1.0.0"
)

func baseSpec() spec.AgentSpec {
	return spec.AgentSpec{
		APIVersion: testAPIVersion,
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "test.agent", Version: "1.0.0"},
		Provider:   spec.ComponentRef{Ref: testProviderRef},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: testSystemRef}},
	}
}

func mustNormalize(t *testing.T, s *spec.AgentSpec) *spec.NormalizedSpec {
	t.Helper()
	ns, err := spec.Normalize(t.Context(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	return ns
}

// TestCanonicalJSON_MapKeyOrder verifies that maps with different insertion
// orders produce identical canonical JSON.
func TestCanonicalJSON_MapKeyOrder(t *testing.T) {
	t.Parallel()

	makeSpec := func(labels map[string]string) *spec.NormalizedSpec {
		s := baseSpec()
		s.Metadata.Labels = labels
		return mustNormalize(t, &s)
	}

	cases := []struct {
		name string
		a    map[string]string
		b    map[string]string
	}{
		{
			name: "two keys swapped",
			a:    map[string]string{"alpha": "1", "beta": "2"},
			b:    map[string]string{"beta": "2", "alpha": "1"},
		},
		{
			name: "three keys shuffled",
			a:    map[string]string{"a": "x", "b": "y", "c": "z"},
			b:    map[string]string{"c": "z", "a": "x", "b": "y"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			nsA := makeSpec(tc.a)
			nsB := makeSpec(tc.b)
			bytesA, err := nsA.CanonicalJSON()
			if err != nil {
				t.Fatalf("CanonicalJSON(a): %v", err)
			}
			bytesB, err := nsB.CanonicalJSON()
			if err != nil {
				t.Fatalf("CanonicalJSON(b): %v", err)
			}
			if string(bytesA) != string(bytesB) {
				t.Errorf("map order should not affect canonical JSON\n  a: %s\n  b: %s", bytesA, bytesB)
			}
		})
	}
}

// TestCanonicalJSON_ConfigMapOrder verifies stability for ComponentRef.Config maps.
func TestCanonicalJSON_ConfigMapOrder(t *testing.T) {
	t.Parallel()

	makeSpec := func(config map[string]any) *spec.NormalizedSpec {
		s := baseSpec()
		s.Provider.Config = config
		return mustNormalize(t, &s)
	}

	cfgA := map[string]any{"model": "claude-opus", "maxTokens": 4096, "temperature": 0}
	cfgB := map[string]any{"temperature": 0, "model": "claude-opus", "maxTokens": 4096}

	nsA := makeSpec(cfgA)
	nsB := makeSpec(cfgB)

	bytesA, err := nsA.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON(a): %v", err)
	}
	bytesB, err := nsB.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON(b): %v", err)
	}
	if string(bytesA) != string(bytesB) {
		t.Errorf("config map order should not affect canonical JSON\n  a: %s\n  b: %s", bytesA, bytesB)
	}
}

// TestCanonicalJSON_EmptyVsNilCollections verifies that nil and empty
// collections produce the same canonical output (YAML authoring quirk guard).
func TestCanonicalJSON_EmptyVsNilCollections(t *testing.T) {
	t.Parallel()

	withNil := baseSpec()
	withEmpty := baseSpec()
	withEmpty.Tools = []spec.ComponentRef{} // explicit empty vs nil

	nsNil := mustNormalize(t, &withNil)
	nsEmpty := mustNormalize(t, &withEmpty)

	bNil, err := nsNil.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON(nil): %v", err)
	}
	bEmpty, err := nsEmpty.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON(empty): %v", err)
	}
	if string(bNil) != string(bEmpty) {
		t.Errorf("nil vs empty tools should produce identical canonical JSON\n  nil:   %s\n  empty: %s", bNil, bEmpty)
	}
}

// TestCanonicalJSON_Memoized verifies that repeated calls return the same
// backing array (no recomputation).
func TestCanonicalJSON_Memoized(t *testing.T) {
	t.Parallel()

	s := baseSpec()
	ns := mustNormalize(t, &s)

	b1, err := ns.CanonicalJSON()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	b2, err := ns.CanonicalJSON()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if &b1[0] != &b2[0] {
		t.Error("expected memoized result (same pointer)")
	}
}

// TestCanonicalJSON_IsValidJSON verifies the output parses as valid JSON.
func TestCanonicalJSON_IsValidJSON(t *testing.T) {
	t.Parallel()

	s := baseSpec()
	s.Metadata.Labels = map[string]string{"env": "prod", "team": "platform"}
	s.Provider.Config = map[string]any{"model": "claude-opus"}
	s.Tools = []spec.ComponentRef{
		{Ref: "tools.http-get@1.0.0"},
		{Ref: "tools.calculator@1.0.0"},
	}
	ns := mustNormalize(t, &s)

	b, err := ns.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	if !json.Valid(b) {
		t.Errorf("canonical output is not valid JSON: %s", b)
	}
}

// TestNormalizedHash_Format verifies the hash is a 64-char lowercase hex string.
func TestNormalizedHash_Format(t *testing.T) {
	t.Parallel()

	ns := mustNormalize(t, func() *spec.AgentSpec { s := baseSpec(); return &s }())
	h, err := ns.NormalizedHash()
	if err != nil {
		t.Fatalf("NormalizedHash: %v", err)
	}
	if len(h) != 64 {
		t.Errorf("expected 64-char hex, got len=%d: %s", len(h), h)
	}
	// Verify it is valid lowercase hex.
	matched, _ := regexp.MatchString(`^[0-9a-f]{64}$`, h)
	if !matched {
		t.Errorf("hash is not lowercase hex: %s", h)
	}
	// Verify it decodes to 32 bytes (SHA-256).
	decoded, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("hex.DecodeString: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(decoded))
	}
}

// TestNormalizedHash_Stable verifies that the same spec always produces the same hash.
func TestNormalizedHash_Stable(t *testing.T) {
	t.Parallel()

	s := baseSpec()
	s.Metadata.Labels = map[string]string{"env": "prod", "region": "us-east"}

	ns1 := mustNormalize(t, &s)
	ns2 := mustNormalize(t, &s)

	h1, err := ns1.NormalizedHash()
	if err != nil {
		t.Fatalf("hash1: %v", err)
	}
	h2, err := ns2.NormalizedHash()
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	if h1 != h2 {
		t.Errorf("same spec should produce same hash\n  h1: %s\n  h2: %s", h1, h2)
	}
}

// TestNormalizedHash_DiffersOnSpecChange verifies different specs hash differently.
func TestNormalizedHash_DiffersOnSpecChange(t *testing.T) {
	t.Parallel()

	s1 := baseSpec()
	s2 := baseSpec()
	s2.Metadata.Version = "2.0.0"

	ns1 := mustNormalize(t, &s1)
	ns2 := mustNormalize(t, &s2)

	h1, err := ns1.NormalizedHash()
	if err != nil {
		t.Fatalf("hash1: %v", err)
	}
	h2, err := ns2.NormalizedHash()
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	if h1 == h2 {
		t.Errorf("different specs should not produce the same hash")
	}
}

// TestNormalizedHash_MapOrderStable verifies that map key order does not affect hash.
func TestNormalizedHash_MapOrderStable(t *testing.T) {
	t.Parallel()

	sA := baseSpec()
	sA.Metadata.Labels = map[string]string{"alpha": "1", "beta": "2"}
	sB := baseSpec()
	sB.Metadata.Labels = map[string]string{"beta": "2", "alpha": "1"}

	nsA := mustNormalize(t, &sA)
	nsB := mustNormalize(t, &sB)

	hA, err := nsA.NormalizedHash()
	if err != nil {
		t.Fatalf("hashA: %v", err)
	}
	hB, err := nsB.NormalizedHash()
	if err != nil {
		t.Fatalf("hashB: %v", err)
	}
	if hA != hB {
		t.Errorf("map key order should not affect hash\n  hA: %s\n  hB: %s", hA, hB)
	}
}

// TestNormalizedHash_Memoized verifies that repeated calls return the same value.
func TestNormalizedHash_Memoized(t *testing.T) {
	t.Parallel()

	ns := mustNormalize(t, func() *spec.AgentSpec { s := baseSpec(); return &s }())

	h1, _ := ns.NormalizedHash()
	h2, _ := ns.NormalizedHash()
	if h1 != h2 {
		t.Errorf("expected same hash on repeated calls")
	}
}
