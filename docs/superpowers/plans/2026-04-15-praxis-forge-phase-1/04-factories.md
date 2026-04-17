> Part of [praxis-forge Phase 1 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-15-praxis-forge-phase-1-design.md`](../../specs/2026-04-15-praxis-forge-phase-1-design.md).

## Task group 4 — Concrete factories

Order is chosen so each task introduces the smallest new surface. Every factory follows the same skeleton:

```go
// factories/<pkg>/factory.go
package <pkg>

import "github.com/praxis-os/praxis-forge/registry"

type Factory struct {
	id   registry.ID
	/* per-factory construction-time dependencies */
}

func NewFactory(id registry.ID /* + deps */) *Factory { ... }

func (f *Factory) ID() registry.ID         { return f.id }
func (f *Factory) Description() string     { return "..." }
func (f *Factory) Build(ctx context.Context, cfg map[string]any) (<ResultType>, error) { ... }
```

Each factory has a `decode` helper that converts `map[string]any` to the factory's typed Config struct, validating field presence and value ranges.

### Task 4.1: `factories/promptassetliteral`

**Files:**
- Create: `factories/promptassetliteral/factory.go`
- Create: `factories/promptassetliteral/factory_test.go`

- [ ] **Step 1: Write failing test**

```go
// factories/promptassetliteral/factory_test.go
package promptassetliteral

import (
	"context"
	"strings"
	"testing"
)

func TestFactory_BuildsText(t *testing.T) {
	f := NewFactory("prompt.sys@1.0.0")
	s, err := f.Build(context.Background(), map[string]any{"text": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello" {
		t.Fatalf("got=%q", s)
	}
}

func TestFactory_RejectsEmptyText(t *testing.T) {
	f := NewFactory("prompt.sys@1.0.0")
	_, err := f.Build(context.Background(), map[string]any{"text": ""})
	if err == nil || !strings.Contains(err.Error(), "text") {
		t.Fatalf("err=%v", err)
	}
}

func TestFactory_RejectsMissingText(t *testing.T) {
	f := NewFactory("prompt.sys@1.0.0")
	_, err := f.Build(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/promptassetliteral/... -v`

- [ ] **Step 3: Implement**

```go
// factories/promptassetliteral/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package promptassetliteral is the simplest prompt_asset factory: it returns
// the literal `text` string from its config. Register one Factory per prompt
// id your application needs.
package promptassetliteral

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "literal prompt asset" }

func (f *Factory) Build(_ context.Context, cfg map[string]any) (string, error) {
	raw, ok := cfg["text"]
	if !ok {
		return "", fmt.Errorf("%s: config.text: required", f.id)
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s: config.text: want string, got %T", f.id, raw)
	}
	if s == "" {
		return "", fmt.Errorf("%s: config.text: must be non-empty", f.id)
	}
	return s, nil
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/promptassetliteral/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/promptassetliteral
git commit -m "feat(factories): prompt.literal@1 — literal prompt asset"
```

---

### Task 4.2: `factories/budgetprofiledefault`

**Files:**
- Create: `factories/budgetprofiledefault/factory.go`
- Create: `factories/budgetprofiledefault/factory_test.go`

Note: praxis's in-memory budget guard lives at `github.com/praxis-os/praxis/budget`. If `budget.NewInMemoryGuard(cfg)` is unavailable, fall back to `budget.NullGuard{}` and document the gap in the commit message. The plan assumes `NullGuard` for Phase 1 since no enforcement is required for tests.

- [ ] **Step 1: Write failing test**

```go
// factories/budgetprofiledefault/factory_test.go
package budgetprofiledefault

import (
	"context"
	"testing"
	"time"
)

func TestFactory_Defaults(t *testing.T) {
	f := NewFactory("budgetprofile.default-tier1@1.0.0")
	bp, err := f.Build(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bp.Guard == nil {
		t.Fatal("guard nil")
	}
	if bp.DefaultConfig.MaxWallClock != 30*time.Second {
		t.Fatalf("wall=%v", bp.DefaultConfig.MaxWallClock)
	}
	if bp.DefaultConfig.MaxToolCalls != 24 {
		t.Fatalf("calls=%d", bp.DefaultConfig.MaxToolCalls)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/budgetprofiledefault/... -v`

- [ ] **Step 3: Implement**

```go
// factories/budgetprofiledefault/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package budgetprofiledefault provides a conservative default-tier-1 budget
// profile: 30s wall, 50k in, 10k out, 24 tool calls, 500k microdollars.
package budgetprofiledefault

import (
	"context"
	"time"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/budget"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "default tier-1 budget profile" }

func (f *Factory) Build(_ context.Context, _ map[string]any) (registry.BudgetProfile, error) {
	return registry.BudgetProfile{
		Guard: budget.NullGuard{},
		DefaultConfig: budget.Config{
			MaxWallClock:        30 * time.Second,
			MaxInputTokens:      50_000,
			MaxOutputTokens:     10_000,
			MaxToolCalls:        24,
			MaxCostMicrodollars: 500_000,
		},
	}, nil
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/budgetprofiledefault/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/budgetprofiledefault
git commit -m "feat(factories): budgetprofile.default-tier1@1 — conservative default ceilings"
```

---

### Task 4.3: `factories/telemetryprofileslog`

**Files:**
- Create: `factories/telemetryprofileslog/factory.go`
- Create: `factories/telemetryprofileslog/factory_test.go`

- [ ] **Step 1: Write failing test**

```go
// factories/telemetryprofileslog/factory_test.go
package telemetryprofileslog

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/event"
)

func TestFactory_EmitsSlog(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	f := NewFactory("telemetryprofile.slog@1.0.0", log)
	tp, err := f.Build(context.Background(), map[string]any{"level": "info"})
	if err != nil {
		t.Fatal(err)
	}
	ev := event.InvocationEvent{Kind: "invocation.started"}
	if err := tp.Emitter.Emit(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "invocation.started") {
		t.Fatalf("buf=%q", buf.String())
	}
}
```

Note: the exact shape of `event.InvocationEvent` may vary. If its field is `Name` not `Kind`, adjust — see [praxis/event](../../../../praxis/event/) when implementing.

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/telemetryprofileslog/... -v`

- [ ] **Step 3: Implement**

```go
// factories/telemetryprofileslog/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package telemetryprofileslog is a telemetry_profile factory that emits one
// slog record per lifecycle event and extracts tenant/user attributes from
// context.
package telemetryprofileslog

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/telemetry"
)

type Factory struct {
	id  registry.ID
	log *slog.Logger
}

func NewFactory(id registry.ID, log *slog.Logger) *Factory {
	if log == nil {
		log = slog.Default()
	}
	return &Factory{id: id, log: log}
}

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "slog lifecycle emitter + enricher" }

type config struct {
	Level string
}

func decode(cfg map[string]any) (config, error) {
	c := config{Level: "info"}
	if raw, ok := cfg["level"]; ok {
		s, ok := raw.(string)
		if !ok {
			return c, fmt.Errorf("level: want string, got %T", raw)
		}
		switch s {
		case "debug", "info":
			c.Level = s
		default:
			return c, fmt.Errorf("level: want debug|info, got %q", s)
		}
	}
	return c, nil
}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.TelemetryProfile, error) {
	c, err := decode(cfg)
	if err != nil {
		return registry.TelemetryProfile{}, fmt.Errorf("%s: %w", f.id, err)
	}
	return registry.TelemetryProfile{
		Emitter:  &emitter{log: f.log, level: c.Level},
		Enricher: &enricher{},
	}, nil
}

type emitter struct {
	log   *slog.Logger
	level string
}

func (e *emitter) Emit(ctx context.Context, ev event.InvocationEvent) error {
	lvl := slog.LevelInfo
	if e.level == "debug" {
		lvl = slog.LevelDebug
	}
	e.log.Log(ctx, lvl, "invocation.event", "kind", ev.Kind)
	return nil
}

type enricher struct{}

// Keys the enricher reads from context. Callers set them via context.WithValue.
type ctxKey string

const (
	TenantKey ctxKey = "forge.tenant"
	UserKey   ctxKey = "forge.user"
)

func (e *enricher) Enrich(ctx context.Context) map[string]string {
	out := map[string]string{}
	if v, ok := ctx.Value(TenantKey).(string); ok && v != "" {
		out["tenant"] = v
	}
	if v, ok := ctx.Value(UserKey).(string); ok && v != "" {
		out["user"] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Compile-time interface checks.
var (
	_ telemetry.LifecycleEventEmitter = (*emitter)(nil)
	_ telemetry.AttributeEnricher     = (*enricher)(nil)
)
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/telemetryprofileslog/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/telemetryprofileslog
git commit -m "feat(factories): telemetryprofile.slog@1 — slog-backed lifecycle emitter"
```

---

### Task 4.4: `factories/credresolverenv`

**Files:**
- Create: `factories/credresolverenv/factory.go`
- Create: `factories/credresolverenv/factory_test.go`

- [ ] **Step 1: Write failing test**

```go
// factories/credresolverenv/factory_test.go
package credresolverenv

import (
	"context"
	"os"
	"testing"
)

func TestResolver_FetchFromEnv(t *testing.T) {
	t.Setenv("FORGE_CRED_NET_HTTP", "secret-value")
	f := NewFactory("credresolver.env@1.0.0")
	r, err := f.Build(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.Fetch(context.Background(), "net:http")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if string(c.Bytes()) != "secret-value" {
		t.Fatalf("got=%q", string(c.Bytes()))
	}
}

func TestResolver_MissingEnvVar(t *testing.T) {
	os.Unsetenv("FORGE_CRED_NOPE")
	f := NewFactory("credresolver.env@1.0.0")
	r, _ := f.Build(context.Background(), nil)
	_, err := r.Fetch(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error")
	}
}
```

Note: `credentials.Credential.Bytes()` and `Close()` signatures come from praxis. If the praxis type uses different accessors, adapt — the principle is that the returned credential carries opaque bytes that are zeroed on `Close`.

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/credresolverenv/... -v`

- [ ] **Step 3: Implement**

```go
// factories/credresolverenv/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package credresolverenv provides a credential resolver factory that reads
// scope-named secrets from environment variables. Scope "net:http" maps to
// FORGE_CRED_NET_HTTP. Colons and dashes become underscores; letters become
// uppercase. Intended for dev and tests; production deployments should use a
// real secret store resolver.
package credresolverenv

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/credentials"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "env-var credential resolver" }

func (f *Factory) Build(_ context.Context, _ map[string]any) (credentials.Resolver, error) {
	return &resolver{}, nil
}

type resolver struct{}

func (r *resolver) Fetch(_ context.Context, scope string) (credentials.Credential, error) {
	envName := scopeToEnv(scope)
	v := os.Getenv(envName)
	if v == "" {
		return nil, fmt.Errorf("credresolver.env: %s not set", envName)
	}
	return newCred([]byte(v)), nil
}

func scopeToEnv(scope string) string {
	s := strings.ToUpper(scope)
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return "FORGE_CRED_" + s
}
```

`newCred` and its type live in a tiny sibling file — whatever shape praxis's `credentials.Credential` expects. The following sketch creates a simple type that implements the praxis contract (adapt `Bytes` / `Close` names if needed):

```go
// factories/credresolverenv/credential.go
// SPDX-License-Identifier: Apache-2.0

package credresolverenv

type cred struct {
	b []byte
}

func newCred(b []byte) *cred { return &cred{b: b} }
func (c *cred) Bytes() []byte { return c.b }
func (c *cred) Close() error {
	for i := range c.b {
		c.b[i] = 0
	}
	c.b = nil
	return nil
}
```

If `credentials.Credential` is an interface with different method names, rename accordingly. Run `go doc github.com/praxis-os/praxis/credentials` to confirm.

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/credresolverenv/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/credresolverenv
git commit -m "feat(factories): credresolver.env@1 — env-var credential resolver"
```

---

### Task 4.5: `factories/identitysignered25519`

**Files:**
- Create: `factories/identitysignered25519/factory.go`
- Create: `factories/identitysignered25519/factory_test.go`

- [ ] **Step 1: Write failing test**

```go
// factories/identitysignered25519/factory_test.go
package identitysignered25519

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestFactory_Signs(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := NewFactory("identitysigner.ed25519@1.0.0", priv)
	s, err := f.Build(context.Background(), map[string]any{
		"issuer":               "test",
		"tokenLifetimeSeconds": 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	tok, err := s.Sign(context.Background(), map[string]any{"sub": "agent-x"})
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}
}

func TestFactory_RejectsInvalidLifetime(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := NewFactory("identitysigner.ed25519@1.0.0", priv)
	_, err := f.Build(context.Background(), map[string]any{"issuer": "t", "tokenLifetimeSeconds": 2})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFactory_RejectsMissingIssuer(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := NewFactory("identitysigner.ed25519@1.0.0", priv)
	_, err := f.Build(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/identitysignered25519/... -v`

- [ ] **Step 3: Implement**

```go
// factories/identitysignered25519/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package identitysignered25519 wraps praxis identity.NewEd25519Signer. The
// private key is supplied at factory construction time; spec config carries
// the issuer and token lifetime.
package identitysignered25519

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/identity"
)

type Factory struct {
	id   registry.ID
	priv ed25519.PrivateKey
}

func NewFactory(id registry.ID, priv ed25519.PrivateKey) *Factory {
	return &Factory{id: id, priv: priv}
}

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "Ed25519 JWT identity signer" }

type config struct {
	Issuer   string
	Lifetime time.Duration
}

func decode(cfg map[string]any) (config, error) {
	var c config
	iss, ok := cfg["issuer"].(string)
	if !ok || iss == "" {
		return c, fmt.Errorf("issuer: required string")
	}
	c.Issuer = iss

	// YAML decodes integers as int, int64, or float64 depending on size.
	lifetime, err := toInt(cfg["tokenLifetimeSeconds"])
	if err != nil {
		return c, fmt.Errorf("tokenLifetimeSeconds: %w", err)
	}
	if lifetime < 5 || lifetime > 300 {
		return c, fmt.Errorf("tokenLifetimeSeconds: want 5..300, got %d", lifetime)
	}
	c.Lifetime = time.Duration(lifetime) * time.Second
	return c, nil
}

func toInt(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		return int64(x), nil
	default:
		return 0, fmt.Errorf("want integer, got %T", v)
	}
}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (identity.Signer, error) {
	c, err := decode(cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.id, err)
	}
	return identity.NewEd25519Signer(
		f.priv,
		identity.WithIssuer(c.Issuer),
		identity.WithTokenLifetime(c.Lifetime),
	), nil
}
```

If `identity.WithTokenLifetime` or `identity.WithIssuer` have different names, substitute — the praxis package has `SignerOption` functional options per [mismatches.md](../../design/mismatches.md).

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/identitysignered25519/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/identitysignered25519
git commit -m "feat(factories): identitysigner.ed25519@1 — wraps praxis Ed25519 signer"
```

---

### Task 4.6: `factories/filtersecretscrubber`

**Files:**
- Create: `factories/filtersecretscrubber/factory.go`
- Create: `factories/filtersecretscrubber/factory_test.go`

- [ ] **Step 1: Write failing test**

```go
// factories/filtersecretscrubber/factory_test.go
package filtersecretscrubber

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
)

func TestFilter_RedactsAnthropicKey(t *testing.T) {
	f, _ := NewFactory("filter.secret-scrubber@1.0.0").Build(context.Background(), nil)
	msgs := []llm.Message{{Role: "user", Content: "please call sk-abc123xyz456789012345678"}}
	out, decs, err := f.Filter(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out[0].Content, "sk-abc123") {
		t.Fatalf("leaked: %s", out[0].Content)
	}
	if len(decs) == 0 || decs[0].Action != hooks.FilterActionRedact {
		t.Fatalf("decs=%v", decs)
	}
}

func TestFilter_PassesCleanMessages(t *testing.T) {
	f, _ := NewFactory("filter.secret-scrubber@1.0.0").Build(context.Background(), nil)
	msgs := []llm.Message{{Role: "user", Content: "hello world"}}
	_, decs, err := f.Filter(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range decs {
		if d.Action == hooks.FilterActionRedact {
			t.Fatalf("unexpected redact: %v", d)
		}
	}
}
```

Note: `llm.Message.Content` may be a rich type in praxis (multimodal blocks). Inspect the real type — if content is not a plain string, apply the regex to each text block. Adjust the test and implementation together.

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/filtersecretscrubber/... -v`

- [ ] **Step 3: Implement**

```go
// factories/filtersecretscrubber/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package filtersecretscrubber is a pre_llm_filter that redacts common secret
// patterns (sk-*, ghp_*, AKIA AWS keys) from outbound LLM messages.
package filtersecretscrubber

import (
	"context"
	"regexp"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
)

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9]{16,}`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
}

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "redacts sk-*, ghp_*, AKIA patterns" }

func (f *Factory) Build(_ context.Context, _ map[string]any) (hooks.PreLLMFilter, error) {
	return &filter{}, nil
}

type filter struct{}

func (f *filter) Filter(_ context.Context, msgs []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	out := make([]llm.Message, len(msgs))
	var decs []hooks.FilterDecision
	for i, m := range msgs {
		scrubbed, hits := scrub(m.Content)
		out[i] = m
		out[i].Content = scrubbed
		if hits > 0 {
			decs = append(decs, hooks.FilterDecision{
				Action: hooks.FilterActionRedact,
				Reason: "secret pattern redacted",
			})
		}
	}
	return out, decs, nil
}

func scrub(s string) (string, int) {
	out := s
	hits := 0
	for _, p := range patterns {
		out, hits = replaceAll(out, p, hits)
	}
	return out, hits
}

func replaceAll(s string, p *regexp.Regexp, hits int) (string, int) {
	loc := p.FindAllStringIndex(s, -1)
	if len(loc) == 0 {
		return s, hits
	}
	return p.ReplaceAllString(s, "[REDACTED]"), hits + len(loc)
}
```

If `llm.Message.Content` is not a plain string, adapt: iterate content blocks, apply `scrub` to each text block, preserve non-text blocks.

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/filtersecretscrubber/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/filtersecretscrubber
git commit -m "feat(factories): filter.secret-scrubber@1 — pre-LLM secret redaction"
```

---

### Task 4.7: `factories/filterpathescape`

**Files:**
- Create: `factories/filterpathescape/factory.go`
- Create: `factories/filterpathescape/factory_test.go`

- [ ] **Step 1: Write failing test**

```go
// factories/filterpathescape/factory_test.go
package filterpathescape

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/tools"
)

func TestFilter_BlocksPathEscape(t *testing.T) {
	f, _ := NewFactory("filter.path-escape@1.0.0").Build(context.Background(), nil)
	call := tools.ToolCall{Name: "read_file", ArgumentsJSON: []byte(`{"path":"../etc/passwd"}`)}
	_, decs, err := f.Filter(context.Background(), call)
	_ = err
	if len(decs) == 0 || decs[0].Action != hooks.FilterActionBlock {
		t.Fatalf("decs=%v", decs)
	}
}

func TestFilter_AllowsCleanPath(t *testing.T) {
	f, _ := NewFactory("filter.path-escape@1.0.0").Build(context.Background(), nil)
	call := tools.ToolCall{Name: "read_file", ArgumentsJSON: []byte(`{"path":"docs/readme.md"}`)}
	_, decs, err := f.Filter(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range decs {
		if d.Action == hooks.FilterActionBlock {
			t.Fatalf("unexpected block: %v", d)
		}
	}
}
```

Note: `tools.ToolCall` field for arguments is named `ArgumentsJSON` (or `Arguments`, per praxis source). Verify and adjust.

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/filterpathescape/... -v`

- [ ] **Step 3: Implement**

```go
// factories/filterpathescape/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package filterpathescape is a pre_tool_filter that blocks any tool call
// whose serialized JSON arguments contain "../" (parent-dir traversal).
package filterpathescape

import (
	"bytes"
	"context"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/tools"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "blocks ../ path traversal in tool args" }

func (f *Factory) Build(_ context.Context, _ map[string]any) (hooks.PreToolFilter, error) {
	return &filter{}, nil
}

type filter struct{}

var needle = []byte("../")

func (f *filter) Filter(_ context.Context, call tools.ToolCall) (tools.ToolCall, []hooks.FilterDecision, error) {
	if bytes.Contains(call.ArgumentsJSON, needle) {
		return call, []hooks.FilterDecision{{
			Action: hooks.FilterActionBlock,
			Reason: "path traversal '../' in tool arguments",
		}}, nil
	}
	return call, nil, nil
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/filterpathescape/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/filterpathescape
git commit -m "feat(factories): filter.path-escape@1 — pre-tool path-traversal block"
```

---

### Task 4.8: `factories/filteroutputtruncate`

**Files:**
- Create: `factories/filteroutputtruncate/factory.go`
- Create: `factories/filteroutputtruncate/factory_test.go`

- [ ] **Step 1: Write failing test**

```go
// factories/filteroutputtruncate/factory_test.go
package filteroutputtruncate

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/tools"
)

func TestFilter_Truncates(t *testing.T) {
	f, _ := NewFactory("filter.output-truncate@1.0.0").Build(context.Background(), map[string]any{"maxBytes": 16})
	r := tools.ToolResult{Name: "x", Output: strings.Repeat("a", 100)}
	out, decs, err := f.Filter(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Output) > 16 {
		t.Fatalf("len=%d", len(out.Output))
	}
	if len(decs) == 0 || decs[0].Action != hooks.FilterActionLog {
		t.Fatalf("decs=%v", decs)
	}
}

func TestFilter_PassThrough(t *testing.T) {
	f, _ := NewFactory("filter.output-truncate@1.0.0").Build(context.Background(), map[string]any{"maxBytes": 100})
	r := tools.ToolResult{Name: "x", Output: "short"}
	out, decs, err := f.Filter(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if out.Output != "short" {
		t.Fatalf("out=%q", out.Output)
	}
	if len(decs) > 0 && decs[0].Action == hooks.FilterActionLog {
		t.Fatal("unexpected log dec")
	}
}

func TestFilter_RequiresMaxBytes(t *testing.T) {
	_, err := NewFactory("filter.output-truncate@1.0.0").Build(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
```

Note: `tools.ToolResult` content field is named `Output` (string) or similar — verify against praxis source. Adjust if multimodal.

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/filteroutputtruncate/... -v`

- [ ] **Step 3: Implement**

```go
// factories/filteroutputtruncate/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package filteroutputtruncate is a post_tool_filter that truncates tool
// output to a configured maxBytes and emits a Log filter decision whenever
// truncation occurs.
package filteroutputtruncate

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/tools"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "truncates tool output to maxBytes" }

func (f *Factory) Build(_ context.Context, cfg map[string]any) (hooks.PostToolFilter, error) {
	n, err := requireInt(cfg, "maxBytes")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.id, err)
	}
	if n <= 0 {
		return nil, fmt.Errorf("%s: maxBytes must be > 0", f.id)
	}
	return &filter{maxBytes: n}, nil
}

func requireInt(cfg map[string]any, key string) (int, error) {
	raw, ok := cfg[key]
	if !ok {
		return 0, fmt.Errorf("%s: required", key)
	}
	switch x := raw.(type) {
	case int:
		return x, nil
	case int64:
		return int(x), nil
	case float64:
		return int(x), nil
	default:
		return 0, fmt.Errorf("%s: want int, got %T", key, raw)
	}
}

type filter struct{ maxBytes int }

func (f *filter) Filter(_ context.Context, r tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	if len(r.Output) <= f.maxBytes {
		return r, nil, nil
	}
	orig := len(r.Output)
	r.Output = r.Output[:f.maxBytes]
	return r, []hooks.FilterDecision{{
		Action: hooks.FilterActionLog,
		Reason: fmt.Sprintf("truncated tool output from %d to %d bytes", orig, f.maxBytes),
	}}, nil
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/filteroutputtruncate/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/filteroutputtruncate
git commit -m "feat(factories): filter.output-truncate@1 — post-tool byte-cap"
```

---

### Task 4.9: `factories/policypackpiiredact`

**Files:**
- Create: `factories/policypackpiiredact/factory.go`
- Create: `factories/policypackpiiredact/factory_test.go`

- [ ] **Step 1: Write failing test**

```go
// factories/policypackpiiredact/factory_test.go
package policypackpiiredact

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
)

func TestPolicy_LogsOnEmailMedium(t *testing.T) {
	pp, err := NewFactory("policypack.pii-redaction@1.0.0").Build(context.Background(), map[string]any{"strictness": "medium"})
	if err != nil {
		t.Fatal(err)
	}
	in := hooks.PolicyInput{Text: "contact me at foo@bar.com"}
	d, err := pp.Hook.Evaluate(context.Background(), hooks.PhasePreLLMInput, in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != hooks.VerdictLog {
		t.Fatalf("verdict=%v", d.Verdict)
	}
	if d.Metadata == nil || d.Metadata["pii.matches"] == nil {
		t.Fatalf("missing metadata: %+v", d.Metadata)
	}
}

func TestPolicy_DeniesSSNHigh(t *testing.T) {
	pp, _ := NewFactory("policypack.pii-redaction@1.0.0").Build(context.Background(), map[string]any{"strictness": "high"})
	in := hooks.PolicyInput{Text: "SSN 123-45-6789"}
	d, _ := pp.Hook.Evaluate(context.Background(), hooks.PhasePreLLMInput, in)
	if d.Verdict != hooks.VerdictDeny {
		t.Fatalf("verdict=%v", d.Verdict)
	}
}

func TestPolicy_AllowsClean(t *testing.T) {
	pp, _ := NewFactory("policypack.pii-redaction@1.0.0").Build(context.Background(), map[string]any{"strictness": "low"})
	in := hooks.PolicyInput{Text: "just a normal sentence"}
	d, _ := pp.Hook.Evaluate(context.Background(), hooks.PhasePreLLMInput, in)
	if d.Verdict != hooks.VerdictAllow {
		t.Fatalf("verdict=%v", d.Verdict)
	}
}

func TestPolicy_RejectsBadStrictness(t *testing.T) {
	_, err := NewFactory("policypack.pii-redaction@1.0.0").Build(context.Background(), map[string]any{"strictness": "nuclear"})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

Note: `hooks.PolicyInput` may not have a direct `Text` field; verify what praxis exposes (likely `Messages []llm.Message` for PhasePreLLMInput). Adapt the test to flatten messages into a string the policy can scan.

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/policypackpiiredact/... -v`

- [ ] **Step 3: Implement**

```go
// factories/policypackpiiredact/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package policypackpiiredact is a policy_pack that scans message text for
// PII. Strictness "low" logs email/phone; "medium" logs email/phone/SSN/CC;
// "high" denies on SSN/CC.
package policypackpiiredact

import (
	"context"
	"fmt"
	"regexp"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/hooks"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "regex PII detector with tiered strictness" }

type strictness string

const (
	strictLow    strictness = "low"
	strictMedium strictness = "medium"
	strictHigh   strictness = "high"
)

var (
	reEmail = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	rePhone = regexp.MustCompile(`\b\d{3}[-.\s]\d{3}[-.\s]\d{4}\b`)
	reSSN   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	reCC    = regexp.MustCompile(`\b(?:\d[ -]?){13,16}\b`)
)

func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.PolicyPack, error) {
	s, ok := cfg["strictness"].(string)
	if !ok {
		return registry.PolicyPack{}, fmt.Errorf("%s: strictness: required string", f.id)
	}
	var mode strictness
	switch s {
	case "low", "medium", "high":
		mode = strictness(s)
	default:
		return registry.PolicyPack{}, fmt.Errorf("%s: strictness: want low|medium|high, got %q", f.id, s)
	}
	return registry.PolicyPack{
		Hook: &hook{mode: mode},
		Descriptors: []registry.PolicyDescriptor{{
			Name: "pii-redaction", Source: string(f.id), PolicyTags: []string{"pii", "privacy"},
		}},
	}, nil
}

type hook struct{ mode strictness }

func (h *hook) Evaluate(_ context.Context, _ hooks.Phase, in hooks.PolicyInput) (hooks.Decision, error) {
	text := extractText(in)

	var matches []string
	if reEmail.MatchString(text) {
		matches = append(matches, "email")
	}
	if rePhone.MatchString(text) {
		matches = append(matches, "phone")
	}
	if reSSN.MatchString(text) {
		matches = append(matches, "ssn")
	}
	if reCC.MatchString(text) {
		matches = append(matches, "credit_card")
	}

	if len(matches) == 0 {
		return hooks.Decision{Verdict: hooks.VerdictAllow}, nil
	}

	if h.mode == strictHigh {
		for _, m := range matches {
			if m == "ssn" || m == "credit_card" {
				return hooks.Decision{
					Verdict:  hooks.VerdictDeny,
					Reason:   "high-risk PII detected: " + m,
					Metadata: map[string]any{"pii.matches": matches},
				}, nil
			}
		}
	}

	// medium + high (non-denying) + low cases that hit → Log.
	return hooks.Decision{
		Verdict:  hooks.VerdictLog,
		Reason:   "PII detected",
		Metadata: map[string]any{"pii.matches": matches},
	}, nil
}
```

The `extractText` helper pulls a flat string from `hooks.PolicyInput`. Inspect the praxis type and implement accordingly — the simplest path is to concatenate every message's text block with a newline. Keep this helper in the same file.

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/policypackpiiredact/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/policypackpiiredact
git commit -m "feat(factories): policypack.pii-redaction@1 — regex PII detector"
```

---

### Task 4.10: `factories/toolpackhttpget`

**Files:**
- Create: `factories/toolpackhttpget/factory.go`
- Create: `factories/toolpackhttpget/factory_test.go`

- [ ] **Step 1: Write failing test**

```go
// factories/toolpackhttpget/factory_test.go
package toolpackhttpget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/tools"
)

func TestTool_Fetches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()
	host := srv.Listener.Addr().String()

	pack, err := NewFactory("toolpack.http-get@1.0.0").Build(context.Background(), map[string]any{
		"allowedHosts": []any{host},
		"timeoutMs":    2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"url": srv.URL})
	res, err := pack.Invoker.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{Name: "http_get", ArgumentsJSON: args})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "hello" {
		t.Fatalf("got=%q", res.Output)
	}
}

func TestTool_BlocksDisallowedHost(t *testing.T) {
	pack, err := NewFactory("toolpack.http-get@1.0.0").Build(context.Background(), map[string]any{
		"allowedHosts": []any{"example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"url": "http://evil.example/"})
	res, _ := pack.Invoker.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{Name: "http_get", ArgumentsJSON: args})
	if res.Status != tools.ToolStatusError {
		t.Fatalf("status=%v", res.Status)
	}
}

func TestFactory_RequiresAllowedHosts(t *testing.T) {
	_, err := NewFactory("toolpack.http-get@1.0.0").Build(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/toolpackhttpget/... -v`

- [ ] **Step 3: Implement**

```go
// factories/toolpackhttpget/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package toolpackhttpget exposes a single http_get(url) tool that fetches a
// URL over HTTP(S). Host allowlist is enforced at config time; all other size
// governance is the post-tool filter's responsibility.
package toolpackhttpget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "single http_get tool with host allowlist" }

type cfgShape struct {
	AllowedHosts []string
	Timeout      time.Duration
}

func decode(cfg map[string]any) (cfgShape, error) {
	var c cfgShape
	raw, ok := cfg["allowedHosts"]
	if !ok {
		return c, fmt.Errorf("allowedHosts: required")
	}
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return c, fmt.Errorf("allowedHosts: must be non-empty list")
	}
	for i, h := range list {
		s, ok := h.(string)
		if !ok {
			return c, fmt.Errorf("allowedHosts[%d]: want string, got %T", i, h)
		}
		c.AllowedHosts = append(c.AllowedHosts, s)
	}
	// Timeout (optional, default 5s).
	c.Timeout = 5 * time.Second
	if raw, ok := cfg["timeoutMs"]; ok {
		n, err := toInt(raw)
		if err != nil {
			return c, fmt.Errorf("timeoutMs: %w", err)
		}
		c.Timeout = time.Duration(n) * time.Millisecond
	}
	return c, nil
}

func toInt(v any) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int64:
		return int(x), nil
	case float64:
		return int(x), nil
	default:
		return 0, fmt.Errorf("want int, got %T", v)
	}
}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.ToolPack, error) {
	c, err := decode(cfg)
	if err != nil {
		return registry.ToolPack{}, fmt.Errorf("%s: %w", f.id, err)
	}
	inv := &invoker{
		client:       &http.Client{Timeout: c.Timeout},
		allowedHosts: c.AllowedHosts,
	}
	def := llm.ToolDefinition{
		Name:        "http_get",
		Description: "Fetch the body of a URL via HTTP GET. Only allow-listed hosts are reachable.",
		InputSchema: []byte(`{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}`),
	}
	desc := registry.ToolDescriptor{
		Name:       "http_get",
		Source:     string(f.id),
		RiskTier:   registry.RiskModerate,
		PolicyTags: []string{"network", "http"},
	}
	return registry.ToolPack{
		Invoker:     inv,
		Definitions: []llm.ToolDefinition{def},
		Descriptors: []registry.ToolDescriptor{desc},
	}, nil
}

type invoker struct {
	client       *http.Client
	allowedHosts []string
}

type args struct {
	URL string `json:"url"`
}

func (i *invoker) Invoke(ctx context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	var a args
	if err := json.Unmarshal(call.ArgumentsJSON, &a); err != nil {
		return tools.ToolResult{Name: call.Name, Status: tools.ToolStatusError, Output: fmt.Sprintf("invalid args: %s", err)}, nil
	}
	u, err := url.Parse(a.URL)
	if err != nil || u.Host == "" {
		return tools.ToolResult{Name: call.Name, Status: tools.ToolStatusError, Output: "invalid url"}, nil
	}
	if !i.hostAllowed(u.Host) {
		return tools.ToolResult{Name: call.Name, Status: tools.ToolStatusError, Output: "host not in allowlist: " + u.Host}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return tools.ToolResult{Name: call.Name, Status: tools.ToolStatusError, Output: err.Error()}, nil
	}
	resp, err := i.client.Do(req)
	if err != nil {
		return tools.ToolResult{Name: call.Name, Status: tools.ToolStatusError, Output: err.Error()}, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tools.ToolResult{Name: call.Name, Status: tools.ToolStatusError, Output: err.Error()}, nil
	}
	return tools.ToolResult{Name: call.Name, Status: tools.ToolStatusSuccess, Output: string(body)}, nil
}

func (i *invoker) hostAllowed(host string) bool {
	for _, h := range i.allowedHosts {
		if host == h {
			return true
		}
	}
	return false
}
```

Note: `llm.ToolDefinition.InputSchema` may accept `json.RawMessage` vs `[]byte` depending on praxis — adapt. Same for `tools.ToolResult.Output` (may be named `Content`). Verify against the real types before implementing.

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/toolpackhttpget/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/toolpackhttpget
git commit -m "feat(factories): toolpack.http-get@1 — allowlisted HTTP GET tool"
```

---

### Task 4.11: `factories/provideranthropic`

**Files:**
- Create: `factories/provideranthropic/factory.go`
- Create: `factories/provideranthropic/factory_test.go`

- [ ] **Step 1: Write failing test**

The factory wraps praxis's `anthropic` package. Unit test only checks config decoding + that the factory produces a non-nil provider when given a fake API key (no network call).

```go
// factories/provideranthropic/factory_test.go
package provideranthropic

import (
	"context"
	"testing"
)

func TestFactory_Builds(t *testing.T) {
	f := NewFactory("provider.anthropic@1.0.0", "test-api-key")
	p, err := f.Build(context.Background(), map[string]any{
		"model":           "claude-sonnet-4-5",
		"maxOutputTokens": 2048,
		"temperature":     0.2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("provider nil")
	}
}

func TestFactory_RejectsMissingModel(t *testing.T) {
	f := NewFactory("provider.anthropic@1.0.0", "test")
	_, err := f.Build(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFactory_RejectsMissingAPIKey(t *testing.T) {
	f := NewFactory("provider.anthropic@1.0.0", "")
	_, err := f.Build(context.Background(), map[string]any{"model": "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./factories/provideranthropic/... -v`

- [ ] **Step 3: Implement**

```go
// factories/provideranthropic/factory.go
// SPDX-License-Identifier: Apache-2.0

// Package provideranthropic wraps praxis's anthropic provider. The API key is
// injected at factory construction time (never via spec); spec config carries
// model, maxOutputTokens, and temperature.
package provideranthropic

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/providers/anthropic"
)

type Factory struct {
	id     registry.ID
	apiKey string
}

func NewFactory(id registry.ID, apiKey string) *Factory {
	return &Factory{id: id, apiKey: apiKey}
}

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "Anthropic Claude provider" }

type config struct {
	Model           string
	MaxOutputTokens int
	Temperature     float64
}

func decode(cfg map[string]any) (config, error) {
	var c config
	model, ok := cfg["model"].(string)
	if !ok || model == "" {
		return c, fmt.Errorf("model: required string")
	}
	c.Model = model
	if raw, ok := cfg["maxOutputTokens"]; ok {
		switch x := raw.(type) {
		case int:
			c.MaxOutputTokens = x
		case int64:
			c.MaxOutputTokens = int(x)
		case float64:
			c.MaxOutputTokens = int(x)
		default:
			return c, fmt.Errorf("maxOutputTokens: want int, got %T", raw)
		}
	}
	if raw, ok := cfg["temperature"]; ok {
		switch x := raw.(type) {
		case float64:
			c.Temperature = x
		case int:
			c.Temperature = float64(x)
		default:
			return c, fmt.Errorf("temperature: want number, got %T", raw)
		}
	}
	return c, nil
}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (llm.Provider, error) {
	if f.apiKey == "" {
		return nil, fmt.Errorf("%s: api key not set at factory construction", f.id)
	}
	c, err := decode(cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.id, err)
	}
	return anthropic.New(
		f.apiKey,
		anthropic.WithDefaultModel(c.Model),
		anthropic.WithMaxOutputTokens(c.MaxOutputTokens),
		anthropic.WithTemperature(c.Temperature),
	), nil
}
```

The exact `anthropic.New` signature and its functional options must come from praxis. Inspect [`praxis/providers/anthropic`](../../../../praxis/providers/anthropic/) before implementing — the options above are plausible names but must match. If the provider constructor takes a plain `*Config` struct, adapt.

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./factories/provideranthropic/... -v`

- [ ] **Step 5: Commit**

```bash
git add factories/provideranthropic
git commit -m "feat(factories): provider.anthropic@1 — Anthropic provider wrapper"
```

---

