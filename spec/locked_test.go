// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"errors"
	"testing"
)

func TestScanLockedDrift_NoOverlays(t *testing.T) {
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Metadata:   Metadata{ID: "my.agent", Version: "1.0.0"},
	}
	var errs Errors
	scanLockedDrift(base, nil, &errs)
	if errs.Len() != 0 {
		t.Fatalf("expected no errors, got %d", errs.Len())
	}
}

func TestScanLockedDrift_OverlayMatchesBase(t *testing.T) {
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Metadata:   Metadata{ID: "my.agent", Version: "1.0.0"},
	}
	overlay := &AgentOverlay{
		APIVersion: expectedAPIVersion,
		Metadata:   OverlayMeta{Name: "test-overlay"},
		Spec: AgentOverlayBody{
			Metadata: &OverlayMetadata{
				ID:      "my.agent",
				Version: "1.0.0",
			},
		},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if errs.Len() != 0 {
		t.Fatalf("expected no errors, got %d", errs.Len())
	}
}

func TestScanLockedDrift_OverlayChangesID(t *testing.T) {
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Metadata:   Metadata{ID: "my.agent", Version: "1.0.0"},
	}
	overlay := &AgentOverlay{
		APIVersion: expectedAPIVersion,
		Metadata:   OverlayMeta{Name: "bad-overlay"},
		Spec: AgentOverlayBody{
			Metadata: &OverlayMetadata{
				ID: "different.agent",
			},
		},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if errs.Len() != 1 {
		t.Fatalf("expected 1 error, got %d", errs.Len())
	}
	if !errors.Is(errs.OrNil(), ErrLockedFieldOverride) {
		t.Fatalf("expected ErrLockedFieldOverride")
	}
}

func TestScanLockedDrift_OverlayChangesVersion(t *testing.T) {
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Metadata:   Metadata{ID: "my.agent", Version: "1.0.0"},
	}
	overlay := &AgentOverlay{
		APIVersion: expectedAPIVersion,
		Metadata:   OverlayMeta{Name: "bad-overlay"},
		Spec: AgentOverlayBody{
			Metadata: &OverlayMetadata{
				Version: "2.0.0",
			},
		},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if errs.Len() != 1 {
		t.Fatalf("expected 1 error, got %d", errs.Len())
	}
	if !errors.Is(errs.OrNil(), ErrLockedFieldOverride) {
		t.Fatalf("expected ErrLockedFieldOverride")
	}
}

func TestScanLockedDrift_OverlayChangesAPIVersion(t *testing.T) {
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Metadata:   Metadata{ID: "my.agent", Version: "1.0.0"},
	}
	overlay := &AgentOverlay{
		APIVersion: "different.version/v1",
		Metadata:   OverlayMeta{Name: "bad-overlay"},
		Spec:       AgentOverlayBody{},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if errs.Len() != 1 {
		t.Fatalf("expected 1 error, got %d", errs.Len())
	}
	if !errors.Is(errs.OrNil(), ErrLockedFieldOverride) {
		t.Fatalf("expected ErrLockedFieldOverride")
	}
}

func TestScanLockedDrift_MultipleOverlayViolations(t *testing.T) {
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Metadata:   Metadata{ID: "my.agent", Version: "1.0.0"},
	}
	overlays := []*AgentOverlay{
		{
			APIVersion: expectedAPIVersion,
			Metadata:   OverlayMeta{Name: "overlay1"},
			Spec: AgentOverlayBody{
				Metadata: &OverlayMetadata{ID: "different.agent"},
			},
		},
		{
			APIVersion: expectedAPIVersion,
			Metadata:   OverlayMeta{Name: "overlay2"},
			Spec: AgentOverlayBody{
				Metadata: &OverlayMetadata{Version: "2.0.0"},
			},
		},
	}
	var errs Errors
	scanLockedDrift(base, overlays, &errs)
	if errs.Len() != 2 {
		t.Fatalf("expected 2 errors, got %d", errs.Len())
	}
}

func TestScanLockedDrift_PartialMetadataOverlay(t *testing.T) {
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Metadata:   Metadata{ID: "my.agent", Version: "1.0.0"},
	}
	// Overlay sets only DisplayName, not ID or Version.
	overlay := &AgentOverlay{
		APIVersion: expectedAPIVersion,
		Metadata:   OverlayMeta{Name: "overlay"},
		Spec: AgentOverlayBody{
			Metadata: &OverlayMetadata{
				DisplayName: "New Display Name",
			},
		},
	}
	var errs Errors
	scanLockedDrift(base, []*AgentOverlay{overlay}, &errs)
	if errs.Len() != 0 {
		t.Fatalf("expected no errors, got %d", errs.Len())
	}
}

func TestLockedMsg_WithName(t *testing.T) {
	msg := lockedMsg("metadata.id", 0, "prod-override", "acme.other", "acme.demo")
	want := `metadata.id: locked, overlay #0 (prod-override) set "acme.other" (base = "acme.demo")`
	if msg != want {
		t.Fatalf("msg mismatch:\nwant: %s\ngot:  %s", want, msg)
	}
}

func TestLockedMsg_WithoutName(t *testing.T) {
	msg := lockedMsg("metadata.id", 1, "", "acme.other", "acme.demo")
	want := `metadata.id: locked, overlay #1 set "acme.other" (base = "acme.demo")`
	if msg != want {
		t.Fatalf("msg mismatch:\nwant: %s\ngot:  %s", want, msg)
	}
}
