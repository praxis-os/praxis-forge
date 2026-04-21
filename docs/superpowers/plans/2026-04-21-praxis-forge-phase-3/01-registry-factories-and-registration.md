# Task 01 — Factory interfaces and registration

Add the two new factory interfaces (`SkillFactory`, `OutputContractFactory`) alongside the existing 11 in [`registry/factories.go`](../../../registry/factories.go), then extend [`registry/registry.go`](../../../registry/registry.go) with matching maps + Register/Lookup methods. Tests mirror the pattern used by existing kinds in [`registry/registry_test.go`](../../../registry/registry_test.go).

## Files

- Modify: [`registry/factories.go`](../../../registry/factories.go)
- Modify: [`registry/registry.go`](../../../registry/registry.go)
- Modify: [`registry/registry_test.go`](../../../registry/registry_test.go)

## Background

The existing pattern (repeat for every kind):

```go
// --- ProviderFactory interface in factories.go ---
type ProviderFactory interface {
    ID() ID
    Description() string
    Build(ctx context.Context, cfg map[string]any) (llm.Provider, error)
}

// --- ComponentRegistry map + methods in registry.go ---
providers map[ID]ProviderFactory  // field inside ComponentRegistry struct

func (r *ComponentRegistry) RegisterProvider(f ProviderFactory) error { ... }
func (r *ComponentRegistry) Provider(id ID) (ProviderFactory, error) { ... }
```

Two key invariants hold throughout (see [`registry/registry.go:46-51`](../../../registry/registry.go#L46-L51) and [`registry/registry.go:55-75`](../../../registry/registry.go#L55-L75)):

1. `Freeze()` blocks further registration — idempotent.
2. Duplicate `(kind, id)` registration returns `ErrDuplicate`; missing id returns `ErrNotFound` wrapped with kind + id details.

Both invariants must carry over to the two new kinds.

## Steps

- [ ] **Step 1: Write failing interface tests**

Create [`registry/factories_test.go`](../../../registry/factories_test.go) (new file):

```go
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"testing"
)

// Compile-time assertion helpers verifying the new interfaces exist and
// have the documented shape.

type testSkillFactory struct{ id ID }

func (f testSkillFactory) ID() ID              { return f.id }
func (f testSkillFactory) Description() string { return "test skill" }
func (f testSkillFactory) Build(context.Context, map[string]any) (Skill, error) {
	return Skill{PromptFragment: "hi"}, nil
}

var _ SkillFactory = testSkillFactory{}

type testOutputContractFactory struct{ id ID }

func (f testOutputContractFactory) ID() ID              { return f.id }
func (f testOutputContractFactory) Description() string { return "test contract" }
func (f testOutputContractFactory) Build(context.Context, map[string]any) (OutputContract, error) {
	return OutputContract{Schema: map[string]any{"type": "object"}}, nil
}

var _ OutputContractFactory = testOutputContractFactory{}

func TestSkillFactory_BuildReturnsSkill(t *testing.T) {
	f := testSkillFactory{id: "skill.structured-output@1.0.0"}
	got, err := f.Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.PromptFragment != "hi" {
		t.Errorf("PromptFragment: %q", got.PromptFragment)
	}
}

func TestOutputContractFactory_BuildReturnsContract(t *testing.T) {
	f := testOutputContractFactory{id: "outputcontract.json-schema@1.0.0"}
	got, err := f.Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Schema["type"] != "object" {
		t.Errorf("Schema.type: %v", got.Schema["type"])
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./registry/ -run 'TestSkillFactory|TestOutputContractFactory' -v`

Expected: FAIL — `undefined: SkillFactory` and `undefined: OutputContractFactory`.

- [ ] **Step 3: Add factory interfaces**

Edit [`registry/factories.go`](../../../registry/factories.go). Append to the end of the file (after `IdentitySignerFactory`):

```go
// SkillFactory builds a Skill value. The Skill carries the skill's
// contributions (prompt fragment, required tools/policies, optional
// required output contract) which the build layer's expansion stage
// auto-injects into the effective composition.
type SkillFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (Skill, error)
}

// OutputContractFactory builds an OutputContract. The factory is
// responsible for structural well-formedness of the returned JSON
// Schema (nil, presence of $schema/type/properties/$ref at the root).
// Runtime semantic validation of LLM output against the schema is
// deferred to a later phase or to the orchestrator.
type OutputContractFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (OutputContract, error)
}
```

- [ ] **Step 4: Run the factory tests to verify they pass**

Run: `go test ./registry/ -run 'TestSkillFactory|TestOutputContractFactory' -v`

Expected: PASS (2 tests).

- [ ] **Step 5: Write failing registry register/lookup tests**

Append to [`registry/registry_test.go`](../../../registry/registry_test.go) (at the end of the file):

```go
// --- Phase 3: Skill + OutputContract registration ---

func TestRegister_Skill_OK(t *testing.T) {
	r := NewComponentRegistry()
	if err := r.RegisterSkill(testSkillFactory{id: "skill.structured-output@1.0.0"}); err != nil {
		t.Fatalf("RegisterSkill: %v", err)
	}
	f, err := r.Skill("skill.structured-output@1.0.0")
	if err != nil {
		t.Fatalf("Skill lookup: %v", err)
	}
	if f.ID() != "skill.structured-output@1.0.0" {
		t.Errorf("Skill.ID: %q", f.ID())
	}
}

func TestRegister_Skill_DuplicateFails(t *testing.T) {
	r := NewComponentRegistry()
	if err := r.RegisterSkill(testSkillFactory{id: "skill.x@1.0.0"}); err != nil {
		t.Fatalf("first RegisterSkill: %v", err)
	}
	err := r.RegisterSkill(testSkillFactory{id: "skill.x@1.0.0"})
	if err == nil {
		t.Fatal("want ErrDuplicate, got nil")
	}
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("want ErrDuplicate, got %v", err)
	}
}

func TestSkill_NotFound(t *testing.T) {
	r := NewComponentRegistry()
	_, err := r.Skill("skill.missing@1.0.0")
	if err == nil {
		t.Fatal("want ErrNotFound, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestRegister_Skill_FrozenRejects(t *testing.T) {
	r := NewComponentRegistry()
	r.Freeze()
	err := r.RegisterSkill(testSkillFactory{id: "skill.x@1.0.0"})
	if err == nil {
		t.Fatal("want ErrRegistryFrozen, got nil")
	}
	if !errors.Is(err, ErrRegistryFrozen) {
		t.Errorf("want ErrRegistryFrozen, got %v", err)
	}
}

func TestRegister_OutputContract_OK(t *testing.T) {
	r := NewComponentRegistry()
	if err := r.RegisterOutputContract(testOutputContractFactory{id: "outputcontract.json-schema@1.0.0"}); err != nil {
		t.Fatalf("RegisterOutputContract: %v", err)
	}
	f, err := r.OutputContract("outputcontract.json-schema@1.0.0")
	if err != nil {
		t.Fatalf("OutputContract lookup: %v", err)
	}
	if f.ID() != "outputcontract.json-schema@1.0.0" {
		t.Errorf("OutputContract.ID: %q", f.ID())
	}
}

func TestRegister_OutputContract_DuplicateFails(t *testing.T) {
	r := NewComponentRegistry()
	if err := r.RegisterOutputContract(testOutputContractFactory{id: "outputcontract.x@1.0.0"}); err != nil {
		t.Fatalf("first RegisterOutputContract: %v", err)
	}
	err := r.RegisterOutputContract(testOutputContractFactory{id: "outputcontract.x@1.0.0"})
	if err == nil {
		t.Fatal("want ErrDuplicate, got nil")
	}
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("want ErrDuplicate, got %v", err)
	}
}

func TestOutputContract_NotFound(t *testing.T) {
	r := NewComponentRegistry()
	_, err := r.OutputContract("outputcontract.missing@1.0.0")
	if err == nil {
		t.Fatal("want ErrNotFound, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestRegister_OutputContract_FrozenRejects(t *testing.T) {
	r := NewComponentRegistry()
	r.Freeze()
	err := r.RegisterOutputContract(testOutputContractFactory{id: "outputcontract.x@1.0.0"})
	if err == nil {
		t.Fatal("want ErrRegistryFrozen, got nil")
	}
	if !errors.Is(err, ErrRegistryFrozen) {
		t.Errorf("want ErrRegistryFrozen, got %v", err)
	}
}
```

- [ ] **Step 6: Run to verify failure**

Run: `go test ./registry/ -run 'TestRegister_Skill|TestSkill_NotFound|TestRegister_OutputContract|TestOutputContract_NotFound' -v`

Expected: FAIL — `r.RegisterSkill undefined`, `r.Skill undefined`, and same for `OutputContract`.

- [ ] **Step 7: Add maps to ComponentRegistry**

Edit [`registry/registry.go`](../../../registry/registry.go). Add two new fields to the struct and initialization:

Find the struct (lines 13-28) and add two fields at the bottom:

```go
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
```

Update `NewComponentRegistry` (lines 30-44):

```go
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
```

- [ ] **Step 8: Add Register + lookup methods**

Append to [`registry/registry.go`](../../../registry/registry.go) (after the existing `IdentitySigner` block at the end of the file):

```go
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
```

- [ ] **Step 9: Run registry tests to verify they pass**

Run: `go test ./registry/ -run 'TestRegister_Skill|TestSkill_NotFound|TestRegister_OutputContract|TestOutputContract_NotFound' -v`

Expected: PASS (8 tests).

- [ ] **Step 10: Run the full registry suite**

Run: `go test ./registry/... -v && go vet ./registry/...`

Expected: all tests pass; no vet complaints.

- [ ] **Step 11: Commit**

```bash
git add registry/factories.go registry/factories_test.go registry/registry.go registry/registry_test.go
git commit -m "$(cat <<'EOF'
feat(registry): SkillFactory and OutputContractFactory + registration

Adds SkillFactory and OutputContractFactory interfaces with the same
ID/Description/Build shape as every other Phase-1 factory. ComponentRegistry
gains two new map fields, four new methods (RegisterSkill/Skill/
RegisterOutputContract/OutputContract), and identical Freeze/Duplicate/
NotFound semantics. Tests mirror the existing registry_test.go patterns.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

## Expected state after this task

- `registry/factories.go`: `SkillFactory` and `OutputContractFactory` interfaces added at the end.
- `registry/factories_test.go`: interface compile-time assertions + 2 Build tests for the test-only fakes.
- `registry/registry.go`: two new map fields in `ComponentRegistry`, `NewComponentRegistry` initializes them, four new exported methods.
- `registry/registry_test.go`: 8 new tests mirroring existing patterns (OK, duplicate, not-found, frozen).
- `go test ./registry/...` green; `go vet` clean.
- One commit added to the branch.
