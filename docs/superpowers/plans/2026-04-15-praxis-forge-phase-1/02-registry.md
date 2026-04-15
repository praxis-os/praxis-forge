> Part of [praxis-forge Phase 1 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-15-praxis-forge-phase-1-design.md`](../../specs/2026-04-15-praxis-forge-phase-1-design.md).

## Task group 2 — `registry/` package

### Task 2.1: Kind enum + ID type + errors

**Files:**
- Create: `registry/kind.go`
- Create: `registry/id.go`
- Create: `registry/errors.go`
- Create: `registry/kind_test.go`

- [ ] **Step 1: Write failing test**

```go
// registry/kind_test.go
package registry

import "testing"

func TestKindString(t *testing.T) {
	if string(KindProvider) != "provider" {
		t.Fatalf("KindProvider=%q", KindProvider)
	}
}

func TestParseID_PropagatesSpecRules(t *testing.T) {
	if _, err := ParseID("bad"); err == nil {
		t.Fatal("expected error")
	}
	id, err := ParseID("provider.foo@1.0.0")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if id != "provider.foo@1.0.0" {
		t.Fatalf("id=%s", id)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./registry/... -v`

- [ ] **Step 3: Implement**

```go
// registry/kind.go
// SPDX-License-Identifier: Apache-2.0

// Package registry holds the typed component registry. Factories register at
// program start and are resolved by (Kind, ID) from declarative specs.
package registry

// Kind enumerates every component category Phase 1 knows about.
type Kind string

const (
	KindProvider           Kind = "provider"
	KindPromptAsset        Kind = "prompt_asset"
	KindToolPack           Kind = "tool_pack"
	KindPolicyPack         Kind = "policy_pack"
	KindPreLLMFilter       Kind = "pre_llm_filter"
	KindPreToolFilter      Kind = "pre_tool_filter"
	KindPostToolFilter     Kind = "post_tool_filter"
	KindBudgetProfile      Kind = "budget_profile"
	KindTelemetryProfile   Kind = "telemetry_profile"
	KindCredentialResolver Kind = "credential_resolver"
	KindIdentitySigner     Kind = "identity_signer"
)
```

```go
// registry/id.go
// SPDX-License-Identifier: Apache-2.0

package registry

import "github.com/praxis-os/praxis-forge/spec"

// ID is a registered factory's stable address. Format: "<dotted>@<semver>".
type ID string

// ParseID validates the id string and returns it unchanged on success.
func ParseID(s string) (ID, error) {
	if _, _, err := spec.ParseID(s); err != nil {
		return "", err
	}
	return ID(s), nil
}
```

```go
// registry/errors.go
// SPDX-License-Identifier: Apache-2.0

package registry

import "errors"

var (
	ErrRegistryFrozen = errors.New("registry: frozen, cannot register after Build")
	ErrDuplicate      = errors.New("registry: duplicate (kind, id)")
	ErrNotFound       = errors.New("registry: factory not found")
	ErrKindMismatch   = errors.New("registry: id registered under different kind")
)
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./registry/... -v`

- [ ] **Step 5: Commit**

```bash
git add registry/kind.go registry/id.go registry/errors.go registry/kind_test.go
git commit -m "feat(registry): Kind enum, ID type, sentinel errors"
```

---

### Task 2.2: Result types

**Files:**
- Create: `registry/types.go`

- [ ] **Step 1: Implement**

```go
// registry/types.go
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"time"

	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/telemetry"
	"github.com/praxis-os/praxis/tools"
)

// RiskTier categorises tools and policies for governance tagging.
type RiskTier string

const (
	RiskLow         RiskTier = "low"
	RiskModerate    RiskTier = "moderate"
	RiskHigh        RiskTier = "high"
	RiskDestructive RiskTier = "destructive"
)

// ToolDescriptor is forge-managed metadata added on top of llm.ToolDefinition.
type ToolDescriptor struct {
	Name        string
	Owner       string
	RiskTier    RiskTier
	PolicyTags  []string
	AuthScopes  []string
	TimeoutHint time.Duration
	Source      string // factory ID
}

// PolicyDescriptor is forge-managed metadata for a policy pack.
type PolicyDescriptor struct {
	Name       string
	Owner      string
	PolicyTags []string
	Source     string
}

// ToolPack is what a ToolPackFactory produces.
type ToolPack struct {
	Invoker     tools.Invoker
	Definitions []llm.ToolDefinition
	Descriptors []ToolDescriptor
}

// PolicyPack is what a PolicyPackFactory produces.
type PolicyPack struct {
	Hook        hooks.PolicyHook
	Descriptors []PolicyDescriptor
}

// BudgetProfile is what a BudgetProfileFactory produces.
type BudgetProfile struct {
	Guard         budget.Guard
	DefaultConfig budget.Config
}

// TelemetryProfile is what a TelemetryProfileFactory produces.
type TelemetryProfile struct {
	Emitter  telemetry.LifecycleEventEmitter
	Enricher telemetry.AttributeEnricher
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./registry/...`

- [ ] **Step 3: Commit**

```bash
git add registry/types.go
git commit -m "feat(registry): result types (ToolPack, PolicyPack, profiles, descriptors)"
```

---

### Task 2.3: Factory interfaces

**Files:**
- Create: `registry/factories.go`

- [ ] **Step 1: Implement**

```go
// registry/factories.go
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"

	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/llm"
)

// ProviderFactory builds an llm.Provider.
type ProviderFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (llm.Provider, error)
}

// PromptAssetFactory builds the string body of a registered prompt asset.
type PromptAssetFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (string, error)
}

// ToolPackFactory builds a governed tool pack.
type ToolPackFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (ToolPack, error)
}

// PolicyPackFactory builds a hooks.PolicyHook plus governance metadata.
type PolicyPackFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (PolicyPack, error)
}

// PreLLMFilterFactory builds a hooks.PreLLMFilter.
type PreLLMFilterFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (hooks.PreLLMFilter, error)
}

// PreToolFilterFactory builds a hooks.PreToolFilter.
type PreToolFilterFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (hooks.PreToolFilter, error)
}

// PostToolFilterFactory builds a hooks.PostToolFilter.
type PostToolFilterFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (hooks.PostToolFilter, error)
}

// BudgetProfileFactory builds a budget guard + default config.
type BudgetProfileFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (BudgetProfile, error)
}

// TelemetryProfileFactory builds emitter + enricher.
type TelemetryProfileFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (TelemetryProfile, error)
}

// CredentialResolverFactory builds a credentials.Resolver.
type CredentialResolverFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (credentials.Resolver, error)
}

// IdentitySignerFactory builds an identity.Signer.
type IdentitySignerFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (identity.Signer, error)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./registry/...`

- [ ] **Step 3: Commit**

```bash
git add registry/factories.go
git commit -m "feat(registry): 11 factory interfaces"
```

---

### Task 2.4: ComponentRegistry — Provider register/lookup + freeze

**Files:**
- Create: `registry/registry.go`
- Create: `registry/registry_test.go`

- [ ] **Step 1: Write failing test**

```go
// registry/registry_test.go
package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/praxis-os/praxis/llm"
)

type fakeProviderFactory struct{ id ID }

func (f fakeProviderFactory) ID() ID                                                        { return f.id }
func (f fakeProviderFactory) Description() string                                           { return "fake" }
func (f fakeProviderFactory) Build(context.Context, map[string]any) (llm.Provider, error)   { return nil, nil }

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
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./registry/... -v`

- [ ] **Step 3: Implement minimal registry with just Provider kind**

```go
// registry/registry.go
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

	providers map[ID]ProviderFactory
	// Other kinds added in subsequent tasks.
}

func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		providers: map[ID]ProviderFactory{},
	}
}

// Freeze blocks further registration. Idempotent.
func (r *ComponentRegistry) Freeze() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
}

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
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./registry/... -v`

- [ ] **Step 5: Commit**

```bash
git add registry/registry.go registry/registry_test.go
git commit -m "feat(registry): ComponentRegistry with provider kind + freeze"
```

---

### Task 2.5: ComponentRegistry — remaining 10 Kinds

**Files:**
- Modify: `registry/registry.go`
- Modify: `registry/registry_test.go`

- [ ] **Step 1: Extend struct + add per-kind methods**

Each kind gets its own map, `Register<Kind>` and lookup. Append to `registry.go`:

```go
type ComponentRegistry struct {
	mu     sync.RWMutex
	frozen bool

	providers        map[ID]ProviderFactory
	promptAssets     map[ID]PromptAssetFactory
	toolPacks        map[ID]ToolPackFactory
	policyPacks      map[ID]PolicyPackFactory
	preLLMFilters    map[ID]PreLLMFilterFactory
	preToolFilters   map[ID]PreToolFilterFactory
	postToolFilters  map[ID]PostToolFilterFactory
	budgetProfiles   map[ID]BudgetProfileFactory
	telemetryProfiles map[ID]TelemetryProfileFactory
	credResolvers    map[ID]CredentialResolverFactory
	identitySigners  map[ID]IdentitySignerFactory
}
```

Update `NewComponentRegistry` to initialize every map.

For each remaining kind, add two methods following the Provider pattern. Example for PromptAsset:

```go
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
```

Repeat for: `ToolPack`, `PolicyPack`, `PreLLMFilter`, `PreToolFilter`, `PostToolFilter`, `BudgetProfile`, `TelemetryProfile`, `CredentialResolver`, `IdentitySigner` — method names: `RegisterToolPack` + `ToolPack(id)`, etc.

- [ ] **Step 2: Add test per kind**

Append one test per kind using the same pattern as `TestRegistry_RegisterAndLookupProvider`, but with a minimal fake factory for that kind. Use `type fakeToolPackFactory struct{ id ID }` etc.

Example:

```go
type fakePromptAssetFactory struct{ id ID }

func (f fakePromptAssetFactory) ID() ID                                               { return f.id }
func (f fakePromptAssetFactory) Description() string                                  { return "fake" }
func (f fakePromptAssetFactory) Build(context.Context, map[string]any) (string, error) { return "hi", nil }

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
```

Repeat this pattern — one `fake<Kind>Factory` and one `TestRegistry_<Kind>` per remaining kind. The return types are per `registry/factories.go`.

- [ ] **Step 3: Run**

Run: `go test ./registry/... -v`
Expected: 11 kind-specific register/lookup tests + freeze/duplicate/not-found tests PASS.

- [ ] **Step 4: Commit**

```bash
git add registry/registry.go registry/registry_test.go
git commit -m "feat(registry): register + lookup for all 11 kinds"
```

---

