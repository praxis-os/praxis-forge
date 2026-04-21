// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"sync"
)

// ComponentRegistry is a typed, per-kind factory registry. Construct with
// NewComponentRegistry; populate during program start; pass to forge.Build
// which will Freeze it on first use.
type ComponentRegistry struct {
	mu     sync.RWMutex
	frozen bool

	providers         map[ID]ProviderFactory
	promptAssets      map[ID]PromptAssetFactory
	toolPacks         map[ID]ToolPackFactory
	policyPacks       map[ID]PolicyPackFactory
	preLLMFilters     map[ID]PreLLMFilterFactory
	preToolFilters    map[ID]PreToolFilterFactory
	postToolFilters   map[ID]PostToolFilterFactory
	budgetProfiles    map[ID]BudgetProfileFactory
	telemetryProfiles map[ID]TelemetryProfileFactory
	credResolvers     map[ID]CredentialResolverFactory
	identitySigners   map[ID]IdentitySignerFactory
	skills            map[ID]SkillFactory
	outputContracts   map[ID]OutputContractFactory
}

func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		providers:         map[ID]ProviderFactory{},
		promptAssets:      map[ID]PromptAssetFactory{},
		toolPacks:         map[ID]ToolPackFactory{},
		policyPacks:       map[ID]PolicyPackFactory{},
		preLLMFilters:     map[ID]PreLLMFilterFactory{},
		preToolFilters:    map[ID]PreToolFilterFactory{},
		postToolFilters:   map[ID]PostToolFilterFactory{},
		budgetProfiles:    map[ID]BudgetProfileFactory{},
		telemetryProfiles: map[ID]TelemetryProfileFactory{},
		credResolvers:     map[ID]CredentialResolverFactory{},
		identitySigners:   map[ID]IdentitySignerFactory{},
		skills:            map[ID]SkillFactory{},
		outputContracts:   map[ID]OutputContractFactory{},
	}
}

// Freeze blocks further registration. Idempotent.
func (r *ComponentRegistry) Freeze() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
}

// --- Provider ---

func (r *ComponentRegistry) RegisterProvider(f ProviderFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.providers[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindProvider, f.ID())
	}
	r.providers[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) Provider(id ID) (ProviderFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindProvider, id)
	}
	return f, nil
}

// --- PromptAsset ---

func (r *ComponentRegistry) RegisterPromptAsset(f PromptAssetFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.promptAssets[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindPromptAsset, f.ID())
	}
	r.promptAssets[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) PromptAsset(id ID) (PromptAssetFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.promptAssets[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindPromptAsset, id)
	}
	return f, nil
}

// --- ToolPack ---

func (r *ComponentRegistry) RegisterToolPack(f ToolPackFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.toolPacks[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindToolPack, f.ID())
	}
	r.toolPacks[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) ToolPack(id ID) (ToolPackFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.toolPacks[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindToolPack, id)
	}
	return f, nil
}

// --- PolicyPack ---

func (r *ComponentRegistry) RegisterPolicyPack(f PolicyPackFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.policyPacks[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindPolicyPack, f.ID())
	}
	r.policyPacks[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) PolicyPack(id ID) (PolicyPackFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.policyPacks[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindPolicyPack, id)
	}
	return f, nil
}

// --- PreLLMFilter ---

func (r *ComponentRegistry) RegisterPreLLMFilter(f PreLLMFilterFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.preLLMFilters[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindPreLLMFilter, f.ID())
	}
	r.preLLMFilters[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) PreLLMFilter(id ID) (PreLLMFilterFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.preLLMFilters[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindPreLLMFilter, id)
	}
	return f, nil
}

// --- PreToolFilter ---

func (r *ComponentRegistry) RegisterPreToolFilter(f PreToolFilterFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.preToolFilters[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindPreToolFilter, f.ID())
	}
	r.preToolFilters[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) PreToolFilter(id ID) (PreToolFilterFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.preToolFilters[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindPreToolFilter, id)
	}
	return f, nil
}

// --- PostToolFilter ---

func (r *ComponentRegistry) RegisterPostToolFilter(f PostToolFilterFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.postToolFilters[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindPostToolFilter, f.ID())
	}
	r.postToolFilters[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) PostToolFilter(id ID) (PostToolFilterFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.postToolFilters[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindPostToolFilter, id)
	}
	return f, nil
}

// --- BudgetProfile ---

func (r *ComponentRegistry) RegisterBudgetProfile(f BudgetProfileFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.budgetProfiles[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindBudgetProfile, f.ID())
	}
	r.budgetProfiles[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) BudgetProfile(id ID) (BudgetProfileFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.budgetProfiles[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindBudgetProfile, id)
	}
	return f, nil
}

// --- TelemetryProfile ---

func (r *ComponentRegistry) RegisterTelemetryProfile(f TelemetryProfileFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.telemetryProfiles[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindTelemetryProfile, f.ID())
	}
	r.telemetryProfiles[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) TelemetryProfile(id ID) (TelemetryProfileFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.telemetryProfiles[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindTelemetryProfile, id)
	}
	return f, nil
}

// --- CredentialResolver ---

func (r *ComponentRegistry) RegisterCredentialResolver(f CredentialResolverFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.credResolvers[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindCredentialResolver, f.ID())
	}
	r.credResolvers[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) CredentialResolver(id ID) (CredentialResolverFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.credResolvers[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindCredentialResolver, id)
	}
	return f, nil
}

// --- IdentitySigner ---

func (r *ComponentRegistry) RegisterIdentitySigner(f IdentitySignerFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.identitySigners[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindIdentitySigner, f.ID())
	}
	r.identitySigners[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) IdentitySigner(id ID) (IdentitySignerFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.identitySigners[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindIdentitySigner, id)
	}
	return f, nil
}

// --- Skill ---

func (r *ComponentRegistry) RegisterSkill(f SkillFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.skills[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindSkill, f.ID())
	}
	r.skills[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) Skill(id ID) (SkillFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.skills[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindSkill, id)
	}
	return f, nil
}

// --- OutputContract ---

func (r *ComponentRegistry) RegisterOutputContract(f OutputContractFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.outputContracts[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindOutputContract, f.ID())
	}
	r.outputContracts[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) OutputContract(id ID) (OutputContractFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.outputContracts[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindOutputContract, id)
	}
	return f, nil
}
