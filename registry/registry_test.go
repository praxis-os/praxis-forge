// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/llm"
)

// --- Fake factories ---

const fakeDesc = "fake"

type fakeProviderFactory struct{ id ID }

func (f fakeProviderFactory) ID() ID              { return f.id }
func (f fakeProviderFactory) Description() string { return fakeDesc }
func (f fakeProviderFactory) Build(context.Context, map[string]any) (llm.Provider, error) {
	return nil, nil
}

type fakePromptAssetFactory struct{ id ID }

func (f fakePromptAssetFactory) ID() ID              { return f.id }
func (f fakePromptAssetFactory) Description() string { return fakeDesc }
func (f fakePromptAssetFactory) Build(context.Context, map[string]any) (string, error) {
	return "hi", nil
}

type fakeToolPackFactory struct{ id ID }

func (f fakeToolPackFactory) ID() ID              { return f.id }
func (f fakeToolPackFactory) Description() string { return fakeDesc }
func (f fakeToolPackFactory) Build(context.Context, map[string]any) (ToolPack, error) {
	return ToolPack{}, nil
}

type fakePolicyPackFactory struct{ id ID }

func (f fakePolicyPackFactory) ID() ID              { return f.id }
func (f fakePolicyPackFactory) Description() string { return fakeDesc }
func (f fakePolicyPackFactory) Build(context.Context, map[string]any) (PolicyPack, error) {
	return PolicyPack{}, nil
}

type fakePreLLMFilterFactory struct{ id ID }

func (f fakePreLLMFilterFactory) ID() ID              { return f.id }
func (f fakePreLLMFilterFactory) Description() string { return fakeDesc }
func (f fakePreLLMFilterFactory) Build(context.Context, map[string]any) (hooks.PreLLMFilter, error) {
	return nil, nil
}

type fakePreToolFilterFactory struct{ id ID }

func (f fakePreToolFilterFactory) ID() ID              { return f.id }
func (f fakePreToolFilterFactory) Description() string { return fakeDesc }
func (f fakePreToolFilterFactory) Build(context.Context, map[string]any) (hooks.PreToolFilter, error) {
	return nil, nil
}

type fakePostToolFilterFactory struct{ id ID }

func (f fakePostToolFilterFactory) ID() ID              { return f.id }
func (f fakePostToolFilterFactory) Description() string { return fakeDesc }
func (f fakePostToolFilterFactory) Build(context.Context, map[string]any) (hooks.PostToolFilter, error) {
	return nil, nil
}

type fakeBudgetProfileFactory struct{ id ID }

func (f fakeBudgetProfileFactory) ID() ID              { return f.id }
func (f fakeBudgetProfileFactory) Description() string { return fakeDesc }
func (f fakeBudgetProfileFactory) Build(context.Context, map[string]any) (BudgetProfile, error) {
	return BudgetProfile{}, nil
}

type fakeTelemetryProfileFactory struct{ id ID }

func (f fakeTelemetryProfileFactory) ID() ID              { return f.id }
func (f fakeTelemetryProfileFactory) Description() string { return fakeDesc }
func (f fakeTelemetryProfileFactory) Build(context.Context, map[string]any) (TelemetryProfile, error) {
	return TelemetryProfile{}, nil
}

type fakeCredentialResolverFactory struct{ id ID }

func (f fakeCredentialResolverFactory) ID() ID              { return f.id }
func (f fakeCredentialResolverFactory) Description() string { return fakeDesc }
func (f fakeCredentialResolverFactory) Build(context.Context, map[string]any) (credentials.Resolver, error) {
	return nil, nil
}

type fakeIdentitySignerFactory struct{ id ID }

func (f fakeIdentitySignerFactory) ID() ID              { return f.id }
func (f fakeIdentitySignerFactory) Description() string { return fakeDesc }
func (f fakeIdentitySignerFactory) Build(context.Context, map[string]any) (identity.Signer, error) {
	return nil, nil
}

type fakeSkillFactory struct{ id ID }

func (f fakeSkillFactory) ID() ID              { return f.id }
func (f fakeSkillFactory) Description() string { return fakeDesc }
func (f fakeSkillFactory) Build(context.Context, map[string]any) (Skill, error) {
	return Skill{}, nil
}

type fakeOutputContractFactory struct{ id ID }

func (f fakeOutputContractFactory) ID() ID              { return f.id }
func (f fakeOutputContractFactory) Description() string { return fakeDesc }
func (f fakeOutputContractFactory) Build(context.Context, map[string]any) (OutputContract, error) {
	return OutputContract{}, nil
}

// --- Tests ---

func TestRegistry_RegisterAndLookupProvider(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeProviderFactory{id: "provider.fake@1.0.0"}
	if err := r.RegisterProvider(f); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, err := r.Provider(f.id)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.ID() != f.id {
		t.Fatalf("got id=%s", got.ID())
	}
}

func TestRegistry_Duplicate(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeProviderFactory{id: "provider.fake@1.0.0"}
	_ = r.RegisterProvider(f)
	err := r.RegisterProvider(f)
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("err=%v", err)
	}
}

func TestRegistry_NotFound(t *testing.T) {
	r := NewComponentRegistry()
	_, err := r.Provider("provider.missing@1.0.0")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestRegistry_Freeze(t *testing.T) {
	r := NewComponentRegistry()
	r.Freeze()
	err := r.RegisterProvider(fakeProviderFactory{id: "provider.a@1.0.0"})
	if !errors.Is(err, ErrRegistryFrozen) {
		t.Fatalf("err=%v", err)
	}
}

func TestRegistry_PromptAsset(t *testing.T) {
	r := NewComponentRegistry()
	f := fakePromptAssetFactory{id: "prompt.fake@1.0.0"}
	if err := r.RegisterPromptAsset(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.PromptAsset(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry_ToolPack(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeToolPackFactory{id: "toolpack.fake@1.0.0"}
	if err := r.RegisterToolPack(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.ToolPack(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry_PolicyPack(t *testing.T) {
	r := NewComponentRegistry()
	f := fakePolicyPackFactory{id: "policypack.fake@1.0.0"}
	if err := r.RegisterPolicyPack(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.PolicyPack(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry_PreLLMFilter(t *testing.T) {
	r := NewComponentRegistry()
	f := fakePreLLMFilterFactory{id: "filter.prellm@1.0.0"}
	if err := r.RegisterPreLLMFilter(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.PreLLMFilter(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry_PreToolFilter(t *testing.T) {
	r := NewComponentRegistry()
	f := fakePreToolFilterFactory{id: "filter.pretool@1.0.0"}
	if err := r.RegisterPreToolFilter(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.PreToolFilter(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry_PostToolFilter(t *testing.T) {
	r := NewComponentRegistry()
	f := fakePostToolFilterFactory{id: "filter.posttool@1.0.0"}
	if err := r.RegisterPostToolFilter(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.PostToolFilter(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry_BudgetProfile(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeBudgetProfileFactory{id: "budgetprofile.fake@1.0.0"}
	if err := r.RegisterBudgetProfile(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.BudgetProfile(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry_TelemetryProfile(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeTelemetryProfileFactory{id: "telemetryprofile.fake@1.0.0"}
	if err := r.RegisterTelemetryProfile(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.TelemetryProfile(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry_CredentialResolver(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeCredentialResolverFactory{id: "credresolver.fake@1.0.0"}
	if err := r.RegisterCredentialResolver(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.CredentialResolver(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry_IdentitySigner(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeIdentitySignerFactory{id: "identitysigner.fake@1.0.0"}
	if err := r.RegisterIdentitySigner(f); err != nil {
		t.Fatal(err)
	}
	got, err := r.IdentitySigner(f.id)
	if err != nil || got.ID() != f.id {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

// --- Skill tests ---

func TestRegister_Skill_OK(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeSkillFactory{id: "skill.fake@1.0.0"}
	if err := r.RegisterSkill(f); err != nil {
		t.Fatalf("RegisterSkill: %v", err)
	}
	got, err := r.Skill(f.id)
	if err != nil {
		t.Fatalf("Skill: %v", err)
	}
	if got.ID() != f.id {
		t.Fatalf("got id=%s", got.ID())
	}
}

func TestRegister_Skill_DuplicateFails(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeSkillFactory{id: "skill.fake@1.0.0"}
	_ = r.RegisterSkill(f)
	err := r.RegisterSkill(f)
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("err=%v", err)
	}
}

func TestSkill_NotFound(t *testing.T) {
	r := NewComponentRegistry()
	_, err := r.Skill("skill.missing@1.0.0")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestRegister_Skill_FrozenRejects(t *testing.T) {
	r := NewComponentRegistry()
	r.Freeze()
	err := r.RegisterSkill(fakeSkillFactory{id: "skill.a@1.0.0"})
	if !errors.Is(err, ErrRegistryFrozen) {
		t.Fatalf("err=%v", err)
	}
}

// --- OutputContract tests ---

func TestRegister_OutputContract_OK(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeOutputContractFactory{id: "contract.fake@1.0.0"}
	if err := r.RegisterOutputContract(f); err != nil {
		t.Fatalf("RegisterOutputContract: %v", err)
	}
	got, err := r.OutputContract(f.id)
	if err != nil {
		t.Fatalf("OutputContract: %v", err)
	}
	if got.ID() != f.id {
		t.Fatalf("got id=%s", got.ID())
	}
}

func TestRegister_OutputContract_DuplicateFails(t *testing.T) {
	r := NewComponentRegistry()
	f := fakeOutputContractFactory{id: "contract.fake@1.0.0"}
	_ = r.RegisterOutputContract(f)
	err := r.RegisterOutputContract(f)
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("err=%v", err)
	}
}

func TestOutputContract_NotFound(t *testing.T) {
	r := NewComponentRegistry()
	_, err := r.OutputContract("contract.missing@1.0.0")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestRegister_OutputContract_FrozenRejects(t *testing.T) {
	r := NewComponentRegistry()
	r.Freeze()
	err := r.RegisterOutputContract(fakeOutputContractFactory{id: "contract.a@1.0.0"})
	if !errors.Is(err, ErrRegistryFrozen) {
		t.Fatalf("err=%v", err)
	}
}
