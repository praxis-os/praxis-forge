# Task group 1 — `AgentOverlay` + `RefList` tri-state + `LoadOverlay`

> Part of [praxis-forge Phase 2a Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md).

**Commit (atomic):** `feat(spec): AgentOverlay + RefList tri-state + LoadOverlay`

**Scope:** declare the overlay file format and its in-Go representation. Three pieces:

1. `RefList` wrapper — tri-state distinguishing absent / null / empty / populated for `[]ComponentRef` lists.
2. `AgentOverlay` + nested types — the typed mirror of `AgentSpec` with every field optional, no phase-gated fields.
3. `LoadOverlay(path)` — strict YAML decoder with `KnownFields(true)` set at the top level.

Tests cover every shape of the four `RefList` states and confirm the strict decoder rejects both unknown keys and phase-gated keys at any depth.

---

## Task 1.1: `RefList` tri-state wrapper

**Files:**
- Create: `spec/overlay.go` (initial chunk — types only)

- [ ] **Step 1: Write the wrapper type and its `UnmarshalYAML`**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// RefList wraps []ComponentRef so the YAML decoder can distinguish
// "field absent" from "field present (set to anything, including null
// or empty)".
//
//	tools:                              → RefList absent in body          → Set=false
//	tools:                              → explicit null after the colon    → Set=true,  Items=nil
//	tools: []                           → explicit empty list               → Set=true,  Items=[]
//	tools: [{ref: ...}]                 → populated                         → Set=true,  Items=[...]
//
// Merge semantics on the apply path: if Set is false the base list is
// preserved; if Set is true the wrapper replaces the base list verbatim
// (including nil and empty cases).
type RefList struct {
	Set   bool
	Items []ComponentRef

	// Line is the 1-based line number of the field in the source YAML, or
	// 0 if the wrapper was constructed in Go.
	Line int
}

// UnmarshalYAML records that the field was present and decodes its
// contents into Items. An explicit null (e.g. `tools:` followed by
// nothing) leaves Items nil but flips Set to true.
func (r *RefList) UnmarshalYAML(node *yaml.Node) error {
	r.Set = true
	r.Line = node.Line

	switch node.Kind {
	case yaml.SequenceNode:
		var items []ComponentRef
		if err := node.Decode(&items); err != nil {
			return fmt.Errorf("decode RefList at line %d: %w", node.Line, err)
		}
		r.Items = items
		return nil
	case yaml.ScalarNode:
		// Only an explicit null is legal as a scalar here.
		if node.Tag == "!!null" || node.Value == "" {
			r.Items = nil
			return nil
		}
		return fmt.Errorf("RefList at line %d: expected sequence or null, got scalar %q", node.Line, node.Value)
	default:
		return fmt.Errorf("RefList at line %d: expected sequence or null, got %s", node.Line, nodeKindName(node.Kind))
	}
}

func nodeKindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return fmt.Sprintf("kind(%d)", k)
	}
}
```

> **Engineer verify-against-tree:** confirm `*yaml.Node.Line` is the 1-based source line in the linked `yaml.v3` version (open item #1 in the design spec). If a different field name is used, capture the same value through the documented helper before finishing this task.

- [ ] **Step 2: Build to surface compile errors**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 1.2: `RefList` decode tests (the four states)

**Files:**
- Create: `spec/overlay_test.go`

- [ ] **Step 1: Write the table-driven test**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// reflist tests cover the four states the wrapper must distinguish.
// They go through the full overlay struct rather than poking RefList
// directly, so we also exercise the strict decoder around it.

type refListProbe struct {
	Tools *RefList `yaml:"tools,omitempty"`
}

func decodeProbe(t *testing.T, doc string) refListProbe {
	t.Helper()
	var p refListProbe
	dec := yaml.NewDecoder(strings.NewReader(doc))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return p
}

func TestRefList_Absent(t *testing.T) {
	p := decodeProbe(t, "name: foo\n")
	// Decoding a field absent from the document leaves the *RefList nil.
	if p.Tools != nil {
		t.Fatalf("expected nil *RefList, got %+v", p.Tools)
	}
}

func TestRefList_ExplicitNull(t *testing.T) {
	p := decodeProbe(t, "tools:\n")
	if p.Tools == nil {
		t.Fatal("expected non-nil *RefList for explicit null")
	}
	if !p.Tools.Set {
		t.Fatal("Set should be true for explicit null")
	}
	if p.Tools.Items != nil {
		t.Fatalf("Items should be nil, got %+v", p.Tools.Items)
	}
}

func TestRefList_ExplicitEmpty(t *testing.T) {
	p := decodeProbe(t, "tools: []\n")
	if p.Tools == nil || !p.Tools.Set {
		t.Fatalf("expected Set=true wrapper, got %+v", p.Tools)
	}
	if p.Tools.Items == nil || len(p.Tools.Items) != 0 {
		t.Fatalf("Items should be empty (non-nil), got %+v", p.Tools.Items)
	}
}

func TestRefList_Populated(t *testing.T) {
	p := decodeProbe(t, "tools:\n  - ref: toolpack.http-get@1.0.0\n  - ref: toolpack.shell@1.0.0\n")
	if p.Tools == nil || !p.Tools.Set {
		t.Fatalf("expected Set=true wrapper, got %+v", p.Tools)
	}
	if len(p.Tools.Items) != 2 {
		t.Fatalf("want 2 items, got %+v", p.Tools.Items)
	}
	if p.Tools.Items[0].Ref != "toolpack.http-get@1.0.0" {
		t.Fatalf("items[0].Ref=%q", p.Tools.Items[0].Ref)
	}
	if p.Tools.Line == 0 {
		t.Fatal("Line should be captured for populated wrapper")
	}
}

func TestRefList_RejectsScalarString(t *testing.T) {
	var p refListProbe
	dec := yaml.NewDecoder(strings.NewReader("tools: not-a-list\n"))
	dec.KnownFields(true)
	err := dec.Decode(&p)
	if err == nil {
		t.Fatal("expected error decoding scalar into RefList")
	}
	if !strings.Contains(err.Error(), "expected sequence") {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run (expect pass for all five states)**

Run: `go test ./spec/... -run TestRefList -v`
Expected: PASS.

---

## Task 1.3: `AgentOverlay` types

**Files:**
- Modify: `spec/overlay.go` (append the rest of the types)

- [ ] **Step 1: Append the overlay struct family**

Append to `spec/overlay.go`:

```go
// AgentOverlay is the top-level overlay document. The strict YAML
// decoder ensures unknown keys at any depth fail to parse.
type AgentOverlay struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   OverlayMeta      `yaml:"metadata"`
	Spec       AgentOverlayBody `yaml:"spec"`

	// File is the source path passed to LoadOverlay, or empty for in-Go
	// constructed overlays. Populated by LoadOverlay; surfaced through
	// OverlayAttribution and Provenance.
	File string `yaml:"-"`
}

// OverlayMeta carries attribution-only metadata for an overlay file.
// metadata.name surfaces in error messages and the manifest.
type OverlayMeta struct {
	Name string `yaml:"name"`
}

// AgentOverlayBody mirrors AgentSpec but every field is optional and
// each replaceable list uses the RefList tri-state wrapper. Phase-gated
// AgentSpec fields (extends, skills, mcpImports, outputContract) are
// deliberately absent so the strict decoder rejects them at parse time.
type AgentOverlayBody struct {
	Metadata    *OverlayMetadata `yaml:"metadata,omitempty"`
	Provider    *ComponentRef    `yaml:"provider,omitempty"`
	Prompt      *PromptBlock     `yaml:"prompt,omitempty"`
	Tools       *RefList         `yaml:"tools,omitempty"`
	Policies    *RefList         `yaml:"policies,omitempty"`
	Filters     *FilterOverlay   `yaml:"filters,omitempty"`
	Budget      *BudgetRef       `yaml:"budget,omitempty"`
	Telemetry   *ComponentRef    `yaml:"telemetry,omitempty"`
	Credentials *CredRef         `yaml:"credentials,omitempty"`
	Identity    *ComponentRef    `yaml:"identity,omitempty"`
}

// OverlayMetadata mirrors Metadata but every field is optional. ID and
// Version are accepted at parse time and rejected at apply time by
// validateLocked when they would change the merged result.
type OverlayMetadata struct {
	ID          string            `yaml:"id,omitempty"`
	Version     string            `yaml:"version,omitempty"`
	DisplayName string            `yaml:"displayName,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Owners      []Owner           `yaml:"owners,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// FilterOverlay wraps each filter slice in its own RefList so each
// stage can be replaced or cleared independently.
type FilterOverlay struct {
	PreLLM   *RefList `yaml:"preLLM,omitempty"`
	PreTool  *RefList `yaml:"preTool,omitempty"`
	PostTool *RefList `yaml:"postTool,omitempty"`
}
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 1.4: `LoadOverlay` strict YAML loader

**Files:**
- Create: `spec/load_overlay.go`

- [ ] **Step 1: Write the loader**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadOverlay reads and decodes an AgentOverlay YAML file with strict
// unknown-field rejection at every depth. It validates only the
// envelope (apiVersion + kind); body validation happens during
// Normalize when the overlay is applied.
func LoadOverlay(path string) (*AgentOverlay, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open overlay: %w", err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var ov AgentOverlay
	if err := dec.Decode(&ov); err != nil {
		return nil, fmt.Errorf("decode overlay %s: %w", path, err)
	}
	if ov.APIVersion != expectedAPIVersion {
		return nil, fmt.Errorf("overlay %s: apiVersion: want %q, got %q",
			path, expectedAPIVersion, ov.APIVersion)
	}
	if ov.Kind != "AgentOverlay" {
		return nil, fmt.Errorf("overlay %s: kind: want %q, got %q",
			path, "AgentOverlay", ov.Kind)
	}
	ov.File = path
	return &ov, nil
}
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 1.5: Overlay decode fixtures + tests

**Files:**
- Create: `spec/testdata/overlay/valid/replace_provider.yaml`
- Create: `spec/testdata/overlay/valid/clear_tools.yaml`
- Create: `spec/testdata/overlay/invalid/unknown_field.yaml` + `.err.txt`
- Create: `spec/testdata/overlay/invalid/phase_gated_skills.yaml` + `.err.txt`
- Create: `spec/testdata/overlay/invalid/phase_gated_extends.yaml` + `.err.txt`
- Create: `spec/testdata/overlay/invalid/wrong_apiversion.yaml` + `.err.txt`
- Create: `spec/testdata/overlay/invalid/wrong_kind.yaml` + `.err.txt`
- Modify: `spec/overlay_test.go`

- [ ] **Step 1: Write the valid fixtures**

```yaml
# spec/testdata/overlay/valid/replace_provider.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: prod-override
spec:
  provider:
    ref: provider.anthropic@1.0.0
    config:
      model: claude-opus-4-7
```

```yaml
# spec/testdata/overlay/valid/clear_tools.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: clear-tools
spec:
  tools: []
```

- [ ] **Step 2: Write the invalid fixtures**

```yaml
# spec/testdata/overlay/invalid/unknown_field.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: bad
  bogus: nope            # unknown key under metadata
spec: {}
```

`spec/testdata/overlay/invalid/unknown_field.err.txt`: `bogus`

```yaml
# spec/testdata/overlay/invalid/phase_gated_skills.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: bad
spec:
  skills:
    - ref: skill.foo@1.0.0
```

`spec/testdata/overlay/invalid/phase_gated_skills.err.txt`: `skills`

```yaml
# spec/testdata/overlay/invalid/phase_gated_extends.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: bad
spec:
  extends:
    - acme.base@1.0.0
```

`spec/testdata/overlay/invalid/phase_gated_extends.err.txt`: `extends`

```yaml
# spec/testdata/overlay/invalid/wrong_apiversion.yaml
apiVersion: nope/v0
kind: AgentOverlay
metadata:
  name: bad
spec: {}
```

`spec/testdata/overlay/invalid/wrong_apiversion.err.txt`: `apiVersion`

```yaml
# spec/testdata/overlay/invalid/wrong_kind.yaml
apiVersion: forge.praxis-os.dev/v0
kind: NotAnOverlay
metadata:
  name: bad
spec: {}
```

`spec/testdata/overlay/invalid/wrong_kind.err.txt`: `kind`

- [ ] **Step 3: Append fixture-driven tests to `spec/overlay_test.go`**

Append to `spec/overlay_test.go`:

```go
import (
	"os"
	"path/filepath"
)

func TestLoadOverlay_ReplaceProvider(t *testing.T) {
	ov, err := LoadOverlay("testdata/overlay/valid/replace_provider.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if ov.Metadata.Name != "prod-override" {
		t.Fatalf("metadata.name=%q", ov.Metadata.Name)
	}
	if ov.Spec.Provider == nil || ov.Spec.Provider.Ref != "provider.anthropic@1.0.0" {
		t.Fatalf("provider not decoded: %+v", ov.Spec.Provider)
	}
	if got := ov.Spec.Provider.Config["model"]; got != "claude-opus-4-7" {
		t.Fatalf("provider.config.model=%v", got)
	}
	if ov.File != "testdata/overlay/valid/replace_provider.yaml" {
		t.Fatalf("File not populated: %q", ov.File)
	}
}

func TestLoadOverlay_ClearTools(t *testing.T) {
	ov, err := LoadOverlay("testdata/overlay/valid/clear_tools.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if ov.Spec.Tools == nil {
		t.Fatal("Tools wrapper should be non-nil for explicit []")
	}
	if !ov.Spec.Tools.Set {
		t.Fatal("Tools.Set should be true for explicit []")
	}
	if ov.Spec.Tools.Items == nil || len(ov.Spec.Tools.Items) != 0 {
		t.Fatalf("Items should be empty (non-nil), got %+v", ov.Spec.Tools.Items)
	}
}

func TestLoadOverlay_InvalidFixtures(t *testing.T) {
	matches, err := filepath.Glob("testdata/overlay/invalid/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no invalid fixtures found")
	}
	for _, p := range matches {
		p := p
		t.Run(filepath.Base(p), func(t *testing.T) {
			wantBytes, err := os.ReadFile(strings.TrimSuffix(p, ".yaml") + ".err.txt")
			if err != nil {
				t.Fatalf("missing .err.txt: %v", err)
			}
			want := strings.TrimSpace(string(wantBytes))

			_, err = LoadOverlay(p)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", want)
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not contain %q", err, want)
			}
		})
	}
}
```

- [ ] **Step 4: Run all overlay tests**

Run: `go test ./spec/... -run "TestRefList|TestLoadOverlay" -v`
Expected: every subtest PASS.

---

## Task 1.6: Lint + commit task group 1

- [ ] **Step 1: Race + full spec suite**

Run: `go test -race ./spec/... -count=1`
Expected: PASS, no race warnings.

- [ ] **Step 2: Lint**

Run: `make lint`
Expected: zero reports.

- [ ] **Step 3: Commit**

```bash
git add spec/overlay.go spec/load_overlay.go spec/overlay_test.go spec/testdata/overlay
git commit -m "feat(spec): AgentOverlay + RefList tri-state + LoadOverlay

Adds the typed overlay format (mirror of AgentSpec with every field
optional) and its strict YAML loader. Replaceable list fields use the
RefList wrapper to disambiguate four states: absent, explicit null,
explicit empty, populated. Phase-gated AgentSpec fields (extends,
skills, mcpImports, outputContract) are deliberately absent from
AgentOverlayBody so the strict decoder rejects them at parse time.

LoadOverlay validates only the envelope (apiVersion + kind); body
validation happens during Normalize when overlays are applied.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```
