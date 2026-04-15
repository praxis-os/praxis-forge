> Part of [praxis-forge Phase 1 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-15-praxis-forge-phase-1-design.md`](../../specs/2026-04-15-praxis-forge-phase-1-design.md).

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

