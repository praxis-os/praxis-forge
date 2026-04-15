# praxis-forge Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the first vertical slice of praxis-forge: parse + validate a declarative `AgentSpec` YAML, resolve every component through a typed `ComponentRegistry`, materialize a stateless `BuiltAgent` backed by `*orchestrator.Orchestrator`, with one concrete factory per kernel seam (11 Kinds) and a runnable realistic demo.

**Architecture:** Module-cohesive Go layout. `spec/` parses + validates YAML; `registry/` holds typed per-Kind factories; `build/` resolves refs, composes multi-component chains into praxis single-instance hooks, and materializes via `orchestrator.New`. `manifest/` records the resolved build. 11 concrete factories live under `factories/<kind>/`. Top-level `forge.go` is a thin facade. No overlays, no `extends`, no skills, no MCP, no lockfile in Phase 1.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3` (strict decode), `log/slog`, `crypto/ed25519`, `net/http`. Depends on `github.com/praxis-os/praxis` via local `replace` to `../praxis`. Tests use stdlib `testing` with table-driven patterns. Lint: `golangci-lint` (reuse praxis config). No external validation library.

**Companion spec:** [`docs/superpowers/specs/2026-04-15-praxis-forge-phase-1-design.md`](../specs/2026-04-15-praxis-forge-phase-1-design.md)

---

## File Structure

Each file has one responsibility. Keep files under ~400 LOC; split if a file grows beyond that.

### `spec/` (Task group 1)

- `spec/ids.go` — `ParseID`, regex, error types
- `spec/types.go` — all YAML-mapped structs (`AgentSpec`, `Metadata`, `ComponentRef`, etc.)
- `spec/load.go` — `LoadSpec(path)` using strict `yaml.v3` decoder
- `spec/validate.go` — `(*AgentSpec).Validate()` running invariants in fixed order
- `spec/errors.go` — `ValidationError`, `Errors` aggregator
- `spec/testdata/valid/*.yaml`, `spec/testdata/invalid/*.yaml`, `*.err.txt`

### `registry/` (Task group 2)

- `registry/kind.go` — `Kind` string type + 11 constants
- `registry/id.go` — `ID` type (re-exports `spec.ParseID` semantics)
- `registry/types.go` — result structs: `ToolPack`, `PolicyPack`, `BudgetProfile`, `TelemetryProfile`, `ToolDescriptor`, `PolicyDescriptor`, `RiskTier`
- `registry/factories.go` — 11 factory interfaces
- `registry/registry.go` — `ComponentRegistry` + per-kind `Register*` / lookup methods + `Freeze`
- `registry/errors.go` — `ErrRegistryFrozen`, `ErrDuplicate`, `ErrNotFound`, `ErrKindMismatch`

### `build/` + `manifest/` (Task group 3)

- `manifest/manifest.go` — `Manifest`, `ResolvedComponent`, JSON marshal
- `build/resolver.go` — walk spec, resolve refs
- `build/policy_chain.go` — `policyChain` adapter
- `build/filter_chains.go` — three filter-stage chain adapters
- `build/tool_router.go` — `toolRouter`
- `build/budget.go` — `applyBudgetOverrides`
- `build/build.go` — top-level `Build` function

### `factories/<kind>/` (Task group 4)

One leaf package per factory, each with `factory.go` + `factory_test.go`.

### Facade + demo (Task group 5)

- `forge.go` — re-exports + `Option` type
- `forge_test.go` — offline integration test
- `internal/testutil/fakeprovider/fakeprovider.go`
- `examples/demo/main.go`, `examples/demo/agent.yaml`
- Root `README.md` update

### Commit 6 (Task group 6)

- Phase 0 doc amendments.

---

## Conventions used throughout

- **TDD**: every task is test-first. Write the failing test, run it, implement, rerun.
- **Error style**: return errors wrapping `fmt.Errorf("%w", sentinel)` for typed matching via `errors.Is`.
- **Imports**: standard Go ordering — stdlib, then external, then this module, separated by blank lines.
- **License header**: first line of every `.go` file is `// SPDX-License-Identifier: Apache-2.0` matching [doc.go](../../../doc.go).
- **Package docs**: each non-leaf package has a `doc.go` with a package comment.
- **Commits**: one commit per task unless the task explicitly says otherwise. Conventional-commit format: `feat(pkg): short line`.

---

## Task group 0 — Repo prep

### Task 0.1: Dev tooling bootstrap

**Files:**
- Create: `Makefile`
- Create: `.golangci.yml` (copy from `../praxis/.golangci.yml` if present)
- Modify: `go.mod` (add `gopkg.in/yaml.v3`)

- [ ] **Step 1: Inspect praxis lint config**

Run: `ls ../praxis/.golangci* 2>/dev/null && cat ../praxis/.golangci.yml`
If present, copy to `./.golangci.yml` verbatim. If absent, create a minimal one (see Step 3).

- [ ] **Step 2: Add YAML dep**

Run: `go get gopkg.in/yaml.v3 && go mod tidy`
Expected: `go.mod` gains `require gopkg.in/yaml.v3 v3.x.y`; `go.sum` created.

- [ ] **Step 3: Create Makefile**

```make
.PHONY: test test-race lint fmt tidy integration

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

integration:
	go test -tags=integration ./examples/demo/...
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum Makefile .golangci.yml
git commit -m "chore: bootstrap dev tooling (Make, lint, yaml dep)"
```

---

## Task group 1 — `spec/` package

### Task 1.1: ID parser

**Files:**
- Create: `spec/ids.go`
- Create: `spec/ids_test.go`

- [ ] **Step 1: Write failing test**

```go
// spec/ids_test.go
package spec

import "testing"

func TestParseID(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantVer  string
		wantErr  bool
	}{
		{"provider.anthropic@1.0.0", "provider.anthropic", "1.0.0", false},
		{"toolpack.http-get@2.3.4", "toolpack.http-get", "2.3.4", false},
		{"bad", "", "", true},
		{"nope@", "", "", true},
		{"@1.0.0", "", "", true},
		{"Foo@1.0.0", "", "", true},        // uppercase rejected
		{"foo@1", "", "", true},             // non-semver rejected
		{"foo@1.2", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			name, ver, err := ParseID(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if name != c.wantName || ver != c.wantVer {
				t.Fatalf("got (%q,%q) want (%q,%q)", name, ver, c.wantName, c.wantVer)
			}
		})
	}
}
```

- [ ] **Step 2: Run test (expect fail)**

Run: `go test ./spec/... -run TestParseID -v`
Expected: build error — `undefined: ParseID`.

- [ ] **Step 3: Implement**

```go
// spec/ids.go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"regexp"
)

var idRegexp = regexp.MustCompile(`^([a-z][a-z0-9.-]*)@(\d+\.\d+\.\d+)$`)

// ParseID splits a component reference `<dotted>@<semver>` into its name and
// version parts. Returns an error if the string does not match.
func ParseID(s string) (name, version string, err error) {
	m := idRegexp.FindStringSubmatch(s)
	if m == nil {
		return "", "", fmt.Errorf("invalid component id %q: want <dotted-name>@<semver>", s)
	}
	return m[1], m[2], nil
}
```

- [ ] **Step 4: Run test (expect pass)**

Run: `go test ./spec/... -run TestParseID -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add spec/ids.go spec/ids_test.go
git commit -m "feat(spec): add ParseID for component refs"
```

---

### Task 1.2: AgentSpec types

**Files:**
- Create: `spec/types.go`

- [ ] **Step 1: Write the types**

```go
// spec/types.go
// SPDX-License-Identifier: Apache-2.0

// Package spec defines the AgentSpec declarative format plus a strict
// YAML loader and validator. Phase 1 shape: no overlays, no extends.
package spec

// AgentSpec is the top-level declarative agent definition.
type AgentSpec struct {
	APIVersion  string         `yaml:"apiVersion"`
	Kind        string         `yaml:"kind"`
	Metadata    Metadata       `yaml:"metadata"`
	Provider    ComponentRef   `yaml:"provider"`
	Prompt      PromptBlock    `yaml:"prompt"`
	Tools       []ComponentRef `yaml:"tools,omitempty"`
	Policies    []ComponentRef `yaml:"policies,omitempty"`
	Filters     FilterBlock    `yaml:"filters,omitempty"`
	Budget      *BudgetRef     `yaml:"budget,omitempty"`
	Telemetry   *ComponentRef  `yaml:"telemetry,omitempty"`
	Credentials *CredRef       `yaml:"credentials,omitempty"`
	Identity    *ComponentRef  `yaml:"identity,omitempty"`

	// Phase-gated: accepted by the parser but must be empty until the
	// corresponding phase ships.
	Extends        []string       `yaml:"extends,omitempty"`
	Skills         []ComponentRef `yaml:"skills,omitempty"`
	MCPImports     []ComponentRef `yaml:"mcpImports,omitempty"`
	OutputContract *ComponentRef  `yaml:"outputContract,omitempty"`
}

type Metadata struct {
	ID          string            `yaml:"id"`
	Version     string            `yaml:"version"`
	DisplayName string            `yaml:"displayName,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Owners      []Owner           `yaml:"owners,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

type Owner struct {
	Team    string `yaml:"team,omitempty"`
	Contact string `yaml:"contact,omitempty"`
}

type ComponentRef struct {
	Ref    string         `yaml:"ref"`
	Config map[string]any `yaml:"config,omitempty"`
}

type PromptBlock struct {
	System *ComponentRef `yaml:"system"`
	User   *ComponentRef `yaml:"user,omitempty"`
}

type FilterBlock struct {
	PreLLM   []ComponentRef `yaml:"preLLM,omitempty"`
	PreTool  []ComponentRef `yaml:"preTool,omitempty"`
	PostTool []ComponentRef `yaml:"postTool,omitempty"`
}

type BudgetRef struct {
	Ref       string            `yaml:"ref"`
	Overrides BudgetOverrides   `yaml:"overrides,omitempty"`
}

type BudgetOverrides struct {
	MaxWallClock        string `yaml:"maxWallClock,omitempty"`   // duration, e.g. "30s"
	MaxInputTokens      int64  `yaml:"maxInputTokens,omitempty"`
	MaxOutputTokens     int64  `yaml:"maxOutputTokens,omitempty"`
	MaxToolCalls        int64  `yaml:"maxToolCalls,omitempty"`
	MaxCostMicrodollars int64  `yaml:"maxCostMicrodollars,omitempty"`
}

type CredRef struct {
	Ref    string   `yaml:"ref"`
	Scopes []string `yaml:"scopes,omitempty"`
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./spec/...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add spec/types.go
git commit -m "feat(spec): add AgentSpec types"
```

---

### Task 1.3: Strict YAML loader

**Files:**
- Create: `spec/load.go`
- Create: `spec/load_test.go`
- Create: `spec/testdata/valid/minimal.yaml`
- Create: `spec/testdata/invalid/unknown_field.yaml`

- [ ] **Step 1: Write fixtures**

```yaml
# spec/testdata/valid/minimal.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.demo
  version: 0.1.0
provider:
  ref: provider.anthropic@1.0.0
  config:
    model: claude-sonnet-4-5
prompt:
  system:
    ref: prompt.demo-system@1.0.0
```

```yaml
# spec/testdata/invalid/unknown_field.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.demo
  version: 0.1.0
  bogus: nope
provider:
  ref: provider.anthropic@1.0.0
prompt:
  system:
    ref: prompt.demo-system@1.0.0
```

- [ ] **Step 2: Write failing test**

```go
// spec/load_test.go
package spec

import (
	"strings"
	"testing"
)

func TestLoadSpec_Valid(t *testing.T) {
	s, err := LoadSpec("testdata/valid/minimal.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.Metadata.ID != "acme.demo" {
		t.Fatalf("id=%q", s.Metadata.ID)
	}
	if s.Provider.Ref != "provider.anthropic@1.0.0" {
		t.Fatalf("provider ref=%q", s.Provider.Ref)
	}
}

func TestLoadSpec_RejectsUnknownField(t *testing.T) {
	_, err := LoadSpec("testdata/invalid/unknown_field.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("error missing field name: %v", err)
	}
}
```

- [ ] **Step 3: Run (expect fail — undefined LoadSpec)**

Run: `go test ./spec/... -run TestLoadSpec -v`

- [ ] **Step 4: Implement loader**

```go
// spec/load.go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadSpec reads and decodes an AgentSpec YAML file with strict unknown-field
// rejection. It does not run validation; call (*AgentSpec).Validate separately.
func LoadSpec(path string) (*AgentSpec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spec: %w", err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var s AgentSpec
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("decode spec %s: %w", path, err)
	}
	return &s, nil
}
```

- [ ] **Step 5: Run (expect pass)**

Run: `go test ./spec/... -run TestLoadSpec -v`
Expected: both tests PASS.

- [ ] **Step 6: Commit**

```bash
git add spec/load.go spec/load_test.go spec/testdata
git commit -m "feat(spec): strict YAML loader with unknown-field rejection"
```

---

### Task 1.4: Validation errors type

**Files:**
- Create: `spec/errors.go`
- Create: `spec/errors_test.go`

- [ ] **Step 1: Write failing test**

```go
// spec/errors_test.go
package spec

import (
	"errors"
	"testing"
)

func TestErrors_AppendAndError(t *testing.T) {
	var e Errors
	e.Addf("first problem")
	e.Addf("second %s", "problem")
	if len(e) != 2 {
		t.Fatalf("len=%d", len(e))
	}
	msg := e.Error()
	if msg == "" {
		t.Fatal("empty error message")
	}
	if !errors.Is(e, ErrValidation) {
		t.Fatal("Errors should match ErrValidation via Is")
	}
}

func TestErrors_OrNilWhenEmpty(t *testing.T) {
	var e Errors
	if e.OrNil() != nil {
		t.Fatal("empty Errors.OrNil should be nil")
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./spec/... -run TestErrors -v`

- [ ] **Step 3: Implement**

```go
// spec/errors.go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"errors"
	"fmt"
	"strings"
)

// ErrValidation is the sentinel for any spec validation failure.
var ErrValidation = errors.New("spec validation failed")

// Errors aggregates one or more validation violations reported by Validate.
type Errors []string

func (e *Errors) Addf(format string, args ...any) {
	*e = append(*e, fmt.Sprintf(format, args...))
}

func (e Errors) OrNil() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

func (e Errors) Error() string {
	return fmt.Sprintf("%s: %s", ErrValidation.Error(), strings.Join(e, "; "))
}

func (e Errors) Is(target error) bool {
	return target == ErrValidation
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./spec/... -run TestErrors -v`

- [ ] **Step 5: Commit**

```bash
git add spec/errors.go spec/errors_test.go
git commit -m "feat(spec): add Errors aggregator + ErrValidation sentinel"
```

---

### Task 1.5: Validator — header invariants

**Files:**
- Create: `spec/validate.go`
- Create: `spec/validate_test.go`

- [ ] **Step 1: Write failing test**

```go
// spec/validate_test.go
package spec

import (
	"errors"
	"strings"
	"testing"
)

func baseValidSpec() *AgentSpec {
	return &AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   Metadata{ID: "acme.demo", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.anthropic@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
	}
}

func TestValidate_Valid(t *testing.T) {
	if err := baseValidSpec().Validate(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidate_BadAPIVersion(t *testing.T) {
	s := baseValidSpec()
	s.APIVersion = "nope"
	err := s.Validate()
	if err == nil || !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "apiVersion") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_BadKind(t *testing.T) {
	s := baseValidSpec()
	s.Kind = "Nope"
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_BadMetadataID(t *testing.T) {
	s := baseValidSpec()
	s.Metadata.ID = "Bad_ID"
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "metadata.id") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_BadSemver(t *testing.T) {
	s := baseValidSpec()
	s.Metadata.Version = "1"
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "metadata.version") {
		t.Fatalf("err=%v", err)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./spec/... -run TestValidate -v`

- [ ] **Step 3: Implement Validate (header subset — more rules in later tasks)**

```go
// spec/validate.go
// SPDX-License-Identifier: Apache-2.0

package spec

import "regexp"

const (
	expectedAPIVersion = "forge.praxis-os.dev/v0"
	expectedKind       = "AgentSpec"
)

var (
	metadataIDRegexp = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z0-9]+)*(-[a-z0-9.]+)?$`)
	semverRegexp     = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
)

// Validate runs every Phase 1 invariant in a fixed order, aggregating failures.
func (s *AgentSpec) Validate() error {
	var errs Errors

	if s.APIVersion != expectedAPIVersion {
		errs.Addf("apiVersion: want %q, got %q", expectedAPIVersion, s.APIVersion)
	}
	if s.Kind != expectedKind {
		errs.Addf("kind: want %q, got %q", expectedKind, s.Kind)
	}
	if !metadataIDRegexp.MatchString(s.Metadata.ID) {
		errs.Addf("metadata.id %q: must be dotted-lowercase", s.Metadata.ID)
	}
	if !semverRegexp.MatchString(s.Metadata.Version) {
		errs.Addf("metadata.version %q: must be semver MAJOR.MINOR.PATCH", s.Metadata.Version)
	}

	return errs.OrNil()
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./spec/... -run TestValidate -v`

- [ ] **Step 5: Commit**

```bash
git add spec/validate.go spec/validate_test.go
git commit -m "feat(spec): validate header (apiVersion, kind, metadata)"
```

---

### Task 1.6: Validator — required blocks + ref format

**Files:**
- Modify: `spec/validate.go`
- Modify: `spec/validate_test.go`

- [ ] **Step 1: Add failing tests**

Append to `spec/validate_test.go`:

```go
func TestValidate_MissingProviderRef(t *testing.T) {
	s := baseValidSpec()
	s.Provider = ComponentRef{}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "provider.ref") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_MissingPromptSystem(t *testing.T) {
	s := baseValidSpec()
	s.Prompt = PromptBlock{}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "prompt.system") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_BadRefFormat(t *testing.T) {
	s := baseValidSpec()
	s.Tools = []ComponentRef{{Ref: "not-a-ref"}}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "tools[0].ref") {
		t.Fatalf("err=%v", err)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./spec/... -run TestValidate -v`

- [ ] **Step 3: Extend Validate**

Add to the body of `Validate`, before `return`:

```go
	// Required top-level refs.
	validateRef(&errs, "provider.ref", s.Provider.Ref)
	if s.Prompt.System == nil || s.Prompt.System.Ref == "" {
		errs.Addf("prompt.system: required")
	} else {
		validateRef(&errs, "prompt.system.ref", s.Prompt.System.Ref)
	}

	// Optional component refs.
	for i, t := range s.Tools {
		validateRef(&errs, fmt.Sprintf("tools[%d].ref", i), t.Ref)
	}
	for i, p := range s.Policies {
		validateRef(&errs, fmt.Sprintf("policies[%d].ref", i), p.Ref)
	}
	for i, f := range s.Filters.PreLLM {
		validateRef(&errs, fmt.Sprintf("filters.preLLM[%d].ref", i), f.Ref)
	}
	for i, f := range s.Filters.PreTool {
		validateRef(&errs, fmt.Sprintf("filters.preTool[%d].ref", i), f.Ref)
	}
	for i, f := range s.Filters.PostTool {
		validateRef(&errs, fmt.Sprintf("filters.postTool[%d].ref", i), f.Ref)
	}
	if s.Budget != nil {
		validateRef(&errs, "budget.ref", s.Budget.Ref)
	}
	if s.Telemetry != nil {
		validateRef(&errs, "telemetry.ref", s.Telemetry.Ref)
	}
	if s.Credentials != nil {
		validateRef(&errs, "credentials.ref", s.Credentials.Ref)
	}
	if s.Identity != nil {
		validateRef(&errs, "identity.ref", s.Identity.Ref)
	}
```

Add this helper and `fmt` import:

```go
import "fmt"

func validateRef(errs *Errors, field, ref string) {
	if ref == "" {
		errs.Addf("%s: required", field)
		return
	}
	if _, _, err := ParseID(ref); err != nil {
		errs.Addf("%s: %s", field, err.Error())
	}
}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./spec/... -v`
Expected: all previous + three new tests PASS.

- [ ] **Step 5: Commit**

```bash
git add spec/validate.go spec/validate_test.go
git commit -m "feat(spec): validate refs and required blocks"
```

---

### Task 1.7: Validator — phase-gated + duplicates

**Files:**
- Modify: `spec/validate.go`
- Modify: `spec/validate_test.go`

- [ ] **Step 1: Add failing tests**

```go
func TestValidate_RejectsExtends(t *testing.T) {
	s := baseValidSpec()
	s.Extends = []string{"acme.base@1.0.0"}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "extends") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_RejectsSkills(t *testing.T) {
	s := baseValidSpec()
	s.Skills = []ComponentRef{{Ref: "skill.foo@1.0.0"}}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "skills") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_RejectsMCP(t *testing.T) {
	s := baseValidSpec()
	s.MCPImports = []ComponentRef{{Ref: "mcp.foo@1.0.0"}}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "mcpImports") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_RejectsOutputContract(t *testing.T) {
	s := baseValidSpec()
	s.OutputContract = &ComponentRef{Ref: "contract.foo@1.0.0"}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "outputContract") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_DuplicateToolRef(t *testing.T) {
	s := baseValidSpec()
	s.Tools = []ComponentRef{
		{Ref: "toolpack.http-get@1.0.0"},
		{Ref: "toolpack.http-get@1.0.0"},
	}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("err=%v", err)
	}
}
```

- [ ] **Step 2: Run (expect fail)**

Run: `go test ./spec/... -run TestValidate -v`

- [ ] **Step 3: Extend Validate**

Before `return errs.OrNil()`:

```go
	// Phase-gated fields: must be empty in v0.
	if len(s.Extends) > 0 {
		errs.Addf("extends: phase-gated (Phase 2); must be empty in v0")
	}
	if len(s.Skills) > 0 {
		errs.Addf("skills: phase-gated (Phase 3); must be empty in v0")
	}
	if len(s.MCPImports) > 0 {
		errs.Addf("mcpImports: phase-gated (Phase 4); must be empty in v0")
	}
	if s.OutputContract != nil {
		errs.Addf("outputContract: phase-gated (Phase 3); must be empty in v0")
	}

	// Duplicate tool refs.
	seen := map[string]int{}
	for i, t := range s.Tools {
		if prev, ok := seen[t.Ref]; ok {
			errs.Addf("tools[%d]: duplicate of tools[%d] (ref=%s)", i, prev, t.Ref)
		} else {
			seen[t.Ref] = i
		}
	}
```

- [ ] **Step 4: Run (expect pass)**

Run: `go test ./spec/... -v`

- [ ] **Step 5: Commit**

```bash
git add spec/validate.go spec/validate_test.go
git commit -m "feat(spec): validate phase-gated fields and duplicate tool refs"
```

---

### Task 1.8: Fixture-driven validator coverage

**Files:**
- Create: `spec/testdata/valid/full.yaml`
- Create: `spec/testdata/invalid/bad_api_version.yaml` + `.err.txt`
- Create: `spec/testdata/invalid/extends_nonempty.yaml` + `.err.txt`
- Create: `spec/testdata/invalid/duplicate_tool.yaml` + `.err.txt`
- Create: `spec/testdata/invalid/bad_ref.yaml` + `.err.txt`
- Modify: `spec/load_test.go` (add fixture-driven test)

- [ ] **Step 1: Create `full.yaml`**

```yaml
# spec/testdata/valid/full.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.support-triage
  version: 1.4.0
  displayName: "Support Triage"
  description: "Triages inbound tickets"
  owners:
    - team: platform-ai
      contact: platform-ai@acme.example
  labels:
    domain: support
provider:
  ref: provider.anthropic@1.0.0
  config:
    model: claude-sonnet-4-5
    maxOutputTokens: 2048
prompt:
  system:
    ref: prompt.demo-system@1.0.0
tools:
  - ref: toolpack.http-get@1.0.0
    config:
      allowedHosts: ["raw.githubusercontent.com"]
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
    maxWallClock: 15s
    maxToolCalls: 12
telemetry:
  ref: telemetryprofile.slog@1.0.0
  config:
    level: info
credentials:
  ref: credresolver.env@1.0.0
  scopes:
    - net:http
identity:
  ref: identitysigner.ed25519@1.0.0
  config:
    issuer: acme
    tokenLifetimeSeconds: 60
```

- [ ] **Step 2: Create invalid fixtures**

```yaml
# spec/testdata/invalid/bad_api_version.yaml
apiVersion: wrong/v0
kind: AgentSpec
metadata: {id: a.b, version: 0.1.0}
provider: {ref: provider.foo@1.0.0}
prompt:
  system:
    ref: prompt.sys@1.0.0
```

`spec/testdata/invalid/bad_api_version.err.txt` contents: `apiVersion`

```yaml
# spec/testdata/invalid/extends_nonempty.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata: {id: a.b, version: 0.1.0}
provider: {ref: provider.foo@1.0.0}
prompt:
  system:
    ref: prompt.sys@1.0.0
extends:
  - acme.base@1.0.0
```

`extends_nonempty.err.txt`: `extends`

```yaml
# spec/testdata/invalid/duplicate_tool.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata: {id: a.b, version: 0.1.0}
provider: {ref: provider.foo@1.0.0}
prompt:
  system:
    ref: prompt.sys@1.0.0
tools:
  - ref: toolpack.http-get@1.0.0
  - ref: toolpack.http-get@1.0.0
```

`duplicate_tool.err.txt`: `duplicate`

```yaml
# spec/testdata/invalid/bad_ref.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata: {id: a.b, version: 0.1.0}
provider: {ref: not-a-ref}
prompt:
  system:
    ref: prompt.sys@1.0.0
```

`bad_ref.err.txt`: `provider.ref`

- [ ] **Step 3: Add fixture-driven test to `spec/load_test.go`**

```go
import (
	"os"
	"path/filepath"
)

func TestValidateFixtures(t *testing.T) {
	matches, err := filepath.Glob("testdata/invalid/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, p := range matches {
		p := p
		t.Run(filepath.Base(p), func(t *testing.T) {
			wantBytes, err := os.ReadFile(strings.TrimSuffix(p, ".yaml") + ".err.txt")
			if err != nil {
				t.Fatalf("missing .err.txt: %v", err)
			}
			want := strings.TrimSpace(string(wantBytes))

			s, err := LoadSpec(p)
			if err != nil {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("load error %q does not contain %q", err, want)
				}
				return // loader caught it
			}
			err = s.Validate()
			if err == nil {
				t.Fatalf("fixture %s: expected validation error containing %q", p, want)
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("validation error %q does not contain %q", err, want)
			}
		})
	}
}

func TestLoadAndValidate_Full(t *testing.T) {
	s, err := LoadSpec("testdata/valid/full.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
```

- [ ] **Step 4: Run all spec tests**

Run: `go test ./spec/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add spec/testdata spec/load_test.go
git commit -m "test(spec): fixture-driven validator coverage"
```

---

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

## Task group 6 — Phase 0 doc amendments

### Task 6.1: Update design docs to match locked decisions

**Files:**
- Modify: `docs/design/registry-interfaces.md`
- Modify: `docs/design/forge-overview.md`
- Modify: `docs/design/agent-spec-v0.md`

- [ ] **Step 1: `registry-interfaces.md`**

In the `Factory — one interface, one Build method per kind` section, remove `ConfigSchema() json.RawMessage` from every factory interface sketch. Add a note:

> **Phase 1 note.** Factory interfaces carry only `ID`, `Description`, and
> `Build(ctx, cfg map[string]any)`. Each factory decodes `cfg` into its own
> typed struct. JSON Schema introspection is deferred to Phase 2 when
> lockfile/manifest tooling needs it.

In the `Kind` enum listing, add `KindPromptAsset` after `KindProvider`.

Add a `PromptAssetFactory` sketch alongside the other factory-per-kind examples:

```go
type PromptAssetFactory interface {
    ID() ID
    Description() string
    Build(ctx context.Context, cfg map[string]any) (string, error)
}
```

- [ ] **Step 2: `forge-overview.md`**

In the `Top-level forge facade` section, change the example from:

```go
built, err := forge.Build(ctx, spec, overlays, registry, ...)
```

to:

```go
built, err := forge.Build(ctx, spec, registry, opts...) // Phase 2 adds overlays
```

In `Phase roadmap` under Phase 1, add to the bullet list: "no overlays in the Build signature; added Phase 2". Under Phase 2: "Build signature grows `overlays []AgentOverlay`".

- [ ] **Step 3: `agent-spec-v0.md`**

In the `Referenced kinds` table, add a row:

| Slot               | Kind           | Resolves to                  |
|--------------------|----------------|------------------------------|
| `prompt.system`    | `prompt_asset` | a string injected per-invoke |

Under `Open questions (raised for Phase 1 review)`, replace the prompt question with a resolution note:

> **Resolved in Phase 1.** Prompts flow through registered `prompt_asset`
> factories. The simplest factory (`prompt.literal@1`) returns its configured
> `text` verbatim. Inline literal prompts in the spec remain prohibited for
> tighter governance.

- [ ] **Step 4: Verify no broken cross-links**

Run: `grep -rn "overlays \[\]AgentOverlay" docs/` and inspect each hit.
Run: `grep -rn "ConfigSchema" docs/` and confirm no surviving references imply Phase 1 support.

- [ ] **Step 5: Commit**

```bash
git add docs/design
git commit -m "docs: amend Phase 0 designs — drop ConfigSchema, add prompt_asset, defer overlays"
```

---

## Final verification

After every task group is complete:

- [ ] **Step V1: Full test pass**

Run: `make test-race`
Expected: all packages green, no race warnings.

- [ ] **Step V2: Lint**

Run: `make lint`
Expected: zero reports.

- [ ] **Step V3: Vet**

Run: `go vet ./...`
Expected: clean.

- [ ] **Step V4: Offline integration**

Run: `go test ./... -count=1`
Expected: `forge_test.go` exercises all 10 non-tool kinds; passes.

- [ ] **Step V5: Live Anthropic round-trip (manual)**

Run: `ANTHROPIC_API_KEY=$KEY go run -tags=integration ./examples/demo`
Expected: demo fetches the allowed URL, returns a non-empty response, logs show one slog line per lifecycle event.

- [ ] **Step V6: Manifest inspection**

In a scratch test, assert `b.Manifest().Resolved` contains one entry per referenced component with the exact config the spec declared.

- [ ] **Step V7: Registry freeze sanity**

Assert `r.RegisterProvider(...)` after `forge.Build` returns `ErrRegistryFrozen`.

---

## Self-review notes (for plan author)

Coverage matrix of the design spec → task mapping:

| Spec section | Task(s) |
|--------------|---------|
| Public API + layering | 5.1 |
| `spec/` contract | 1.1–1.8 |
| `registry/` contract | 2.1–2.5 |
| `build/` pipeline + chains + router + budget | 3.1–3.8 |
| 11 concrete factories | 4.1–4.11 |
| Testing strategy (unit + offline integration + tagged live) | 5.3, 5.4, per-factory tests |
| Delivery shape (5 atomic commit tracks) | 1.x, 2.x, 3.x, 4.x, 5.x task groups |
| Phase 0 doc amendments | 6.1 |
| Verification | Final verification block |
| Out of scope | Documented in plan context; not implemented |

Red-flag re-check — searched this plan for each phrase from the "No Placeholders" list:
- No "TBD"/"TODO"/"implement later".
- Every step that changes code shows the code.
- Expected command output is named (PASS / FAIL / clean build).
- Every referenced type is introduced in an earlier task or imported from praxis/praxis-forge.

Type consistency checks:
- `registry.BudgetProfile.Guard` and `.DefaultConfig` used identically in Tasks 3.5, 3.7, 3.8, 4.2, 5.3.
- `hooks.FilterActionBlock` / `FilterActionRedact` / `FilterActionLog` / `FilterActionPass` used with matching spelling in Tasks 3.3, 4.6, 4.7, 4.8.
- `tools.ToolCall.ArgumentsJSON` and `tools.ToolResult.Output` appear in Tasks 4.7, 4.8, 4.10 — flagged as "verify against real praxis type" in each task. Engineer must reconcile before writing the test.
- Factory ID strings (e.g. `provider.anthropic@1.0.0`) consistent between test fixtures, integration test, and demo YAML.

Known items the engineer must verify against the real praxis source before implementing (flagged inline in the relevant tasks):
1. `tools.ToolCall` arguments field name (`ArgumentsJSON` vs `Arguments`).
2. `tools.ToolResult` content field name (`Output` vs `Content`).
3. `llm.Message.Content` type (plain string vs multimodal blocks).
4. `event.InvocationEvent` field name (`Kind` vs `Name`).
5. `anthropic.New` signature and functional options.
6. `identity.NewEd25519Signer` functional options (`WithIssuer`, `WithTokenLifetime`).
7. `credentials.Credential` interface method names.
8. `hooks.PolicyInput` shape for `PhasePreLLMInput`.
9. `budget.NullGuard` vs `NewInMemoryGuard` availability.

These are unavoidable — the plan cannot pin names it has not confirmed. Each is flagged at the point of use so the engineer resolves it in context.

