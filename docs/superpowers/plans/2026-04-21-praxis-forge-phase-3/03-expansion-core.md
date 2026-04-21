# Task 03 — Skill expansion core

Build the `build/expand.go` module: a `CanonicalConfigsEqual` helper in `spec/`, an `ExpandedSpec` type, and the `expandSkills` function that resolves skill + output-contract factories, auto-injects their contributions into the effective composition, and fails the build on every conflict row from the design spec's expansion-semantics table.

## Files

- Modify: [`spec/canonical.go`](../../../spec/canonical.go) — add `CanonicalConfigsEqual`
- Modify: [`spec/canonical_test.go`](../../../spec/canonical_test.go) — add equality tests
- Create: `build/expand.go` — `ExpandedSpec`, `expandSkills`, conflict helpers
- Create: `build/expand_test.go` — unit tests covering every expansion-semantics row
- Modify: [`build/resolver.go`](../../../build/resolver.go) — extend the `resolved` struct with skill + output-contract fields (added now, wired in Task 06)

## Background

The design spec's expansion-semantics table (design doc §"Expansion semantics"):

| State | Result | Error code |
|-------|--------|-----------|
| Same id, canonical-identical Config | idempotent no-op (still recorded in attribution) | — |
| Same `kind.<name>`, different semver | fail | `skill_conflict_version_divergence` |
| Same id, Config differs canonically | fail | `skill_conflict_config_divergence` |
| Two skills require different output contracts | fail | `skill_conflict_output_contract_multiple` |
| Skill contract vs. user contract divergent | fail | `skill_conflict_output_contract_user_override` |
| Skill references unregistered component | fail | `skill_unresolved_required_component` |
| Skill with empty contribution | fail | `skill_empty_contribution` |

Attribution rule (design doc §"Manifest additions"):

- User-declared only → `InjectedBySkill` empty.
- Skill-injected only → `InjectedBySkill = <skill id>`.
- Both declared identically → `InjectedBySkill` empty (user wins).
- Multiple skills inject identically, no user → first skill in declaration order wins.

Canonical config comparison uses the same encoder as Phase 2b ([`spec/canonical.go:110`](../../../spec/canonical.go#L110) `canonicalEncode` + `pruneEmpty`).

## Steps

### Part A — spec.CanonicalConfigsEqual helper

- [ ] **Step 1: Write failing test for CanonicalConfigsEqual**

Append to [`spec/canonical_test.go`](../../../spec/canonical_test.go):

```go
func TestCanonicalConfigsEqual_SameContent(t *testing.T) {
	a := map[string]any{"timeoutMs": 5000, "host": "example.com"}
	b := map[string]any{"host": "example.com", "timeoutMs": 5000}
	eq, err := CanonicalConfigsEqual(a, b)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !eq {
		t.Error("want equal (map key order should not matter)")
	}
}

func TestCanonicalConfigsEqual_DifferentContent(t *testing.T) {
	a := map[string]any{"timeoutMs": 5000}
	b := map[string]any{"timeoutMs": 7000}
	eq, err := CanonicalConfigsEqual(a, b)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eq {
		t.Error("want unequal")
	}
}

func TestCanonicalConfigsEqual_NilVsEmpty(t *testing.T) {
	// Per Phase 2b: empty maps and nil are canonically equivalent.
	eq, err := CanonicalConfigsEqual(nil, map[string]any{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !eq {
		t.Error("want nil == empty map (Phase 2b pruneEmpty contract)")
	}
}

func TestCanonicalConfigsEqual_NestedMaps(t *testing.T) {
	a := map[string]any{
		"headers": map[string]any{"User-Agent": "forge", "X-Trace": "abc"},
	}
	b := map[string]any{
		"headers": map[string]any{"X-Trace": "abc", "User-Agent": "forge"},
	}
	eq, err := CanonicalConfigsEqual(a, b)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !eq {
		t.Error("want equal (nested map key order should not matter)")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./spec/ -run TestCanonicalConfigsEqual -v`

Expected: FAIL — `undefined: CanonicalConfigsEqual`.

- [ ] **Step 3: Add CanonicalConfigsEqual**

Edit [`spec/canonical.go`](../../../spec/canonical.go). Append at the end of the file:

```go
// CanonicalConfigsEqual reports whether two ComponentRef-style config
// maps encode to identical canonical JSON. Nil and empty collections
// compare equal (per the Phase 2b pruneEmpty contract). Useful to
// detect idempotent skill requirements during build-time expansion.
func CanonicalConfigsEqual(a, b map[string]any) (bool, error) {
	ab, err := canonicalizeConfig(a)
	if err != nil {
		return false, err
	}
	bb, err := canonicalizeConfig(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ab, bb), nil
}

// canonicalizeConfig reuses the same pipeline as computeCanonicalJSON,
// but scoped to a raw config map: marshal → unmarshal (UseNumber) → prune
// empty → canonicalEncode.
func canonicalizeConfig(cfg map[string]any) ([]byte, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("spec: canonical config marshal: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, fmt.Errorf("spec: canonical config decode: %w", err)
	}
	tree = pruneEmpty(tree)
	if tree == nil {
		return []byte("null"), nil
	}
	var buf bytes.Buffer
	if err := canonicalEncode(&buf, tree); err != nil {
		return nil, fmt.Errorf("spec: canonical config encode: %w", err)
	}
	return buf.Bytes(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./spec/ -run TestCanonicalConfigsEqual -v`

Expected: PASS (4 tests).

- [ ] **Step 5: Commit Part A**

```bash
git add spec/canonical.go spec/canonical_test.go
git commit -m "$(cat <<'EOF'
feat(spec): CanonicalConfigsEqual helper for idempotency checks

Phase 3 expansion needs to detect when two ComponentRef config maps are
canonically equivalent (map key order irrelevant, nil ≡ empty). Reuse
the Phase 2b canonical encoder + pruneEmpty pipeline, no new logic.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Part B — ExpandedSpec type and the skeleton

- [ ] **Step 6: Write failing test for ExpandedSpec basic shape**

Create `build/expand_test.go` (new file):

```go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

// --- Test fakes: skill + output-contract factories ---

type fakeSkill struct {
	id registry.ID
	s  registry.Skill
}

func (f fakeSkill) ID() registry.ID     { return f.id }
func (f fakeSkill) Description() string { return "fake skill" }
func (f fakeSkill) Build(context.Context, map[string]any) (registry.Skill, error) {
	return f.s, nil
}

type fakeOutputContract struct {
	id registry.ID
	oc registry.OutputContract
}

func (f fakeOutputContract) ID() registry.ID     { return f.id }
func (f fakeOutputContract) Description() string { return "fake output contract" }
func (f fakeOutputContract) Build(context.Context, map[string]any) (registry.OutputContract, error) {
	return f.oc, nil
}

// --- Test helpers ---

func baseSpec() *spec.AgentSpec {
	return &spec.AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "acme.demo", Version: "0.1.0"},
		Provider:   spec.ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: "prompt.sys@1.0.0"}},
	}
}

func regWithSkill(t *testing.T, skillID registry.ID, sk registry.Skill) *registry.ComponentRegistry {
	t.Helper()
	r := registry.NewComponentRegistry()
	if err := r.RegisterSkill(fakeSkill{id: skillID, s: sk}); err != nil {
		t.Fatalf("RegisterSkill: %v", err)
	}
	return r
}

// --- Basic shape ---

func TestExpandSkills_NoSkillsReturnsEmptySpec(t *testing.T) {
	s := baseSpec()
	r := registry.NewComponentRegistry()
	got, err := expandSkills(context.Background(), s, r)
	if err != nil {
		t.Fatalf("expandSkills: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil ExpandedSpec")
	}
	if len(got.Skills) != 0 {
		t.Errorf("Skills: want empty, got %d", len(got.Skills))
	}
	if len(got.InjectedBy) != 0 {
		t.Errorf("InjectedBy: want empty, got %v", got.InjectedBy)
	}
}
```

- [ ] **Step 7: Run to verify failure**

Run: `go test ./build/ -run TestExpandSkills_NoSkillsReturnsEmptySpec -v`

Expected: FAIL — `undefined: expandSkills` / `undefined: ExpandedSpec`.

- [ ] **Step 8: Create build/expand.go with the type and a no-op path**

Create `build/expand.go`:

```go
// SPDX-License-Identifier: Apache-2.0

// Package build resolves components, composes praxis hooks, and
// materializes the BuiltAgent. This file holds the Phase-3 skill
// expansion stage.
package build

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

// ExpandedSpec is the post-skill-expansion composition. Spec carries
// the effective AgentSpec with Tools / Policies / OutputContract
// rewritten to include skill contributions. Skills carries the raw
// skill ids + configs + resolved values so the manifest can emit them.
// InjectedBy maps "kind:id" entries to the skill id that drove inclusion
// (attribution rules: see design doc §"Manifest additions").
type ExpandedSpec struct {
	Spec       spec.AgentSpec
	Skills     []ResolvedSkill
	InjectedBy map[string]registry.ID
}

// ResolvedSkill records a resolved skill factory's contribution for the
// manifest. Config is the authoring-site config from spec.skills[].config
// (kept verbatim for audit); Value is the built Skill.
type ResolvedSkill struct {
	ID     registry.ID
	Config map[string]any
	Value  registry.Skill
}

// ResolvedOutputContract mirrors ResolvedSkill for the single
// optional output-contract slot.
type ResolvedOutputContract struct {
	ID     registry.ID
	Config map[string]any
	Value  registry.OutputContract
}

// Error codes emitted by expandSkills. Each is wrapped into an error
// string that includes the skill id and the conflicting target.
const (
	errCodeEmptyContribution      = "skill_empty_contribution"
	errCodeUnresolvedRequired     = "skill_unresolved_required_component"
	errCodeVersionDivergence      = "skill_conflict_version_divergence"
	errCodeConfigDivergence       = "skill_conflict_config_divergence"
	errCodeOutputMultiple         = "skill_conflict_output_contract_multiple"
	errCodeOutputUserOverride     = "skill_conflict_output_contract_user_override"
)

// expandSkills resolves every spec.skills[] factory, auto-injects each
// skill's RequiredTools / RequiredPolicies / RequiredOutputContract
// into the effective composition, and returns an ExpandedSpec with the
// rewritten AgentSpec + attribution.
//
// Detection semantics and conflict codes are documented on the error
// codes above and in docs/superpowers/specs/…-phase-3-design.md
// §"Expansion semantics".
func expandSkills(
	ctx context.Context,
	s *spec.AgentSpec,
	r *registry.ComponentRegistry,
) (*ExpandedSpec, error) {
	out := &ExpandedSpec{
		Spec:       *s, // shallow copy; slices repointed below when we mutate
		InjectedBy: map[string]registry.ID{},
	}
	if len(s.Skills) == 0 {
		return out, nil
	}

	// TODO: Steps 11+ populate this.
	return nil, fmt.Errorf("expandSkills: not implemented yet")
}
```

- [ ] **Step 9: Run the no-skills test — should pass**

Run: `go test ./build/ -run TestExpandSkills_NoSkillsReturnsEmptySpec -v`

Expected: PASS.

### Part C — Happy path: resolve skills + idempotent injection

- [ ] **Step 10: Write failing test for a single-skill happy path**

Append to `build/expand_test.go`:

```go
func TestExpandSkills_InjectsToolFromSkill(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{{Ref: "skill.has-http@1.0.0"}}

	skillValue := registry.Skill{
		PromptFragment: "Use http-get for lookups.",
		RequiredTools: []registry.RequiredComponent{
			{ID: "toolpack.http-get@1.0.0"},
		},
		Descriptor: registry.SkillDescriptor{Name: "has-http"},
	}
	r := regWithSkill(t, "skill.has-http@1.0.0", skillValue)

	got, err := expandSkills(context.Background(), s, r)
	if err != nil {
		t.Fatalf("expandSkills: %v", err)
	}
	if len(got.Spec.Tools) != 1 {
		t.Fatalf("Spec.Tools: want 1, got %d", len(got.Spec.Tools))
	}
	if got.Spec.Tools[0].Ref != "toolpack.http-get@1.0.0" {
		t.Errorf("Spec.Tools[0].Ref: %q", got.Spec.Tools[0].Ref)
	}
	if got.InjectedBy["tool_pack:toolpack.http-get@1.0.0"] != "skill.has-http@1.0.0" {
		t.Errorf("InjectedBy: %v", got.InjectedBy)
	}
	if len(got.Skills) != 1 || got.Skills[0].ID != "skill.has-http@1.0.0" {
		t.Errorf("Skills: %+v", got.Skills)
	}
}

func TestExpandSkills_IdempotentWhenUserAlreadyDeclared(t *testing.T) {
	s := baseSpec()
	s.Tools = []spec.ComponentRef{
		{Ref: "toolpack.http-get@1.0.0"},
	}
	s.Skills = []spec.ComponentRef{{Ref: "skill.has-http@1.0.0"}}

	skillValue := registry.Skill{
		RequiredTools: []registry.RequiredComponent{
			{ID: "toolpack.http-get@1.0.0"},
		},
	}
	r := regWithSkill(t, "skill.has-http@1.0.0", skillValue)

	got, err := expandSkills(context.Background(), s, r)
	if err != nil {
		t.Fatalf("expandSkills: %v", err)
	}
	if len(got.Spec.Tools) != 1 {
		t.Errorf("Spec.Tools: want 1 (no dup), got %d", len(got.Spec.Tools))
	}
	// User-explicit wins attribution: InjectedBy must not record this.
	if _, found := got.InjectedBy["tool_pack:toolpack.http-get@1.0.0"]; found {
		t.Errorf("InjectedBy must be empty for user-declared tool, got %v", got.InjectedBy)
	}
}
```

- [ ] **Step 11: Run to verify failure**

Run: `go test ./build/ -run 'TestExpandSkills_(Injects|Idempotent)' -v`

Expected: FAIL with the "not implemented yet" placeholder.

- [ ] **Step 12: Implement happy-path expansion**

Replace the body of `expandSkills` in `build/expand.go`. Full replacement (entire file after the const block):

```go
// expandSkills resolves every spec.skills[] factory, auto-injects each
// skill's RequiredTools / RequiredPolicies / RequiredOutputContract
// into the effective composition, and returns an ExpandedSpec with the
// rewritten AgentSpec + attribution.
func expandSkills(
	ctx context.Context,
	s *spec.AgentSpec,
	r *registry.ComponentRegistry,
) (*ExpandedSpec, error) {
	// Start with a value-copy of spec; take fresh slices so mutation
	// does not leak back to the caller's AgentSpec.
	out := &ExpandedSpec{
		Spec:       *s,
		InjectedBy: map[string]registry.ID{},
	}
	out.Spec.Tools = append([]spec.ComponentRef(nil), s.Tools...)
	out.Spec.Policies = append([]spec.ComponentRef(nil), s.Policies...)
	if s.OutputContract != nil {
		oc := *s.OutputContract
		out.Spec.OutputContract = &oc
	}

	if len(s.Skills) == 0 {
		return out, nil
	}

	// Resolve each skill factory in declaration order. Build the Skill
	// values first so all validation runs before any injection.
	for i, skillRef := range s.Skills {
		skillID := registry.ID(skillRef.Ref)
		fac, err := r.Skill(skillID)
		if err != nil {
			return nil, fmt.Errorf("resolve skills[%d] %s: %w", i, skillRef.Ref, err)
		}
		skVal, err := fac.Build(ctx, skillRef.Config)
		if err != nil {
			return nil, fmt.Errorf("build skills[%d] %s: %w", i, skillRef.Ref, err)
		}
		if isEmptyContribution(skVal) {
			return nil, fmt.Errorf(
				"skills[%d] %s: %s: skill contributes no prompt fragment, no required components",
				i, skillRef.Ref, errCodeEmptyContribution,
			)
		}
		out.Skills = append(out.Skills, ResolvedSkill{
			ID:     skillID,
			Config: skillRef.Config,
			Value:  skVal,
		})

		// Auto-inject RequiredTools.
		for _, rc := range skVal.RequiredTools {
			if err := injectRequired(&out.Spec.Tools, out.InjectedBy, "tool_pack", skillID, rc); err != nil {
				return nil, err
			}
		}
		// Auto-inject RequiredPolicies.
		for _, rc := range skVal.RequiredPolicies {
			if err := injectRequired(&out.Spec.Policies, out.InjectedBy, "policy_pack", skillID, rc); err != nil {
				return nil, err
			}
		}
		// Auto-inject RequiredOutputContract (singular).
		if skVal.RequiredOutputContract != nil {
			if err := injectOutputContract(out, skillID, *skVal.RequiredOutputContract); err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}

// isEmptyContribution reports whether a Skill has no meaningful output:
// no prompt fragment, no required tools/policies, no output contract.
func isEmptyContribution(sk registry.Skill) bool {
	return sk.PromptFragment == "" &&
		len(sk.RequiredTools) == 0 &&
		len(sk.RequiredPolicies) == 0 &&
		sk.RequiredOutputContract == nil
}

// injectRequired merges one RequiredComponent into the target slice
// with strict conflict detection. kindLabel is the short kind slug used
// in the InjectedBy map key (e.g. "tool_pack", "policy_pack") and in
// error messages.
func injectRequired(
	target *[]spec.ComponentRef,
	injectedBy map[string]registry.ID,
	kindLabel string,
	skillID registry.ID,
	rc registry.RequiredComponent,
) error {
	if rc.ID == "" {
		return fmt.Errorf("%s %s: RequiredComponent missing ID", kindLabel, skillID)
	}
	wantName, wantVersion, err := spec.ParseID(string(rc.ID))
	if err != nil {
		return fmt.Errorf("%s %s: %w", kindLabel, skillID, err)
	}

	for _, existing := range *target {
		gotName, gotVersion, err := spec.ParseID(existing.Ref)
		if err != nil {
			return fmt.Errorf("%s %s: existing ref %q: %w", kindLabel, skillID, existing.Ref, err)
		}
		if gotName != wantName {
			continue
		}
		if gotVersion != wantVersion {
			return fmt.Errorf(
				"%s: skill %s wants %s at version %s but composition already has %s (existing ref %s)",
				errCodeVersionDivergence, skillID, wantName, wantVersion, gotVersion, existing.Ref,
			)
		}
		// Same id. Compare configs.
		eq, err := spec.CanonicalConfigsEqual(existing.Config, rc.Config)
		if err != nil {
			return fmt.Errorf("%s: %w", errCodeConfigDivergence, err)
		}
		if !eq {
			return fmt.Errorf(
				"%s: skill %s wants %s with a different config than the one already in the composition",
				errCodeConfigDivergence, skillID, rc.ID,
			)
		}
		return nil // idempotent — user or earlier skill already declared this; preserve existing attribution.
	}

	// Not present; append and attribute to this skill.
	*target = append(*target, spec.ComponentRef{Ref: string(rc.ID), Config: rc.Config})
	injectedBy[kindLabel+":"+string(rc.ID)] = skillID
	return nil
}

// injectOutputContract applies the singular output contract slot.
func injectOutputContract(out *ExpandedSpec, skillID registry.ID, rc registry.RequiredComponent) error {
	if rc.ID == "" {
		return fmt.Errorf("output_contract skill %s: RequiredOutputContract missing ID", skillID)
	}
	if _, _, err := spec.ParseID(string(rc.ID)); err != nil {
		return fmt.Errorf("output_contract skill %s: %w", skillID, err)
	}

	if out.Spec.OutputContract == nil {
		out.Spec.OutputContract = &spec.ComponentRef{Ref: string(rc.ID), Config: rc.Config}
		out.InjectedBy["output_contract:"+string(rc.ID)] = skillID
		return nil
	}

	// An output contract is already present — from the user or a previous skill.
	if out.Spec.OutputContract.Ref != string(rc.ID) {
		// Tell the author which collision path applied.
		if _, skillInjected := out.InjectedBy["output_contract:"+out.Spec.OutputContract.Ref]; skillInjected {
			return fmt.Errorf(
				"%s: skills require different output contracts (%s vs %s)",
				errCodeOutputMultiple, out.Spec.OutputContract.Ref, rc.ID,
			)
		}
		return fmt.Errorf(
			"%s: skill %s requires output contract %s but user declared %s",
			errCodeOutputUserOverride, skillID, rc.ID, out.Spec.OutputContract.Ref,
		)
	}
	// Same id. Compare configs.
	eq, err := spec.CanonicalConfigsEqual(out.Spec.OutputContract.Config, rc.Config)
	if err != nil {
		return fmt.Errorf("%s: %w", errCodeOutputUserOverride, err)
	}
	if !eq {
		// Disambiguate based on whether the existing entry came from user or another skill.
		if _, skillInjected := out.InjectedBy["output_contract:"+out.Spec.OutputContract.Ref]; skillInjected {
			return fmt.Errorf(
				"%s: two skills require %s with divergent configs",
				errCodeOutputMultiple, rc.ID,
			)
		}
		return fmt.Errorf(
			"%s: skill %s requires %s but user config differs",
			errCodeOutputUserOverride, skillID, rc.ID,
		)
	}
	return nil // idempotent.
}
```

Also update [`build/resolver.go`](../../../build/resolver.go) `resolved` struct — add fields for skills and output contract. They stay zero for now; Task 06 populates them. Find the struct (lines 17-63) and add at the end (before `specSnapshot`):

```go
	skills            []registry.Skill
	skillIDs          []registry.ID
	skillCfgs         []map[string]any

	outputContract    *registry.OutputContract
	outputContractID  registry.ID
	outputContractCfg map[string]any
```

- [ ] **Step 13: Run the happy-path tests to verify they pass**

Run: `go test ./build/ -run 'TestExpandSkills_(Injects|Idempotent|NoSkills)' -v`

Expected: PASS (3 tests).

### Part D — Conflict cases

- [ ] **Step 14: Write failing tests for every conflict row**

Append to `build/expand_test.go`:

```go
func TestExpandSkills_ConflictVersionDivergence(t *testing.T) {
	s := baseSpec()
	s.Tools = []spec.ComponentRef{{Ref: "toolpack.http-get@1.0.0"}}
	s.Skills = []spec.ComponentRef{{Ref: "skill.has-http@1.0.0"}}

	skillValue := registry.Skill{
		RequiredTools: []registry.RequiredComponent{
			{ID: "toolpack.http-get@2.0.0"},
		},
	}
	r := regWithSkill(t, "skill.has-http@1.0.0", skillValue)

	_, err := expandSkills(context.Background(), s, r)
	if err == nil {
		t.Fatal("want version-divergence error, got nil")
	}
	if !strings.Contains(err.Error(), "skill_conflict_version_divergence") {
		t.Errorf("want skill_conflict_version_divergence, got: %v", err)
	}
}

func TestExpandSkills_ConflictConfigDivergence(t *testing.T) {
	s := baseSpec()
	s.Policies = []spec.ComponentRef{{
		Ref:    "policypack.pii-redaction@1.0.0",
		Config: map[string]any{"strictness": "low"},
	}}
	s.Skills = []spec.ComponentRef{{Ref: "skill.needs-medium@1.0.0"}}

	skillValue := registry.Skill{
		RequiredPolicies: []registry.RequiredComponent{
			{
				ID:     "policypack.pii-redaction@1.0.0",
				Config: map[string]any{"strictness": "medium"},
			},
		},
	}
	r := regWithSkill(t, "skill.needs-medium@1.0.0", skillValue)

	_, err := expandSkills(context.Background(), s, r)
	if err == nil {
		t.Fatal("want config-divergence error")
	}
	if !strings.Contains(err.Error(), "skill_conflict_config_divergence") {
		t.Errorf("want skill_conflict_config_divergence, got: %v", err)
	}
}

func TestExpandSkills_IdempotentOverlapKeepsOne(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{
		{Ref: "skill.a@1.0.0"},
		{Ref: "skill.b@1.0.0"},
	}

	shared := registry.RequiredComponent{
		ID:     "policypack.pii-redaction@1.0.0",
		Config: map[string]any{"strictness": "medium"},
	}
	aVal := registry.Skill{RequiredPolicies: []registry.RequiredComponent{shared}}
	bVal := registry.Skill{RequiredPolicies: []registry.RequiredComponent{shared}}

	r := registry.NewComponentRegistry()
	if err := r.RegisterSkill(fakeSkill{id: "skill.a@1.0.0", s: aVal}); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterSkill(fakeSkill{id: "skill.b@1.0.0", s: bVal}); err != nil {
		t.Fatal(err)
	}

	got, err := expandSkills(context.Background(), s, r)
	if err != nil {
		t.Fatalf("expandSkills: %v", err)
	}
	if len(got.Spec.Policies) != 1 {
		t.Errorf("Policies: want 1 (deduped), got %d", len(got.Spec.Policies))
	}
	// First skill in declaration order wins attribution.
	if got.InjectedBy["policy_pack:policypack.pii-redaction@1.0.0"] != "skill.a@1.0.0" {
		t.Errorf("InjectedBy: want skill.a, got %v", got.InjectedBy)
	}
}

func TestExpandSkills_ConflictOutputContractMultiple(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{
		{Ref: "skill.a@1.0.0"},
		{Ref: "skill.b@1.0.0"},
	}

	aVal := registry.Skill{
		RequiredOutputContract: &registry.RequiredComponent{ID: "outputcontract.json@1.0.0"},
	}
	bVal := registry.Skill{
		RequiredOutputContract: &registry.RequiredComponent{ID: "outputcontract.proto@1.0.0"},
	}

	r := registry.NewComponentRegistry()
	_ = r.RegisterSkill(fakeSkill{id: "skill.a@1.0.0", s: aVal})
	_ = r.RegisterSkill(fakeSkill{id: "skill.b@1.0.0", s: bVal})

	_, err := expandSkills(context.Background(), s, r)
	if err == nil {
		t.Fatal("want multiple-contract error")
	}
	if !strings.Contains(err.Error(), "skill_conflict_output_contract_multiple") {
		t.Errorf("want skill_conflict_output_contract_multiple, got: %v", err)
	}
}

func TestExpandSkills_ConflictOutputContractUserOverride(t *testing.T) {
	s := baseSpec()
	s.OutputContract = &spec.ComponentRef{Ref: "outputcontract.user@1.0.0"}
	s.Skills = []spec.ComponentRef{{Ref: "skill.a@1.0.0"}}

	aVal := registry.Skill{
		RequiredOutputContract: &registry.RequiredComponent{ID: "outputcontract.skill@1.0.0"},
	}
	r := regWithSkill(t, "skill.a@1.0.0", aVal)

	_, err := expandSkills(context.Background(), s, r)
	if err == nil {
		t.Fatal("want user-override error")
	}
	if !strings.Contains(err.Error(), "skill_conflict_output_contract_user_override") {
		t.Errorf("want skill_conflict_output_contract_user_override, got: %v", err)
	}
}

func TestExpandSkills_OutputContractIdempotent(t *testing.T) {
	s := baseSpec()
	s.OutputContract = &spec.ComponentRef{
		Ref: "outputcontract.json@1.0.0",
		Config: map[string]any{"schema": map[string]any{"type": "object"}},
	}
	s.Skills = []spec.ComponentRef{{Ref: "skill.a@1.0.0"}}

	aVal := registry.Skill{
		RequiredOutputContract: &registry.RequiredComponent{
			ID:     "outputcontract.json@1.0.0",
			Config: map[string]any{"schema": map[string]any{"type": "object"}},
		},
	}
	r := regWithSkill(t, "skill.a@1.0.0", aVal)

	got, err := expandSkills(context.Background(), s, r)
	if err != nil {
		t.Fatalf("expandSkills: %v", err)
	}
	if got.Spec.OutputContract == nil || got.Spec.OutputContract.Ref != "outputcontract.json@1.0.0" {
		t.Errorf("OutputContract: %+v", got.Spec.OutputContract)
	}
	// User-declared wins attribution — InjectedBy must be empty for this slot.
	if _, found := got.InjectedBy["output_contract:outputcontract.json@1.0.0"]; found {
		t.Errorf("InjectedBy must be empty when user declared identical, got %v", got.InjectedBy)
	}
}

func TestExpandSkills_UnresolvedRequiredSkill(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{{Ref: "skill.missing@1.0.0"}}
	r := registry.NewComponentRegistry()

	_, err := expandSkills(context.Background(), s, r)
	if err == nil {
		t.Fatal("want not-found error")
	}
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("want wraps ErrNotFound, got: %v", err)
	}
}

func TestExpandSkills_EmptyContribution(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{{Ref: "skill.empty@1.0.0"}}

	emptyVal := registry.Skill{
		Descriptor: registry.SkillDescriptor{Name: "empty"},
		// no PromptFragment, no RequiredTools, no RequiredPolicies, no RequiredOutputContract
	}
	r := regWithSkill(t, "skill.empty@1.0.0", emptyVal)

	_, err := expandSkills(context.Background(), s, r)
	if err == nil {
		t.Fatal("want empty-contribution error")
	}
	if !strings.Contains(err.Error(), "skill_empty_contribution") {
		t.Errorf("want skill_empty_contribution, got: %v", err)
	}
}

func TestExpandSkills_PromptOnlyContributionAccepted(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{{Ref: "skill.prompt-only@1.0.0"}}

	val := registry.Skill{
		PromptFragment: "Think step by step.",
	}
	r := regWithSkill(t, "skill.prompt-only@1.0.0", val)

	got, err := expandSkills(context.Background(), s, r)
	if err != nil {
		t.Fatalf("expandSkills: %v", err)
	}
	if len(got.Skills) != 1 {
		t.Errorf("Skills: want 1, got %d", len(got.Skills))
	}
}

func TestExpandSkills_DeclarationOrderDeterministic(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{
		{Ref: "skill.b@1.0.0"},
		{Ref: "skill.a@1.0.0"},
	}

	aVal := registry.Skill{RequiredTools: []registry.RequiredComponent{{ID: "toolpack.alpha@1.0.0"}}}
	bVal := registry.Skill{RequiredTools: []registry.RequiredComponent{{ID: "toolpack.beta@1.0.0"}}}

	r := registry.NewComponentRegistry()
	_ = r.RegisterSkill(fakeSkill{id: "skill.a@1.0.0", s: aVal})
	_ = r.RegisterSkill(fakeSkill{id: "skill.b@1.0.0", s: bVal})

	got, err := expandSkills(context.Background(), s, r)
	if err != nil {
		t.Fatalf("expandSkills: %v", err)
	}
	if len(got.Spec.Tools) != 2 {
		t.Fatalf("Tools: want 2, got %d", len(got.Spec.Tools))
	}
	// Skills[] order: b then a. Injected tools must follow that order.
	if got.Spec.Tools[0].Ref != "toolpack.beta@1.0.0" {
		t.Errorf("Tools[0] should come from skill.b, got %q", got.Spec.Tools[0].Ref)
	}
	if got.Spec.Tools[1].Ref != "toolpack.alpha@1.0.0" {
		t.Errorf("Tools[1] should come from skill.a, got %q", got.Spec.Tools[1].Ref)
	}
}
```

- [ ] **Step 15: Run to verify all pass**

Run: `go test ./build/ -run 'TestExpandSkills_' -v`

Expected: PASS — all 12 tests (happy-path + all conflict rows + determinism).

- [ ] **Step 16: Run full package vet + test**

Run: `go vet ./build/... && go test ./build/... -v`

Expected: all build-package tests still pass (the pre-existing ones rely on `resolved` which we just extended — check that struct literal assignments still compile).

If struct-literal compilation fails in existing tests, convert to named-field initialization (they are all in `build/build_test.go` / `build_hash_test.go`). The safe form is already used; no change needed if fields are appended at the end. Verify with `go build ./...` first.

- [ ] **Step 17: Commit**

```bash
git add build/expand.go build/expand_test.go build/resolver.go
git commit -m "$(cat <<'EOF'
feat(build): skill expansion core with strict conflict detection

expand.go resolves spec.skills[] through the registry, auto-injects
RequiredTools, RequiredPolicies, and RequiredOutputContract into the
effective composition, and fails the build on every row of the
expansion-semantics table: empty contribution, unresolved ref, version
divergence, config divergence, multiple output contracts, user override.

ExpandedSpec carries the rewritten AgentSpec + resolved skills +
InjectedBy attribution map consumed later by buildManifest.

resolver.go gains three skill fields and three output-contract fields
on the resolved struct; populated in Task 06 when Build is wired.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

## Expected state after this task

- `spec/canonical.go`: `CanonicalConfigsEqual` helper + its test.
- `build/expand.go`: `ExpandedSpec`, `ResolvedSkill`, `ResolvedOutputContract`, `expandSkills`, `isEmptyContribution`, `injectRequired`, `injectOutputContract`, 6 named error-code constants.
- `build/expand_test.go`: 12 unit tests covering every expansion-semantics row + declaration-order determinism + prompt-only contribution acceptance.
- `build/resolver.go`: `resolved` struct extended with skill + output-contract fields (unused until Task 06).
- `go test ./build/...` green (existing tests still pass since new fields have zero defaults).
- Two commits added to the branch.
