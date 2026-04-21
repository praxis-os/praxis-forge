# Task 07 — Vertical slice factories

Ship the two concrete factories that exercise the full Phase-3 path end-to-end: `outputcontract.json-schema@1` and `skill.structured-output@1`. Both follow the existing factory package pattern ([`factories/toolpackhttpget/factory.go`](../../../factories/toolpackhttpget/factory.go), [`factories/policypackpiiredact/factory.go`](../../../factories/policypackpiiredact/factory.go)).

## Files

- Create: `factories/outputcontractjsonschema/factory.go`
- Create: `factories/outputcontractjsonschema/factory_test.go`
- Create: `factories/skillstructuredoutput/factory.go`
- Create: `factories/skillstructuredoutput/factory_test.go`

## Steps

### Part A — outputcontract.json-schema@1

- [ ] **Step 1: Write failing factory test**

Create `factories/outputcontractjsonschema/factory_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0

package outputcontractjsonschema

import (
	"context"
	"strings"
	"testing"
)

func TestFactory_BuildsWithValidSchema(t *testing.T) {
	oc, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{
		"schema": map[string]any{
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
			"properties": map[string]any{
				"answer": map[string]any{"type": "string"},
			},
			"required": []any{"answer"},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if oc.Schema["type"] != "object" {
		t.Errorf("Schema.type: %v", oc.Schema["type"])
	}
	if oc.Descriptor.Name == "" {
		t.Error("Descriptor.Name should be set")
	}
}

func TestFactory_RejectsMissingSchema(t *testing.T) {
	_, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "schema: required") {
		t.Fatalf("want 'schema: required', got %v", err)
	}
}

func TestFactory_RejectsNonMapSchema(t *testing.T) {
	_, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{
		"schema": "not-a-map",
	})
	if err == nil || !strings.Contains(err.Error(), "schema:") {
		t.Fatalf("want schema type error, got %v", err)
	}
}

func TestFactory_RejectsStructurallyEmptySchema(t *testing.T) {
	// Empty map is not a valid JSON Schema (must have at least one of
	// $schema/type/properties/$ref at the root).
	_, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{
		"schema": map[string]any{},
	})
	if err == nil || !strings.Contains(err.Error(), "schema:") {
		t.Fatalf("want structural error, got %v", err)
	}
}

func TestFactory_AcceptsTypeOnly(t *testing.T) {
	// A schema containing only "type" is the minimum acceptable form.
	oc, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{
		"schema": map[string]any{"type": "string"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if oc.Schema["type"] != "string" {
		t.Errorf("Schema.type: %v", oc.Schema["type"])
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./factories/outputcontractjsonschema/ -v`

Expected: FAIL — `cannot find package`.

- [ ] **Step 3: Implement the factory**

Create `factories/outputcontractjsonschema/factory.go`:

```go
// SPDX-License-Identifier: Apache-2.0

// Package outputcontractjsonschema is the Phase-3 vertical-slice
// output-contract factory. It accepts a JSON Schema document as a
// decoded Go map and stamps it into OutputContract.Schema. Structural
// well-formedness is checked at build time (non-nil map, at least one
// of $schema/type/properties/$ref at the root). Semantic validation of
// LLM outputs against the schema is explicitly out of scope —
// Phase 3 stays zero-dep and hands the raw schema to consumers.
package outputcontractjsonschema

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
)

type Factory struct{ id registry.ID }

// NewFactory constructs a JSON-Schema output-contract factory with the
// given registry id. The id must match `outputcontract.<name>@<semver>`.
func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "JSON Schema output contract (structural validation only)" }

// allowedRootKeys are the keys a valid JSON Schema must expose at least
// one of at the root. Bare validation — no library dependency.
var allowedRootKeys = []string{"$schema", "type", "properties", "$ref"}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.OutputContract, error) {
	raw, ok := cfg["schema"]
	if !ok {
		return registry.OutputContract{}, fmt.Errorf("%s: schema: required", f.id)
	}
	schema, ok := raw.(map[string]any)
	if !ok {
		return registry.OutputContract{}, fmt.Errorf("%s: schema: want map, got %T", f.id, raw)
	}
	if len(schema) == 0 {
		return registry.OutputContract{}, fmt.Errorf("%s: schema: empty; need at least one of %v at the root", f.id, allowedRootKeys)
	}
	// Require at least one well-known root keyword.
	hasKey := false
	for _, k := range allowedRootKeys {
		if _, ok := schema[k]; ok {
			hasKey = true
			break
		}
	}
	if !hasKey {
		return registry.OutputContract{}, fmt.Errorf("%s: schema: must contain at least one of %v at the root", f.id, allowedRootKeys)
	}
	return registry.OutputContract{
		Schema: schema,
		Descriptor: registry.OutputContractDescriptor{
			Name:    "json-schema",
			Owner:   "core",
			Summary: "structural JSON Schema; semantic validation deferred",
		},
	}, nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./factories/outputcontractjsonschema/ -v`

Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add factories/outputcontractjsonschema/
git commit -m "$(cat <<'EOF'
feat(factories): outputcontract.json-schema@1 (structural JSON Schema)

Vertical-slice output-contract factory. Accepts a JSON Schema document
in config.schema, checks structural well-formedness (non-empty map with
at least one root-level keyword from $schema/type/properties/$ref), and
stamps it into OutputContract.Schema. Zero-dep: semantic validation of
LLM outputs against the schema is deferred to the orchestrator or a
future library-bearing factory.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Part B — skill.structured-output@1

- [ ] **Step 6: Write failing factory test**

Create `factories/skillstructuredoutput/factory_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0

package skillstructuredoutput

import (
	"context"
	"strings"
	"testing"
)

func TestFactory_ProducesPromptFragment(t *testing.T) {
	sk, err := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if sk.PromptFragment == "" {
		t.Error("PromptFragment should be non-empty")
	}
	if !strings.Contains(sk.PromptFragment, "JSON") {
		t.Errorf("PromptFragment should mention JSON; got %q", sk.PromptFragment)
	}
}

func TestFactory_RequiresPIIRedactionPolicy(t *testing.T) {
	sk, err := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(sk.RequiredPolicies) != 1 {
		t.Fatalf("want 1 policy, got %d", len(sk.RequiredPolicies))
	}
	if sk.RequiredPolicies[0].ID != "policypack.pii-redaction@1.0.0" {
		t.Errorf("policy id: %s", sk.RequiredPolicies[0].ID)
	}
}

func TestFactory_RequiresOutputContract(t *testing.T) {
	sk, err := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if sk.RequiredOutputContract == nil {
		t.Fatal("RequiredOutputContract should be non-nil")
	}
	if sk.RequiredOutputContract.ID != "outputcontract.json-schema@1.0.0" {
		t.Errorf("contract id: %s", sk.RequiredOutputContract.ID)
	}
}

func TestFactory_AcceptsEmptyConfig(t *testing.T) {
	_, err := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("nil cfg should work: %v", err)
	}
	_, err = NewFactory("skill.structured-output@1.0.0").Build(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("empty cfg should work: %v", err)
	}
}

func TestFactory_Descriptor(t *testing.T) {
	sk, _ := NewFactory("skill.structured-output@1.0.0").Build(context.Background(), nil)
	if sk.Descriptor.Name != "structured-output" {
		t.Errorf("Descriptor.Name: %s", sk.Descriptor.Name)
	}
	if sk.Descriptor.Owner != "core" {
		t.Errorf("Descriptor.Owner: %s", sk.Descriptor.Owner)
	}
}
```

- [ ] **Step 7: Run to verify failure**

Run: `go test ./factories/skillstructuredoutput/ -v`

Expected: FAIL — `cannot find package`.

- [ ] **Step 8: Implement the factory**

Create `factories/skillstructuredoutput/factory.go`:

```go
// SPDX-License-Identifier: Apache-2.0

// Package skillstructuredoutput is the Phase-3 vertical-slice skill
// factory. It contributes:
//
//   - a fixed prompt fragment instructing the model to emit JSON only
//     matching the required schema, no surrounding prose;
//   - a required policy pack (policypack.pii-redaction@1.0.0) so the
//     structured output path defaults to PII-scrubbed logging;
//   - a required output contract (outputcontract.json-schema@1.0.0)
//     which the consumer supplies with their schema via config.
package skillstructuredoutput

import (
	"context"

	"github.com/praxis-os/praxis-forge/registry"
)

type Factory struct{ id registry.ID }

// NewFactory constructs the structured-output skill factory. The id
// must match `skill.<name>@<semver>`.
func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "forces structured JSON output matching a supplied schema" }

const promptFragment = "Respond with JSON matching the required schema. Do not include prose outside the JSON."

func (f *Factory) Build(_ context.Context, _ map[string]any) (registry.Skill, error) {
	return registry.Skill{
		PromptFragment: promptFragment,
		RequiredPolicies: []registry.RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &registry.RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			// No Config: the consumer supplies the schema by declaring
			// spec.outputContract directly, or via another skill layer.
		},
		Descriptor: registry.SkillDescriptor{
			Name:    "structured-output",
			Owner:   "core",
			Summary: "Emit JSON matching a schema; default PII-redaction policy.",
			Tags:    []string{"structured", "json", "governance"},
		},
	}, nil
}
```

- [ ] **Step 9: Run tests to verify pass**

Run: `go test ./factories/skillstructuredoutput/ -v`

Expected: PASS (5 tests).

- [ ] **Step 10: Run full suite to ensure nothing broke**

Run: `go vet ./... && go test ./... -count=1`

Expected: full project green.

- [ ] **Step 11: Commit**

```bash
git add factories/skillstructuredoutput/
git commit -m "$(cat <<'EOF'
feat(factories): skill.structured-output@1 (vertical-slice skill)

Vertical-slice skill. Prompts for JSON-only output, auto-injects
policypack.pii-redaction@1.0.0 (strictness: medium), and requires
outputcontract.json-schema@1.0.0 which the consumer's spec must
configure with a concrete JSON Schema document.

Pairs with factories/outputcontractjsonschema to prove the Phase-3
skill → expansion → contract path end-to-end in the demo (Task 09)
and in build integration tests (Task 08).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

## Expected state after this task

- `factories/outputcontractjsonschema/factory.go` + `factory_test.go`: 5 tests, structural JSON Schema validation.
- `factories/skillstructuredoutput/factory.go` + `factory_test.go`: 5 tests, forces JSON output + required policy + required contract.
- `go test ./factories/...` green.
- Two commits added.
