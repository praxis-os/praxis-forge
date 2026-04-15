> Part of [praxis-forge Phase 1 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-15-praxis-forge-phase-1-design.md`](../../specs/2026-04-15-praxis-forge-phase-1-design.md).

## Task group 3 — `manifest/` + `build/`

### Task 3.1: Manifest type with stable JSON

**Files:**
- Create: `manifest/manifest.go`
- Create: `manifest/manifest_test.go`

- [ ] **Step 1: Write failing test**

```go
// manifest/manifest_test.go
package manifest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestManifest_MarshalStableOrder(t *testing.T) {
	m := Manifest{
		SpecID:      "acme.demo",
		SpecVersion: "0.1.0",
		BuiltAt:     time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		Resolved: []ResolvedComponent{
			{Kind: "provider", ID: "provider.anthropic@1.0.0", Config: map[string]any{"model": "x"}},
			{Kind: "tool_pack", ID: "toolpack.http-get@1.0.0", Config: map[string]any{"timeoutMs": 5000}},
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	// Resolved components appear in declaration order; Config keys sorted.
	if !strings.Contains(s, `"specId":"acme.demo"`) {
		t.Fatalf("specId missing: %s", s)
	}
	if strings.Index(s, `"provider.anthropic@1.0.0"`) > strings.Index(s, `"toolpack.http-get@1.0.0"`) {
		t.Fatal("resolved order not preserved")
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./manifest/... -v`

- [ ] **Step 3: Implement**

```go
// manifest/manifest.go
// SPDX-License-Identifier: Apache-2.0

// Package manifest holds the inspectable build record for a BuiltAgent.
package manifest

import "time"

type Manifest struct {
	SpecID      string              `json:"specId"`
	SpecVersion string              `json:"specVersion"`
	BuiltAt     time.Time           `json:"builtAt"`
	Resolved    []ResolvedComponent `json:"resolved"`
}

type ResolvedComponent struct {
	Kind        string         `json:"kind"`
	ID          string         `json:"id"`
	Config      map[string]any `json:"config,omitempty"`
	Descriptors any            `json:"descriptors,omitempty"`
}
```

(`encoding/json` sorts map keys automatically, so Config ordering is stable without extra work.)

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./manifest/... -v`

- [ ] **Step 5: Commit**

```bash
git add manifest/manifest.go manifest/manifest_test.go
git commit -m "feat(manifest): Manifest + ResolvedComponent with stable JSON"
```

---

### Task 3.2: Policy chain adapter

**Files:**
- Create: `build/policy_chain.go`
- Create: `build/policy_chain_test.go`

- [ ] **Step 1: Write failing test**

```go
// build/policy_chain_test.go
package build

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
)

type fixedHook struct {
	decision hooks.Decision
	calls    *int
}

func (f fixedHook) Evaluate(ctx context.Context, phase hooks.Phase, in hooks.PolicyInput) (hooks.Decision, error) {
	*f.calls++
	return f.decision, nil
}

func TestPolicyChain_AllAllowContinues(t *testing.T) {
	var a, b int
	chain := policyChain{
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &a},
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &b},
	}
	d, err := chain.Evaluate(context.Background(), hooks.PhasePreInvocation, hooks.PolicyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != hooks.VerdictAllow {
		t.Fatalf("verdict=%v", d.Verdict)
	}
	if a != 1 || b != 1 {
		t.Fatalf("calls a=%d b=%d", a, b)
	}
}

func TestPolicyChain_DenyShortCircuits(t *testing.T) {
	var a, b int
	chain := policyChain{
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictDeny, Reason: "nope"}, calls: &a},
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &b},
	}
	d, err := chain.Evaluate(context.Background(), hooks.PhasePreInvocation, hooks.PolicyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != hooks.VerdictDeny {
		t.Fatalf("verdict=%v", d.Verdict)
	}
	if b != 0 {
		t.Fatal("second hook should not run after deny")
	}
}

func TestPolicyChain_RequireApprovalShortCircuits(t *testing.T) {
	var a, b int
	chain := policyChain{
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictRequireApproval}, calls: &a},
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &b},
	}
	d, _ := chain.Evaluate(context.Background(), hooks.PhasePreInvocation, hooks.PolicyInput{})
	if d.Verdict != hooks.VerdictRequireApproval || b != 0 {
		t.Fatalf("verdict=%v b=%d", d.Verdict, b)
	}
}

func TestPolicyChain_LogContinues(t *testing.T) {
	var a, b int
	chain := policyChain{
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictLog}, calls: &a},
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &b},
	}
	d, _ := chain.Evaluate(context.Background(), hooks.PhasePreInvocation, hooks.PolicyInput{})
	if d.Verdict != hooks.VerdictAllow || a != 1 || b != 1 {
		t.Fatalf("verdict=%v a=%d b=%d", d.Verdict, a, b)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./build/... -v`

- [ ] **Step 3: Implement**

```go
// build/policy_chain.go
// SPDX-License-Identifier: Apache-2.0

// Package build resolves a validated AgentSpec against a ComponentRegistry,
// composes multi-component chains into praxis single-instance hooks, and
// materializes a *BuiltAgent backed by a *orchestrator.Orchestrator.
package build

import (
	"context"

	"github.com/praxis-os/praxis/hooks"
)

// policyChain fans a single praxis PolicyHook call across multiple hooks with
// short-circuit semantics. VerdictDeny / VerdictRequireApproval return at
// once; VerdictLog records and continues; VerdictAllow / VerdictContinue
// continue without recording.
type policyChain []hooks.PolicyHook

func (c policyChain) Evaluate(ctx context.Context, phase hooks.Phase, in hooks.PolicyInput) (hooks.Decision, error) {
	for _, h := range c {
		d, err := h.Evaluate(ctx, phase, in)
		if err != nil {
			return hooks.Decision{}, err
		}
		switch d.Verdict {
		case hooks.VerdictDeny, hooks.VerdictRequireApproval:
			return d, nil
		case hooks.VerdictLog, hooks.VerdictAllow, hooks.VerdictContinue:
			// keep going
		default:
			// Unknown verdict: pass through so later praxis changes surface via tests.
			return d, nil
		}
	}
	return hooks.Decision{Verdict: hooks.VerdictAllow}, nil
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./build/... -v`

- [ ] **Step 5: Commit**

```bash
git add build/policy_chain.go build/policy_chain_test.go
git commit -m "feat(build): policyChain adapter with short-circuit semantics"
```

---

### Task 3.3: Filter chain adapters

**Files:**
- Create: `build/filter_chains.go`
- Create: `build/filter_chains_test.go`

- [ ] **Step 1: Write failing test**

```go
// build/filter_chains_test.go
package build

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

type fakePreLLM struct {
	action hooks.FilterAction
	reason string
	mutate func([]llm.Message) []llm.Message
	calls  *int
}

func (f fakePreLLM) Filter(ctx context.Context, msgs []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	*f.calls++
	out := msgs
	if f.mutate != nil {
		out = f.mutate(msgs)
	}
	return out, []hooks.FilterDecision{{Action: f.action, Reason: f.reason}}, nil
}

func TestPreLLMFilterChain_PassPassBothRun(t *testing.T) {
	var a, b int
	chain := preLLMFilterChain{
		fakePreLLM{action: hooks.FilterActionPass, calls: &a},
		fakePreLLM{action: hooks.FilterActionPass, calls: &b},
	}
	_, decs, err := chain.Filter(context.Background(), []llm.Message{})
	if err != nil {
		t.Fatal(err)
	}
	if a != 1 || b != 1 || len(decs) != 2 {
		t.Fatalf("a=%d b=%d decs=%d", a, b, len(decs))
	}
}

func TestPreLLMFilterChain_BlockShortCircuits(t *testing.T) {
	var a, b int
	chain := preLLMFilterChain{
		fakePreLLM{action: hooks.FilterActionBlock, reason: "no", calls: &a},
		fakePreLLM{action: hooks.FilterActionPass, calls: &b},
	}
	_, decs, err := chain.Filter(context.Background(), []llm.Message{})
	if err == nil {
		t.Fatal("expected block error")
	}
	if b != 0 {
		t.Fatal("second filter should not run after block")
	}
	if len(decs) == 0 || decs[0].Action != hooks.FilterActionBlock {
		t.Fatalf("decs=%v", decs)
	}
}

type fakePreTool struct {
	action hooks.FilterAction
	calls  *int
}

func (f fakePreTool) Filter(ctx context.Context, call tools.ToolCall) (tools.ToolCall, []hooks.FilterDecision, error) {
	*f.calls++
	return call, []hooks.FilterDecision{{Action: f.action}}, nil
}

func TestPreToolFilterChain_Block(t *testing.T) {
	var a, b int
	chain := preToolFilterChain{
		fakePreTool{action: hooks.FilterActionBlock, calls: &a},
		fakePreTool{action: hooks.FilterActionPass, calls: &b},
	}
	_, _, err := chain.Filter(context.Background(), tools.ToolCall{})
	if err == nil || b != 0 {
		t.Fatalf("err=%v b=%d", err, b)
	}
}

type fakePostTool struct {
	action hooks.FilterAction
	calls  *int
}

func (f fakePostTool) Filter(ctx context.Context, r tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	*f.calls++
	return r, []hooks.FilterDecision{{Action: f.action}}, nil
}

func TestPostToolFilterChain_Block(t *testing.T) {
	var a, b int
	chain := postToolFilterChain{
		fakePostTool{action: hooks.FilterActionBlock, calls: &a},
		fakePostTool{action: hooks.FilterActionPass, calls: &b},
	}
	_, _, err := chain.Filter(context.Background(), tools.ToolResult{})
	if err == nil || b != 0 {
		t.Fatalf("err=%v b=%d", err, b)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./build/... -v`

- [ ] **Step 3: Implement**

```go
// build/filter_chains.go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

// ErrFilterBlocked is returned by any filter chain when a stage is aborted
// by FilterActionBlock. Orchestrator callers can match this via errors.Is.
var ErrFilterBlocked = fmt.Errorf("filter: stage blocked")

// blocked produces an ErrFilterBlocked wrapper carrying the filter's reason.
func blocked(reason string) error {
	if reason == "" {
		return ErrFilterBlocked
	}
	return fmt.Errorf("%w: %s", ErrFilterBlocked, reason)
}

type preLLMFilterChain []hooks.PreLLMFilter

func (c preLLMFilterChain) Filter(ctx context.Context, msgs []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	var all []hooks.FilterDecision
	cur := msgs
	for _, f := range c {
		out, decs, err := f.Filter(ctx, cur)
		if err != nil {
			return nil, nil, err
		}
		all = append(all, decs...)
		if blockingDec(decs) != nil {
			return nil, all, blocked(blockingDec(decs).Reason)
		}
		cur = out
	}
	return cur, all, nil
}

type preToolFilterChain []hooks.PreToolFilter

func (c preToolFilterChain) Filter(ctx context.Context, call tools.ToolCall) (tools.ToolCall, []hooks.FilterDecision, error) {
	var all []hooks.FilterDecision
	cur := call
	for _, f := range c {
		out, decs, err := f.Filter(ctx, cur)
		if err != nil {
			return tools.ToolCall{}, nil, err
		}
		all = append(all, decs...)
		if blockingDec(decs) != nil {
			return tools.ToolCall{}, all, blocked(blockingDec(decs).Reason)
		}
		cur = out
	}
	return cur, all, nil
}

type postToolFilterChain []hooks.PostToolFilter

func (c postToolFilterChain) Filter(ctx context.Context, r tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	var all []hooks.FilterDecision
	cur := r
	for _, f := range c {
		out, decs, err := f.Filter(ctx, cur)
		if err != nil {
			return tools.ToolResult{}, nil, err
		}
		all = append(all, decs...)
		if blockingDec(decs) != nil {
			return tools.ToolResult{}, all, blocked(blockingDec(decs).Reason)
		}
		cur = out
	}
	return cur, all, nil
}

func blockingDec(decs []hooks.FilterDecision) *hooks.FilterDecision {
	for i := range decs {
		if decs[i].Action == hooks.FilterActionBlock {
			return &decs[i]
		}
	}
	return nil
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./build/... -v`

- [ ] **Step 5: Commit**

```bash
git add build/filter_chains.go build/filter_chains_test.go
git commit -m "feat(build): pre/pre/post tool filter chain adapters"
```

---

### Task 3.4: Tool router

**Files:**
- Create: `build/tool_router.go`
- Create: `build/tool_router_test.go`

- [ ] **Step 1: Write failing test**

```go
// build/tool_router_test.go
package build

import (
	"context"
	"errors"
	"testing"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

type canned struct {
	names []string
}

func (c canned) Invoke(ctx context.Context, ictx tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	return tools.ToolResult{Name: call.Name, Status: tools.ToolStatusSuccess}, nil
}

func TestToolRouter_DispatchByName(t *testing.T) {
	packA := registry.ToolPack{
		Invoker:     canned{names: []string{"a"}},
		Definitions: []llm.ToolDefinition{{Name: "a"}},
	}
	packB := registry.ToolPack{
		Invoker:     canned{names: []string{"b"}},
		Definitions: []llm.ToolDefinition{{Name: "b"}},
	}
	r, defs, err := newToolRouter([]registry.ToolPack{packA, packB})
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 2 {
		t.Fatalf("defs=%d", len(defs))
	}
	out, err := r.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "a" {
		t.Fatalf("name=%s", out.Name)
	}
}

func TestToolRouter_Collision(t *testing.T) {
	pack := registry.ToolPack{
		Invoker:     canned{},
		Definitions: []llm.ToolDefinition{{Name: "dup"}},
	}
	_, _, err := newToolRouter([]registry.ToolPack{pack, pack})
	if !errors.Is(err, ErrToolNameCollision) {
		t.Fatalf("err=%v", err)
	}
}

func TestToolRouter_Unknown(t *testing.T) {
	pack := registry.ToolPack{
		Invoker:     canned{},
		Definitions: []llm.ToolDefinition{{Name: "a"}},
	}
	r, _, err := newToolRouter([]registry.ToolPack{pack})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{Name: "missing"})
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("err=%v", err)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./build/... -v`

- [ ] **Step 3: Implement**

```go
// build/tool_router.go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"errors"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

var (
	ErrToolNameCollision = errors.New("tool name collision across tool packs")
	ErrToolNotFound      = errors.New("tool not found in router")
)

type toolRouter struct {
	byName map[string]tools.Invoker
}

func newToolRouter(packs []registry.ToolPack) (*toolRouter, []llm.ToolDefinition, error) {
	r := &toolRouter{byName: map[string]tools.Invoker{}}
	var defs []llm.ToolDefinition
	for _, p := range packs {
		for _, def := range p.Definitions {
			if _, exists := r.byName[def.Name]; exists {
				return nil, nil, fmt.Errorf("%w: %s", ErrToolNameCollision, def.Name)
			}
			r.byName[def.Name] = p.Invoker
			defs = append(defs, def)
		}
	}
	return r, defs, nil
}

func (r *toolRouter) Invoke(ctx context.Context, ictx tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	inv, ok := r.byName[call.Name]
	if !ok {
		return tools.ToolResult{}, fmt.Errorf("%w: %s", ErrToolNotFound, call.Name)
	}
	return inv.Invoke(ctx, ictx, call)
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./build/... -v`

- [ ] **Step 5: Commit**

```bash
git add build/tool_router.go build/tool_router_test.go
git commit -m "feat(build): tool router with name-collision detection"
```

---

### Task 3.5: Budget override — only tighten

**Files:**
- Create: `build/budget.go`
- Create: `build/budget_test.go`

- [ ] **Step 1: Write failing test**

```go
// build/budget_test.go
package build

import (
	"errors"
	"testing"
	"time"

	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/budget"
)

func TestApplyBudgetOverrides_Tighten(t *testing.T) {
	defaults := budget.Config{
		MaxWallClock:    30 * time.Second,
		MaxInputTokens:  50000,
		MaxOutputTokens: 10000,
		MaxToolCalls:    24,
	}
	cfg, err := applyBudgetOverrides(defaults, spec.BudgetOverrides{
		MaxWallClock: "15s",
		MaxToolCalls: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxWallClock != 15*time.Second {
		t.Fatalf("wall=%v", cfg.MaxWallClock)
	}
	if cfg.MaxToolCalls != 12 {
		t.Fatalf("calls=%d", cfg.MaxToolCalls)
	}
	if cfg.MaxInputTokens != 50000 {
		t.Fatalf("input untouched expected: %d", cfg.MaxInputTokens)
	}
}

func TestApplyBudgetOverrides_RejectsLoosen(t *testing.T) {
	defaults := budget.Config{MaxToolCalls: 10}
	_, err := applyBudgetOverrides(defaults, spec.BudgetOverrides{MaxToolCalls: 100})
	if !errors.Is(err, ErrBudgetLoosening) {
		t.Fatalf("err=%v", err)
	}
}

func TestApplyBudgetOverrides_BadDuration(t *testing.T) {
	_, err := applyBudgetOverrides(budget.Config{MaxWallClock: time.Minute}, spec.BudgetOverrides{MaxWallClock: "banana"})
	if err == nil {
		t.Fatal("expected parse error")
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./build/... -v`

- [ ] **Step 3: Implement**

```go
// build/budget.go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"errors"
	"fmt"
	"time"

	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/budget"
)

// ErrBudgetLoosening is returned when an override would loosen a profile
// ceiling beyond its registered default.
var ErrBudgetLoosening = errors.New("budget override loosens a profile ceiling")

// applyBudgetOverrides returns the default config with tightening overrides
// applied. Any override that would loosen a ceiling (increase max*) fails.
// Zero-valued override fields mean "unset, keep default".
func applyBudgetOverrides(defaults budget.Config, ov spec.BudgetOverrides) (budget.Config, error) {
	out := defaults

	if ov.MaxWallClock != "" {
		d, err := time.ParseDuration(ov.MaxWallClock)
		if err != nil {
			return budget.Config{}, fmt.Errorf("budget.overrides.maxWallClock: %w", err)
		}
		if defaults.MaxWallClock > 0 && d > defaults.MaxWallClock {
			return budget.Config{}, fmt.Errorf("%w: maxWallClock %v > default %v", ErrBudgetLoosening, d, defaults.MaxWallClock)
		}
		out.MaxWallClock = d
	}
	if err := tightenInt64("maxInputTokens", ov.MaxInputTokens, defaults.MaxInputTokens, &out.MaxInputTokens); err != nil {
		return budget.Config{}, err
	}
	if err := tightenInt64("maxOutputTokens", ov.MaxOutputTokens, defaults.MaxOutputTokens, &out.MaxOutputTokens); err != nil {
		return budget.Config{}, err
	}
	if err := tightenInt64("maxToolCalls", ov.MaxToolCalls, defaults.MaxToolCalls, &out.MaxToolCalls); err != nil {
		return budget.Config{}, err
	}
	if err := tightenInt64("maxCostMicrodollars", ov.MaxCostMicrodollars, defaults.MaxCostMicrodollars, &out.MaxCostMicrodollars); err != nil {
		return budget.Config{}, err
	}
	return out, nil
}

func tightenInt64(field string, override, def int64, dst *int64) error {
	if override == 0 {
		return nil
	}
	if def > 0 && override > def {
		return fmt.Errorf("%w: %s %d > default %d", ErrBudgetLoosening, field, override, def)
	}
	*dst = override
	return nil
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./build/... -v`

- [ ] **Step 5: Commit**

```bash
git add build/budget.go build/budget_test.go
git commit -m "feat(build): budget override tightening (reject loosening)"
```

---

### Task 3.6: Resolver — spec refs to factories

**Files:**
- Create: `build/resolver.go`
- Create: `build/resolver_test.go`

- [ ] **Step 1: Write failing test**

```go
// build/resolver_test.go
package build

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/llm"
)

type provFac struct{ id registry.ID }

func (f provFac) ID() registry.ID                                                        { return f.id }
func (f provFac) Description() string                                                    { return "p" }
func (f provFac) Build(context.Context, map[string]any) (llm.Provider, error)            { return nil, nil }

type promptFac struct{ id registry.ID }

func (f promptFac) ID() registry.ID                                                      { return f.id }
func (f promptFac) Description() string                                                  { return "p" }
func (f promptFac) Build(context.Context, map[string]any) (string, error)                { return "hi", nil }

func TestResolveProviderAndPrompt(t *testing.T) {
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(provFac{id: "provider.fake@1.0.0"})
	_ = r.RegisterPromptAsset(promptFac{id: "prompt.sys@1.0.0"})

	s := &spec.AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "a.b", Version: "0.1.0"},
		Provider:   spec.ComponentRef{Ref: "provider.fake@1.0.0"},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: "prompt.sys@1.0.0"}},
	}
	res, err := resolve(context.Background(), s, r)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.provider == nil {
		t.Fatal("provider factory nil")
	}
	if res.systemPrompt != "hi" {
		t.Fatalf("prompt=%s", res.systemPrompt)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./build/... -v`

- [ ] **Step 3: Implement resolver (provider + prompt only for now; extend in next task)**

```go
// build/resolver.go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/llm"
)

// resolved holds every factory plus its built value for the materializer.
type resolved struct {
	provider     llm.Provider
	providerID   registry.ID
	providerCfg  map[string]any
	systemPrompt string
	promptID     registry.ID

	// Populated in later tasks:
	toolPacks           []registry.ToolPack
	policyHooks         []policyEntry
	preLLM              []filterEntry[any]
	preTool             []filterEntry[any]
	postTool            []filterEntry[any]
	budgetProfile       *budgetEntry
	telemetryProfile    *telemetryEntry
	credResolverID      registry.ID
	credentials         any // credentials.Resolver
	identityID          registry.ID
	identity            any // identity.Signer
	specSnapshot        *spec.AgentSpec
}

type policyEntry struct {
	id  registry.ID
	cfg map[string]any
	// hook hooks.PolicyHook
	hook any
}

type filterEntry[T any] struct {
	id     registry.ID
	cfg    map[string]any
	filter T
}

type budgetEntry struct {
	id  registry.ID
	cfg map[string]any
	bp  registry.BudgetProfile
}

type telemetryEntry struct {
	id  registry.ID
	cfg map[string]any
	tp  registry.TelemetryProfile
}

func resolve(ctx context.Context, s *spec.AgentSpec, r *registry.ComponentRegistry) (*resolved, error) {
	out := &resolved{specSnapshot: s}

	provFactory, err := r.Provider(registry.ID(s.Provider.Ref))
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)
	}
	prov, err := provFactory.Build(ctx, s.Provider.Config)
	if err != nil {
		return nil, fmt.Errorf("build provider %s: %w", s.Provider.Ref, err)
	}
	out.provider, out.providerID, out.providerCfg = prov, provFactory.ID(), s.Provider.Config

	promptFactory, err := r.PromptAsset(registry.ID(s.Prompt.System.Ref))
	if err != nil {
		return nil, fmt.Errorf("resolve prompt.system: %w", err)
	}
	text, err := promptFactory.Build(ctx, s.Prompt.System.Config)
	if err != nil {
		return nil, fmt.Errorf("build prompt %s: %w", s.Prompt.System.Ref, err)
	}
	out.systemPrompt, out.promptID = text, promptFactory.ID()

	return out, nil
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./build/... -v`

- [ ] **Step 5: Commit**

```bash
git add build/resolver.go build/resolver_test.go
git commit -m "feat(build): resolver — provider + prompt asset"
```

---

### Task 3.7: Resolver — remaining kinds

**Files:**
- Modify: `build/resolver.go`
- Modify: `build/resolver_test.go`

- [ ] **Step 1: Extend resolver**

In `resolve`, after the prompt block, append resolution for every remaining slot in fixed order (tools → policies → preLLM → preTool → postTool → budget → telemetry → credentials → identity). Pattern per slot:

```go
	for i, ref := range s.Tools {
		f, err := r.ToolPack(registry.ID(ref.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve tools[%d]: %w", i, err)
		}
		pack, err := f.Build(ctx, ref.Config)
		if err != nil {
			return nil, fmt.Errorf("build tools[%d] %s: %w", i, ref.Ref, err)
		}
		out.toolPacks = append(out.toolPacks, pack)
	}

	for i, ref := range s.Policies {
		f, err := r.PolicyPack(registry.ID(ref.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve policies[%d]: %w", i, err)
		}
		pp, err := f.Build(ctx, ref.Config)
		if err != nil {
			return nil, fmt.Errorf("build policies[%d] %s: %w", i, ref.Ref, err)
		}
		out.policyHooks = append(out.policyHooks, policyEntry{id: f.ID(), cfg: ref.Config, hook: pp.Hook})
	}
```

Repeat for each of: `s.Filters.PreLLM → PreLLMFilter`, `s.Filters.PreTool → PreToolFilter`, `s.Filters.PostTool → PostToolFilter`, `s.Budget → BudgetProfile`, `s.Telemetry → TelemetryProfile`, `s.Credentials → CredentialResolver`, `s.Identity → IdentitySigner`. Each slot's value lands in the matching field on `resolved`.

For optional slots (`Budget`, `Telemetry`, `Credentials`, `Identity`) guard with `if s.X != nil`.

Drop the `any`-typed placeholders in `resolved` — rewrite to concrete types once every field is populated, e.g.:

```go
type resolved struct {
	provider     llm.Provider
	providerID   registry.ID
	providerCfg  map[string]any
	systemPrompt string
	promptID     registry.ID

	toolPacks          []registry.ToolPack
	toolPackIDs        []registry.ID
	toolPackCfgs       []map[string]any

	policyHooks        []hooks.PolicyHook
	policyHookIDs      []registry.ID
	policyHookCfgs     []map[string]any

	preLLMFilters      []hooks.PreLLMFilter
	preLLMIDs          []registry.ID
	preLLMCfgs         []map[string]any

	preToolFilters     []hooks.PreToolFilter
	preToolIDs         []registry.ID
	preToolCfgs        []map[string]any

	postToolFilters    []hooks.PostToolFilter
	postToolIDs        []registry.ID
	postToolCfgs       []map[string]any

	budget             *registry.BudgetProfile
	budgetID           registry.ID
	budgetCfg          map[string]any
	budgetOverrides    spec.BudgetOverrides

	telemetry          *registry.TelemetryProfile
	telemetryID        registry.ID
	telemetryCfg       map[string]any

	credResolver       credentials.Resolver
	credResolverID     registry.ID
	credResolverCfg    map[string]any

	identity           identity.Signer
	identityID         registry.ID
	identityCfg        map[string]any

	specSnapshot       *spec.AgentSpec
}
```

Imports to add at top of file:

```go
import (
	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/identity"
)
```

Delete the old placeholder types (`policyEntry`, `filterEntry[T]`, etc.) if they become unused.

- [ ] **Step 2: Extend test**

Add a test that registers one fake factory per kind (minimal implementations) and asserts each field on `resolved` is non-zero after `resolve`.

Use this pattern — declare a minimal fake per kind right in the test file, constructed inline in `registry.Register<Kind>(...)` calls. Re-use `fakeHTTPToolPack`, `fakePolicyHook`, etc. style structs; each fake's `Build` returns a zero-value-ish but non-nil result of the right type.

- [ ] **Step 3: Run (expect pass)**

Run: `go test ./build/... -v`

- [ ] **Step 4: Commit**

```bash
git add build/resolver.go build/resolver_test.go
git commit -m "feat(build): resolver handles all 11 kinds"
```

---

### Task 3.8: `Build` assembler

**Files:**
- Create: `build/build.go`
- Create: `build/build_test.go`

- [ ] **Step 1: Write failing test**

This test uses the internal test fakes to exercise the full path end-to-end; the heavy integration test lives in Task 5.2 using a real fakeprovider package. Here we only check that `Build` calls `orchestrator.New`, wraps the result, and populates the manifest.

```go
// build/build_test.go
package build

import (
	"context"
	"testing"
	"time"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
)

// Minimal fake provider: implements llm.Provider by panicking in Complete
// (the build test never invokes it).
type minProvider struct{}

func (minProvider) Name() string                                           { return "min" }
func (minProvider) SupportsParallelToolCalls() bool                        { return false }
func (minProvider) Capabilities() llm.Capabilities                         { return llm.Capabilities{} }
func (minProvider) Complete(context.Context, llm.LLMRequest) (llm.LLMResponse, error) {
	return llm.LLMResponse{}, nil
}
func (minProvider) Stream(context.Context, llm.LLMRequest) (<-chan llm.LLMStreamChunk, error) {
	ch := make(chan llm.LLMStreamChunk)
	close(ch)
	return ch, nil
}

type minProvFac struct{ id registry.ID }

func (f minProvFac) ID() registry.ID                                        { return f.id }
func (f minProvFac) Description() string                                    { return "" }
func (f minProvFac) Build(context.Context, map[string]any) (llm.Provider, error) {
	return minProvider{}, nil
}

type minPromptFac struct{ id registry.ID }

func (f minPromptFac) ID() registry.ID                                      { return f.id }
func (f minPromptFac) Description() string                                  { return "" }
func (f minPromptFac) Build(context.Context, map[string]any) (string, error) { return "hi", nil }

type minBudgetFac struct{ id registry.ID }

func (f minBudgetFac) ID() registry.ID                                      { return f.id }
func (f minBudgetFac) Description() string                                  { return "" }
func (f minBudgetFac) Build(context.Context, map[string]any) (registry.BudgetProfile, error) {
	return registry.BudgetProfile{
		Guard:         budget.NullGuard{},
		DefaultConfig: budget.Config{MaxWallClock: 30 * time.Second, MaxToolCalls: 10},
	}, nil
}

func TestBuild_MinimalSpec(t *testing.T) {
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterBudgetProfile(minBudgetFac{id: "budgetprofile.default@1.0.0"})

	s := &spec.AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "a.b", Version: "0.1.0"},
		Provider:   spec.ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: "prompt.sys@1.0.0"}},
		Budget:     &spec.BudgetRef{Ref: "budgetprofile.default@1.0.0"},
	}

	built, err := Build(context.Background(), s, r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if built.Orchestrator == nil {
		t.Fatal("orchestrator nil")
	}
	m := built.Manifest
	if m.SpecID != "a.b" || m.SpecVersion != "0.1.0" {
		t.Fatalf("manifest=%+v", m)
	}
	// Resolved components: provider, prompt, budget.
	if len(m.Resolved) != 3 {
		t.Fatalf("resolved=%d", len(m.Resolved))
	}

	// Registry should now be frozen.
	err = r.RegisterProvider(minProvFac{id: "provider.other@1.0.0"})
	if err == nil {
		t.Fatal("expected registry frozen")
	}

	// Silence unused suppression.
	_ = hooks.Allow
}
```

(`hooks.Allow` is a no-op compile-time check; keep or drop — see note.)

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./build/... -v`

- [ ] **Step 3: Implement Build**

```go
// build/build.go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"
	"time"

	"github.com/praxis-os/praxis-forge/manifest"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/orchestrator"
)

// BuiltAgent is a stateless wiring + metadata bundle. Per-turn state lives in
// the embedded Orchestrator; conversation history is the caller's concern.
type BuiltAgent struct {
	Orchestrator *orchestrator.Orchestrator
	Manifest     manifest.Manifest
}

// Build validates the spec, resolves every component through the registry,
// composes chains, and materializes a *orchestrator.Orchestrator.
func Build(ctx context.Context, s *spec.AgentSpec, r *registry.ComponentRegistry) (*BuiltAgent, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	r.Freeze()

	res, err := resolve(ctx, s, r)
	if err != nil {
		return nil, err
	}

	var opts []orchestrator.Option

	// Prompt: orchestrator has no WithSystemPrompt; system prompt flows
	// via InvocationRequest.SystemPrompt per turn. The resolved text is
	// surfaced via BuiltAgent.SystemPrompt (added in Task 5.1).
	// Here we stash it on the manifest only.

	// Tools.
	if len(res.toolPacks) > 0 {
		router, _, err := newToolRouter(res.toolPacks)
		if err != nil {
			return nil, fmt.Errorf("tool router: %w", err)
		}
		opts = append(opts, orchestrator.WithToolInvoker(router))
	}

	// Policy chain.
	if len(res.policyHooks) > 0 {
		opts = append(opts, orchestrator.WithPolicyHook(policyChain(res.policyHooks)))
	}

	// Filter chains.
	if len(res.preLLMFilters) > 0 {
		opts = append(opts, orchestrator.WithPreLLMFilter(preLLMFilterChain(res.preLLMFilters)))
	}
	if len(res.preToolFilters) > 0 {
		opts = append(opts, orchestrator.WithPreToolFilter(preToolFilterChain(res.preToolFilters)))
	}
	if len(res.postToolFilters) > 0 {
		opts = append(opts, orchestrator.WithPostToolFilter(postToolFilterChain(res.postToolFilters)))
	}

	// Budget.
	if res.budget != nil {
		cfg, err := applyBudgetOverrides(res.budget.DefaultConfig, res.budgetOverrides)
		if err != nil {
			return nil, err
		}
		opts = append(opts, orchestrator.WithBudgetGuard(res.budget.Guard))
		_ = cfg // cfg is applied per-InvocationRequest in Task 5.1.
	}

	// Telemetry.
	if res.telemetry != nil {
		opts = append(opts, orchestrator.WithLifecycleEmitter(res.telemetry.Emitter))
		opts = append(opts, orchestrator.WithAttributeEnricher(res.telemetry.Enricher))
	}

	// Credentials.
	if res.credResolver != nil {
		opts = append(opts, orchestrator.WithCredentialResolver(res.credResolver))
	}

	// Identity.
	if res.identity != nil {
		opts = append(opts, orchestrator.WithIdentitySigner(res.identity))
	}

	orch, err := orchestrator.New(res.provider, opts...)
	if err != nil {
		return nil, fmt.Errorf("orchestrator.New: %w", err)
	}

	return &BuiltAgent{
		Orchestrator: orch,
		Manifest:     buildManifest(s, res),
	}, nil
}

func buildManifest(s *spec.AgentSpec, res *resolved) manifest.Manifest {
	m := manifest.Manifest{
		SpecID:      s.Metadata.ID,
		SpecVersion: s.Metadata.Version,
		BuiltAt:     time.Now().UTC(),
	}
	m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
		Kind:   string(registry.KindProvider),
		ID:     string(res.providerID),
		Config: res.providerCfg,
	})
	m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
		Kind: string(registry.KindPromptAsset),
		ID:   string(res.promptID),
	})
	for i, id := range res.toolPackIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:   string(registry.KindToolPack),
			ID:     string(id),
			Config: res.toolPackCfgs[i],
		})
	}
	for i, id := range res.policyHookIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:   string(registry.KindPolicyPack),
			ID:     string(id),
			Config: res.policyHookCfgs[i],
		})
	}
	// Same pattern for every other slot:
	for i, id := range res.preLLMIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindPreLLMFilter), ID: string(id), Config: res.preLLMCfgs[i],
		})
	}
	for i, id := range res.preToolIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindPreToolFilter), ID: string(id), Config: res.preToolCfgs[i],
		})
	}
	for i, id := range res.postToolIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindPostToolFilter), ID: string(id), Config: res.postToolCfgs[i],
		})
	}
	if res.budget != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindBudgetProfile), ID: string(res.budgetID), Config: res.budgetCfg,
		})
	}
	if res.telemetry != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindTelemetryProfile), ID: string(res.telemetryID), Config: res.telemetryCfg,
		})
	}
	if res.credResolver != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindCredentialResolver), ID: string(res.credResolverID), Config: res.credResolverCfg,
		})
	}
	if res.identity != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindIdentitySigner), ID: string(res.identityID), Config: res.identityCfg,
		})
	}
	return m
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./build/... -v`

- [ ] **Step 5: Commit**

```bash
git add build/build.go build/build_test.go
git commit -m "feat(build): Build assembles orchestrator + manifest"
```

---

