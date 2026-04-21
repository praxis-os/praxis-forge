# Task 00 — Registry kinds and value types

Activate `KindSkill` and `KindOutputContract` alongside the other active kinds in the registry package, and add the value types that skill and output-contract factories produce.

## Files

- Modify: [`registry/kind.go`](../../../registry/kind.go)
- Modify: [`registry/types.go`](../../../registry/types.go)
- Modify: [`registry/kind_test.go`](../../../registry/kind_test.go)

## Background

Phase 0 docs reserved `skill` and `output_contract` as future kinds ([`docs/design/registry-interfaces.md:44-46`](../../../docs/design/registry-interfaces.md#L44-L46) and [`docs/design/agent-spec-v0.md:158-159`](../../../docs/design/agent-spec-v0.md#L158-L159)). The current `registry/kind.go` lists only the 11 Phase-1 kinds ([`registry/kind.go:10-22`](../../../registry/kind.go#L10-L22)). This task adds the two new kinds as string constants so subsequent tasks can pattern-match on them.

The companion value types (`Skill`, `OutputContract`, `RequiredComponent`, `SkillDescriptor`, `OutputContractDescriptor`) live in `registry/types.go` alongside the existing `ToolPack`, `PolicyPack`, etc. They are pure data carriers — no methods in this task.

## Steps

- [ ] **Step 1: Write the failing test for new Kind constants**

Append to [`registry/kind_test.go`](../../../registry/kind_test.go) a new test. First read the current content to find the end of the file — the test below assumes there's an existing `TestKindString` or similar; if not, use the snippet as a standalone test function.

Append:

```go
func TestKind_Phase3ActiveKinds(t *testing.T) {
	cases := []struct {
		kind Kind
		want string
	}{
		{KindSkill, "skill"},
		{KindOutputContract, "output_contract"},
	}
	for _, tc := range cases {
		if string(tc.kind) != tc.want {
			t.Errorf("kind %v: want %q, got %q", tc.kind, tc.want, string(tc.kind))
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./registry/ -run TestKind_Phase3ActiveKinds -v`

Expected: FAIL — `undefined: KindSkill` and `undefined: KindOutputContract`.

- [ ] **Step 3: Add the kind constants**

Edit [`registry/kind.go`](../../../registry/kind.go). Extend the existing `const` block to add the two new kinds at the end:

```go
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
	KindSkill              Kind = "skill"
	KindOutputContract     Kind = "output_contract"
)
```

Update the file-level doc comment to drop "Phase 1 knows about" in favor of the current reality. Replace the package doc for `Kind`:

```go
// Kind enumerates every component category the registry knows about.
type Kind string
```

- [ ] **Step 4: Run the kind test to verify it passes**

Run: `go test ./registry/ -run TestKind_Phase3ActiveKinds -v`

Expected: PASS.

- [ ] **Step 5: Run the full registry test suite**

Run: `go test ./registry/... -v`

Expected: all pre-existing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add registry/kind.go registry/kind_test.go
git commit -m "$(cat <<'EOF'
feat(registry): activate KindSkill and KindOutputContract

Phase 3 opens up skill + output-contract kinds. Both were reserved in
Phase 0 docs (docs/design/registry-interfaces.md) but gated at the type
level. Adding the consts unblocks the factory interfaces and
registration wiring in Task 01.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 7: Write the failing test for Skill + OutputContract types**

Create [`registry/types_test.go`](../../../registry/types_test.go) (new file):

```go
// SPDX-License-Identifier: Apache-2.0

package registry

import "testing"

func TestSkill_ZeroValueAcceptable(t *testing.T) {
	// Skill with an empty prompt fragment and no requirements must be
	// representable; validation lives in the build layer, not in the type.
	var s Skill
	if s.PromptFragment != "" {
		t.Errorf("zero PromptFragment: want %q, got %q", "", s.PromptFragment)
	}
	if s.RequiredTools != nil {
		t.Errorf("zero RequiredTools: want nil, got %v", s.RequiredTools)
	}
	if s.RequiredOutputContract != nil {
		t.Errorf("zero RequiredOutputContract: want nil, got %v", s.RequiredOutputContract)
	}
}

func TestSkill_FullyPopulated(t *testing.T) {
	s := Skill{
		PromptFragment: "be helpful",
		RequiredTools: []RequiredComponent{
			{ID: "toolpack.http-get@1.0.0"},
		},
		RequiredPolicies: []RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			Config: map[string]any{"schema": map[string]any{"type": "object"}},
		},
		Descriptor: SkillDescriptor{
			Name:    "structured-output",
			Owner:   "core",
			Summary: "forces JSON output",
			Tags:    []string{"structured", "json"},
		},
	}
	if len(s.RequiredTools) != 1 {
		t.Errorf("RequiredTools len: want 1, got %d", len(s.RequiredTools))
	}
	if s.RequiredOutputContract.ID != "outputcontract.json-schema@1.0.0" {
		t.Errorf("RequiredOutputContract.ID: %s", s.RequiredOutputContract.ID)
	}
	if s.Descriptor.Name != "structured-output" {
		t.Errorf("Descriptor.Name: %s", s.Descriptor.Name)
	}
}

func TestOutputContract_FullyPopulated(t *testing.T) {
	oc := OutputContract{
		Schema: map[string]any{
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
			"properties": map[string]any{
				"answer": map[string]any{"type": "string"},
			},
			"required": []any{"answer"},
		},
		Descriptor: OutputContractDescriptor{
			Name:    "answer-schema",
			Owner:   "core",
			Summary: "Q&A contract",
		},
	}
	if oc.Schema["type"] != "object" {
		t.Errorf("Schema.type: %v", oc.Schema["type"])
	}
	if oc.Descriptor.Name != "answer-schema" {
		t.Errorf("Descriptor.Name: %s", oc.Descriptor.Name)
	}
}

func TestRequiredComponent_NilConfigAllowed(t *testing.T) {
	rc := RequiredComponent{ID: "toolpack.http-get@1.0.0"}
	if rc.Config != nil {
		t.Errorf("default Config: want nil, got %v", rc.Config)
	}
}
```

- [ ] **Step 8: Run the tests to verify they fail**

Run: `go test ./registry/ -run 'TestSkill|TestOutputContract|TestRequiredComponent' -v`

Expected: FAIL — `undefined: Skill`, `undefined: OutputContract`, `undefined: RequiredComponent`, `undefined: SkillDescriptor`, `undefined: OutputContractDescriptor`.

- [ ] **Step 9: Add the value types**

Edit [`registry/types.go`](../../../registry/types.go). Append (after the existing `TelemetryProfile` struct at the end of the file):

```go
// RequiredComponent is a skill's declaration that a specific registered
// factory must be part of the effective composition. During skill
// expansion the build layer compares Config canonically against any
// pre-existing ref of the same ID: identical = idempotent, different =
// conflict (see build/expand.go).
//
// ID format matches the existing spec.ParseID contract:
// "<dotted-name>@<MAJOR>.<MINOR>.<PATCH>".
type RequiredComponent struct {
	// ID is a registered factory id, e.g. "toolpack.http-fetch@1.0.0".
	ID ID
	// Config is the factory config the skill wants applied. Nil means
	// "use whatever config the user or an earlier skill supplied, or the
	// factory's own default if none".
	Config map[string]any
}

// SkillDescriptor is forge-managed metadata about a skill factory.
// Surfaced in the manifest; does not affect composition semantics.
type SkillDescriptor struct {
	Name    string
	Owner   string
	Summary string
	Tags    []string
}

// Skill is the value produced by a SkillFactory. It carries the
// contributions the skill wants merged into the agent's composition.
//
// Semantics:
//   - PromptFragment: literal text appended after the base system prompt
//     during build (see build/expand.go). May be empty.
//   - RequiredTools / RequiredPolicies: auto-injected by the expansion
//     stage with strict conflict detection. See design doc §"Expansion
//     semantics".
//   - RequiredOutputContract: at most one output contract contribution
//     per skill. Two skills requiring different contracts → build error
//     "skill_conflict_output_contract_multiple".
//
// A skill with no non-empty contribution (no fragment, no requirements)
// fails the build with "skill_empty_contribution".
type Skill struct {
	PromptFragment         string
	RequiredTools          []RequiredComponent
	RequiredPolicies       []RequiredComponent
	RequiredOutputContract *RequiredComponent
	Descriptor             SkillDescriptor
}

// OutputContractDescriptor is forge-managed metadata about an
// output-contract factory. Surfaced in the manifest.
type OutputContractDescriptor struct {
	Name    string
	Owner   string
	Summary string
}

// OutputContract is the value produced by an OutputContractFactory.
// Phase 3 stores the schema as a decoded map and enforces only
// structural well-formedness at factory-Build time (see
// factories/outputcontractjsonschema). Semantic JSON Schema validation
// of LLM outputs is deferred to the orchestrator or a later
// dep-bearing phase.
type OutputContract struct {
	// Schema is a JSON Schema document as a decoded map. The map keys
	// are schema keywords ("type", "properties", "required", "$schema",
	// etc.). Canonical comparison uses spec.canonicalEncode.
	Schema     map[string]any
	Descriptor OutputContractDescriptor
}
```

- [ ] **Step 10: Run the tests to verify they pass**

Run: `go test ./registry/ -run 'TestSkill|TestOutputContract|TestRequiredComponent' -v`

Expected: PASS (4 tests).

- [ ] **Step 11: Run the full registry test suite**

Run: `go test ./registry/... -v`

Expected: all tests pass. `go vet ./registry/...` prints nothing.

- [ ] **Step 12: Commit**

```bash
git add registry/types.go registry/types_test.go
git commit -m "$(cat <<'EOF'
feat(registry): Skill, OutputContract, RequiredComponent value types

Three new types a skill factory can produce, one type an output-contract
factory produces. Descriptors carry governance metadata (name, owner,
summary, tags) that will surface in the manifest during Task 06 wiring.
No behavior yet — pure data carriers.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

## Expected state after this task

- `registry/kind.go`: `KindSkill` and `KindOutputContract` are in the active `const` block.
- `registry/types.go`: `Skill`, `OutputContract`, `RequiredComponent`, `SkillDescriptor`, `OutputContractDescriptor` are defined with documented semantics.
- `registry/kind_test.go`: `TestKind_Phase3ActiveKinds` covers both new kinds.
- `registry/types_test.go`: 4 tests cover zero-value and fully-populated construction.
- All existing registry tests still pass.
- Two commits on the branch.
