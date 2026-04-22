// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/praxis-os/praxis-forge/factories/outputcontractjsonschema"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
	"github.com/praxis-os/praxis-forge/factories/toolpackhttpget"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)


// --- Test fixtures: PASS scenarios ---

func TestSkillsFixture_BasicSkill(t *testing.T) {
	s, _ := spec.LoadSpec("../spec/testdata/skills/basic-skill/spec.yaml")

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.json-schema@1.0.0"))
	// Register custom skill with matching outputContract config
	_ = r.RegisterSkill(fakeSkill{id: "skill.structured-output@1.0.0", s: registry.Skill{
		PromptFragment: "Respond with JSON matching the required schema. Do not include prose outside the JSON.",
		RequiredPolicies: []registry.RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &registry.RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			Config: map[string]any{"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{"answer": map[string]any{"type": "string"}},
				"required": []string{"answer"},
			}},
		},
		Descriptor: registry.SkillDescriptor{
			Name: "structured-output",
			Owner: "core",
			Summary: "Emit JSON matching a schema; default PII-redaction policy.",
			Tags: []string{"structured", "json", "governance"},
		},
	}})

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	gotHash := built.Manifest.ExpandedHash
	wantHashBytes, err := os.ReadFile("../spec/testdata/skills/basic-skill/want.expanded.hash")
	if err != nil {
		t.Fatalf("missing want.expanded.hash: %v", err)
	}
	wantHash := strings.TrimSpace(string(wantHashBytes))

	if os.Getenv("WRITE_GOLDEN") == "1" {
		_ = os.WriteFile("../spec/testdata/skills/basic-skill/want.expanded.hash", []byte(gotHash+"\n"), 0o600)
		t.Logf("WRITE_GOLDEN: wrote hash %s", gotHash)
	} else if gotHash != wantHash {
		t.Errorf("ExpandedHash:\n  want: %s\n  got:  %s", wantHash, gotHash)
	}
}

func TestSkillsFixture_AutoInjectTool(t *testing.T) {
	s, _ := spec.LoadSpec("../spec/testdata/skills/auto-inject-tool/spec.yaml")

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterToolPack(toolpackhttpget.NewFactory("toolpack.http-get@1.0.0"))
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.json-schema@1.0.0"))
	_ = r.RegisterSkill(fakeSkill{id: "skill.structured-output@1.0.0", s: registry.Skill{
		PromptFragment: "Respond with JSON matching the required schema. Do not include prose outside the JSON.",
		RequiredPolicies: []registry.RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &registry.RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			Config: map[string]any{"schema": map[string]any{"type": "string"}},
		},
		Descriptor: registry.SkillDescriptor{Name: "structured-output", Owner: "core", Summary: "Emit JSON matching a schema; default PII-redaction policy.", Tags: []string{"structured", "json", "governance"}},
	}})

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	gotHash := built.Manifest.ExpandedHash
	wantHashBytes, err := os.ReadFile("../spec/testdata/skills/auto-inject-tool/want.expanded.hash")
	if err != nil {
		t.Fatalf("missing want.expanded.hash: %v", err)
	}
	wantHash := strings.TrimSpace(string(wantHashBytes))

	if os.Getenv("WRITE_GOLDEN") == "1" {
		_ = os.WriteFile("../spec/testdata/skills/auto-inject-tool/want.expanded.hash", []byte(gotHash+"\n"), 0o600)
		t.Logf("WRITE_GOLDEN: wrote hash %s", gotHash)
	} else if gotHash != wantHash {
		t.Errorf("ExpandedHash:\n  want: %s\n  got:  %s", wantHash, gotHash)
	}
}

func TestSkillsFixture_IdempotentOverlap(t *testing.T) {
	s, _ := spec.LoadSpec("../spec/testdata/skills/idempotent-overlap/spec.yaml")

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.json-schema@1.0.0"))
	_ = r.RegisterSkill(fakeSkill{id: "skill.structured-output@1.0.0", s: registry.Skill{
		PromptFragment: "Respond with JSON matching the required schema. Do not include prose outside the JSON.",
		RequiredPolicies: []registry.RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &registry.RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			Config: map[string]any{"schema": map[string]any{"type": "string"}},
		},
		Descriptor: registry.SkillDescriptor{Name: "structured-output", Owner: "core", Summary: "Emit JSON matching a schema; default PII-redaction policy.", Tags: []string{"structured", "json", "governance"}},
	}})

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	gotHash := built.Manifest.ExpandedHash
	wantHashBytes, err := os.ReadFile("../spec/testdata/skills/idempotent-overlap/want.expanded.hash")
	if err != nil {
		t.Fatalf("missing want.expanded.hash: %v", err)
	}
	wantHash := strings.TrimSpace(string(wantHashBytes))

	if os.Getenv("WRITE_GOLDEN") == "1" {
		_ = os.WriteFile("../spec/testdata/skills/idempotent-overlap/want.expanded.hash", []byte(gotHash+"\n"), 0o600)
		t.Logf("WRITE_GOLDEN: wrote hash %s", gotHash)
	} else if gotHash != wantHash {
		t.Errorf("ExpandedHash:\n  want: %s\n  got:  %s", wantHash, gotHash)
	}

	// Also assert attribution: the policy was user-declared, so
	// InjectedBySkill must be empty for it.
	for _, rc := range built.Manifest.Resolved {
		if rc.Kind == string(registry.KindPolicyPack) && rc.InjectedBySkill != "" {
			t.Errorf("user-declared policy should not carry InjectedBySkill: %+v", rc)
		}
	}
}

func TestSkillsFixture_OutputContractAutoInject(t *testing.T) {
	s, _ := spec.LoadSpec("../spec/testdata/skills/output-contract-auto-inject/spec.yaml")

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.json-schema@1.0.0"))
	_ = r.RegisterSkill(fakeSkill{id: "skill.structured-output@1.0.0", s: registry.Skill{
		PromptFragment: "Respond with JSON matching the required schema. Do not include prose outside the JSON.",
		RequiredPolicies: []registry.RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &registry.RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			Config: map[string]any{"schema": map[string]any{"type": "object"}},
		},
		Descriptor: registry.SkillDescriptor{Name: "structured-output", Owner: "core", Summary: "Emit JSON matching a schema; default PII-redaction policy.", Tags: []string{"structured", "json", "governance"}},
	}})

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	gotHash := built.Manifest.ExpandedHash
	wantHashBytes, err := os.ReadFile("../spec/testdata/skills/output-contract-auto-inject/want.expanded.hash")
	if err != nil {
		t.Fatalf("missing want.expanded.hash: %v", err)
	}
	wantHash := strings.TrimSpace(string(wantHashBytes))

	if os.Getenv("WRITE_GOLDEN") == "1" {
		_ = os.WriteFile("../spec/testdata/skills/output-contract-auto-inject/want.expanded.hash", []byte(gotHash+"\n"), 0o600)
		t.Logf("WRITE_GOLDEN: wrote hash %s", gotHash)
	} else if gotHash != wantHash {
		t.Errorf("ExpandedHash:\n  want: %s\n  got:  %s", wantHash, gotHash)
	}
}

func TestSkillsFixture_OutputContractUserOverride(t *testing.T) {
	s, err := spec.LoadSpec("../spec/testdata/skills/output-contract-user-override/spec.yaml")
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.json-schema@1.0.0"))
	_ = r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.alt@1.0.0"))
	_ = r.RegisterSkill(fakeSkill{id: "skill.structured-output@1.0.0", s: registry.Skill{
		PromptFragment: "Respond with JSON matching the required schema. Do not include prose outside the JSON.",
		RequiredPolicies: []registry.RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &registry.RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			Config: map[string]any{"schema": map[string]any{"type": "object"}},
		},
		Descriptor: registry.SkillDescriptor{Name: "structured-output", Owner: "core", Summary: "Emit JSON matching a schema; default PII-redaction policy.", Tags: []string{"structured", "json", "governance"}},
	}})

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	_, err = Build(context.Background(), ns, r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	want := "skill_conflict_output_contract_user_override"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("err %q does not contain %q", err.Error(), want)
	}
}

func TestSkillsFixture_ConflictConfig(t *testing.T) {
	s, err := spec.LoadSpec("../spec/testdata/skills/conflict-config/spec.yaml")
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.json-schema@1.0.0"))
	_ = r.RegisterSkill(fakeSkill{id: "skill.structured-output@1.0.0", s: registry.Skill{
		PromptFragment: "Respond with JSON matching the required schema. Do not include prose outside the JSON.",
		RequiredPolicies: []registry.RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &registry.RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			Config: map[string]any{"schema": map[string]any{"type": "object"}},
		},
		Descriptor: registry.SkillDescriptor{Name: "structured-output", Owner: "core", Summary: "Emit JSON matching a schema; default PII-redaction policy.", Tags: []string{"structured", "json", "governance"}},
	}})

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	_, err = Build(context.Background(), ns, r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	want := "skill_conflict_config_divergence"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("err %q does not contain %q", err.Error(), want)
	}
}

// --- Test fixtures: ERROR scenarios with custom registries ---

func TestSkillsFixture_ConflictVersion(t *testing.T) {
	// Custom fixture: user-declared toolpack.http-get@1 plus skill that
	// requires @2. Uses a local registry since newSkillFixtureRegistry's
	// shared vertical-slice skill does not require a toolpack.
	s, err := spec.LoadSpec("../spec/testdata/skills/conflict-version/spec.yaml")
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterToolPack(toolpackhttpget.NewFactory("toolpack.http-get@1.0.0"))
	_ = r.RegisterToolPack(toolpackhttpget.NewFactory("toolpack.http-get@2.0.0"))
	_ = r.RegisterSkill(fakeSkill{id: "skill.needs-http2@1.0.0", s: registry.Skill{
		PromptFragment: "needs v2",
		RequiredTools:  []registry.RequiredComponent{{ID: "toolpack.http-get@2.0.0"}},
	}})

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	_, err = Build(context.Background(), ns, r)
	if err == nil {
		t.Fatal("want version-divergence error")
	}
	want, _ := os.ReadFile("../spec/testdata/skills/conflict-version/want.err.txt")
	if !strings.Contains(err.Error(), strings.TrimSpace(string(want))) {
		t.Errorf("err %q missing %q", err, want)
	}
}

func TestSkillsFixture_EmptyContribution(t *testing.T) {
	s, err := spec.LoadSpec("../spec/testdata/skills/empty-contribution/spec.yaml")
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterSkill(fakeSkill{id: "skill.empty@1.0.0", s: registry.Skill{}})

	ns, _ := spec.Normalize(context.Background(), s, nil, nil)
	_, err = Build(context.Background(), ns, r)
	if err == nil {
		t.Fatal("want empty-contribution error")
	}
	if !strings.Contains(err.Error(), "skill_empty_contribution") {
		t.Errorf("err %q missing skill_empty_contribution", err)
	}
}

// --- Test fixtures: multi-input and fragment tests ---

func TestSkillsFixture_ExpandedHashStable(t *testing.T) {
	makeRegistry := func() *registry.ComponentRegistry {
		r := registry.NewComponentRegistry()
		_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
		_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
		_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
		_ = r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.json-schema@1.0.0"))
		_ = r.RegisterSkill(fakeSkill{id: "skill.structured-output@1.0.0", s: registry.Skill{
			PromptFragment: "Respond with JSON matching the required schema. Do not include prose outside the JSON.",
			RequiredPolicies: []registry.RequiredComponent{
				{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
			},
			RequiredOutputContract: &registry.RequiredComponent{
				ID: "outputcontract.json-schema@1.0.0",
				Config: map[string]any{"schema": map[string]any{"type": "object"}},
			},
			Descriptor: registry.SkillDescriptor{Name: "structured-output", Owner: "core", Summary: "Emit JSON matching a schema; default PII-redaction policy.", Tags: []string{"structured", "json", "governance"}},
		}})
		return r
	}

	loadBuild := func(p string) string {
		s, err := spec.LoadSpec(p)
		if err != nil {
			t.Fatalf("LoadSpec %s: %v", p, err)
		}
		ns, err := spec.Normalize(context.Background(), s, nil, nil)
		if err != nil {
			t.Fatalf("Normalize %s: %v", p, err)
		}
		// Each call uses a fresh registry — Freeze is irreversible, so
		// reusing a registry across Build calls is unsafe.
		built, err := Build(context.Background(), ns, makeRegistry())
		if err != nil {
			t.Fatalf("Build %s: %v", p, err)
		}
		return built.Manifest.ExpandedHash
	}

	a := loadBuild("../spec/testdata/skills/expanded-hash-stable/base-a.yaml")
	b := loadBuild("../spec/testdata/skills/expanded-hash-stable/base-b.yaml")
	if a != b {
		t.Errorf("ExpandedHash should match across equivalent compositions:\n  a=%s\n  b=%s", a, b)
	}
}

func TestSkillsFixture_FragmentDedup(t *testing.T) {
	s, _ := spec.LoadSpec("../spec/testdata/skills/fragment-dedup/spec.yaml")

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	shared := registry.Skill{PromptFragment: "Be safe."}
	_ = r.RegisterSkill(fakeSkill{id: "skill.safe-a@1.0.0", s: shared})
	_ = r.RegisterSkill(fakeSkill{id: "skill.safe-b@1.0.0", s: shared})

	ns, _ := spec.Normalize(context.Background(), s, nil, nil)
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatal(err)
	}
	want := "hi\n\nBe safe."
	if built.SystemPrompt != want {
		t.Errorf("SystemPrompt:\n  want: %q\n  got:  %q", want, built.SystemPrompt)
	}
	// Manifest Resolved must still list both skills.
	var skillCount int
	for _, rc := range built.Manifest.Resolved {
		if rc.Kind == string(registry.KindSkill) {
			skillCount++
		}
	}
	if skillCount != 2 {
		t.Errorf("want 2 skill entries in Resolved (audit preserved), got %d", skillCount)
	}
}

func TestSkillsFixture_FragmentOrder(t *testing.T) {
	s, _ := spec.LoadSpec("../spec/testdata/skills/fragment-order/spec.yaml")

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterSkill(fakeSkill{id: "skill.first@1.0.0", s: registry.Skill{PromptFragment: "first"}})
	_ = r.RegisterSkill(fakeSkill{id: "skill.second@1.0.0", s: registry.Skill{PromptFragment: "second"}})

	ns, _ := spec.Normalize(context.Background(), s, nil, nil)
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatal(err)
	}
	want := "hi\n\nfirst\n\nsecond"
	if built.SystemPrompt != want {
		t.Errorf("SystemPrompt:\n  want: %q\n  got:  %q", want, built.SystemPrompt)
	}
}

// --- Manifest attribution integration test ---

func TestBuild_SkillAttributionInManifest(t *testing.T) {
	s, _ := spec.LoadSpec("../spec/testdata/skills/basic-skill/spec.yaml")

	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.json-schema@1.0.0"))
	_ = r.RegisterSkill(fakeSkill{id: "skill.structured-output@1.0.0", s: registry.Skill{
		PromptFragment: "Respond with JSON matching the required schema. Do not include prose outside the JSON.",
		RequiredPolicies: []registry.RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &registry.RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			Config: map[string]any{"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{"answer": map[string]any{"type": "string"}},
				"required": []string{"answer"},
			}},
		},
		Descriptor: registry.SkillDescriptor{
			Name: "structured-output",
			Owner: "core",
			Summary: "Emit JSON matching a schema; default PII-redaction policy.",
			Tags: []string{"structured", "json", "governance"},
		},
	}})

	ns, _ := spec.Normalize(context.Background(), s, nil, nil)
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatal(err)
	}

	// User-declared policy: InjectedBySkill empty.
	// Skill row: Kind=skill, ID=skill.structured-output@1.0.0.
	var sawSkill bool
	for _, rc := range built.Manifest.Resolved {
		if rc.Kind == string(registry.KindSkill) && rc.ID == "skill.structured-output@1.0.0" {
			sawSkill = true
		}
		if rc.Kind == string(registry.KindPolicyPack) {
			if rc.InjectedBySkill != "" {
				t.Errorf("user-declared policy shouldn't be attributed; got %q", rc.InjectedBySkill)
			}
		}
	}
	if !sawSkill {
		t.Error("manifest missing Resolved entry for skill.structured-output")
	}

	// ExpandedHash must be set.
	if built.Manifest.ExpandedHash == "" {
		t.Error("ExpandedHash should be set when skills exist")
	}
}
