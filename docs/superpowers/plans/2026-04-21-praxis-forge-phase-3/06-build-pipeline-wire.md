# Task 06 — Wire expansion into Build

Integrate `expandSkills` into the `Build` pipeline, extend `resolve` to also resolve skills + output-contract factories after expansion, extend `buildManifest` to emit the new `Resolved` rows + `ExpandedHash` + `InjectedBySkill`, and extend `computeCapabilities` to surface `skill` and `output_contract`.

Prompt-fragment append (base system prompt + skill fragments joined with `"\n\n"`, byte-identical dedupe) lands in this task too.

## Files

- Modify: [`build/build.go`](../../../build/build.go)
- Modify: [`build/resolver.go`](../../../build/resolver.go)
- Modify: [`build/capabilities.go`](../../../build/capabilities.go)

## Background

Current `Build` at [`build/build.go:29-103`](../../../build/build.go#L29-L103):

```go
func Build(ctx context.Context, ns *spec.NormalizedSpec, r *registry.ComponentRegistry) (*BuiltAgent, error) {
    r.Freeze()

    res, err := resolve(ctx, &ns.Spec, r)
    ...
}
```

Phase 3 changes this to:

```go
func Build(ctx context.Context, ns *spec.NormalizedSpec, r *registry.ComponentRegistry) (*BuiltAgent, error) {
    r.Freeze()

    expanded, err := expandSkills(ctx, &ns.Spec, r)
    if err != nil { return nil, err }

    // Resolve the output-contract factory (if any) before the main resolve.
    // It does not participate in orchestrator wiring; just manifest attribution.
    if err := resolveOutputContract(ctx, expanded, r); err != nil {
        return nil, err
    }

    res, err := resolve(ctx, &expanded.Spec, r)
    // attach expanded + resolved output contract to res for manifest use
    ...
}
```

Prompt fragment append happens after `resolve()` sets `res.systemPrompt`; before materializing the orchestrator.

## Steps

### Part A — resolve output-contract factory

- [ ] **Step 1: Write failing test for output-contract resolution**

Append to `build/expand_test.go`:

```go
func TestResolveOutputContract_StampsValue(t *testing.T) {
	s := baseSpec()
	s.OutputContract = &spec.ComponentRef{
		Ref:    "outputcontract.json@1.0.0",
		Config: map[string]any{"schema": map[string]any{"type": "object"}},
	}
	es, err := expandSkills(context.Background(), s, registry.NewComponentRegistry())
	if err != nil {
		t.Fatalf("expandSkills: %v", err)
	}

	r := registry.NewComponentRegistry()
	schema := map[string]any{"type": "object"}
	if err := r.RegisterOutputContract(fakeOutputContract{
		id: "outputcontract.json@1.0.0",
		oc: registry.OutputContract{Schema: schema},
	}); err != nil {
		t.Fatal(err)
	}

	if err := resolveOutputContract(context.Background(), es, r); err != nil {
		t.Fatalf("resolveOutputContract: %v", err)
	}
	if es.ResolvedOutputContract == nil {
		t.Fatal("ResolvedOutputContract nil after resolve")
	}
	if es.ResolvedOutputContract.Value.Schema["type"] != "object" {
		t.Errorf("Schema.type: %v", es.ResolvedOutputContract.Value.Schema["type"])
	}
}

func TestResolveOutputContract_NilWhenAbsent(t *testing.T) {
	s := baseSpec()
	es, _ := expandSkills(context.Background(), s, registry.NewComponentRegistry())
	if err := resolveOutputContract(context.Background(), es, registry.NewComponentRegistry()); err != nil {
		t.Fatal(err)
	}
	if es.ResolvedOutputContract != nil {
		t.Error("ResolvedOutputContract should remain nil when spec has no outputContract")
	}
}

func TestResolveOutputContract_MissingFactory(t *testing.T) {
	s := baseSpec()
	s.OutputContract = &spec.ComponentRef{Ref: "outputcontract.missing@1.0.0"}
	es, _ := expandSkills(context.Background(), s, registry.NewComponentRegistry())
	err := resolveOutputContract(context.Background(), es, registry.NewComponentRegistry())
	if err == nil {
		t.Fatal("want ErrNotFound")
	}
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("want wraps ErrNotFound, got: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./build/ -run TestResolveOutputContract -v`

Expected: FAIL — `undefined: resolveOutputContract`, `undefined: ExpandedSpec.ResolvedOutputContract`.

- [ ] **Step 3: Add ResolvedOutputContract field + resolveOutputContract**

Edit `build/expand.go`. Extend the `ExpandedSpec` struct to add the field:

```go
type ExpandedSpec struct {
	Spec                   spec.AgentSpec
	Skills                 []ResolvedSkill
	ResolvedOutputContract *ResolvedOutputContract
	InjectedBy             map[string]registry.ID
}
```

At the end of the file add:

```go
// resolveOutputContract looks up the output-contract factory (if
// es.Spec.OutputContract is set) and stamps the built value onto the
// ExpandedSpec. No-op when the spec has no output contract. Called by
// Build after expandSkills so the contract reflects any skill-driven
// injection.
func resolveOutputContract(
	ctx context.Context,
	es *ExpandedSpec,
	r *registry.ComponentRegistry,
) error {
	if es.Spec.OutputContract == nil {
		return nil
	}
	id := registry.ID(es.Spec.OutputContract.Ref)
	fac, err := r.OutputContract(id)
	if err != nil {
		return fmt.Errorf("resolve outputContract: %w", err)
	}
	val, err := fac.Build(ctx, es.Spec.OutputContract.Config)
	if err != nil {
		return fmt.Errorf("build outputContract %s: %w", id, err)
	}
	es.ResolvedOutputContract = &ResolvedOutputContract{
		ID:     id,
		Config: es.Spec.OutputContract.Config,
		Value:  val,
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./build/ -run TestResolveOutputContract -v`

Expected: PASS (3 tests).

### Part B — Wire Build to use ExpandedSpec + prompt append

- [ ] **Step 5: Write failing integration test**

Append to `build/expand_test.go`:

```go
func TestBuild_AppendsSkillPromptFragment(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{{Ref: "skill.polite@1.0.0"}}

	r := registry.NewComponentRegistry()
	if err := r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterSkill(fakeSkill{id: "skill.polite@1.0.0", s: registry.Skill{
		PromptFragment: "Always be polite.",
	}}); err != nil {
		t.Fatal(err)
	}

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := "hi\n\nAlways be polite."
	if built.SystemPrompt != want {
		t.Errorf("SystemPrompt:\n  want: %q\n  got:  %q", want, built.SystemPrompt)
	}
}

func TestBuild_DedupesIdenticalPromptFragments(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{
		{Ref: "skill.a@1.0.0"},
		{Ref: "skill.b@1.0.0"},
	}

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterSkill(fakeSkill{id: "skill.a@1.0.0", s: registry.Skill{PromptFragment: "Be safe."}})
	_ = r.RegisterSkill(fakeSkill{id: "skill.b@1.0.0", s: registry.Skill{PromptFragment: "Be safe."}})

	ns, _ := spec.Normalize(context.Background(), s, nil, nil)
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := "hi\n\nBe safe."
	if built.SystemPrompt != want {
		t.Errorf("SystemPrompt:\n  want: %q\n  got:  %q", want, built.SystemPrompt)
	}
}
```

- [ ] **Step 6: Run to verify failure**

Run: `go test ./build/ -run 'TestBuild_AppendsSkillPromptFragment|TestBuild_DedupesIdenticalPromptFragments' -v`

Expected: FAIL — skills are resolved but fragments are not yet appended by `Build`.

- [ ] **Step 7: Wire expansion into Build**

Edit [`build/build.go`](../../../build/build.go). Replace the existing `Build` function with:

```go
func Build(ctx context.Context, ns *spec.NormalizedSpec, r *registry.ComponentRegistry) (*BuiltAgent, error) {
	r.Freeze()

	expanded, err := expandSkills(ctx, &ns.Spec, r)
	if err != nil {
		return nil, err
	}
	if err := resolveOutputContract(ctx, expanded, r); err != nil {
		return nil, err
	}

	res, err := resolve(ctx, &expanded.Spec, r)
	if err != nil {
		return nil, err
	}
	// Stamp expansion artefacts on res so buildManifest can attribute them.
	res.skills = make([]registry.Skill, 0, len(expanded.Skills))
	res.skillIDs = make([]registry.ID, 0, len(expanded.Skills))
	res.skillCfgs = make([]map[string]any, 0, len(expanded.Skills))
	for _, rs := range expanded.Skills {
		res.skills = append(res.skills, rs.Value)
		res.skillIDs = append(res.skillIDs, rs.ID)
		res.skillCfgs = append(res.skillCfgs, rs.Config)
	}
	if expanded.ResolvedOutputContract != nil {
		oc := expanded.ResolvedOutputContract.Value
		res.outputContract = &oc
		res.outputContractID = expanded.ResolvedOutputContract.ID
		res.outputContractCfg = expanded.ResolvedOutputContract.Config
	}

	// Append skill prompt fragments to the base system prompt (design
	// §"Prompt fragment merge"): declaration order, "\n\n" separator,
	// byte-identical dedupe.
	res.systemPrompt = appendSkillFragments(res.systemPrompt, expanded.Skills)

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
		Manifest:       buildManifest(&ns.Spec, res, ns, expanded),
		SystemPrompt:   res.systemPrompt,
		ToolDefs:       toolDefs,
		NormalizedSpec: ns,
	}, nil
}

// appendSkillFragments appends each skill's PromptFragment to base.
// Order: skills[] declaration order. Separator: "\n\n". Byte-identical
// fragments deduplicate silently (audit still shows each contributing
// skill in the manifest Resolved list).
func appendSkillFragments(base string, skills []ResolvedSkill) string {
	if len(skills) == 0 {
		return base
	}
	seen := map[string]bool{}
	out := base
	for _, rs := range skills {
		frag := rs.Value.PromptFragment
		if frag == "" {
			continue
		}
		if seen[frag] {
			continue
		}
		seen[frag] = true
		if out != "" {
			out += "\n\n"
		}
		out += frag
	}
	return out
}
```

The `buildManifest` signature changes (adds `expanded *ExpandedSpec` parameter). That is covered in the next step.

### Part C — buildManifest extensions

- [ ] **Step 8: Update buildManifest**

Replace the `buildManifest` function in [`build/build.go`](../../../build/build.go) with:

```go
func buildManifest(s *spec.AgentSpec, res *resolved, ns *spec.NormalizedSpec, expanded *ExpandedSpec) manifest.Manifest {
	hash, _ := ns.NormalizedHash() // error impossible: ns passed validation
	m := manifest.Manifest{
		SpecID:         s.Metadata.ID,
		SpecVersion:    s.Metadata.Version,
		BuiltAt:        time.Now().UTC(),
		NormalizedHash: hash,
		Capabilities:   computeCapabilities(s, res, expanded),
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

	// Phase 3: expanded hash emitted only when skills[] was non-empty.
	if len(s.Skills) > 0 {
		eh, _ := computeExpandedHash(expanded)
		m.ExpandedHash = eh
	}

	m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
		Kind:   string(registry.KindProvider),
		ID:     string(res.providerID),
		Config: res.providerCfg,
	})
	m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
		Kind: string(registry.KindPromptAsset),
		ID:   string(res.promptID),
	})
	for i, id := range res.toolPackIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:            string(registry.KindToolPack),
			ID:              string(id),
			Config:          res.toolPackCfgs[i],
			InjectedBySkill: lookupInjector(expanded, "tool_pack", string(id)),
		})
	}
	for i, id := range res.policyHookIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:            string(registry.KindPolicyPack),
			ID:              string(id),
			Config:          res.policyHookCfgs[i],
			InjectedBySkill: lookupInjector(expanded, "policy_pack", string(id)),
		})
	}
	for i, id := range res.preLLMIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindPreLLMFilter), ID: string(id), Config: res.preLLMCfgs[i],
		})
	}
	for i, id := range res.preToolIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindPreToolFilter), ID: string(id), Config: res.preToolCfgs[i],
		})
	}
	for i, id := range res.postToolIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindPostToolFilter), ID: string(id), Config: res.postToolCfgs[i],
		})
	}
	if res.budget != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindBudgetProfile), ID: string(res.budgetID), Config: res.budgetCfg,
		})
	}
	if res.telemetry != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindTelemetryProfile), ID: string(res.telemetryID), Config: res.telemetryCfg,
		})
	}
	if res.credResolver != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindCredentialResolver), ID: string(res.credResolverID), Config: res.credResolverCfg,
		})
	}
	if res.identity != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindIdentitySigner), ID: string(res.identityID), Config: res.identityCfg,
		})
	}
	// Skills and output contract (Phase 3).
	for i, id := range res.skillIDs {
		desc := res.skills[i].Descriptor
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:        string(registry.KindSkill),
			ID:          string(id),
			Config:      res.skillCfgs[i],
			Descriptors: desc,
		})
	}
	if res.outputContract != nil {
		desc := res.outputContract.Descriptor
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:            string(registry.KindOutputContract),
			ID:              string(res.outputContractID),
			Config:          res.outputContractCfg,
			Descriptors:     desc,
			InjectedBySkill: lookupInjector(expanded, "output_contract", string(res.outputContractID)),
		})
	}
	return m
}

// lookupInjector returns the skill id that drove inclusion of a
// specific (kindLabel, id) pair, or empty string if the component was
// user-declared or there was no expansion.
func lookupInjector(expanded *ExpandedSpec, kindLabel, id string) string {
	if expanded == nil || expanded.InjectedBy == nil {
		return ""
	}
	return string(expanded.InjectedBy[kindLabel+":"+id])
}
```

### Part D — capabilities extension

- [ ] **Step 9: Write failing capabilities test**

Append to the existing build tests (create `build/capabilities_phase3_test.go` if the build package does not already have a capabilities test file):

```go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"sort"
	"testing"

	"github.com/praxis-os/praxis-forge/manifest"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

func TestCapabilities_PresentIncludesSkillAndContract(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{{Ref: "skill.a@1.0.0"}}
	s.OutputContract = &spec.ComponentRef{Ref: "outputcontract.json@1.0.0"}

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterSkill(fakeSkill{id: "skill.a@1.0.0", s: registry.Skill{PromptFragment: "x"}})
	_ = r.RegisterOutputContract(fakeOutputContract{
		id: "outputcontract.json@1.0.0",
		oc: registry.OutputContract{Schema: map[string]any{"type": "object"}},
	})

	ns, _ := spec.Normalize(context.Background(), s, nil, nil)
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	present := built.Manifest.Capabilities.Present
	sort.Strings(present)
	// Present should include both.
	if !contains(present, "skill") {
		t.Errorf("Present missing skill: %v", present)
	}
	if !contains(present, "output_contract") {
		t.Errorf("Present missing output_contract: %v", present)
	}
}

func TestCapabilities_SkippedWhenAbsent(t *testing.T) {
	s := baseSpec()

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})

	ns, _ := spec.Normalize(context.Background(), s, nil, nil)
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	skipped := built.Manifest.Capabilities.Skipped

	var gotSkill, gotContract bool
	for _, s := range skipped {
		if s.Kind == "skill" && s.Reason == "not_specified" {
			gotSkill = true
		}
		if s.Kind == "output_contract" && s.Reason == "not_specified" {
			gotContract = true
		}
	}
	if !gotSkill {
		t.Errorf("Skipped missing skill/not_specified: %+v", skipped)
	}
	if !gotContract {
		t.Errorf("Skipped missing output_contract/not_specified: %+v", skipped)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

var _ manifest.Manifest // keep import live for future expansion
```

- [ ] **Step 10: Run to verify failure**

Run: `go test ./build/ -run TestCapabilities_ -v`

Expected: FAIL — computeCapabilities signature mismatch (old one takes `(s, res)`, new must take `(s, res, expanded)`) plus missing kinds in Present/Skipped.

- [ ] **Step 11: Update computeCapabilities**

Edit [`build/capabilities.go`](../../../build/capabilities.go). Change the signature and body:

```go
// computeCapabilities builds the manifest.Capabilities summary from the
// resolved components and the normalized spec.
//
// Phase 3: Skill and OutputContract kinds are included. A skill kind
// is "present" when spec.skills[] is non-empty; output_contract is
// "present" when the effective OutputContract (user-declared or
// skill-injected) is set on the expanded spec. Both kinds are "skipped"
// with reason "not_specified" when absent.
func computeCapabilities(s *spec.AgentSpec, res *resolved, expanded *ExpandedSpec) manifest.Capabilities {
	var present []string
	var skipped []manifest.CapabilitySkip

	// Required kinds: always present.
	present = append(present, string(registry.KindProvider))
	present = append(present, string(registry.KindPromptAsset))

	if len(res.toolPackIDs) > 0 {
		present = append(present, string(registry.KindToolPack))
	}
	if len(res.policyHookIDs) > 0 {
		present = append(present, string(registry.KindPolicyPack))
	}
	if len(res.preLLMIDs) > 0 {
		present = append(present, string(registry.KindPreLLMFilter))
	}
	if len(res.preToolIDs) > 0 {
		present = append(present, string(registry.KindPreToolFilter))
	}
	if len(res.postToolIDs) > 0 {
		present = append(present, string(registry.KindPostToolFilter))
	}

	// Singular optional kinds: present when built, skipped when spec field nil.
	if s.Budget != nil {
		present = append(present, string(registry.KindBudgetProfile))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindBudgetProfile), Reason: "not_specified"})
	}
	if s.Telemetry != nil {
		present = append(present, string(registry.KindTelemetryProfile))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindTelemetryProfile), Reason: "not_specified"})
	}
	if s.Credentials != nil {
		present = append(present, string(registry.KindCredentialResolver))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindCredentialResolver), Reason: "not_specified"})
	}
	if s.Identity != nil {
		present = append(present, string(registry.KindIdentitySigner))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindIdentitySigner), Reason: "not_specified"})
	}

	// Phase 3: skills and output contract.
	if len(s.Skills) > 0 {
		present = append(present, string(registry.KindSkill))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindSkill), Reason: "not_specified"})
	}
	// Effective output contract = user-declared OR skill-injected. The
	// expanded spec holds the resolved truth.
	hasContract := expanded != nil && expanded.Spec.OutputContract != nil
	if hasContract {
		present = append(present, string(registry.KindOutputContract))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindOutputContract), Reason: "not_specified"})
	}

	sort.Strings(present)
	return manifest.Capabilities{
		Present: present,
		Skipped: skipped,
	}
}
```

- [ ] **Step 12: Run the full build suite**

Run: `go vet ./... && go test ./build/... -v`

Expected: all tests pass.

The older build tests (`build_test.go`, `build_hash_test.go`) must still pass; they used `computeCapabilities(s, res)` internally via `buildManifest`. Because `buildManifest` now takes `expanded` too, confirm no test calls `computeCapabilities` or `buildManifest` directly. If a test does call them directly, update the call site.

- [ ] **Step 13: Commit**

```bash
git add build/build.go build/capabilities.go build/expand.go build/expand_test.go build/capabilities_phase3_test.go
git commit -m "$(cat <<'EOF'
feat(build): wire skill expansion into Build pipeline

Build now: expandSkills → resolveOutputContract → resolve → prompt
fragment append → orchestrator.New. buildManifest emits skill and
output-contract Resolved rows, plus InjectedBySkill attribution on
injected tool/policy rows and ExpandedHash when skills ran.
computeCapabilities surfaces skill and output_contract in Present /
Skipped.

Prompt fragment append: "\n\n" separator, declaration order,
byte-identical dedupe; audit preserved in Resolved list.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

## Expected state after this task

- `build/build.go`: `Build` drives `expandSkills` + `resolveOutputContract`, then passes the expanded spec to `resolve`, appends prompt fragments, and hands `expanded` to `buildManifest`.
- `build/capabilities.go`: `computeCapabilities(s, res, expanded)` includes `skill` / `output_contract` in Present/Skipped.
- `build/expand.go`: `ResolvedOutputContract` field on `ExpandedSpec`; `resolveOutputContract` helper.
- `build/expand_test.go`: +3 output-contract resolution tests + 2 prompt-fragment tests.
- `build/capabilities_phase3_test.go`: 2 capability tests.
- Full `go test ./...` green.
- One commit added.
