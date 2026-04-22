# Task group 3 — Build pipeline: resolveMCPBindings, wiring, capabilities, prefix guard

> Part of [praxis-forge Phase 4 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-22-praxis-forge-phase-4-design.md`](../../specs/2026-04-22-praxis-forge-phase-4-design.md).

**Commits (4 atomic):**
1. `feat(build): resolveMCPBindings with cross-kind validation`
2. `feat(build): wire MCP resolution into Build pipeline and manifest Resolved`
3. `feat(manifest): mcp_binding capability flag (present/skipped)`
4. `feat(build): reserve mcp.* tool-name prefix for runtime MCP projection`

---

### Task 8: resolveMCPBindings in build/

**Files:**
- Create: `build/mcp.go`
- Create: `build/mcp_test.go`

- [ ] **Step 1: Write failing tests**

Create [build/mcp_test.go](../../../../build/mcp_test.go):

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

Create [build/mcp.go](../../../../build/mcp.go):

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
- Create: `build/build_mcp_test.go` (pipeline smoke test; full fixtures land in Task 12)

- [ ] **Step 1: Add failing integration assertion**

Create [build/build_mcp_test.go](../../../../build/build_mcp_test.go) (full end-to-end lands in Task 12; this is the smallest pipeline test):

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

In [build/resolver.go](../../../../build/resolver.go), add fields next to the existing Phase-3 block (after `outputContractCfg`):

```go
	mcpBindings    []registry.MCPBinding
	mcpBindingIDs  []registry.ID
	mcpBindingCfgs []map[string]any
```

- [ ] **Step 4: Call `resolveMCPBindings` in `Build`**

In [build/build.go](../../../../build/build.go), after the block that stamps skills + output contract onto `res` (around line 59) and before the `appendSkillFragments` call, add:

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
Expected: `mcp_binding` entry now present in Resolved; still fails on `Capabilities.Present` (Task 10 handles that). Commit the current diff anyway so the wiring is captured as a discrete change.

- [ ] **Step 6: Commit**

```bash
git add build/build.go build/resolver.go build/build_mcp_test.go
git commit -m "feat(build): wire MCP resolution into Build pipeline and manifest Resolved"
```

---

### Task 10: Extend capabilities flag

**Files:**
- Modify: `build/capabilities.go`
- Modify: `build/capabilities_phase3_test.go` (or the file containing `TestComputeCapabilities`)

- [ ] **Step 1: Write failing test**

Append to [build/capabilities_phase3_test.go](../../../../build/capabilities_phase3_test.go):

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

In [build/capabilities.go](../../../../build/capabilities.go), append after the output-contract block (around line 80, inside `computeCapabilities`, before the final `sort.Strings(present)`):

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

Append to [build/tool_router_test.go](../../../../build/tool_router_test.go):

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

(`canned{}` is the existing stub invoker defined at the top of [tool_router_test.go](../../../../build/tool_router_test.go#L15-L19).)

- [ ] **Step 2: Run test — expect failure**

Run: `go test ./build/ -run TestToolRouter_RejectsReservedMCPPrefix`
Expected: `undefined: ErrToolNameReservedPrefix`.

- [ ] **Step 3: Add reserved-prefix check**

In [build/tool_router.go](../../../../build/tool_router.go), add a new error and extend `newToolRouter`:

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
