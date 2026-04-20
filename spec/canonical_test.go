// SPDX-License-Identifier: Apache-2.0

package spec_test

import (
	"encoding/json"
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
