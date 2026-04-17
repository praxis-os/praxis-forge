> Part of [praxis-forge Phase 1 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-15-praxis-forge-phase-1-design.md`](../../specs/2026-04-15-praxis-forge-phase-1-design.md).

## Task group 5 — Facade, fakeprovider, integration, demo

### Task 5.1: `forge.go` facade

**Files:**
- Modify: `doc.go` (package comment already exists — no change needed)
- Create: `forge.go`

- [ ] **Step 1: Write**

```go
// forge.go
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"

	praxis "github.com/praxis-os/praxis"

	"github.com/praxis-os/praxis-forge/build"
	"github.com/praxis-os/praxis-forge/manifest"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

// BuiltAgent is the result of a successful Build. Its Invoke is a stateless
// pass-through to the embedded Orchestrator; conversation history lives in
// the caller.
type BuiltAgent struct {
	inner *build.BuiltAgent
}

func (b *BuiltAgent) Invoke(ctx context.Context, req praxis.InvocationRequest) (*praxis.InvocationResult, error) {
	return b.inner.Orchestrator.Invoke(ctx, req)
}

func (b *BuiltAgent) Manifest() manifest.Manifest { return b.inner.Manifest }

// LoadSpec reads and decodes an AgentSpec YAML file.
func LoadSpec(path string) (*spec.AgentSpec, error) {
	return spec.LoadSpec(path)
}

// Build validates the spec, freezes the registry, resolves every component,
// composes the kernel options, and materializes a BuiltAgent.
func Build(ctx context.Context, s *spec.AgentSpec, r *registry.ComponentRegistry, opts ...Option) (*BuiltAgent, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	inner, err := build.Build(ctx, s, r)
	if err != nil {
		return nil, err
	}
	return &BuiltAgent{inner: inner}, nil
}

// Option is a build-time knob for forge itself (distinct from kernel options).
type Option func(*options)

type options struct {
	// Reserved for Phase 2. Phase 1 has no knobs but keeps the type shape
	// stable so callers can adopt now.
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add forge.go
git commit -m "feat: top-level forge facade (LoadSpec, Build, BuiltAgent)"
```

---

### Task 5.2: `internal/testutil/fakeprovider`

**Files:**
- Create: `internal/testutil/fakeprovider/fakeprovider.go`
- Create: `internal/testutil/fakeprovider/fakeprovider_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/testutil/fakeprovider/fakeprovider_test.go
package fakeprovider

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/llm"
)

func TestProvider_CompleteReturnsCanned(t *testing.T) {
	p := New(llm.LLMResponse{Message: llm.Message{Role: "assistant", Content: "hi"}})
	resp, err := p.Complete(context.Background(), llm.LLMRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message.Content != "hi" {
		t.Fatalf("content=%q", resp.Message.Content)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./internal/testutil/fakeprovider/... -v`

- [ ] **Step 3: Implement**

```go
// internal/testutil/fakeprovider/fakeprovider.go
// SPDX-License-Identifier: Apache-2.0

// Package fakeprovider is a deterministic llm.Provider for unit tests. It
// returns the canned LLMResponse passed to New on every Complete call.
package fakeprovider

import (
	"context"

	"github.com/praxis-os/praxis/llm"
)

type Provider struct{ resp llm.LLMResponse }

func New(resp llm.LLMResponse) *Provider { return &Provider{resp: resp} }

func (p *Provider) Name() string                    { return "fake" }
func (p *Provider) SupportsParallelToolCalls() bool { return false }
func (p *Provider) Capabilities() llm.Capabilities  { return llm.Capabilities{} }

func (p *Provider) Complete(_ context.Context, _ llm.LLMRequest) (llm.LLMResponse, error) {
	return p.resp, nil
}

func (p *Provider) Stream(_ context.Context, _ llm.LLMRequest) (<-chan llm.LLMStreamChunk, error) {
	ch := make(chan llm.LLMStreamChunk)
	close(ch)
	return ch, nil
}
```

Verify the real `llm.LLMResponse` and `llm.Message` shapes — the test and implementation must match them exactly.

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./internal/testutil/fakeprovider/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/testutil/fakeprovider
git commit -m "test: fakeprovider for offline integration tests"
```

---

### Task 5.3: Offline full-stack integration test

**Files:**
- Create: `forge_test.go`
- Create: `testdata/agent.yaml`

- [ ] **Step 1: Create fixture**

```yaml
# testdata/agent.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: forge.integration.test
  version: 0.1.0
provider:
  ref: provider.fake@1.0.0
prompt:
  system:
    ref: prompt.sys@1.0.0
    config:
      text: "You are a test agent."
tools: []
policies:
  - ref: policypack.pii-redaction@1.0.0
    config:
      strictness: low
filters:
  preLLM:
    - ref: filter.secret-scrubber@1.0.0
  preTool:
    - ref: filter.path-escape@1.0.0
  postTool:
    - ref: filter.output-truncate@1.0.0
      config:
        maxBytes: 1024
budget:
  ref: budgetprofile.default-tier1@1.0.0
  overrides:
    maxToolCalls: 5
telemetry:
  ref: telemetryprofile.slog@1.0.0
  config:
    level: info
credentials:
  ref: credresolver.env@1.0.0
identity:
  ref: identitysigner.ed25519@1.0.0
  config:
    issuer: forge-test
    tokenLifetimeSeconds: 60
```

- [ ] **Step 2: Write integration test**

```go
// forge_test.go
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	praxis "github.com/praxis-os/praxis"

	"github.com/praxis-os/praxis-forge"
	"github.com/praxis-os/praxis-forge/factories/budgetprofiledefault"
	"github.com/praxis-os/praxis-forge/factories/credresolverenv"
	"github.com/praxis-os/praxis-forge/factories/filteroutputtruncate"
	"github.com/praxis-os/praxis-forge/factories/filterpathescape"
	"github.com/praxis-os/praxis-forge/factories/filtersecretscrubber"
	"github.com/praxis-os/praxis-forge/factories/identitysignered25519"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
	"github.com/praxis-os/praxis-forge/factories/promptassetliteral"
	"github.com/praxis-os/praxis-forge/factories/telemetryprofileslog"
	"github.com/praxis-os/praxis-forge/internal/testutil/fakeprovider"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
)

// fakeProviderFactory wraps fakeprovider as a forge Provider factory.
type fakeProviderFactory struct {
	id   registry.ID
	resp llm.LLMResponse
}

func (f fakeProviderFactory) ID() registry.ID     { return f.id }
func (f fakeProviderFactory) Description() string { return "fake" }
func (f fakeProviderFactory) Build(context.Context, map[string]any) (llm.Provider, error) {
	return fakeprovider.New(f.resp), nil
}

func TestForge_FullSlice_Offline(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	r := registry.NewComponentRegistry()
	mustRegister(t, r.RegisterProvider(fakeProviderFactory{
		id:   "provider.fake@1.0.0",
		resp: llm.LLMResponse{Message: llm.Message{Role: "assistant", Content: "hi"}},
	}))
	mustRegister(t, r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.sys@1.0.0")))
	mustRegister(t, r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0")))
	mustRegister(t, r.RegisterPreLLMFilter(filtersecretscrubber.NewFactory("filter.secret-scrubber@1.0.0")))
	mustRegister(t, r.RegisterPreToolFilter(filterpathescape.NewFactory("filter.path-escape@1.0.0")))
	mustRegister(t, r.RegisterPostToolFilter(filteroutputtruncate.NewFactory("filter.output-truncate@1.0.0")))
	mustRegister(t, r.RegisterBudgetProfile(budgetprofiledefault.NewFactory("budgetprofile.default-tier1@1.0.0")))
	mustRegister(t, r.RegisterTelemetryProfile(telemetryprofileslog.NewFactory("telemetryprofile.slog@1.0.0", nil)))
	mustRegister(t, r.RegisterCredentialResolver(credresolverenv.NewFactory("credresolver.env@1.0.0")))
	mustRegister(t, r.RegisterIdentitySigner(identitysignered25519.NewFactory("identitysigner.ed25519@1.0.0", priv)))

	s, err := forge.LoadSpec("testdata/agent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	b, err := forge.Build(context.Background(), s, r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Manifest should list every one of the 10 resolved (no tool_pack in fixture).
	m := b.Manifest()
	got := map[string]bool{}
	for _, rc := range m.Resolved {
		got[rc.Kind] = true
	}
	wantKinds := []string{
		"provider", "prompt_asset", "policy_pack",
		"pre_llm_filter", "pre_tool_filter", "post_tool_filter",
		"budget_profile", "telemetry_profile", "credential_resolver", "identity_signer",
	}
	for _, k := range wantKinds {
		if !got[k] {
			t.Errorf("manifest missing kind %q: %+v", k, m.Resolved)
		}
	}

	// Registry now frozen.
	if err := r.RegisterProvider(fakeProviderFactory{id: "provider.other@1.0.0"}); err == nil {
		t.Fatal("expected registry frozen after Build")
	}

	// Invoke round-trip through the fake provider.
	res, err := b.Invoke(context.Background(), praxis.InvocationRequest{
		Model:        "fake",
		SystemPrompt: "You are a test agent.",
		Messages:     []llm.Message{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
}

func mustRegister(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("register: %v", err)
	}
}
```

- [ ] **Step 3: Run**

Run: `go test ./... -race`
Expected: every package green, including the new `forge_test.go`.

- [ ] **Step 4: Commit**

```bash
git add forge_test.go testdata
git commit -m "test: full-stack offline integration test through all 10 non-tool kinds"
```

---

### Task 5.4: `examples/demo` — real Anthropic round-trip

**Files:**
- Create: `examples/demo/main.go`
- Create: `examples/demo/agent.yaml`

- [ ] **Step 1: Create demo spec**

```yaml
# examples/demo/agent.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: forge.examples.demo
  version: 0.1.0
  displayName: "Forge demo agent"
provider:
  ref: provider.anthropic@1.0.0
  config:
    model: claude-sonnet-4-5
    maxOutputTokens: 512
    temperature: 0.2
prompt:
  system:
    ref: prompt.demo-system@1.0.0
    config:
      text: |
        You are a careful research assistant. Use the http_get tool to
        fetch the URL the user names, then summarize its content in
        2-3 sentences. Do not fetch other URLs.
tools:
  - ref: toolpack.http-get@1.0.0
    config:
      allowedHosts:
        - raw.githubusercontent.com
      timeoutMs: 5000
policies:
  - ref: policypack.pii-redaction@1.0.0
    config:
      strictness: medium
filters:
  preLLM:
    - ref: filter.secret-scrubber@1.0.0
  preTool:
    - ref: filter.path-escape@1.0.0
  postTool:
    - ref: filter.output-truncate@1.0.0
      config:
        maxBytes: 16384
budget:
  ref: budgetprofile.default-tier1@1.0.0
  overrides:
    maxWallClock: 20s
    maxToolCalls: 4
telemetry:
  ref: telemetryprofile.slog@1.0.0
  config:
    level: info
credentials:
  ref: credresolver.env@1.0.0
identity:
  ref: identitysigner.ed25519@1.0.0
  config:
    issuer: forge-demo
    tokenLifetimeSeconds: 60
```

- [ ] **Step 2: Write the demo main**

```go
//go:build integration
// +build integration

// SPDX-License-Identifier: Apache-2.0

// Command demo exercises the full praxis-forge Phase 1 path against a real
// Anthropic provider. Build-tagged "integration" so plain `go test ./...`
// stays offline.
//
// Usage:
//   ANTHROPIC_API_KEY=sk-... go run -tags=integration ./examples/demo
//   ANTHROPIC_API_KEY=sk-... go test -tags=integration ./examples/demo
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"log/slog"
	"os"

	praxis "github.com/praxis-os/praxis"

	"github.com/praxis-os/praxis-forge"
	"github.com/praxis-os/praxis-forge/factories/budgetprofiledefault"
	"github.com/praxis-os/praxis-forge/factories/credresolverenv"
	"github.com/praxis-os/praxis-forge/factories/filteroutputtruncate"
	"github.com/praxis-os/praxis-forge/factories/filterpathescape"
	"github.com/praxis-os/praxis-forge/factories/filtersecretscrubber"
	"github.com/praxis-os/praxis-forge/factories/identitysignered25519"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
	"github.com/praxis-os/praxis-forge/factories/promptassetliteral"
	"github.com/praxis-os/praxis-forge/factories/provideranthropic"
	"github.com/praxis-os/praxis-forge/factories/telemetryprofileslog"
	"github.com/praxis-os/praxis-forge/factories/toolpackhttpget"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY not set")
	}
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	r := registry.NewComponentRegistry()
	must(r.RegisterProvider(provideranthropic.NewFactory("provider.anthropic@1.0.0", apiKey)))
	must(r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.demo-system@1.0.0")))
	must(r.RegisterToolPack(toolpackhttpget.NewFactory("toolpack.http-get@1.0.0")))
	must(r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0")))
	must(r.RegisterPreLLMFilter(filtersecretscrubber.NewFactory("filter.secret-scrubber@1.0.0")))
	must(r.RegisterPreToolFilter(filterpathescape.NewFactory("filter.path-escape@1.0.0")))
	must(r.RegisterPostToolFilter(filteroutputtruncate.NewFactory("filter.output-truncate@1.0.0")))
	must(r.RegisterBudgetProfile(budgetprofiledefault.NewFactory("budgetprofile.default-tier1@1.0.0")))
	must(r.RegisterTelemetryProfile(telemetryprofileslog.NewFactory("telemetryprofile.slog@1.0.0", slog.Default())))
	must(r.RegisterCredentialResolver(credresolverenv.NewFactory("credresolver.env@1.0.0")))
	must(r.RegisterIdentitySigner(identitysignered25519.NewFactory("identitysigner.ed25519@1.0.0", priv)))

	ctx := context.Background()
	s, err := forge.LoadSpec("examples/demo/agent.yaml")
	if err != nil {
		log.Fatalf("load spec: %v", err)
	}

	b, err := forge.Build(ctx, s, r)
	if err != nil {
		log.Fatalf("build: %v", err)
	}

	url := "https://raw.githubusercontent.com/praxis-os/praxis-forge/main/README.md"
	res, err := b.Invoke(ctx, praxis.InvocationRequest{
		Model:        "claude-sonnet-4-5",
		SystemPrompt: "(resolved by forge)",
		Messages: []llm.Message{{
			Role:    "user",
			Content: fmt.Sprintf("Fetch %s and summarize what praxis-forge does.", url),
		}},
	})
	if err != nil {
		log.Fatalf("invoke: %v", err)
	}
	fmt.Printf("response: %s\n", res.Response.Content)
	fmt.Printf("manifest components: %d\n", len(b.Manifest().Resolved))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 3: Build verification (offline only)**

Run: `go build -tags=integration ./examples/demo/...`
Expected: clean build. Do not invoke the binary unless `ANTHROPIC_API_KEY` is set.

- [ ] **Step 4: Commit**

```bash
git add examples/demo
git commit -m "examples: realistic demo agent (Anthropic + http_get + full filter chain)"
```

---

### Task 5.5: README update

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Replace the status section**

Change:

```markdown
## Status

**Phase 0 — architecture and contracts.** No runtime code yet.
See [`docs/`](docs/):
```

to:

```markdown
## Status

**Phase 1 — minimum vertical slice (in progress).** See
[`docs/superpowers/specs/2026-04-15-praxis-forge-phase-1-design.md`](docs/superpowers/specs/2026-04-15-praxis-forge-phase-1-design.md)
and [`docs/superpowers/plans/2026-04-15-praxis-forge-phase-1.md`](docs/superpowers/plans/2026-04-15-praxis-forge-phase-1.md).

Phase 0 architecture docs remain authoritative:
```

(Keep the bullet list of ADR + design docs.)

Add a quickstart block after:

````markdown
## Quickstart

```go
r := registry.NewComponentRegistry()
// ... Register factories for every kind referenced by the spec ...

s, err := forge.LoadSpec("agent.yaml")
if err != nil { log.Fatal(err) }

b, err := forge.Build(ctx, s, r)
if err != nil { log.Fatal(err) }

res, err := b.Invoke(ctx, praxis.InvocationRequest{
    Model:    "claude-sonnet-4-5",
    Messages: []llm.Message{{Role: "user", Content: "hello"}},
})
```

See [`examples/demo`](examples/demo/) for a full realistic example.
````

- [ ] **Step 2: Verify render**

Run: `cat README.md | head -60` and eyeball for typos.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): Phase 1 status + quickstart"
```

---

### Task 5.6: CI workflow

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write workflow**

```yaml
# .github/workflows/ci.yml
name: ci
on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          path: praxis-forge
      - uses: actions/checkout@v4
        with:
          repository: praxis-os/praxis
          path: praxis
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
          cache-dependency-path: praxis-forge/go.sum
      - name: Test
        working-directory: praxis-forge
        run: go test ./... -race
      - name: Vet
        working-directory: praxis-forge
        run: go vet ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          path: praxis-forge
      - uses: actions/checkout@v4
        with:
          repository: praxis-os/praxis
          path: praxis
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
      - uses: golangci/golangci-lint-action@v6
        with:
          working-directory: praxis-forge
          version: latest
```

The workflow side-by-side clones praxis because `go.mod` uses `replace ../praxis`.

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: unit tests + vet + golangci-lint"
```

---

