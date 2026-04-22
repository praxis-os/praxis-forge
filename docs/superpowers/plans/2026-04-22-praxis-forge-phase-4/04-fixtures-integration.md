# Task group 4 — Fixtures + end-to-end integration tests

> Part of [praxis-forge Phase 4 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-22-praxis-forge-phase-4-design.md`](../../specs/2026-04-22-praxis-forge-phase-4-design.md).

**Commit (atomic):** `test(spec+build): MCP fixtures and end-to-end integration`

---

### Task 12: Fixtures + integration tests

**Files:**
- Create: 10 fixture directories under `spec/testdata/mcp/` with `spec.yaml` + (success) `want.expanded.hash` or (failure) `err.txt`.
- Modify: `build/build_mcp_test.go` — add fixture-driven cases.

- [ ] **Step 1: Create success fixtures**

Create [spec/testdata/mcp/minimal-stdio/spec.yaml](../../../../spec/testdata/mcp/minimal-stdio/spec.yaml):

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

Create [spec/testdata/mcp/stdio-with-env/spec.yaml](../../../../spec/testdata/mcp/stdio-with-env/spec.yaml):

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

Create [spec/testdata/mcp/http-with-bearer/spec.yaml](../../../../spec/testdata/mcp/http-with-bearer/spec.yaml):

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

Create [spec/testdata/mcp/http-with-apikey/spec.yaml](../../../../spec/testdata/mcp/http-with-apikey/spec.yaml):

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

Create [spec/testdata/mcp/multiple-bindings/spec.yaml](../../../../spec/testdata/mcp/multiple-bindings/spec.yaml):

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

Create [spec/testdata/mcp/on-new-tool-variants/spec.yaml](../../../../spec/testdata/mcp/on-new-tool-variants/spec.yaml):

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

Create [spec/testdata/mcp/unresolved-policy/spec.yaml](../../../../spec/testdata/mcp/unresolved-policy/spec.yaml):

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

Create [spec/testdata/mcp/unresolved-policy/err.txt](../../../../spec/testdata/mcp/unresolved-policy/err.txt):

```
mcp_unresolved_policy
```

Create [spec/testdata/mcp/unresolved-credential/spec.yaml](../../../../spec/testdata/mcp/unresolved-credential/spec.yaml):

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

Create [spec/testdata/mcp/unresolved-credential/err.txt](../../../../spec/testdata/mcp/unresolved-credential/err.txt):

```
mcp_unresolved_credential
```

Create [spec/testdata/mcp/duplicate-binding-id/spec.yaml](../../../../spec/testdata/mcp/duplicate-binding-id/spec.yaml):

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

Create [spec/testdata/mcp/duplicate-binding-id/err.txt](../../../../spec/testdata/mcp/duplicate-binding-id/err.txt):

```
mcp_duplicate_id
```

Create [spec/testdata/mcp/transport-field-mismatch/spec.yaml](../../../../spec/testdata/mcp/transport-field-mismatch/spec.yaml):

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

Create [spec/testdata/mcp/transport-field-mismatch/err.txt](../../../../spec/testdata/mcp/transport-field-mismatch/err.txt):

```
mcp_transport_field_mismatch
```

- [ ] **Step 3: Write fixture-driven tests**

Append to [build/build_mcp_test.go](../../../../build/build_mcp_test.go):

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
		"unresolved-policy":     "mcp_unresolved_policy",
		"unresolved-credential": "mcp_unresolved_credential",
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
		"duplicate-binding-id":     "mcp_duplicate_id",
		"transport-field-mismatch": "mcp_transport_field_mismatch",
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

Add `"encoding/json"` to the import block of `build/build_mcp_test.go` if not already present.

Also add the missing factory imports to the `buildFromFixture` helper's file:

```go
import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/praxis-os/praxis-forge/factories/credresolverenv"
	"github.com/praxis-os/praxis-forge/factories/mcpbinding"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)
```

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
