# Task group 1 — Spec: unlock mcpImports gate, structural validation, extends-chain merge

> Part of [praxis-forge Phase 4 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-22-praxis-forge-phase-4-design.md`](../../specs/2026-04-22-praxis-forge-phase-4-design.md).

**Commits (3 atomic):**
1. `feat(spec): unlock mcpImports phase gate; enforce mcp. ref prefix`
2. `feat(spec): structural validation for mcpImports (id/transport/auth/onNewTool/globs)`
3. `feat(spec): extends-chain propagation of mcpImports; refresh overlay comment`

---

### Task 5: Unlock mcpImports validation gate + ref prefix check

**Files:**
- Modify: `spec/validate.go`
- Modify: `spec/validate_test.go`

- [ ] **Step 1: Write failing tests**

Append to [spec/validate_test.go](../../../../spec/validate_test.go):

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

Also **delete** the existing phase-gate assertion test `TestValidate_RejectsMCP` at [spec/validate_test.go:144-150](../../../../spec/validate_test.go#L144-L150) — the gate it asserts is about to disappear. Remove the entire function (lines 144–150).

- [ ] **Step 2: Run tests — expect failures**

Run: `go test ./spec/ -run TestValidate_MCPImports -v`
Expected: `TestValidate_MCPImports_GateRemoved` fails with `phase-gated (Phase 4)`; `TestValidate_MCPImports_BadRefPrefix` fails because the existing gate fires first.

- [ ] **Step 3: Remove gate and add prefix validation**

In [spec/validate.go](../../../../spec/validate.go), replace the block at lines 78–84:

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
- Create: `spec/validate_mcp.go`
- Modify: `spec/validate_test.go`

- [ ] **Step 1: Write failing tests**

Append to [spec/validate_test.go](../../../../spec/validate_test.go):

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

Create [spec/validate_mcp.go](../../../../spec/validate_mcp.go):

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

Wire it from [spec/validate.go](../../../../spec/validate.go) by adding one line immediately after the `validateKindPrefixedRef` loop you added in Task 5:

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

Mirrors how Phase 3 added Skills/OutputContract handling in `mergeChain` ([spec/normalize.go:246-255](../../../../spec/normalize.go#L246-L255)). Parent specs that declare `mcpImports` must propagate through the extends chain to children. Overlay-file-level add/remove/replace of MCPImports is deliberately deferred to Phase 4.1+ (consistent with how Phase 3 left `AgentOverlayBody` without Skills/OutputContract fields).

**Files:**
- Modify: `spec/normalize.go`
- Create: `spec/mcp_extends_test.go`
- Modify: `spec/overlay.go` (comment refresh only)

- [ ] **Step 1: Write failing test**

Create [spec/mcp_extends_test.go](../../../../spec/mcp_extends_test.go):

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

Note: check [spec/store.go](../../../../spec/store.go) for the exact `NewMapStore` constructor name; adapt if it differs (Phase 2a fixtures use it, search `NewMapStore` or `MapStore` in existing tests to confirm signature).

- [ ] **Step 2: Run tests — expect failure**

Run: `go test ./spec/ -run TestMergeChain_MCPImports -v`
Expected: parent's MCPImports do not propagate to child.

- [ ] **Step 3: Extend `mergeChain`**

In [spec/normalize.go](../../../../spec/normalize.go), find the Phase-3 block handling `Skills` and `OutputContract` (around lines 246–255) and append:

```go
	if child.MCPImports != nil {
		merged.MCPImports = child.MCPImports
		prov.MCPImports = p
	}
```

Update the comment on that block (line 246) to: `// Phase 3: Skills and OutputContract; Phase 4: MCPImports. All propagate child-wins per replaceable-list semantics.`

If `prov` (Provenance struct) does not yet have an `MCPImports` field, add it. Check [spec/provenance.go](../../../../spec/provenance.go) first:

```bash
grep -n "Skills\|OutputContract\|MCPImports" spec/provenance.go
```

If `MCPImports` is absent, mirror the `Skills` field definition exactly.

- [ ] **Step 4: Refresh the stale overlay comment**

In [spec/overlay.go](../../../../spec/overlay.go), replace the outdated comment (search for the line containing "extends, skills, mcpImports, outputContract are absent"):

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
