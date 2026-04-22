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

// --- Part A: resolveOutputContract tests ---

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

// --- Part B: Build integration tests ---

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

