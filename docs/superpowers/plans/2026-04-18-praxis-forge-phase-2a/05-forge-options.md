# Task group 5 — `WithOverlays` + `WithSpecStore` + Build wiring

> Part of [praxis-forge Phase 2a Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md).

**Commit (atomic):** `feat(forge): WithOverlays + WithSpecStore options`

**Scope:** wire `Normalize` into the build pipeline through new functional options on `forge.Build`. Three pieces:

1. `forge.options` gains `overlays []*spec.AgentOverlay` and `store spec.SpecStore`.
2. New options `forge.WithOverlays(...)` and `forge.WithSpecStore(...)`.
3. `forge.Build` calls `spec.Normalize` before delegating to `build.Build`. `build.Build` is refactored to consume `*spec.NormalizedSpec` instead of `*spec.AgentSpec`, and its internal `s.Validate()` call is removed (Normalize already validated).

Existing `TestForge_FullSlice_Offline` keeps passing unchanged because the new options default to empty/nil and `Normalize` handles a spec with no extends and no overlays as a thin pass-through to `Validate`.

---

## Task 5.1: `forge.options` fields + `WithOverlays` + `WithSpecStore`

**Files:**
- Modify: `forge.go`

- [ ] **Step 1: Replace the `options` block**

Replace the existing `Option`/`options` declarations in `forge.go` with:

```go
// Option is a build-time knob for forge itself (distinct from kernel
// options). Options are applied in order; later WithOverlays calls
// append to the overlays slice.
type Option func(*options)

type options struct {
	overlays []*spec.AgentOverlay
	store    spec.SpecStore
}

// WithOverlays appends overlays to the build, applied in slice order
// (last wins). Multiple WithOverlays calls accumulate; passing nil or
// no overlays is a no-op.
func WithOverlays(ov ...*spec.AgentOverlay) Option {
	return func(o *options) {
		o.overlays = append(o.overlays, ov...)
	}
}

// WithSpecStore configures the SpecStore used to resolve parent specs
// referenced by AgentSpec.Extends. Required whenever the spec (or any
// resolved parent) declares Extends; without it Build returns
// ErrNoSpecStore.
func WithSpecStore(store spec.SpecStore) Option {
	return func(o *options) { o.store = store }
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: clean build (no consumers yet for the new fields).

---

## Task 5.2: Refactor `build.Build` to consume `*spec.NormalizedSpec`

**Files:**
- Modify: `build/build.go`

- [ ] **Step 1: Change the signature and rewire**

Replace the top of `build/build.go` (the `Build` function) with:

```go
// Build resolves every component referenced by the normalized spec
// through the registry, composes chains, and materializes a
// *orchestrator.Orchestrator.
//
// Phase 2a change: Build now consumes *spec.NormalizedSpec (the
// product of spec.Normalize), not the raw *spec.AgentSpec. Validation
// is the responsibility of Normalize and is no longer re-run here.
func Build(ctx context.Context, ns *spec.NormalizedSpec, r *registry.ComponentRegistry) (*BuiltAgent, error) {
	r.Freeze()

	s := &ns.Spec
	res, err := resolve(ctx, s, r)
	if err != nil {
		return nil, err
	}

	var opts []orchestrator.Option
	var toolDefs []llm.ToolDefinition

	// Tools.
	if len(res.toolPacks) > 0 {
		router, defs, err := newToolRouter(res.toolPacks)
		if err != nil {
			return nil, fmt.Errorf("tool router: %w", err)
		}
		opts = append(opts, orchestrator.WithToolInvoker(router))
		toolDefs = defs
	}

	// Policy chain.
	if len(res.policyHooks) > 0 {
		opts = append(opts, orchestrator.WithPolicyHook(policyChain(res.policyHooks)))
	}

	// Filter chains.
	if len(res.preLLMFilters) > 0 {
		opts = append(opts, orchestrator.WithPreLLMFilter(preLLMFilterChain(res.preLLMFilters)))
	}
	if len(res.preToolFilters) > 0 {
		opts = append(opts, orchestrator.WithPreToolFilter(preToolFilterChain(res.preToolFilters)))
	}
	if len(res.postToolFilters) > 0 {
		opts = append(opts, orchestrator.WithPostToolFilter(postToolFilterChain(res.postToolFilters)))
	}

	// Budget.
	if res.budget != nil {
		_, err := applyBudgetOverrides(res.budget.DefaultConfig, res.budgetOverrides)
		if err != nil {
			return nil, err
		}
		opts = append(opts, orchestrator.WithBudgetGuard(res.budget.Guard))
	}

	// Telemetry.
	if res.telemetry != nil {
		opts = append(opts, orchestrator.WithLifecycleEmitter(res.telemetry.Emitter))
		opts = append(opts, orchestrator.WithAttributeEnricher(res.telemetry.Enricher))
	}

	// Credentials.
	if res.credResolver != nil {
		opts = append(opts, orchestrator.WithCredentialResolver(res.credResolver))
	}

	// Identity.
	if res.identity != nil {
		opts = append(opts, orchestrator.WithIdentitySigner(res.identity))
	}

	orch, err := orchestrator.New(res.provider, opts...)
	if err != nil {
		return nil, fmt.Errorf("orchestrator.New: %w", err)
	}

	return &BuiltAgent{
		Orchestrator:   orch,
		Manifest:       buildManifest(s, res, ns),
		SystemPrompt:   res.systemPrompt,
		ToolDefs:       toolDefs,
		NormalizedSpec: ns,
	}, nil
}
```

- [ ] **Step 2: Add `NormalizedSpec` to `BuiltAgent`**

In the same file, replace the `BuiltAgent` struct declaration with:

```go
// BuiltAgent is a stateless wiring + metadata bundle. Per-turn state lives in
// the embedded Orchestrator; conversation history is the caller's concern.
type BuiltAgent struct {
	Orchestrator   *orchestrator.Orchestrator
	Manifest       manifest.Manifest
	SystemPrompt   string
	ToolDefs       []llm.ToolDefinition
	NormalizedSpec *spec.NormalizedSpec
}
```

- [ ] **Step 3: Update `buildManifest` signature**

Replace `buildManifest` with the version below (one new arg, two new field assignments). Manifest's new `ExtendsChain` and `Overlays` fields are introduced in task group 6 (commit 7); for this commit we leave the assignments commented as TODO so the file still compiles. **Engineer note:** the placeholder is removed in task 6.1 — do not commit this file with TODO comments; this task group's final commit excludes the manifest delta.

```go
func buildManifest(s *spec.AgentSpec, res *resolved, ns *spec.NormalizedSpec) manifest.Manifest {
	m := manifest.Manifest{
		SpecID:      s.Metadata.ID,
		SpecVersion: s.Metadata.Version,
		BuiltAt:     time.Now().UTC(),
	}
	// ExtendsChain + Overlays assignment lands in task group 6 once
	// manifest.Manifest grows the fields. The ns argument is threaded
	// now so the call-site stabilizes; the body is wired in commit 7.
	_ = ns

	m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
		Kind:   string(registry.KindProvider),
		ID:     string(res.providerID),
		Config: res.providerCfg,
	})
	// (rest of existing body unchanged)
	// ...
	return m
}
```

> Keep the rest of `buildManifest`'s body (the `Resolved` appends for prompt asset, tool packs, policy hooks, filters, budget, telemetry, credentials, identity) exactly as it is in the current file. Only the signature and the new `_ = ns` placeholder change in this commit.

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 5: Run existing build/ unit tests**

Run: `go test ./build/... -v`
Expected: PASS — internal tests don't exercise the new field, but they should still build and pass.

---

## Task 5.3: Wire `forge.Build` to call `spec.Normalize`

**Files:**
- Modify: `forge.go`

- [ ] **Step 1: Replace the `Build` function**

Replace the existing `Build` (and `BuiltAgent` accessors) in `forge.go` with:

```go
// Build validates the spec, resolves the extends chain (if any), applies
// overlays (if any), freezes the registry, resolves every component, and
// materializes a stateless BuiltAgent backed by *orchestrator.Orchestrator.
//
// Phase 2a additions:
//   - Pass spec.AgentOverlay values via WithOverlays(...). Overlays are
//     applied in slice order, last wins.
//   - Pass a spec.SpecStore via WithSpecStore(...) when the spec uses
//     Extends. Without it, Build returns ErrNoSpecStore.
//   - The full normalize pipeline runs before any component resolution;
//     callers can inspect the normalized result via
//     BuiltAgent.NormalizedSpec().
func Build(
	ctx context.Context,
	s *spec.AgentSpec,
	r *registry.ComponentRegistry,
	opts ...Option,
) (*BuiltAgent, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	ns, err := spec.Normalize(ctx, s, o.overlays, o.store)
	if err != nil {
		return nil, err
	}

	inner, err := build.Build(ctx, ns, r)
	if err != nil {
		return nil, err
	}
	return &BuiltAgent{inner: inner}, nil
}
```

> Keep `LoadSpec`, the `BuiltAgent` wrapper struct, and the existing accessors (`Invoke`, `Manifest`, `SystemPrompt`) as they are. The `NormalizedSpec` accessor lands in task group 7 (commit 8); for now `BuiltAgent.NormalizedSpec()` is not part of the public surface.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: clean build.

---

## Task 5.4: Confirm Phase 1 integration test still passes unchanged

**Files:**
- (Read-only) `forge_test.go`

- [ ] **Step 1: Run the existing integration test**

Run: `go test ./... -run TestForge_FullSlice_Offline -v`
Expected: PASS — the call site `forge.Build(context.Background(), s, r)` works unchanged because no options are passed (overlays empty, store nil) and the spec under test has no extends.

If this test fails: do not patch the test. Diagnose `forge.Build` or `spec.Normalize` and fix the underlying logic; the existing test is the load-bearing contract.

---

## Task 5.5: Lint + commit task group 5

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
git add forge.go build/build.go
git commit -m "feat(forge): WithOverlays + WithSpecStore options

Wires spec.Normalize into the build pipeline through two new functional
options. forge.Build's signature is unchanged at the call site —
overlays and stores enter via WithOverlays(...) and WithSpecStore(...).

build.Build now consumes *spec.NormalizedSpec instead of
*spec.AgentSpec; the internal s.Validate() call is removed because
Normalize already validated. BuiltAgent gains an unexported reference
to the NormalizedSpec, surfaced through forge.BuiltAgent in commit 8.

Manifest.ExtendsChain and Manifest.Overlays are NOT yet populated in
this commit — buildManifest threads the NormalizedSpec but the
assignment lands in commit 7 once Manifest grows the fields.

Existing TestForge_FullSlice_Offline passes unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```
