# Task group 0 — Registry: KindMCPBinding, types, factory interface, register/lookup

> Part of [praxis-forge Phase 4 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-22-praxis-forge-phase-4-design.md`](../../specs/2026-04-22-praxis-forge-phase-4-design.md).

**Commits (4 atomic):**
1. `feat(registry): activate KindMCPBinding`
2. `feat(registry): MCPBinding type + connection/auth/trust/onNewTool`
3. `feat(registry): MCPBindingFactory interface`
4. `feat(registry): RegisterMCPBinding + MCPBinding lookup`

---

### Task 1: Activate KindMCPBinding

**Files:**
- Modify: `registry/kind.go`
- Modify: `registry/kind_test.go`

- [ ] **Step 1: Add failing assertion for active Phase-4 kind**

Append to [registry/kind_test.go](../../../../registry/kind_test.go):

```go
func TestKind_Phase4ActiveKinds(t *testing.T) {
	if string(KindMCPBinding) != "mcp_binding" {
		t.Fatalf("KindMCPBinding=%q, expected \"mcp_binding\"", KindMCPBinding)
	}
}
```

- [ ] **Step 2: Run test — expect compile failure**

Run: `go test ./registry/ -run TestKind_Phase4ActiveKinds`
Expected: build fails with `undefined: KindMCPBinding`.

- [ ] **Step 3: Activate the kind**

Replace the `const (` block in [registry/kind.go](../../../../registry/kind.go) with:

```go
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
	KindSkill              Kind = "skill"
	KindOutputContract     Kind = "output_contract"
	KindMCPBinding         Kind = "mcp_binding"
)
```

Update the package doc comment at [registry/kind.go:7](../../../../registry/kind.go#L7) to say `Phase 1-4` instead of `Phase 1-3`.

- [ ] **Step 4: Run test — expect pass**

Run: `go test ./registry/ -run TestKind_Phase4ActiveKinds -v`
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add registry/kind.go registry/kind_test.go
git commit -m "feat(registry): activate KindMCPBinding"
```

---

### Task 2: MCPBinding types

**Files:**
- Modify: `registry/types.go`

- [ ] **Step 1: Write failing test for types existence**

Append to [registry/registry_test.go](../../../../registry/registry_test.go):

```go
func TestMCPBindingType_ShapeSmoke(t *testing.T) {
	b := MCPBinding{
		ID: "fs",
		Connection: MCPConnection{
			Transport: MCPTransportStdio,
			Command:   []string{"/bin/true"},
		},
		Allow:     []string{"read_*"},
		Deny:      []string{"write_*"},
		Policies:  []ID{"policypack.pii-redaction@1.0.0"},
		Trust:     MCPTrust{Tier: "medium", Owner: "demo"},
		OnNewTool: OnNewToolBlock,
	}
	if b.Connection.Transport != "stdio" {
		t.Fatalf("Transport=%q", b.Connection.Transport)
	}
	if b.OnNewTool != "block" {
		t.Fatalf("OnNewTool=%q", b.OnNewTool)
	}
}
```

- [ ] **Step 2: Run test — expect compile failure**

Run: `go test ./registry/ -run TestMCPBindingType_ShapeSmoke`
Expected: `undefined: MCPBinding`, `undefined: MCPConnection`, etc.

- [ ] **Step 3: Add types**

Append to [registry/types.go](../../../../registry/types.go):

```go
// MCPBinding is the value produced by an MCPBindingFactory. It is a
// governance contract, not a resolved tool set. Runtime (praxis) opens
// the MCP session, discovers tools live, applies Allow/Deny + Policies,
// and enforces OnNewTool when the server surface changes.
type MCPBinding struct {
	ID         string
	Connection MCPConnection
	Auth       *MCPAuth
	Allow      []string
	Deny       []string
	Policies   []ID
	Trust      MCPTrust
	OnNewTool  OnNewToolPolicy
	Descriptor MCPBindingDescriptor
}

// MCPTransport enumerates the declarable MCP connection transports.
type MCPTransport string

const (
	MCPTransportStdio MCPTransport = "stdio"
	MCPTransportHTTP  MCPTransport = "http"
)

// MCPConnection holds transport-specific connection details. Only the
// fields matching Transport are meaningful; the others are ignored.
type MCPConnection struct {
	Transport MCPTransport
	Command   []string
	Env       map[string]string
	URL       string
	Headers   map[string]string
}

// MCPAuth describes how the runtime authenticates against the MCP
// server. Forge never reads the secret; only CredentialRef is stored.
type MCPAuth struct {
	CredentialRef ID
	Scheme        string
	HeaderName    string
}

// MCPTrust holds governance-level metadata for a binding.
type MCPTrust struct {
	Tier  string
	Owner string
	Tags  []string
}

// OnNewToolPolicy governs how the runtime reacts when the remote MCP
// server exposes a tool that was not observed when the agent was built.
type OnNewToolPolicy string

const (
	OnNewToolBlock            OnNewToolPolicy = "block"
	OnNewToolAllowIfMatch     OnNewToolPolicy = "allow-if-match-allowlist"
	OnNewToolRequireReapprove OnNewToolPolicy = "require-reapproval"
)

// MCPBindingDescriptor is forge-managed metadata for an MCP binding.
type MCPBindingDescriptor struct {
	Name    string
	Summary string
	Tags    []string
}
```

- [ ] **Step 4: Run test — expect pass**

Run: `go test ./registry/ -run TestMCPBindingType_ShapeSmoke -v`
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add registry/types.go registry/registry_test.go
git commit -m "feat(registry): MCPBinding type + connection/auth/trust/onNewTool"
```

---

### Task 3: MCPBindingFactory interface

**Files:**
- Modify: `registry/factories.go`

- [ ] **Step 1: Write failing test for interface satisfaction**

Append to [registry/registry_test.go](../../../../registry/registry_test.go):

```go
type fakeMCPBindingFactory struct{ id ID }

func (f fakeMCPBindingFactory) ID() ID              { return f.id }
func (f fakeMCPBindingFactory) Description() string { return fakeDesc }
func (f fakeMCPBindingFactory) Build(context.Context, map[string]any) (MCPBinding, error) {
	return MCPBinding{}, nil
}

func TestMCPBindingFactory_InterfaceSatisfied(t *testing.T) {
	var f MCPBindingFactory = fakeMCPBindingFactory{id: "mcp.binding@1.0.0"}
	if f.ID() != "mcp.binding@1.0.0" {
		t.Fatalf("ID=%q", f.ID())
	}
}
```

- [ ] **Step 2: Run test — expect compile failure**

Run: `go test ./registry/ -run TestMCPBindingFactory_InterfaceSatisfied`
Expected: `undefined: MCPBindingFactory`.

- [ ] **Step 3: Add interface**

Append to [registry/factories.go](../../../../registry/factories.go):

```go
// MCPBindingFactory builds an MCPBinding.
type MCPBindingFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (MCPBinding, error)
}
```

- [ ] **Step 4: Run test — expect pass**

Run: `go test ./registry/ -run TestMCPBindingFactory_InterfaceSatisfied -v`
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add registry/factories.go registry/registry_test.go
git commit -m "feat(registry): MCPBindingFactory interface"
```

---

### Task 4: Register + lookup methods

**Files:**
- Modify: `registry/registry.go`
- Modify: `registry/registry_test.go`

- [ ] **Step 1: Write failing test**

Append to [registry/registry_test.go](../../../../registry/registry_test.go):

```go
func TestRegistry_MCPBinding_RegisterLookupDuplicateFrozen(t *testing.T) {
	r := NewComponentRegistry()

	// Register.
	if err := r.RegisterMCPBinding(fakeMCPBindingFactory{id: "mcp.binding@1.0.0"}); err != nil {
		t.Fatalf("RegisterMCPBinding: %v", err)
	}

	// Lookup.
	got, err := r.MCPBinding("mcp.binding@1.0.0")
	if err != nil {
		t.Fatalf("MCPBinding: %v", err)
	}
	if got.ID() != "mcp.binding@1.0.0" {
		t.Fatalf("got.ID=%q", got.ID())
	}

	// Duplicate.
	err = r.RegisterMCPBinding(fakeMCPBindingFactory{id: "mcp.binding@1.0.0"})
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("want ErrDuplicate, got %v", err)
	}

	// Miss.
	_, err = r.MCPBinding("mcp.missing@1.0.0")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	// Frozen.
	r.Freeze()
	err = r.RegisterMCPBinding(fakeMCPBindingFactory{id: "mcp.other@1.0.0"})
	if !errors.Is(err, ErrRegistryFrozen) {
		t.Fatalf("want ErrRegistryFrozen, got %v", err)
	}
}
```

- [ ] **Step 2: Run test — expect compile failure**

Run: `go test ./registry/ -run TestRegistry_MCPBinding_RegisterLookupDuplicateFrozen`
Expected: `registry has no field or method RegisterMCPBinding`.

- [ ] **Step 3: Add map + methods**

In [registry/registry.go](../../../../registry/registry.go), add the map field inside `ComponentRegistry` (after `outputContracts`):

```go
	mcpBindings      map[ID]MCPBindingFactory
```

Add initialisation inside `NewComponentRegistry` (after `outputContracts: map[ID]OutputContractFactory{}`):

```go
		mcpBindings:       map[ID]MCPBindingFactory{},
```

Append methods at the end of the file:

```go
// --- MCPBinding ---

func (r *ComponentRegistry) RegisterMCPBinding(f MCPBindingFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.mcpBindings[f.ID()]; exists {
		return fmt.Errorf("%w: kind=%s id=%s", ErrDuplicate, KindMCPBinding, f.ID())
	}
	r.mcpBindings[f.ID()] = f
	return nil
}

func (r *ComponentRegistry) MCPBinding(id ID) (MCPBindingFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.mcpBindings[id]
	if !ok {
		return nil, fmt.Errorf("%w: kind=%s id=%s", ErrNotFound, KindMCPBinding, id)
	}
	return f, nil
}
```

- [ ] **Step 4: Run test — expect pass**

Run: `go test ./registry/ -run TestRegistry_MCPBinding_RegisterLookupDuplicateFrozen -v`
Expected: `PASS`.

Also run: `go test -race -count=1 ./registry/...`
Expected: entire package stays green.

- [ ] **Step 5: Commit**

```bash
git add registry/registry.go registry/registry_test.go
git commit -m "feat(registry): RegisterMCPBinding + MCPBinding lookup"
```
