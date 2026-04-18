# Task group 6 — `Manifest.ExtendsChain` + `Manifest.Overlays`

> Part of [praxis-forge Phase 2a Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md).

**Commit (atomic):** `feat(manifest): ExtendsChain + Overlays attribution`

**Scope:** add `ExtendsChain []string` and `Overlays []OverlayAttribution` (plus the type) to `manifest.Manifest`, both with `omitempty` so a Phase 1 manifest serializes identically. Wire `build.buildManifest` to read those values from the threaded `*spec.NormalizedSpec` and populate the new fields. Add a focused unit test that round-trips a manifest with both fields populated through `encoding/json`.

This commit is small but discrete; keep it separate from task group 5 so the bisect bisects cleanly when manifest serialization changes.

---

## Task 6.1: Extend `manifest.Manifest`

**Files:**
- Modify: `manifest/manifest.go`

- [ ] **Step 1: Replace the file**

```go
// SPDX-License-Identifier: Apache-2.0

// Package manifest holds the inspectable build record for a BuiltAgent.
package manifest

import "time"

// Manifest is the build record returned alongside every BuiltAgent. It
// is JSON-serializable so callers can persist it for audit, diff, and
// inspection workflows.
type Manifest struct {
	SpecID       string               `json:"specId"`
	SpecVersion  string               `json:"specVersion"`
	BuiltAt      time.Time            `json:"builtAt"`
	ExtendsChain []string             `json:"extendsChain,omitempty"`
	Overlays     []OverlayAttribution `json:"overlays,omitempty"`
	Resolved     []ResolvedComponent  `json:"resolved"`
}

// OverlayAttribution identifies one overlay that contributed to the
// build. Mirror of spec.OverlayAttribution; duplicated here so the
// manifest package keeps zero internal dependencies.
type OverlayAttribution struct {
	Name string `json:"name"`
	File string `json:"file,omitempty"`
}

type ResolvedComponent struct {
	Kind        string         `json:"kind"`
	ID          string         `json:"id"`
	Config      map[string]any `json:"config,omitempty"`
	Descriptors any            `json:"descriptors,omitempty"`
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: clean build.

---

## Task 6.2: Manifest serialization tests

**Files:**
- Create: `manifest/manifest_test.go`

- [ ] **Step 1: Write the round-trip tests**

```go
// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/praxis-os/praxis-forge/manifest"
)

func TestManifest_EmptyChainAndOverlaysOmitted(t *testing.T) {
	m := manifest.Manifest{
		SpecID:      "acme.demo",
		SpecVersion: "0.1.0",
		BuiltAt:     time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		Resolved: []manifest.ResolvedComponent{
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
	m := manifest.Manifest{
		SpecID:      "acme.demo",
		SpecVersion: "0.1.0",
		BuiltAt:     time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		ExtendsChain: []string{
			"acme.grand@1.0.0",
			"acme.parent@1.0.0",
		},
		Overlays: []manifest.OverlayAttribution{
			{Name: "prod-override", File: "overlays/prod.yaml"},
			{Name: "ad-hoc"}, // File omitted
		},
		Resolved: []manifest.ResolvedComponent{
			{Kind: "provider", ID: "provider.fake@1.0.0"},
		},
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var got manifest.Manifest
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
```

- [ ] **Step 2: Run (expect pass)**

Run: `go test ./manifest/... -v`
Expected: PASS.

---

## Task 6.3: Wire `build.buildManifest` to populate the new fields

**Files:**
- Modify: `build/build.go`

- [ ] **Step 1: Replace the placeholder block**

In `build/build.go`'s `buildManifest`, replace the `_ = ns` placeholder block (introduced in task group 5) with the actual assignment:

```go
func buildManifest(s *spec.AgentSpec, res *resolved, ns *spec.NormalizedSpec) manifest.Manifest {
	m := manifest.Manifest{
		SpecID:      s.Metadata.ID,
		SpecVersion: s.Metadata.Version,
		BuiltAt:     time.Now().UTC(),
	}

	if len(ns.ExtendsChain) > 0 {
		m.ExtendsChain = append([]string(nil), ns.ExtendsChain...)
	}
	if len(ns.Overlays) > 0 {
		m.Overlays = make([]manifest.OverlayAttribution, 0, len(ns.Overlays))
		for _, o := range ns.Overlays {
			m.Overlays = append(m.Overlays, manifest.OverlayAttribution{
				Name: o.Name,
				File: o.File,
			})
		}
	}

	m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
		Kind:   string(registry.KindProvider),
		ID:     string(res.providerID),
		Config: res.providerCfg,
	})
	// (rest of existing body unchanged: prompt asset, tool packs, policy hooks,
	// pre-llm filters, pre-tool filters, post-tool filters, budget, telemetry,
	// credentials, identity)
	// ...
	return m
}
```

> Keep all `m.Resolved = append(...)` lines from the current file exactly as they are. Only the leading block (chain + overlays) is added.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: clean build.

---

## Task 6.4: Confirm Phase 1 integration test still passes (manifest still empty)

**Files:**
- (Read-only) `forge_test.go`

- [ ] **Step 1: Run the existing integration test**

Run: `go test ./... -run TestForge_FullSlice_Offline -v`
Expected: PASS — the spec under test has no extends and no overlays, so `m.ExtendsChain` and `m.Overlays` remain nil; `omitempty` keeps the JSON shape identical to Phase 1.

If the test starts asserting on serialized JSON shape, leave it: those assertions confirm Phase 1 round-trip holds.

---

## Task 6.5: Lint + commit task group 6

- [ ] **Step 1: Race + full suite**

Run: `make test-race`
Expected: PASS, no race warnings.

- [ ] **Step 2: Lint**

Run: `make lint`
Expected: zero reports.

- [ ] **Step 3: Vet**

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add manifest/manifest.go manifest/manifest_test.go build/build.go
git commit -m "feat(manifest): ExtendsChain + Overlays attribution

Adds two omitempty fields to manifest.Manifest plus the
OverlayAttribution carrier type, and wires buildManifest to populate
them from the NormalizedSpec threaded in during commit 6.

Phase 1 manifests serialize identically — both new fields are nil for
specs without extends or overlays, and omitempty drops them from the
JSON output. New round-trip tests in manifest_test.go cover both
shapes (empty + populated).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```
