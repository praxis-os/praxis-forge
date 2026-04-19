# Task group 7 — `NormalizedSpec` accessor + `LoadOverlays` helper + integration test

> Part of [praxis-forge Phase 2a Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md).

**Commit (atomic):** `feat(forge): NormalizedSpec accessor + LoadOverlays helper + integration test`

**Scope:** finish the public surface and prove the whole pipeline end to end.

1. `forge.LoadOverlay(path)` re-export of `spec.LoadOverlay`.
2. `forge.LoadOverlays(paths ...string)` convenience that loads each path and returns a slice.
3. `BuiltAgent.NormalizedSpec()` accessor returning the merged `*spec.NormalizedSpec`.
4. New integration test `TestForge_ExtendsAndOverlays_Offline` in `forge_test.go` exercising the full path: 2-deep extends chain + 2 overlays via `MapSpecStore`, manifest assertions, normalized spec assertions, and the negative `ErrNoSpecStore` case.

After this commit, all the public Phase 2a APIs declared in the spec exist and are exercised end-to-end.

---

## Task 7.1: `forge.LoadOverlay` + `forge.LoadOverlays`

**Files:**
- Modify: `forge.go`

- [ ] **Step 1: Add the re-export and the convenience helper**

Append to `forge.go` (alongside the existing `LoadSpec` declaration):

```go
// LoadOverlay reads and decodes an AgentOverlay YAML file with strict
// unknown-field rejection. Re-export of spec.LoadOverlay for callers
// that import only the forge package.
func LoadOverlay(path string) (*spec.AgentOverlay, error) {
	return spec.LoadOverlay(path)
}

// LoadOverlays loads each path in turn and returns the slice in the
// same order. The first failing load aborts and returns its error
// (no partial result). Pass-through for the common case where a caller
// would otherwise write the loop themselves.
func LoadOverlays(paths ...string) ([]*spec.AgentOverlay, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	out := make([]*spec.AgentOverlay, 0, len(paths))
	for _, p := range paths {
		ov, err := LoadOverlay(p)
		if err != nil {
			return nil, err
		}
		out = append(out, ov)
	}
	return out, nil
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: clean build.

---

## Task 7.2: `BuiltAgent.NormalizedSpec()` accessor

**Files:**
- Modify: `forge.go`

- [ ] **Step 1: Add the accessor on the wrapper**

Append next to the existing `Manifest()` and `SystemPrompt()` accessors in `forge.go`:

```go
// NormalizedSpec returns the canonical merge result that drove this
// build: the flattened AgentSpec, the resolved extends chain, the
// overlay attribution list, and per-field provenance. Callers can
// inspect this for debugging, audit, and "what is inside this agent"
// queries.
//
// Returned pointer aliases internal state — treat it as read-only.
func (b *BuiltAgent) NormalizedSpec() *spec.NormalizedSpec {
	return b.inner.NormalizedSpec
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: clean build.

---

## Task 7.3: Integration test fixture (3 specs in MapSpecStore)

**Files:**
- Create: `testdata/extends_overlays/base.yaml`
- Create: `testdata/extends_overlays/parent-direct.yaml`
- Create: `testdata/extends_overlays/parent-grand.yaml`
- Create: `testdata/extends_overlays/overlay-1.yaml`
- Create: `testdata/extends_overlays/overlay-2.yaml`

> **Engineer note:** these fixtures live at the repo root under `testdata/extends_overlays/` because `forge_test.go` is in the root package; that's where the existing `testdata/agent.yaml` lives.

- [ ] **Step 1: Write the base spec**

```yaml
# testdata/extends_overlays/base.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.demo
  version: 0.1.0
provider:
  ref: provider.fake@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
extends:
  - acme.parent@1.0.0
```

- [ ] **Step 2: Write the direct parent**

```yaml
# testdata/extends_overlays/parent-direct.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.parent
  version: 0.1.0
provider:
  ref: provider.from-parent@1.0.0
policies:
  - ref: policypack.pii-redaction@1.0.0
extends:
  - acme.grand@1.0.0
```

- [ ] **Step 3: Write the grandparent**

```yaml
# testdata/extends_overlays/parent-grand.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.grand
  version: 0.1.0
filters:
  preLLM:
    - ref: filter.secret-scrubber@1.0.0
```

- [ ] **Step 4: Write overlay 1**

```yaml
# testdata/extends_overlays/overlay-1.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: prod-provider
spec:
  provider:
    ref: provider.fake@1.0.0
```

- [ ] **Step 5: Write overlay 2**

```yaml
# testdata/extends_overlays/overlay-2.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: extra-budget
spec:
  budget:
    ref: budgetprofile.default-tier1@1.0.0
    overrides:
      maxToolCalls: 24
```

---

## Task 7.4: Integration test wiring

**Files:**
- Modify: `forge_test.go`

- [ ] **Step 1: Append the positive integration test**

Append to `forge_test.go`:

```go
import (
	"errors"

	"github.com/praxis-os/praxis-forge/spec"
)

func TestForge_ExtendsAndOverlays_Offline(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	canned := llm.LLMResponse{
		StopReason: llm.StopReasonEndTurn,
		Message: llm.Message{
			Role:  llm.RoleAssistant,
			Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: "hi"}},
		},
	}

	r := registry.NewComponentRegistry()
	mustRegister(t, r.RegisterProvider(fakeProviderFactory{
		id:   "provider.fake@1.0.0",
		resp: canned,
	}))
	mustRegister(t, r.RegisterProvider(fakeProviderFactory{
		id:   "provider.from-parent@1.0.0",
		resp: canned,
	}))
	mustRegister(t, r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.sys@1.0.0")))
	mustRegister(t, r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0")))
	mustRegister(t, r.RegisterPreLLMFilter(filtersecretscrubber.NewFactory("filter.secret-scrubber@1.0.0")))
	mustRegister(t, r.RegisterBudgetProfile(budgetprofiledefault.NewFactory("budgetprofile.default-tier1@1.0.0")))
	mustRegister(t, r.RegisterIdentitySigner(identitysignered25519.NewFactory("identitysigner.ed25519@1.0.0", priv)))

	base, err := forge.LoadSpec("testdata/extends_overlays/base.yaml")
	if err != nil {
		t.Fatal(err)
	}
	parentDirect, err := forge.LoadSpec("testdata/extends_overlays/parent-direct.yaml")
	if err != nil {
		t.Fatal(err)
	}
	parentGrand, err := forge.LoadSpec("testdata/extends_overlays/parent-grand.yaml")
	if err != nil {
		t.Fatal(err)
	}
	overlays, err := forge.LoadOverlays(
		"testdata/extends_overlays/overlay-1.yaml",
		"testdata/extends_overlays/overlay-2.yaml",
	)
	if err != nil {
		t.Fatalf("load overlays: %v", err)
	}

	store := spec.MapSpecStore{
		"acme.parent@1.0.0": parentDirect,
		"acme.grand@1.0.0":  parentGrand,
	}

	b, err := forge.Build(
		context.Background(),
		base,
		r,
		forge.WithOverlays(overlays...),
		forge.WithSpecStore(store),
	)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Manifest carries the resolved chain root-first.
	m := b.Manifest()
	wantChain := []string{"acme.grand@1.0.0", "acme.parent@1.0.0"}
	if len(m.ExtendsChain) != len(wantChain) {
		t.Fatalf("ExtendsChain=%v, want %v", m.ExtendsChain, wantChain)
	}
	for i, w := range wantChain {
		if m.ExtendsChain[i] != w {
			t.Errorf("ExtendsChain[%d]=%q, want %q", i, m.ExtendsChain[i], w)
		}
	}

	// Manifest carries both overlay names + files.
	if len(m.Overlays) != 2 {
		t.Fatalf("Overlays=%+v, want 2", m.Overlays)
	}
	if m.Overlays[0].Name != "prod-provider" || m.Overlays[1].Name != "extra-budget" {
		t.Errorf("overlay names = %+v", m.Overlays)
	}
	if m.Overlays[0].File == "" || m.Overlays[1].File == "" {
		t.Errorf("overlay file paths should be populated by LoadOverlay: %+v", m.Overlays)
	}

	// NormalizedSpec exposes the merged result. Provider should be the
	// overlay's, not the parent's (overlay-1 set provider.fake@1.0.0,
	// parent-direct set provider.from-parent@1.0.0).
	ns := b.NormalizedSpec()
	if ns == nil {
		t.Fatal("NormalizedSpec() returned nil")
	}
	if ns.Spec.Provider.Ref != "provider.fake@1.0.0" {
		t.Errorf("provider.ref=%q, want overlay's ref", ns.Spec.Provider.Ref)
	}
	// Policies came from parent-direct (base did not set them).
	if len(ns.Spec.Policies) != 1 || ns.Spec.Policies[0].Ref != "policypack.pii-redaction@1.0.0" {
		t.Errorf("policies=%+v", ns.Spec.Policies)
	}
	// Filters came from grandparent.
	if len(ns.Spec.Filters.PreLLM) != 1 ||
		ns.Spec.Filters.PreLLM[0].Ref != "filter.secret-scrubber@1.0.0" {
		t.Errorf("filters.preLLM=%+v", ns.Spec.Filters.PreLLM)
	}
	// Provenance for provider points at overlay #0; budget at overlay #1.
	if pv, _ := ns.Provenance("provider"); pv.Role != spec.RoleOverlay || pv.Step != 0 {
		t.Errorf("provider provenance = %+v", pv)
	}
	if pv, _ := ns.Provenance("budget"); pv.Role != spec.RoleOverlay || pv.Step != 1 {
		t.Errorf("budget provenance = %+v", pv)
	}

	// Round-trip an Invoke through the fake provider.
	res, err := b.Invoke(context.Background(), praxis.InvocationRequest{
		Model:        "fake",
		SystemPrompt: b.SystemPrompt(),
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: "ping"}},
		}},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
}

func TestForge_ExtendsWithoutSpecStoreFails(t *testing.T) {
	r := registry.NewComponentRegistry()
	// Registry need not be populated; the build aborts before resolution.

	base, err := forge.LoadSpec("testdata/extends_overlays/base.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = forge.Build(context.Background(), base, r)
	if !errors.Is(err, spec.ErrNoSpecStore) {
		t.Fatalf("err=%v, want spec.ErrNoSpecStore", err)
	}
}

func TestForge_LockedFieldOverrideSurfaces(t *testing.T) {
	r := registry.NewComponentRegistry()

	base, err := forge.LoadSpec("testdata/extends_overlays/base.yaml")
	if err != nil {
		t.Fatal(err)
	}
	parentDirect, err := forge.LoadSpec("testdata/extends_overlays/parent-direct.yaml")
	if err != nil {
		t.Fatal(err)
	}
	parentGrand, err := forge.LoadSpec("testdata/extends_overlays/parent-grand.yaml")
	if err != nil {
		t.Fatal(err)
	}

	// Construct an overlay that violates locked metadata.id.
	bad := &spec.AgentOverlay{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentOverlay",
		Metadata:   spec.OverlayMeta{Name: "bad-rebrand"},
		Spec: spec.AgentOverlayBody{
			Metadata: &spec.OverlayMetadata{ID: "acme.other"},
		},
	}

	store := spec.MapSpecStore{
		"acme.parent@1.0.0": parentDirect,
		"acme.grand@1.0.0":  parentGrand,
	}
	_, err = forge.Build(
		context.Background(),
		base,
		r,
		forge.WithOverlays(bad),
		forge.WithSpecStore(store),
	)
	if !errors.Is(err, spec.ErrLockedFieldOverride) {
		t.Fatalf("err=%v, want spec.ErrLockedFieldOverride", err)
	}
	if !strings.Contains(err.Error(), "metadata.id") {
		t.Fatalf("error must name field: %v", err)
	}
	if !strings.Contains(err.Error(), "overlay #0") {
		t.Fatalf("error must attribute to overlay: %v", err)
	}
}
```

> If `strings` is not yet imported in `forge_test.go`, add it. The existing test imports list is small; keep ordering: stdlib, blank line, external, blank line, this module.

- [ ] **Step 2: Run the new tests**

Run: `go test ./... -run "TestForge_ExtendsAndOverlays_Offline|TestForge_ExtendsWithoutSpecStoreFails|TestForge_LockedFieldOverrideSurfaces" -v`
Expected: every subtest PASS.

- [ ] **Step 3: Run the original Phase 1 test**

Run: `go test ./... -run TestForge_FullSlice_Offline -v`
Expected: PASS — Phase 1 path is unaffected.

---

## Task 7.5: Lint + commit task group 7

- [ ] **Step 1: Race + full suite**

Run: `make test-race`
Expected: every package PASS, no race warnings.

- [ ] **Step 2: Lint**

Run: `make lint`
Expected: zero reports.

- [ ] **Step 3: Vet**

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add forge.go forge_test.go testdata/extends_overlays
git commit -m "feat(forge): NormalizedSpec accessor + LoadOverlays helper + integration test

Finishes the public Phase 2a surface:
  - forge.LoadOverlay re-export (single overlay).
  - forge.LoadOverlays(paths ...string) convenience for the common loop.
  - BuiltAgent.NormalizedSpec() accessor exposing the merged result.

Adds three integration tests in forge_test.go:
  - TestForge_ExtendsAndOverlays_Offline: 2-deep chain + 2 overlays via
    MapSpecStore. Asserts manifest carries ExtendsChain and Overlays
    attribution, NormalizedSpec exposes the merged provider/policies/
    filters, provenance points at the right overlay step.
  - TestForge_ExtendsWithoutSpecStoreFails: confirms ErrNoSpecStore
    surfaces when extends is non-empty without WithSpecStore.
  - TestForge_LockedFieldOverrideSurfaces: a struct-literal overlay
    that touches metadata.id is rejected with ErrLockedFieldOverride
    and the error attributes the change to 'overlay #0'.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```
