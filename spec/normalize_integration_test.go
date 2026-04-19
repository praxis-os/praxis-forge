// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestNormalize_NoExtendsNoOverlays tests a simple spec with no extends or overlays.
func TestNormalize_NoExtendsNoOverlays(t *testing.T) {
	runFixtureTest(t, "no-extends-no-overlays")
}

// TestNormalize_LinearChain tests a simple linear extends chain (child → parent).
func TestNormalize_LinearChain(t *testing.T) {
	runFixtureTest(t, "linear-extend")
}

// TestNormalize_SingleOverlay tests applying a single overlay that replaces tools.
func TestNormalize_SingleOverlay(t *testing.T) {
	runFixtureTest(t, "single-overlay")
}

// TestNormalize_OverlayClearTools tests clearing a field via explicit null overlay.
func TestNormalize_OverlayClearTools(t *testing.T) {
	runFixtureTest(t, "overlay-clear-tools")
}

// TestNormalize_LockedFieldViolation tests that overlays cannot change locked fields.
func TestNormalize_LockedFieldViolation(t *testing.T) {
	runFixtureTest(t, "locked-field-violation")
}

// fixtureResult matches the JSON structure in want.json files.
type fixtureResult struct {
	Spec         *AgentSpec           `json:"spec"`
	ExtendsChain []string             `json:"extendsChain"`
	Overlays     []OverlayAttribution `json:"overlays"`
}

// runFixtureTest loads a test scenario from testdata/normalize/{scenario}/,
// executes Normalize, and compares the result against want.json or want.err.
func runFixtureTest(t *testing.T, scenario string) {
	baseSpec := loadFixtureBaseSpec(t, scenario)
	store := loadFixtureParentSpecs(t, scenario)
	overlays := loadFixtureOverlays(t, scenario)

	ctx := context.Background()
	result, err := Normalize(ctx, baseSpec, overlays, store)

	checkFixtureResult(t, scenario, result, err)
}

func loadFixtureBaseSpec(t *testing.T, scenario string) *AgentSpec {
	basePath := filepath.Join("testdata", "normalize", scenario, "base.yaml")
	baseSpec, err := LoadSpec(basePath)
	if err != nil {
		t.Fatalf("failed to load base spec: %v", err)
	}
	return baseSpec
}

func loadFixtureParentSpecs(t *testing.T, scenario string) MapSpecStore {
	store := MapSpecStore{}
	for i := 1; i <= 10; i++ {
		parentPath := filepath.Join("testdata", "normalize", scenario, "parent-"+itoa(i)+".yaml")
		if _, err := os.Stat(parentPath); err == nil {
			parentSpec, err := LoadSpec(parentPath)
			if err != nil {
				t.Fatalf("failed to load parent spec %d: %v", i, err)
			}
			store[parentSpec.Metadata.ID] = parentSpec
		}
	}
	return store
}

func loadFixtureOverlays(t *testing.T, scenario string) []*AgentOverlay {
	var overlays []*AgentOverlay
	for i := 1; i <= 10; i++ {
		overlayPath := filepath.Join("testdata", "normalize", scenario, "overlay-"+itoa(i)+".yaml")
		if _, err := os.Stat(overlayPath); err == nil {
			overlay, err := LoadOverlay(overlayPath)
			if err != nil {
				t.Fatalf("failed to load overlay %d: %v", i, err)
			}
			overlays = append(overlays, overlay)
		}
	}
	return overlays
}

func checkFixtureResult(t *testing.T, scenario string, result *NormalizedSpec, err error) {
	errPath := filepath.Join("testdata", "normalize", scenario, "want.err")
	if _, errStat := os.Stat(errPath); errStat == nil {
		checkFixtureError(t, scenario, result, err)
		return
	}
	checkFixtureSuccess(t, scenario, result, err)
}

func checkFixtureError(t *testing.T, scenario string, result *NormalizedSpec, err error) {
	errPath := filepath.Join("testdata", "normalize", scenario, "want.err")
	errContent, errRead := os.ReadFile(errPath)
	if errRead != nil {
		t.Fatalf("failed to read want.err: %v", errRead)
	}
	wantErr := string(errContent)
	if result != nil {
		t.Fatalf("expected error, got success: %+v", result)
	}
	if wantErr[len(wantErr)-1] == '\n' {
		wantErr = wantErr[:len(wantErr)-1]
	}
	actualErr := ""
	if err != nil {
		actualErr = err.Error()
	}
	if actualErr != wantErr {
		t.Fatalf("error mismatch:\nwant: %q\ngot:  %q", wantErr, actualErr)
	}
}

func checkFixtureSuccess(t *testing.T, scenario string, result *NormalizedSpec, err error) {
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result")
	}

	wantPath := filepath.Join("testdata", "normalize", scenario, "want.json")
	wantData, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("failed to read want.json: %v", err)
	}

	var want fixtureResult
	if err := json.Unmarshal(wantData, &want); err != nil {
		t.Fatalf("failed to parse want.json: %v", err)
	}

	compareSpec(t, want.Spec, &result.Spec)

	if !slicesEqual(want.ExtendsChain, result.ExtendsChain) {
		t.Errorf("extendsChain mismatch:\nwant: %v\ngot:  %v", want.ExtendsChain, result.ExtendsChain)
	}

	if len(want.Overlays) != len(result.Overlays) {
		t.Errorf("overlay count mismatch: want %d, got %d", len(want.Overlays), len(result.Overlays))
	}
	for i, wantOv := range want.Overlays {
		if i >= len(result.Overlays) {
			break
		}
		if wantOv.Name != result.Overlays[i].Name {
			t.Errorf("overlay[%d] name mismatch: want %q, got %q", i, wantOv.Name, result.Overlays[i].Name)
		}
	}
}

// compareSpec recursively compares two AgentSpec values.
func compareSpec(t *testing.T, want, got *AgentSpec) {
	if want.APIVersion != got.APIVersion {
		t.Errorf("apiVersion: want %q, got %q", want.APIVersion, got.APIVersion)
	}
	if want.Kind != got.Kind {
		t.Errorf("kind: want %q, got %q", want.Kind, got.Kind)
	}
	if want.Metadata.ID != got.Metadata.ID {
		t.Errorf("metadata.id: want %q, got %q", want.Metadata.ID, got.Metadata.ID)
	}
	if want.Metadata.Version != got.Metadata.Version {
		t.Errorf("metadata.version: want %q, got %q", want.Metadata.Version, got.Metadata.Version)
	}
	if want.Provider.Ref != got.Provider.Ref {
		t.Errorf("provider.ref: want %q, got %q", want.Provider.Ref, got.Provider.Ref)
	}
	if !promptBlockEqual(want.Prompt, got.Prompt) {
		t.Errorf("prompt mismatch: want %+v, got %+v", want.Prompt, got.Prompt)
	}
	if !componentRefsEqual(want.Tools, got.Tools) {
		t.Errorf("tools mismatch: want %+v, got %+v", want.Tools, got.Tools)
	}
	if !componentRefsEqual(want.Policies, got.Policies) {
		t.Errorf("policies mismatch: want %+v, got %+v", want.Policies, got.Policies)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func componentRefsEqual(a, b []ComponentRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Ref != b[i].Ref {
			return false
		}
	}
	return true
}

func promptBlockEqual(a, b PromptBlock) bool {
	if (a.System == nil) != (b.System == nil) {
		return false
	}
	if a.System != nil && b.System != nil && a.System.Ref != b.System.Ref {
		return false
	}
	if (a.User == nil) != (b.User == nil) {
		return false
	}
	if a.User != nil && b.User != nil && a.User.Ref != b.User.Ref {
		return false
	}
	return true
}
