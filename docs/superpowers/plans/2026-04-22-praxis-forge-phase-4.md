# praxis-forge Phase 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Activate MCP consume as a declarative runtime binding contract — forge validates and stamps a governable binding (connection, auth ref, allow/deny, policies, trust, onNewTool) into the manifest. No network I/O during `Build`.

**Architecture:** New `KindMCPBinding` activates in the registry with a generic `mcp.binding@1` factory. `spec.mcpImports` unlocks with structural validation. A new `build/mcp.go` resolves bindings with cross-kind validation (policies and credentials must already be registered). Resolved bindings flow through the existing `resolved` struct into manifest `ResolvedComponent` entries. No new hash; `ExpandedHash` semantics unchanged. Vertical slice: a filesystem-server binding exercised via a `-mcp` demo flag.

**Tech Stack:** Go 1.23, existing patterns — registry factories, canonical-JSON manifest, spec/validate error aggregation, TDD with golden hash files.

**Spec:** [`docs/superpowers/specs/2026-04-22-praxis-forge-phase-4-design.md`](../specs/2026-04-22-praxis-forge-phase-4-design.md)

---

## File structure

### Modified

- [registry/kind.go](../../../registry/kind.go) — move `KindMCPBinding` from deferred to active.
- [registry/kind_test.go](../../../registry/kind_test.go) — assert `KindMCPBinding == "mcp_binding"`.
- [registry/types.go](../../../registry/types.go) — add `MCPBinding`, `MCPConnection`, `MCPTransport`, `MCPAuth`, `MCPTrust`, `OnNewToolPolicy`, `MCPBindingDescriptor`.
- [registry/factories.go](../../../registry/factories.go) — add `MCPBindingFactory` interface.
- [registry/registry.go](../../../registry/registry.go) — add `mcpBindings` map, `RegisterMCPBinding`, `MCPBinding(id)` lookup.
- [registry/registry_test.go](../../../registry/registry_test.go) — add `fakeMCPBindingFactory` + duplicate/lookup/frozen test.
- [spec/validate.go](../../../spec/validate.go) — remove mcpImports phase-gate at lines 82–84; add ref + structural validation.
- [spec/validate_test.go](../../../spec/validate_test.go) — MCP validation tests.
- [build/resolver.go](../../../build/resolver.go) — add `mcpBindings` / `mcpBindingIDs` / `mcpBindingCfgs` fields to `resolved`.
- [build/build.go](../../../build/build.go) — call `resolveMCPBindings` after `resolve`, stamp results onto `res`, emit manifest entries, include in capabilities.
- [build/capabilities.go](../../../build/capabilities.go) — add `mcp_binding` to present/skipped lists.
- [build/tool_router.go](../../../build/tool_router.go) — reserve `mcp.*` tool-name prefix; reject collisions.
- [examples/demo/main.go](../../../examples/demo/main.go) — add `-mcp` flag and registration.
- [docs/design/agent-spec-v0.md](../../design/agent-spec-v0.md) — drop `mcpImports` from deferred list.
- [docs/design/forge-overview.md](../../design/forge-overview.md) — mark Phase 4 shipped.

### Created

- [build/mcp.go](../../../build/mcp.go) — `resolveMCPBindings`, config decode, cross-kind validation.
- [build/mcp_test.go](../../../build/mcp_test.go) — unit tests for `resolveMCPBindings`.
- [build/build_mcp_test.go](../../../build/build_mcp_test.go) — end-to-end integration.
- [factories/mcpbinding/factory.go](../../../factories/mcpbinding/factory.go) — `mcp.binding@1` generic factory.
- [factories/mcpbinding/factory_test.go](../../../factories/mcpbinding/factory_test.go) — factory unit tests.
- `spec/testdata/mcp/minimal-stdio/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/stdio-with-env/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/http-with-bearer/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/http-with-apikey/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/multiple-bindings/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/on-new-tool-variants/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/unresolved-policy/spec.yaml` + `err.txt`
- `spec/testdata/mcp/unresolved-credential/spec.yaml` + `err.txt`
- `spec/testdata/mcp/duplicate-binding-id/spec.yaml` + `err.txt`
- `spec/testdata/mcp/transport-field-mismatch/spec.yaml` + `err.txt`
- [examples/demo/agent-mcp.yaml](../../../examples/demo/agent-mcp.yaml) — demo spec.

Working directory for all commands: `/Users/francescofiore/Coding/praxis-os/praxis-forge`.
Baseline verification: `go test -race -count=1 ./...` must be green before starting.

---

### Task 1: Activate KindMCPBinding

**Files:**
- Modify: `registry/kind.go`
- Modify: `registry/kind_test.go`

- [ ] **Step 1: Add failing assertion for active Phase-4 kind**

Append to [registry/kind_test.go](../../../registry/kind_test.go):

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

Replace the `const (` block in [registry/kind.go](../../../registry/kind.go) with:

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

Update the package doc comment at [registry/kind.go:7](../../../registry/kind.go#L7) to say `Phase 1-4` instead of `Phase 1-3`.

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

Append to [registry/registry_test.go](../../../registry/registry_test.go):

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

Append to [registry/types.go](../../../registry/types.go):

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

Append to [registry/registry_test.go](../../../registry/registry_test.go):

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

Append to [registry/factories.go](../../../registry/factories.go):

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

Append to [registry/registry_test.go](../../../registry/registry_test.go):

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

In [registry/registry.go](../../../registry/registry.go), add the map field inside `ComponentRegistry` (after `outputContracts`):

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

---

### Task 5: Unlock mcpImports validation gate + ref prefix check

**Files:**
- Modify: `spec/validate.go`
- Modify: `spec/validate_test.go`

- [ ] **Step 1: Write failing tests**

Append to [spec/validate_test.go](../../../spec/validate_test.go):

```go
func TestValidate_MCPImports_GateRemoved(t *testing.T) {
	s := baseValidSpec()
	s.MCPImports = []ComponentRef{
		{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "demo"},
		}},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_MCPImports_BadRefPrefix(t *testing.T) {
	s := baseValidSpec()
	s.MCPImports = []ComponentRef{
		{Ref: "toolpack.foo@1.0.0", Config: map[string]any{
			"id":         "fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "demo"},
		}},
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "must start with \"mcp.\"") {
		t.Fatalf("want mcp. prefix error, got %v", err)
	}
}
```

Also **delete** the existing phase-gate assertion test `TestValidate_RejectsMCP` at [spec/validate_test.go:144-150](../../../spec/validate_test.go#L144-L150) — the gate it asserts is about to disappear. Remove the entire function (lines 144–150).

- [ ] **Step 2: Run tests — expect failures**

Run: `go test ./spec/ -run TestValidate_MCPImports -v`
Expected: `TestValidate_MCPImports_GateRemoved` fails with `phase-gated (Phase 4)`; `TestValidate_MCPImports_BadRefPrefix` fails because the existing gate fires first.

- [ ] **Step 3: Remove gate and add prefix validation**

In [spec/validate.go](../../../spec/validate.go), replace the block at lines 78–84:

```go
	// Phase-gated fields: extends and mcpImports remain gated; skills + outputContract now validated by prefix.
	if len(s.Extends) > 0 {
		errs.Addf("extends: phase-gated (Phase 2); must be empty in v0")
	}
	if len(s.MCPImports) > 0 {
		errs.Addf("mcpImports: phase-gated (Phase 4); must be empty in v0")
	}
```

with:

```go
	// Phase-gated fields: extends remains gated; skills, outputContract, and mcpImports now validated by prefix + structure.
	if len(s.Extends) > 0 {
		errs.Addf("extends: phase-gated (Phase 2); must be empty in v0")
	}

	// MCP imports validation (Phase 4): each ref must be prefixed with "mcp.".
	for i, mi := range s.MCPImports {
		validateKindPrefixedRef(&errs, fmt.Sprintf("mcpImports[%d].ref", i), mi.Ref, "mcp.")
	}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./spec/ -run TestValidate_MCPImports -v`
Expected: `PASS`.

Also run: `go test -race -count=1 ./spec/...`
Expected: entire package green.

- [ ] **Step 5: Commit**

```bash
git add spec/validate.go spec/validate_test.go
git commit -m "feat(spec): unlock mcpImports phase gate; enforce mcp. ref prefix"
```

---

### Task 6: Structural validation (id, transport, onNewTool, auth, globs)

**Files:**
- Modify: `spec/validate.go`
- Modify: `spec/validate_test.go`

- [ ] **Step 1: Write failing tests**

Append to [spec/validate_test.go](../../../spec/validate_test.go):

```go
func mcpSpecWithBinding(cfg map[string]any) *AgentSpec {
	s := baseValidSpec()
	s.MCPImports = []ComponentRef{{Ref: "mcp.binding@1.0.0", Config: cfg}}
	return s
}

func TestValidate_MCPImports_MissingID(t *testing.T) {
	s := mcpSpecWithBinding(map[string]any{
		"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
	})
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp_missing_id") {
		t.Fatalf("want mcp_missing_id, got %v", err)
	}
}

func TestValidate_MCPImports_DuplicateID(t *testing.T) {
	s := baseValidSpec()
	binding := map[string]any{
		"id":         "fs",
		"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
	}
	s.MCPImports = []ComponentRef{
		{Ref: "mcp.binding@1.0.0", Config: binding},
		{Ref: "mcp.binding@1.0.0", Config: binding},
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp_duplicate_id") {
		t.Fatalf("want mcp_duplicate_id, got %v", err)
	}
}

func TestValidate_MCPImports_TransportInvalid(t *testing.T) {
	s := mcpSpecWithBinding(map[string]any{
		"id":         "fs",
		"connection": map[string]any{"transport": "websocket"},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
	})
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp_transport_invalid") {
		t.Fatalf("want mcp_transport_invalid, got %v", err)
	}
}

func TestValidate_MCPImports_StdioRequiresCommand(t *testing.T) {
	s := mcpSpecWithBinding(map[string]any{
		"id":         "fs",
		"connection": map[string]any{"transport": "stdio"},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
	})
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp_transport_field_mismatch") {
		t.Fatalf("want mcp_transport_field_mismatch, got %v", err)
	}
}

func TestValidate_MCPImports_HTTPWithCommand(t *testing.T) {
	s := mcpSpecWithBinding(map[string]any{
		"id":         "fs",
		"connection": map[string]any{"transport": "http", "url": "https://e/mcp", "command": []any{"/bin/true"}},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
	})
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp_transport_field_mismatch") {
		t.Fatalf("want mcp_transport_field_mismatch, got %v", err)
	}
}

func TestValidate_MCPImports_OnNewToolInvalid(t *testing.T) {
	s := mcpSpecWithBinding(map[string]any{
		"id":         "fs",
		"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
		"onNewTool":  "YOLO",
	})
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp_on_new_tool_invalid") {
		t.Fatalf("want mcp_on_new_tool_invalid, got %v", err)
	}
}

func TestValidate_MCPImports_AuthAPIKeyRequiresHeader(t *testing.T) {
	s := mcpSpecWithBinding(map[string]any{
		"id":         "fs",
		"connection": map[string]any{"transport": "http", "url": "https://e/mcp"},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
		"auth":       map[string]any{"credentialRef": "credresolver.env@1.0.0", "scheme": "api-key"},
	})
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp_missing_header_name") {
		t.Fatalf("want mcp_missing_header_name, got %v", err)
	}
}

func TestValidate_MCPImports_InvalidGlob(t *testing.T) {
	s := mcpSpecWithBinding(map[string]any{
		"id":         "fs",
		"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
		"allow":      []any{"read_["},
	})
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp_invalid_glob") {
		t.Fatalf("want mcp_invalid_glob, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests — expect failures**

Run: `go test ./spec/ -run TestValidate_MCPImports -v`
Expected: 8 new tests fail with generic pass or wrong error.

- [ ] **Step 3: Add a dedicated validator**

Create [spec/validate_mcp.go](../../../spec/validate_mcp.go):

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"path/filepath"
)

// validateMCPImportsStructure checks the typed-map shape of each
// spec.mcpImports[].config. It operates on the decoded map rather than
// a typed struct because AgentSpec.MCPImports is []ComponentRef for
// uniformity with other factory kinds. The factory's own Build method
// re-validates as defense in depth (factories/mcpbinding).
func validateMCPImportsStructure(errs *Errors, imports []ComponentRef) {
	seenID := map[string]int{}
	for i, ref := range imports {
		field := fmt.Sprintf("mcpImports[%d].config", i)
		cfg := ref.Config
		if cfg == nil {
			errs.Addf("%s: mcp_missing_id: config required", field)
			continue
		}

		id := stringAt(cfg, "id")
		if id == "" {
			errs.Addf("%s.id: mcp_missing_id", field)
		} else if prev, dup := seenID[id]; dup {
			errs.Addf("%s.id %q: mcp_duplicate_id of mcpImports[%d]", field, id, prev)
		} else {
			seenID[id] = i
		}

		validateMCPConnection(errs, field+".connection", mapAt(cfg, "connection"))
		validateMCPOnNewTool(errs, field+".onNewTool", stringAt(cfg, "onNewTool"))
		validateMCPAuth(errs, field+".auth", mapAt(cfg, "auth"))
		validateGlobList(errs, field+".allow", anyListAt(cfg, "allow"))
		validateGlobList(errs, field+".deny", anyListAt(cfg, "deny"))
	}
}

func validateMCPConnection(errs *Errors, field string, conn map[string]any) {
	if conn == nil {
		errs.Addf("%s: mcp_transport_invalid: connection required", field)
		return
	}
	transport := stringAt(conn, "transport")
	switch transport {
	case "stdio":
		if len(anyListAt(conn, "command")) == 0 {
			errs.Addf("%s: mcp_transport_field_mismatch: stdio requires command", field)
		}
		if stringAt(conn, "url") != "" {
			errs.Addf("%s: mcp_transport_field_mismatch: stdio must not set url", field)
		}
	case "http":
		if stringAt(conn, "url") == "" {
			errs.Addf("%s: mcp_transport_field_mismatch: http requires url", field)
		}
		if len(anyListAt(conn, "command")) > 0 {
			errs.Addf("%s: mcp_transport_field_mismatch: http must not set command", field)
		}
	default:
		errs.Addf("%s.transport %q: mcp_transport_invalid (want stdio|http)", field, transport)
	}
}

func validateMCPOnNewTool(errs *Errors, field, v string) {
	if v == "" { // default "block"; omission is fine.
		return
	}
	switch v {
	case "block", "allow-if-match-allowlist", "require-reapproval":
		return
	default:
		errs.Addf("%s %q: mcp_on_new_tool_invalid (want block|allow-if-match-allowlist|require-reapproval)", field, v)
	}
}

func validateMCPAuth(errs *Errors, field string, auth map[string]any) {
	if auth == nil {
		return
	}
	if stringAt(auth, "credentialRef") == "" {
		errs.Addf("%s.credentialRef: mcp_missing_credential_ref", field)
	}
	scheme := stringAt(auth, "scheme")
	if scheme == "" {
		errs.Addf("%s.scheme: mcp_missing_auth_scheme", field)
	}
	if scheme == "api-key" && stringAt(auth, "headerName") == "" {
		errs.Addf("%s.headerName: mcp_missing_header_name (required for scheme api-key)", field)
	}
}

func validateGlobList(errs *Errors, field string, patterns []any) {
	for i, p := range patterns {
		s, ok := p.(string)
		if !ok {
			errs.Addf("%s[%d]: mcp_invalid_glob: want string, got %T", field, i, p)
			continue
		}
		if _, err := filepath.Match(s, ""); err != nil {
			errs.Addf("%s[%d] %q: mcp_invalid_glob: %s", field, i, s, err.Error())
		}
	}
}

// --- small helpers ---

func stringAt(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

func mapAt(m map[string]any, k string) map[string]any {
	v, _ := m[k].(map[string]any)
	return v
}

func anyListAt(m map[string]any, k string) []any {
	switch v := m[k].(type) {
	case []any:
		return v
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out
	default:
		return nil
	}
}
```

Wire it from [spec/validate.go](../../../spec/validate.go) by adding one line immediately after the `validateKindPrefixedRef` loop you added in Task 5:

```go
	validateMCPImportsStructure(&errs, s.MCPImports)
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./spec/ -run TestValidate_MCPImports -v`
Expected: all 8 new tests `PASS`.

Run full package: `go test -race -count=1 ./spec/...`
Expected: green.

- [ ] **Step 5: Commit**

```bash
git add spec/validate.go spec/validate_mcp.go spec/validate_test.go
git commit -m "feat(spec): structural validation for mcpImports (id/transport/auth/onNewTool/globs)"
```

---

### Task 6.5: Extends-chain merge handling for MCPImports

Mirrors how Phase 3 added Skills/OutputContract handling in `mergeChain` ([spec/normalize.go:246-255](../../../spec/normalize.go#L246-L255)). Parent specs that declare `mcpImports` must propagate through the extends chain to children. Overlay-file-level add/remove/replace of MCPImports is deliberately deferred to Phase 4.1+ (consistent with how Phase 3 left `AgentOverlayBody` without Skills/OutputContract fields).

**Files:**
- Modify: `spec/normalize.go`
- Modify: `spec/normalize_test.go` or add new file
- Modify: `spec/overlay.go` (comment refresh only)

- [ ] **Step 1: Write failing test**

Create a new test helper file or append to existing test. Create [spec/mcp_extends_test.go](../../../spec/mcp_extends_test.go):

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"context"
	"testing"
)

func TestMergeChain_MCPImportsPropagatesFromParent(t *testing.T) {
	parent := &AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   Metadata{ID: "parent", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
		MCPImports: []ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "parent"},
		}}},
	}
	child := &AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   Metadata{ID: "child", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
		Extends:    []string{"parent@0.1.0"},
	}
	store := NewMapStore(map[string]*AgentSpec{"parent@0.1.0": parent})
	ns, err := Normalize(context.Background(), child, store, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if len(ns.Spec.MCPImports) != 1 || ns.Spec.MCPImports[0].Ref != "mcp.binding@1.0.0" {
		t.Fatalf("MCPImports not propagated: %+v", ns.Spec.MCPImports)
	}
}

func TestMergeChain_MCPImportsChildWins(t *testing.T) {
	parent := &AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   Metadata{ID: "parent", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
		MCPImports: []ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{"id": "parent-fs"}}},
	}
	child := &AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   Metadata{ID: "child", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
		Extends:    []string{"parent@0.1.0"},
		MCPImports: []ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{"id": "child-fs"}}},
	}
	store := NewMapStore(map[string]*AgentSpec{"parent@0.1.0": parent})
	ns, err := Normalize(context.Background(), child, store, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if len(ns.Spec.MCPImports) != 1 || ns.Spec.MCPImports[0].Config["id"] != "child-fs" {
		t.Fatalf("child did not win: %+v", ns.Spec.MCPImports)
	}
}
```

Note: check [spec/store.go](../../../spec/store.go) for the exact `NewMapStore` constructor name; adapt if it differs (Phase 2a fixtures use it, search `NewMapStore` or `MapStore` in existing tests to confirm signature).

- [ ] **Step 2: Run tests — expect failure**

Run: `go test ./spec/ -run TestMergeChain_MCPImports -v`
Expected: parent's MCPImports do not propagate to child.

- [ ] **Step 3: Extend `mergeChain`**

In [spec/normalize.go](../../../spec/normalize.go), find the Phase-3 block handling `Skills` and `OutputContract` (around lines 246–255) and append:

```go
	if child.MCPImports != nil {
		merged.MCPImports = child.MCPImports
		prov.MCPImports = p
	}
```

Update the comment on that block (line 246) to: `// Phase 3: Skills and OutputContract; Phase 4: MCPImports. All propagate child-wins per replaceable-list semantics.`

If `prov` (Provenance struct) does not yet have an `MCPImports` field, add it. Check [spec/provenance.go](../../../spec/provenance.go) first:

```bash
grep -n "Skills\|OutputContract\|MCPImports" spec/provenance.go
```

If `MCPImports` is absent, mirror the `Skills` field definition exactly.

- [ ] **Step 4: Refresh the stale overlay comment**

In [spec/overlay.go](../../../spec/overlay.go), replace the outdated comment at lines 99–101:

```go
// AgentOverlayBody mirrors AgentSpec but every field is optional and
// each replaceable list uses the RefList tri-state wrapper. Phase-gated
// AgentSpec fields (extends, skills, mcpImports, outputContract) are
// deliberately absent so the strict decoder rejects them at parse time.
```

with:

```go
// AgentOverlayBody mirrors AgentSpec but every field is optional and
// each replaceable list uses the RefList tri-state wrapper.
// AgentSpec fields that overlays cannot modify today (extends, skills,
// mcpImports, outputContract) are deliberately absent so the strict
// decoder rejects them at parse time. Extends-chain propagation of
// skills / outputContract / mcpImports still works via mergeChain;
// overlay-file-level editing of these fields is a Phase 4.1+ concern.
```

- [ ] **Step 5: Run tests — expect pass**

Run: `go test ./spec/ -run TestMergeChain_MCPImports -v`
Expected: both tests `PASS`.

Run full package: `go test -race -count=1 ./spec/...`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add spec/normalize.go spec/provenance.go spec/overlay.go spec/mcp_extends_test.go
git commit -m "feat(spec): extends-chain propagation of mcpImports; refresh overlay comment"
```

---

### Task 7: mcp.binding@1 factory

**Files:**
- Create: `factories/mcpbinding/factory.go`
- Create: `factories/mcpbinding/factory_test.go`

- [ ] **Step 1: Write failing tests**

Create [factories/mcpbinding/factory_test.go](../../../factories/mcpbinding/factory_test.go):

```go
// SPDX-License-Identifier: Apache-2.0

package mcpbinding

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis-forge/registry"
)

const factoryID registry.ID = "mcp.binding@1.0.0"

func TestFactory_Stdio_Minimal(t *testing.T) {
	b, err := NewFactory(factoryID).Build(context.Background(), map[string]any{
		"id": "fs",
		"connection": map[string]any{
			"transport": "stdio",
			"command":   []any{"/bin/true"},
		},
		"trust": map[string]any{"tier": "low", "owner": "demo"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if b.ID != "fs" || b.Connection.Transport != registry.MCPTransportStdio {
		t.Fatalf("unexpected binding: %+v", b)
	}
	// OnNewTool defaults to "block" when omitted.
	if b.OnNewTool != registry.OnNewToolBlock {
		t.Fatalf("OnNewTool=%q, want block", b.OnNewTool)
	}
}

func TestFactory_HTTPWithBearer(t *testing.T) {
	b, err := NewFactory(factoryID).Build(context.Background(), map[string]any{
		"id": "notion",
		"connection": map[string]any{
			"transport": "http",
			"url":       "https://api.example.com/mcp",
		},
		"auth": map[string]any{
			"credentialRef": "credresolver.env@1.0.0",
			"scheme":        "bearer",
		},
		"trust":    map[string]any{"tier": "medium", "owner": "platform"},
		"policies": []any{"policypack.pii-redaction@1.0.0"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if b.Auth == nil || b.Auth.CredentialRef != "credresolver.env@1.0.0" || b.Auth.Scheme != "bearer" {
		t.Fatalf("Auth=%+v", b.Auth)
	}
	if len(b.Policies) != 1 || b.Policies[0] != "policypack.pii-redaction@1.0.0" {
		t.Fatalf("Policies=%+v", b.Policies)
	}
}

func TestFactory_RejectsTransportFieldMismatch(t *testing.T) {
	_, err := NewFactory(factoryID).Build(context.Background(), map[string]any{
		"id": "fs",
		"connection": map[string]any{
			"transport": "stdio",
			"url":       "https://wrong",
		},
		"trust": map[string]any{"tier": "low", "owner": "demo"},
	})
	if err == nil || !strings.Contains(err.Error(), "transport") {
		t.Fatalf("want transport mismatch, got %v", err)
	}
}

func TestFactory_RejectsMissingID(t *testing.T) {
	_, err := NewFactory(factoryID).Build(context.Background(), map[string]any{
		"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
	})
	if err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("want id error, got %v", err)
	}
}
```

- [ ] **Step 2: Run test — expect compile failure**

Run: `go test ./factories/mcpbinding/...`
Expected: package does not exist.

- [ ] **Step 3: Create factory**

Create [factories/mcpbinding/factory.go](../../../factories/mcpbinding/factory.go):

```go
// SPDX-License-Identifier: Apache-2.0

// Package mcpbinding is the Phase-4 vertical-slice MCP binding factory.
// It decodes a typed config describing an MCP server connection, allow
// / deny policy, attached forge policy packs, trust metadata, and
// on-new-tool behaviour, and produces a registry.MCPBinding value.
// Build is pure: no network, no subprocess. Runtime (praxis) opens the
// session, lists tools live, applies Allow/Deny + Policies, and enforces
// OnNewTool when the server surface changes.
package mcpbinding

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/praxis-os/praxis-forge/registry"
)

type Factory struct{ id registry.ID }

// NewFactory constructs the generic MCP binding factory. The id must
// match `mcp.<name>@<semver>`.
func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "generic MCP binding contract (stdio or streamable-HTTP)" }

// Build decodes cfg into registry.MCPBinding, re-validating as defense
// in depth. Reuses the spec-side rules without importing spec.
func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.MCPBinding, error) {
	if cfg == nil {
		return registry.MCPBinding{}, fmt.Errorf("%s: config required", f.id)
	}

	id, _ := cfg["id"].(string)
	if id == "" {
		return registry.MCPBinding{}, fmt.Errorf("%s: id: required", f.id)
	}

	conn, err := decodeConnection(f.id, cfg["connection"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	auth, err := decodeAuth(f.id, cfg["auth"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	allow, err := decodeGlobList(f.id, "allow", cfg["allow"])
	if err != nil {
		return registry.MCPBinding{}, err
	}
	deny, err := decodeGlobList(f.id, "deny", cfg["deny"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	policies, err := decodePolicyIDs(f.id, cfg["policies"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	trust, err := decodeTrust(f.id, cfg["trust"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	onNewTool, err := decodeOnNewTool(f.id, cfg["onNewTool"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	return registry.MCPBinding{
		ID:         id,
		Connection: conn,
		Auth:       auth,
		Allow:      allow,
		Deny:       deny,
		Policies:   policies,
		Trust:      trust,
		OnNewTool:  onNewTool,
		Descriptor: registry.MCPBindingDescriptor{
			Name:    stringOr(cfg, "name", "generic-mcp-binding"),
			Summary: stringOr(cfg, "summary", "generic MCP binding contract"),
			Tags:    stringListOr(cfg, "tags"),
		},
	}, nil
}

func decodeConnection(id registry.ID, raw any) (registry.MCPConnection, error) {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return registry.MCPConnection{}, fmt.Errorf("%s: connection: required", id)
	}
	transport, _ := m["transport"].(string)
	switch transport {
	case "stdio":
		cmd, err := stringList(m["command"])
		if err != nil || len(cmd) == 0 {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.command: stdio transport requires non-empty command", id)
		}
		if s, _ := m["url"].(string); s != "" {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.url: stdio transport must not set url", id)
		}
		env, err := stringMap(m["env"])
		if err != nil {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.env: %w", id, err)
		}
		return registry.MCPConnection{Transport: registry.MCPTransportStdio, Command: cmd, Env: env}, nil
	case "http":
		url, _ := m["url"].(string)
		if url == "" {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.url: http transport requires url", id)
		}
		if cmd, _ := stringList(m["command"]); len(cmd) > 0 {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.command: http transport must not set command", id)
		}
		headers, err := stringMap(m["headers"])
		if err != nil {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.headers: %w", id, err)
		}
		return registry.MCPConnection{Transport: registry.MCPTransportHTTP, URL: url, Headers: headers}, nil
	default:
		return registry.MCPConnection{}, fmt.Errorf("%s: connection.transport %q: want stdio|http", id, transport)
	}
}

func decodeAuth(id registry.ID, raw any) (*registry.MCPAuth, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: auth: want map, got %T", id, raw)
	}
	credRef, _ := m["credentialRef"].(string)
	if credRef == "" {
		return nil, fmt.Errorf("%s: auth.credentialRef: required", id)
	}
	scheme, _ := m["scheme"].(string)
	if scheme == "" {
		return nil, fmt.Errorf("%s: auth.scheme: required", id)
	}
	headerName, _ := m["headerName"].(string)
	if scheme == "api-key" && headerName == "" {
		return nil, fmt.Errorf("%s: auth.headerName: required when scheme == api-key", id)
	}
	parsed, err := registry.ParseID(credRef)
	if err != nil {
		return nil, fmt.Errorf("%s: auth.credentialRef: %w", id, err)
	}
	return &registry.MCPAuth{CredentialRef: parsed, Scheme: scheme, HeaderName: headerName}, nil
}

func decodeGlobList(id registry.ID, field string, raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	list, err := stringList(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", id, field, err)
	}
	for i, p := range list {
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("%s: %s[%d] %q: %w", id, field, i, p, err)
		}
	}
	return list, nil
}

func decodePolicyIDs(id registry.ID, raw any) ([]registry.ID, error) {
	if raw == nil {
		return nil, nil
	}
	list, err := stringList(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: policies: %w", id, err)
	}
	out := make([]registry.ID, 0, len(list))
	for i, s := range list {
		parsed, err := registry.ParseID(s)
		if err != nil {
			return nil, fmt.Errorf("%s: policies[%d] %q: %w", id, i, s, err)
		}
		out = append(out, parsed)
	}
	return out, nil
}

func decodeTrust(id registry.ID, raw any) (registry.MCPTrust, error) {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return registry.MCPTrust{}, fmt.Errorf("%s: trust: required", id)
	}
	tier, _ := m["tier"].(string)
	owner, _ := m["owner"].(string)
	if tier == "" || owner == "" {
		return registry.MCPTrust{}, fmt.Errorf("%s: trust.tier and trust.owner: required", id)
	}
	tags, err := stringList(m["tags"])
	if err != nil {
		return registry.MCPTrust{}, fmt.Errorf("%s: trust.tags: %w", id, err)
	}
	return registry.MCPTrust{Tier: tier, Owner: owner, Tags: tags}, nil
}

func decodeOnNewTool(id registry.ID, raw any) (registry.OnNewToolPolicy, error) {
	s, ok := raw.(string)
	if !ok || s == "" {
		return registry.OnNewToolBlock, nil // default
	}
	switch s {
	case "block":
		return registry.OnNewToolBlock, nil
	case "allow-if-match-allowlist":
		return registry.OnNewToolAllowIfMatch, nil
	case "require-reapproval":
		return registry.OnNewToolRequireReapprove, nil
	default:
		return "", fmt.Errorf("%s: onNewTool %q: want block|allow-if-match-allowlist|require-reapproval", id, s)
	}
}

func stringList(raw any) ([]string, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case []any:
		out := make([]string, 0, len(v))
		for i, el := range v {
			s, ok := el.(string)
			if !ok {
				return nil, fmt.Errorf("[%d]: want string, got %T", i, el)
			}
			out = append(out, s)
		}
		return out, nil
	case []string:
		return append([]string(nil), v...), nil
	default:
		return nil, fmt.Errorf("want []string or []any, got %T", raw)
	}
}

func stringMap(raw any) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("want map[string]any, got %T", raw)
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%q: want string, got %T", k, v)
		}
		out[k] = s
	}
	return out, nil
}

func stringOr(m map[string]any, key, fallback string) string {
	if s, ok := m[key].(string); ok && s != "" {
		return s
	}
	return fallback
}

func stringListOr(m map[string]any, key string) []string {
	list, _ := stringList(m[key])
	return list
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./factories/mcpbinding/... -v`
Expected: all four tests `PASS`.

Run full suite: `go test -race -count=1 ./...`
Expected: green (no regressions).

- [ ] **Step 5: Commit**

```bash
git add factories/mcpbinding/factory.go factories/mcpbinding/factory_test.go
git commit -m "feat(factories): mcp.binding@1 generic factory (stdio + http transports)"
```

---

### Task 8: resolveMCPBindings in build/

**Files:**
- Create: `build/mcp.go`
- Create: `build/mcp_test.go`

- [ ] **Step 1: Write failing tests**

Create [build/mcp_test.go](../../../build/mcp_test.go):

```go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis-forge/factories/credresolverenv"
	"github.com/praxis-os/praxis-forge/factories/mcpbinding"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

func baseMCPRegistry(t *testing.T) *registry.ComponentRegistry {
	t.Helper()
	r := registry.NewComponentRegistry()
	_ = r.RegisterMCPBinding(mcpbinding.NewFactory("mcp.binding@1.0.0"))
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterCredentialResolver(credresolverenv.NewFactory("credresolver.env@1.0.0"))
	return r
}

func TestResolveMCPBindings_Happy_StdioNoAuth(t *testing.T) {
	s := &spec.AgentSpec{
		MCPImports: []spec.ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "demo"},
			"policies":   []any{"policypack.pii-redaction@1.0.0"},
		}}},
	}
	r := baseMCPRegistry(t)

	res, err := resolveMCPBindings(context.Background(), s, r)
	if err != nil {
		t.Fatalf("resolveMCPBindings: %v", err)
	}
	if len(res) != 1 || res[0].Value.ID != "fs" {
		t.Fatalf("res=%+v", res)
	}
	if len(res[0].Value.Policies) != 1 {
		t.Fatalf("Policies=%+v", res[0].Value.Policies)
	}
}

func TestResolveMCPBindings_UnresolvedPolicy(t *testing.T) {
	s := &spec.AgentSpec{
		MCPImports: []spec.ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "demo"},
			"policies":   []any{"policypack.missing@9.9.9"},
		}}},
	}
	r := baseMCPRegistry(t)

	_, err := resolveMCPBindings(context.Background(), s, r)
	if err == nil || !strings.Contains(err.Error(), "mcp_unresolved_policy") {
		t.Fatalf("want mcp_unresolved_policy, got %v", err)
	}
}

func TestResolveMCPBindings_UnresolvedCredential(t *testing.T) {
	s := &spec.AgentSpec{
		MCPImports: []spec.ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "notion",
			"connection": map[string]any{"transport": "http", "url": "https://e/mcp"},
			"trust":      map[string]any{"tier": "medium", "owner": "demo"},
			"auth":       map[string]any{"credentialRef": "credresolver.missing@9.9.9", "scheme": "bearer"},
		}}},
	}
	r := baseMCPRegistry(t)

	_, err := resolveMCPBindings(context.Background(), s, r)
	if err == nil || !strings.Contains(err.Error(), "mcp_unresolved_credential") {
		t.Fatalf("want mcp_unresolved_credential, got %v", err)
	}
}

func TestResolveMCPBindings_FactoryMissing(t *testing.T) {
	s := &spec.AgentSpec{
		MCPImports: []spec.ComponentRef{{Ref: "mcp.notregistered@1.0.0", Config: map[string]any{}}},
	}
	r := baseMCPRegistry(t)

	_, err := resolveMCPBindings(context.Background(), s, r)
	if err == nil || !strings.Contains(err.Error(), "mcp_unresolved_factory") {
		t.Fatalf("want mcp_unresolved_factory, got %v", err)
	}
}

func TestResolveMCPBindings_EmptyImportsReturnsNil(t *testing.T) {
	s := &spec.AgentSpec{}
	r := baseMCPRegistry(t)

	res, err := resolveMCPBindings(context.Background(), s, r)
	if err != nil {
		t.Fatalf("resolveMCPBindings: %v", err)
	}
	if res != nil {
		t.Fatalf("res=%+v, want nil", res)
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure**

Run: `go test ./build/ -run TestResolveMCPBindings`
Expected: `undefined: resolveMCPBindings`.

- [ ] **Step 3: Implement resolver**

Create [build/mcp.go](../../../build/mcp.go):

```go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

// ResolvedMCPBinding captures one built MCP binding plus the authoring
// ComponentRef so buildManifest can emit the full config verbatim.
type ResolvedMCPBinding struct {
	ID     registry.ID
	Config map[string]any
	Value  registry.MCPBinding
}

// Error codes emitted by resolveMCPBindings. Each wraps into an error
// message that names the binding id and the conflicting reference.
const (
	errCodeMCPUnresolvedFactory    = "mcp_unresolved_factory"
	errCodeMCPUnresolvedPolicy     = "mcp_unresolved_policy"
	errCodeMCPUnresolvedCredential = "mcp_unresolved_credential"
)

// resolveMCPBindings walks spec.mcpImports, resolves each binding's
// factory, builds it, and cross-validates that every Policy and Auth
// credential reference already exists in the component registry. It
// never touches the network: runtime (praxis) owns MCP I/O.
func resolveMCPBindings(
	ctx context.Context,
	s *spec.AgentSpec,
	r *registry.ComponentRegistry,
) ([]ResolvedMCPBinding, error) {
	if len(s.MCPImports) == 0 {
		return nil, nil
	}
	out := make([]ResolvedMCPBinding, 0, len(s.MCPImports))
	for i, ref := range s.MCPImports {
		fac, err := r.MCPBinding(registry.ID(ref.Ref))
		if err != nil {
			return nil, fmt.Errorf("%s: mcpImports[%d] %s: %w", errCodeMCPUnresolvedFactory, i, ref.Ref, err)
		}
		val, err := fac.Build(ctx, ref.Config)
		if err != nil {
			return nil, fmt.Errorf("build mcpImports[%d] %s: %w", i, ref.Ref, err)
		}
		// Cross-kind validation: every declared policy must already be registered.
		for j, pid := range val.Policies {
			if _, err := r.PolicyPack(pid); err != nil {
				return nil, fmt.Errorf("%s: mcpImports[%d] %s: policies[%d] %s: %w",
					errCodeMCPUnresolvedPolicy, i, ref.Ref, j, pid, err)
			}
		}
		if val.Auth != nil {
			if _, err := r.CredentialResolver(val.Auth.CredentialRef); err != nil {
				return nil, fmt.Errorf("%s: mcpImports[%d] %s: auth.credentialRef %s: %w",
					errCodeMCPUnresolvedCredential, i, ref.Ref, val.Auth.CredentialRef, err)
			}
		}
		out = append(out, ResolvedMCPBinding{
			ID:     registry.ID(ref.Ref),
			Config: ref.Config,
			Value:  val,
		})
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./build/ -run TestResolveMCPBindings -v`
Expected: all 5 tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add build/mcp.go build/mcp_test.go
git commit -m "feat(build): resolveMCPBindings with cross-kind validation"
```

---

### Task 9: Wire MCP resolution into Build pipeline

**Files:**
- Modify: `build/resolver.go`
- Modify: `build/build.go`

- [ ] **Step 1: Add failing integration assertion**

Create placeholder [build/build_mcp_test.go](../../../build/build_mcp_test.go) (full end-to-end lands in Task 11; this is the smallest pipeline test):

```go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis-forge/factories/mcpbinding"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

func TestBuild_Pipeline_StampsMCPBindingOnManifest(t *testing.T) {
	s := &spec.AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "mcp.smoke", Version: "0.1.0"},
		Provider:   spec.ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: "prompt.sys@1.0.0"}},
		Policies: []spec.ComponentRef{{Ref: "policypack.pii-redaction@1.0.0",
			Config: map[string]any{"strictness": "medium"}}},
		MCPImports: []spec.ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "demo"},
			"policies":   []any{"policypack.pii-redaction@1.0.0"},
		}}},
	}
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterMCPBinding(mcpbinding.NewFactory("mcp.binding@1.0.0"))

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found := false
	for _, rc := range built.Manifest.Resolved {
		if rc.Kind == "mcp_binding" && rc.ID == "mcp.binding@1.0.0" {
			found = true
			if rc.Config["id"] != "fs" {
				t.Errorf("binding id=%v", rc.Config["id"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("manifest missing mcp_binding entry: %+v", built.Manifest.Resolved)
	}

	present := built.Manifest.Capabilities.Present
	seen := false
	for _, k := range present {
		if k == "mcp_binding" {
			seen = true
		}
	}
	if !seen {
		t.Errorf("mcp_binding not in Capabilities.Present: %v", present)
	}
}
```

- [ ] **Step 2: Run test — expect failure**

Run: `go test ./build/ -run TestBuild_Pipeline_StampsMCPBindingOnManifest`
Expected: fails because `Build` does not yet call `resolveMCPBindings`; manifest has no `mcp_binding` entry.

- [ ] **Step 3: Extend `resolved` struct**

In [build/resolver.go](../../../build/resolver.go), add fields next to the existing Phase-3 block (after `outputContractCfg`):

```go
	mcpBindings    []registry.MCPBinding
	mcpBindingIDs  []registry.ID
	mcpBindingCfgs []map[string]any
```

- [ ] **Step 4: Call `resolveMCPBindings` in `Build`**

In [build/build.go](../../../build/build.go), after the block that stamps skills + output contract onto `res` (around line 59) and before the `appendSkillFragments` call, add:

```go
	mcpBindings, err := resolveMCPBindings(ctx, &expanded.Spec, r)
	if err != nil {
		return nil, err
	}
	res.mcpBindings = make([]registry.MCPBinding, 0, len(mcpBindings))
	res.mcpBindingIDs = make([]registry.ID, 0, len(mcpBindings))
	res.mcpBindingCfgs = make([]map[string]any, 0, len(mcpBindings))
	for _, rb := range mcpBindings {
		res.mcpBindings = append(res.mcpBindings, rb.Value)
		res.mcpBindingIDs = append(res.mcpBindingIDs, rb.ID)
		res.mcpBindingCfgs = append(res.mcpBindingCfgs, rb.Config)
	}
```

Then inside `buildManifest` (same file), after the output-contract stamping block at lines 261–270, append the MCP loop:

```go
	for i, id := range res.mcpBindingIDs {
		desc := res.mcpBindings[i].Descriptor
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:        string(registry.KindMCPBinding),
			ID:          string(id),
			Config:      res.mcpBindingCfgs[i],
			Descriptors: desc,
		})
	}
```

- [ ] **Step 5: Run test — expect failure on capabilities check only**

Run: `go test ./build/ -run TestBuild_Pipeline_StampsMCPBindingOnManifest -v`
Expected: `mcp_binding` entry now present; still fails on `Capabilities.Present` (Task 10 handles that). Commit the current diff anyway so the wiring is captured as a discrete change.

- [ ] **Step 6: Commit**

```bash
git add build/build.go build/resolver.go build/build_mcp_test.go
git commit -m "feat(build): wire MCP resolution into Build pipeline and manifest Resolved"
```

---

### Task 10: Extend capabilities flag

**Files:**
- Modify: `build/capabilities.go`
- Modify: existing capability tests (search for `TestComputeCapabilities` or `capabilities_phase3_test.go`)

- [ ] **Step 1: Write failing test**

Append to [build/capabilities_phase3_test.go](../../../build/capabilities_phase3_test.go):

```go
func TestComputeCapabilities_MCPPresentAndSkipped(t *testing.T) {
	// Present: one binding.
	res := &resolved{mcpBindingIDs: []registry.ID{"mcp.binding@1.0.0"}}
	s := &spec.AgentSpec{MCPImports: []spec.ComponentRef{{Ref: "mcp.binding@1.0.0"}}}
	caps := computeCapabilities(s, res, &ExpandedSpec{Spec: *s})
	presentHasMCP := false
	for _, k := range caps.Present {
		if k == "mcp_binding" {
			presentHasMCP = true
		}
	}
	if !presentHasMCP {
		t.Errorf("Present missing mcp_binding: %v", caps.Present)
	}

	// Skipped: no imports.
	res2 := &resolved{}
	s2 := &spec.AgentSpec{}
	caps2 := computeCapabilities(s2, res2, &ExpandedSpec{Spec: *s2})
	found := false
	for _, sk := range caps2.Skipped {
		if sk.Kind == "mcp_binding" && sk.Reason == "not_specified" {
			found = true
		}
	}
	if !found {
		t.Errorf("Skipped missing mcp_binding/not_specified: %+v", caps2.Skipped)
	}
}
```

- [ ] **Step 2: Run test — expect failure**

Run: `go test ./build/ -run TestComputeCapabilities_MCPPresentAndSkipped -v`
Expected: `mcp_binding` not found in `Present`.

- [ ] **Step 3: Update `computeCapabilities`**

In [build/capabilities.go](../../../build/capabilities.go), append after the output-contract block (around line 80, inside `computeCapabilities`, before the final `sort.Strings(present)`):

```go
	if len(s.MCPImports) > 0 {
		present = append(present, string(registry.KindMCPBinding))
	} else {
		skipped = append(skipped, manifest.CapabilitySkip{Kind: string(registry.KindMCPBinding), Reason: "not_specified"})
	}
```

Update the file-level comment on `computeCapabilities` (lines 13–20) to add a sentence: `Phase 4: mcp_binding is present when spec.mcpImports[] is non-empty, skipped with reason "not_specified" when absent.`

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./build/ -run 'TestComputeCapabilities|TestBuild_Pipeline_StampsMCPBindingOnManifest' -v`
Expected: both tests `PASS`.

Run full package: `go test -race -count=1 ./build/...`
Expected: green.

- [ ] **Step 5: Commit**

```bash
git add build/capabilities.go build/capabilities_phase3_test.go
git commit -m "feat(manifest): mcp_binding capability flag (present/skipped)"
```

---

### Task 11: Reserve `mcp.` tool-name prefix

**Files:**
- Modify: `build/tool_router.go`
- Modify: `build/tool_router_test.go`

- [ ] **Step 1: Write failing test**

Append to [build/tool_router_test.go](../../../build/tool_router_test.go):

```go
func TestToolRouter_RejectsReservedMCPPrefix(t *testing.T) {
	pack := registry.ToolPack{
		Invoker:     canned{},
		Definitions: []llm.ToolDefinition{{Name: "mcp.fs.read_file"}},
	}
	_, _, err := newToolRouter([]registry.ToolPack{pack})
	if err == nil || !errors.Is(err, ErrToolNameReservedPrefix) {
		t.Fatalf("want ErrToolNameReservedPrefix, got %v", err)
	}
}
```

(`canned{}` is the existing stub invoker defined at the top of [tool_router_test.go](../../../build/tool_router_test.go#L15-L19).)

- [ ] **Step 2: Run test — expect failure**

Run: `go test ./build/ -run TestToolRouter_RejectsReservedMCPPrefix`
Expected: `undefined: ErrToolNameReservedPrefix`.

- [ ] **Step 3: Add reserved-prefix check**

In [build/tool_router.go](../../../build/tool_router.go), add a new error and extend `newToolRouter`:

```go
var ErrToolNameReservedPrefix = errors.New("tool name uses reserved prefix mcp.")
```

Inside `newToolRouter`, before the collision check:

```go
		for _, def := range p.Definitions {
			if strings.HasPrefix(def.Name, "mcp.") {
				return nil, nil, fmt.Errorf("%w: %s", ErrToolNameReservedPrefix, def.Name)
			}
			if _, exists := r.byName[def.Name]; exists {
				return nil, nil, fmt.Errorf("%w: %s", ErrToolNameCollision, def.Name)
			}
			...
		}
```

Add `"strings"` to the import block.

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./build/ -run TestToolRouter -v`
Expected: all router tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add build/tool_router.go build/tool_router_test.go
git commit -m "feat(build): reserve mcp.* tool-name prefix for runtime MCP projection"
```

---

### Task 12: Fixtures + integration tests

**Files:**
- Create: 10 fixture directories under `spec/testdata/mcp/` with `spec.yaml` + (success) `want.expanded.hash` or (failure) `err.txt`.
- Modify: `build/build_mcp_test.go` — add fixture-driven cases.

- [ ] **Step 1: Create success fixtures**

Create [spec/testdata/mcp/minimal-stdio/spec.yaml](../../../spec/testdata/mcp/minimal-stdio/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.minimal-stdio
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: fs
      connection:
        transport: stdio
        command:
          - "/bin/true"
      trust:
        tier: low
        owner: demo
```

Create [spec/testdata/mcp/stdio-with-env/spec.yaml](../../../spec/testdata/mcp/stdio-with-env/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.stdio-env
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: fs
      connection:
        transport: stdio
        command:
          - "/bin/true"
        env:
          MCP_ROOT: /tmp/demo
      trust:
        tier: low
        owner: demo
      onNewTool: block
```

Create [spec/testdata/mcp/http-with-bearer/spec.yaml](../../../spec/testdata/mcp/http-with-bearer/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.http-bearer
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: notion
      connection:
        transport: http
        url: https://api.example.com/mcp
      auth:
        credentialRef: credresolver.env@1.0.0
        scheme: bearer
      trust:
        tier: medium
        owner: platform
      policies:
        - policypack.pii-redaction@1.0.0
      allow:
        - "search_*"
      deny:
        - "delete_*"
      onNewTool: require-reapproval
```

Create [spec/testdata/mcp/http-with-apikey/spec.yaml](../../../spec/testdata/mcp/http-with-apikey/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.http-apikey
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: svc
      connection:
        transport: http
        url: https://api.example.com/mcp
      auth:
        credentialRef: credresolver.env@1.0.0
        scheme: api-key
        headerName: X-Api-Key
      trust:
        tier: medium
        owner: platform
```

Create [spec/testdata/mcp/multiple-bindings/spec.yaml](../../../spec/testdata/mcp/multiple-bindings/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.multiple
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: fs
      connection:
        transport: stdio
        command:
          - "/bin/true"
      trust:
        tier: low
        owner: demo
  - ref: mcp.binding@1.0.0
    config:
      id: notion
      connection:
        transport: http
        url: https://api.example.com/mcp
      auth:
        credentialRef: credresolver.env@1.0.0
        scheme: bearer
      trust:
        tier: medium
        owner: platform
```

Create [spec/testdata/mcp/on-new-tool-variants/spec.yaml](../../../spec/testdata/mcp/on-new-tool-variants/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.on-new-tool
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: a
      connection: {transport: stdio, command: ["/bin/true"]}
      trust: {tier: low, owner: demo}
      onNewTool: block
  - ref: mcp.binding@1.0.0
    config:
      id: b
      connection: {transport: stdio, command: ["/bin/true"]}
      trust: {tier: low, owner: demo}
      onNewTool: allow-if-match-allowlist
  - ref: mcp.binding@1.0.0
    config:
      id: c
      connection: {transport: stdio, command: ["/bin/true"]}
      trust: {tier: low, owner: demo}
      onNewTool: require-reapproval
```

- [ ] **Step 2: Create failure fixtures**

Create [spec/testdata/mcp/unresolved-policy/spec.yaml](../../../spec/testdata/mcp/unresolved-policy/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.unresolved-policy
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: fs
      connection:
        transport: stdio
        command:
          - "/bin/true"
      trust: {tier: low, owner: demo}
      policies:
        - policypack.does-not-exist@1.0.0
```

Create [spec/testdata/mcp/unresolved-policy/err.txt](../../../spec/testdata/mcp/unresolved-policy/err.txt):

```
mcp_unresolved_policy
```

Create [spec/testdata/mcp/unresolved-credential/spec.yaml](../../../spec/testdata/mcp/unresolved-credential/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.unresolved-cred
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: svc
      connection:
        transport: http
        url: https://e/mcp
      trust: {tier: low, owner: demo}
      auth:
        credentialRef: credresolver.does-not-exist@1.0.0
        scheme: bearer
```

Create [spec/testdata/mcp/unresolved-credential/err.txt](../../../spec/testdata/mcp/unresolved-credential/err.txt):

```
mcp_unresolved_credential
```

Create [spec/testdata/mcp/duplicate-binding-id/spec.yaml](../../../spec/testdata/mcp/duplicate-binding-id/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.dup-id
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: fs
      connection: {transport: stdio, command: ["/bin/true"]}
      trust: {tier: low, owner: demo}
  - ref: mcp.binding@1.0.0
    config:
      id: fs
      connection: {transport: stdio, command: ["/bin/true"]}
      trust: {tier: low, owner: demo}
```

Create [spec/testdata/mcp/duplicate-binding-id/err.txt](../../../spec/testdata/mcp/duplicate-binding-id/err.txt):

```
mcp_duplicate_id
```

Create [spec/testdata/mcp/transport-field-mismatch/spec.yaml](../../../spec/testdata/mcp/transport-field-mismatch/spec.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: mcp.fixture.transport-mismatch
  version: 0.1.0
provider:
  ref: provider.min@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: fs
      connection:
        transport: stdio
        url: https://should-not-be-here
      trust: {tier: low, owner: demo}
```

Create [spec/testdata/mcp/transport-field-mismatch/err.txt](../../../spec/testdata/mcp/transport-field-mismatch/err.txt):

```
mcp_transport_field_mismatch
```

- [ ] **Step 3: Write fixture-driven tests**

Append to [build/build_mcp_test.go](../../../build/build_mcp_test.go):

```go
func buildFromFixture(t *testing.T, path string) (*BuiltAgent, error) {
	t.Helper()
	s, err := spec.LoadSpec(path)
	if err != nil {
		return nil, err
	}
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterCredentialResolver(credresolverenv.NewFactory("credresolver.env@1.0.0"))
	_ = r.RegisterMCPBinding(mcpbinding.NewFactory("mcp.binding@1.0.0"))
	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		return nil, err
	}
	return Build(context.Background(), ns, r)
}

func TestMCPFixtures_Success(t *testing.T) {
	cases := []string{
		"minimal-stdio",
		"stdio-with-env",
		"http-with-bearer",
		"http-with-apikey",
		"multiple-bindings",
		"on-new-tool-variants",
	}
	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			built, err := buildFromFixture(t, "../spec/testdata/mcp/"+name+"/spec.yaml")
			if err != nil {
				t.Fatalf("Build %s: %v", name, err)
			}
			// Determinism: a second Build of the same spec yields the same NormalizedHash.
			built2, err := buildFromFixture(t, "../spec/testdata/mcp/"+name+"/spec.yaml")
			if err != nil {
				t.Fatalf("second Build %s: %v", name, err)
			}
			if built.Manifest.NormalizedHash != built2.Manifest.NormalizedHash {
				t.Errorf("NormalizedHash drift between builds: %s vs %s",
					built.Manifest.NormalizedHash, built2.Manifest.NormalizedHash)
			}
			// No secret material in manifest JSON (credentialRef stays, but
			// no scheme-specific secret appears because Auth holds only refs).
			raw, err := json.Marshal(built.Manifest)
			if err != nil {
				t.Fatalf("marshal manifest: %v", err)
			}
			if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "password") {
				t.Errorf("manifest contains secret-looking field: %s", raw)
			}
		})
	}
}

func TestMCPFixtures_BuildErrors(t *testing.T) {
	cases := map[string]string{
		"unresolved-policy":       "mcp_unresolved_policy",
		"unresolved-credential":   "mcp_unresolved_credential",
	}
	for name, marker := range cases {
		name, marker := name, marker
		t.Run(name, func(t *testing.T) {
			_, err := buildFromFixture(t, "../spec/testdata/mcp/"+name+"/spec.yaml")
			if err == nil || !strings.Contains(err.Error(), marker) {
				t.Fatalf("want marker %q in error, got %v", marker, err)
			}
		})
	}
}

func TestMCPFixtures_ValidateErrors(t *testing.T) {
	cases := map[string]string{
		"duplicate-binding-id":       "mcp_duplicate_id",
		"transport-field-mismatch":   "mcp_transport_field_mismatch",
	}
	for name, marker := range cases {
		name, marker := name, marker
		t.Run(name, func(t *testing.T) {
			s, err := spec.LoadSpec("../spec/testdata/mcp/" + name + "/spec.yaml")
			if err != nil {
				t.Fatalf("LoadSpec: %v", err)
			}
			err = s.Validate()
			if err == nil || !strings.Contains(err.Error(), marker) {
				t.Fatalf("want marker %q in Validate error, got %v", marker, err)
			}
		})
	}
}
```

Add `"encoding/json"` to the import block if not already present.

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./build/ -run 'TestMCPFixtures|TestBuild_Pipeline_StampsMCPBindingOnManifest' -v`
Expected: all `PASS`.

Run full suite: `go test -race -count=1 ./...`
Expected: green.

- [ ] **Step 5: Commit**

```bash
git add spec/testdata/mcp build/build_mcp_test.go
git commit -m "test(spec+build): MCP fixtures and end-to-end integration"
```

---

### Task 13: Demo flag `-mcp`

**Files:**
- Modify: `examples/demo/main.go`
- Create: `examples/demo/agent-mcp.yaml`

- [ ] **Step 1: Write demo spec**

Create [examples/demo/agent-mcp.yaml](../../../examples/demo/agent-mcp.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: demo.mcp
  version: 0.1.0
  displayName: mcp-binding demo
provider:
  ref: provider.anthropic@1.0.0
  config:
    model: claude-sonnet-4-5
    maxTokens: 1024
prompt:
  system:
    ref: prompt.demo-system@1.0.0
policies:
  - ref: policypack.pii-redaction@1.0.0
    config:
      strictness: medium
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: fs
      connection:
        transport: stdio
        command:
          - npx
          - -y
          - "@modelcontextprotocol/server-filesystem"
          - /tmp/demo
      allow:
        - "read_*"
        - "list_*"
      deny:
        - "write_*"
      policies:
        - policypack.pii-redaction@1.0.0
      trust:
        tier: medium
        owner: demo
      onNewTool: block
```

- [ ] **Step 2: Add `-mcp` flag + registration**

In [examples/demo/main.go](../../../examples/demo/main.go), add a new flag alongside the `structured` one:

```go
	structured := flag.Bool("structured", false, "use the Phase-3 structured-output skill path")
	mcpDemo := flag.Bool("mcp", false, "use the Phase-4 MCP binding demo spec (filesystem server)")
	flag.Parse()
```

Add the import: `"github.com/praxis-os/praxis-forge/factories/mcpbinding"`.

After the `if *structured { ... }` block (around line 67), add:

```go
	if *mcpDemo {
		must(r.RegisterMCPBinding(mcpbinding.NewFactory("mcp.binding@1.0.0")))
	}
```

Extend the spec-path selection around line 73:

```go
	specPath := "examples/demo/agent.yaml"
	switch {
	case *structured:
		specPath = "examples/demo/agent-structured.yaml"
	case *mcpDemo:
		specPath = "examples/demo/agent-mcp.yaml"
	}
```

At the end of `main` (after `Build` succeeds and the existing output prints), add a dedicated MCP-demo print when `*mcpDemo` is true. Read the current tail of `main.go` first, then insert:

```go
	if *mcpDemo {
		fmt.Println("--- MCP binding contract ---")
		for _, rc := range built.Manifest.Resolved {
			if rc.Kind == "mcp_binding" {
				raw, _ := json.MarshalIndent(rc, "", "  ")
				fmt.Println(string(raw))
			}
		}
		fmt.Println("NOTE: binding is a contract; actual MCP invocation is a runtime concern.")
		return
	}
```

Add `"encoding/json"` and `"fmt"` to imports if not present. Verify that `fmt` and `json` are already imported and reuse the existing pattern if so.

- [ ] **Step 3: Run the integration build**

Run: `go build -tags=integration ./examples/demo/`
Expected: compiles clean.

Run: `ANTHROPIC_API_KEY=dummy go run -tags=integration ./examples/demo -mcp`
Expected: prints the `mcp_binding` resolved entry JSON, ending with the NOTE line, then exits before any LLM call. If it errors on Anthropic API key, that's fine — we only need to reach the MCP branch.

- [ ] **Step 4: Verify plain path still works**

Run: `go build -tags=integration ./examples/demo/`
Expected: still clean.

Run: `go test -race -count=1 ./...`
Expected: green.

- [ ] **Step 5: Commit**

```bash
git add examples/demo/main.go examples/demo/agent-mcp.yaml
git commit -m "examples(demo): -mcp flag exercising filesystem binding contract"
```

---

### Task 14: Docs amendments

**Files:**
- Modify: `docs/design/agent-spec-v0.md`
- Modify: `docs/design/forge-overview.md`

- [ ] **Step 1: Drop `mcpImports` from the deferred list**

In [docs/design/agent-spec-v0.md](../../design/agent-spec-v0.md), find the §"Explicit deferrals" / deferred-fields section. Remove any bullet referencing `mcpImports (Phase 4)` (mirroring the way Phase 3 dropped `skills` and `outputContract` bullets). If there is a "Phase 4" reserved line, replace with a sentence acknowledging Phase 4 shipped: `mcpImports: active (Phase 4, landed 2026-04-22) — generic mcp.binding@1 factory, runtime binding contract.`

Search for the exact string using: `grep -n "mcpImports" docs/design/agent-spec-v0.md` first.

- [ ] **Step 2: Mark Phase 4 shipped in the roadmap**

In [docs/design/forge-overview.md](../../design/forge-overview.md), locate the Phase 4 bullet at lines 185–188. Replace:

```markdown
- **Phase 4 — MCP consume.** MCP imports, remote metadata normalization,
  auth/trust metadata, allowlist/denylist, projection into forge tool
  namespace.
```

with:

```markdown
- **Phase 4 (shipped):** MCP imports activated as a declarative runtime
  binding contract. Generic `mcp.binding@1` factory, stdio + streamable-HTTP
  transports, allow/deny globs, policy chain, trust metadata, on-new-tool
  policy. Build time produces a governable contract; runtime (praxis)
  owns session establishment and drift handling. See
  [`docs/superpowers/specs/2026-04-22-praxis-forge-phase-4-design.md`](../superpowers/specs/2026-04-22-praxis-forge-phase-4-design.md).
```

- [ ] **Step 3: Verify links resolve and docs render**

Run: `go test -race -count=1 ./...`
Expected: unchanged / green (docs don't affect tests).

- [ ] **Step 4: Lint + format gate**

Run: `make fmt && make vet && make lint`
Expected: all three pass.

Also run: `make banned-grep`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add docs/design/agent-spec-v0.md docs/design/forge-overview.md
git commit -m "docs: record Phase 4 shipped (MCP consume as runtime binding contract)"
```

---

### Task 15: Final verification

- [ ] **Step 1: Full test suite**

Run: `go test -race -count=1 ./...`
Expected: green, no skips beyond the existing integration-tagged package.

- [ ] **Step 2: Lint + format**

Run: `make fmt && make vet && make lint`
Expected: all green.

- [ ] **Step 3: Banned-grep**

Run: `make banned-grep`
Expected: clean.

- [ ] **Step 4: Smoke the demo (offline)**

Run: `ANTHROPIC_API_KEY=dummy go run -tags=integration ./examples/demo -mcp`
Expected: prints the `mcp_binding` contract JSON + NOTE line, exits cleanly before any LLM call.

- [ ] **Step 5: Inspect manifest shape**

Write a one-off inspection:

```bash
go test -race -count=1 -run TestMCPFixtures_Success ./build/ -v
```

Expected: all fixture sub-tests pass, and the output includes the `minimal-stdio`, `stdio-with-env`, `http-with-bearer`, `http-with-apikey`, `multiple-bindings`, `on-new-tool-variants` sub-tests.

- [ ] **Step 6: Final commit if docs/state drifted**

If any diff remains (e.g. formatting artefacts):

```bash
git status
```

Commit cleanly per-concern. Otherwise, nothing to do.

---

## Completion checklist

- [ ] `KindMCPBinding` active and visible via `Each` enumeration paths (registry).
- [ ] `spec.mcpImports` unlocked, prefix + structural validation live.
- [ ] `factories/mcpbinding/mcp.binding@1` ships and decodes both transports.
- [ ] `build/resolveMCPBindings` runs with cross-kind validation (policies + credentials).
- [ ] Manifest emits `mcp_binding` `ResolvedComponent`s; `Capabilities.Present/Skipped` includes `mcp_binding`.
- [ ] `mcp.` tool-name prefix reserved in the tool router.
- [ ] 10 fixtures under `spec/testdata/mcp/` cover happy paths + all declared error codes.
- [ ] `-mcp` demo flag exercises the path end-to-end without network.
- [ ] `docs/design/agent-spec-v0.md` and `docs/design/forge-overview.md` reflect Phase 4 shipped.
- [ ] `make fmt && make vet && make lint && make banned-grep` all green.
- [ ] `go test -race -count=1 ./...` green.
