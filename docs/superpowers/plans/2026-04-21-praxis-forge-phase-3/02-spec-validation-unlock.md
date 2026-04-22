# Task 02 — Unlock spec validation for skills and output contracts

Remove the two phase-gate blocks in [`spec/validate.go`](../../../spec/validate.go) that reject non-empty `spec.skills` and non-nil `spec.outputContract`, add referential id-prefix validation, and update the matching tests.

## Files

- Modify: [`spec/validate.go`](../../../spec/validate.go)
- Modify: [`spec/validate_test.go`](../../../spec/validate_test.go)

## Scope notes

- **Overlay body is untouched.** The `AgentOverlayBody` struct in [`spec/overlay.go:102-113`](../../../spec/overlay.go#L102-L113) still omits `skills`, `extends`, `mcpImports`, and `outputContract`. The strict YAML decoder still rejects them at parse time. The fixture `spec/testdata/overlay/invalid/phase_gated_skills.yaml` stays valid — it tests a different boundary (overlays cannot introduce skills; only the base/extends chain can). No fixture churn.
- **`mcpImports` phase-gate stays.** Phase 4 work.
- **`extends` is already handled** by normalize (Phase 2a). The `len(s.Extends) > 0` check in validate.go is a pre-normalize check that fires on the raw `AgentSpec` before `spec.Normalize` flattens the chain. Leave alone.

## Background

Current state of [`spec/validate.go:77-89`](../../../spec/validate.go#L77-L89):

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
```

And the matching tests at [`spec/validate_test.go:92-114`](../../../spec/validate_test.go#L92-L114):

```go
func TestValidate_RejectsSkills(t *testing.T) { ... }
func TestValidate_RejectsOutputContract(t *testing.T) { ... }
```

## Steps

- [ ] **Step 1: Write failing tests for new positive+negative validation**

Replace the two tests `TestValidate_RejectsSkills` and `TestValidate_RejectsOutputContract` in [`spec/validate_test.go`](../../../spec/validate_test.go). Remove the existing two test functions (the rejection tests) and add the new ones in the same position.

Before deletion, note the current placement is lines 92-114 (between `TestValidate_RejectsExtends` and `TestValidate_RejectsMCP`).

Replace with:

```go
func TestValidate_AcceptsSkills(t *testing.T) {
	s := baseValidSpec()
	s.Skills = []ComponentRef{{Ref: "skill.structured-output@1.0.0"}}
	if err := s.Validate(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidate_RejectsSkillWrongPrefix(t *testing.T) {
	s := baseValidSpec()
	s.Skills = []ComponentRef{{Ref: "toolpack.http-get@1.0.0"}} // wrong prefix for skills
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "skills[0].ref") {
		t.Fatalf("expected skills[0].ref error, got %v", err)
	}
	if !strings.Contains(err.Error(), "must start with \"skill.\"") {
		t.Fatalf("expected prefix guidance, got %v", err)
	}
}

func TestValidate_RejectsSkillEmptyRef(t *testing.T) {
	s := baseValidSpec()
	s.Skills = []ComponentRef{{Ref: ""}}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "skills[0].ref: required") {
		t.Fatalf("expected 'required' error, got %v", err)
	}
}

func TestValidate_AcceptsOutputContract(t *testing.T) {
	s := baseValidSpec()
	s.OutputContract = &ComponentRef{Ref: "outputcontract.json-schema@1.0.0"}
	if err := s.Validate(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidate_RejectsOutputContractWrongPrefix(t *testing.T) {
	s := baseValidSpec()
	s.OutputContract = &ComponentRef{Ref: "contract.foo@1.0.0"} // missing "outputcontract." prefix
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "outputContract.ref") {
		t.Fatalf("expected outputContract.ref error, got %v", err)
	}
	if !strings.Contains(err.Error(), "must start with \"outputcontract.\"") {
		t.Fatalf("expected prefix guidance, got %v", err)
	}
}

func TestValidate_RejectsOutputContractEmptyRef(t *testing.T) {
	s := baseValidSpec()
	s.OutputContract = &ComponentRef{Ref: ""}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "outputContract.ref: required") {
		t.Fatalf("expected 'required' error, got %v", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./spec/ -run 'TestValidate_(Accepts|Rejects)(Skills|Skill|OutputContract)' -v`

Expected: FAIL. The `AcceptsSkills` and `AcceptsOutputContract` tests should fail with the existing "phase-gated" error; the prefix/empty tests should fail because the current validator returns "phase-gated" rather than the prefix message.

- [ ] **Step 3: Unlock the phase-gates and add referential validation**

Edit [`spec/validate.go`](../../../spec/validate.go). Make two changes.

**Change A** — replace the two phase-gate lines. Replace this block (lines 77-89):

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
```

With:

```go
// Phase-gated fields.
if len(s.Extends) > 0 {
    errs.Addf("extends: phase-gated (Phase 2); must be empty in v0")
}
if len(s.MCPImports) > 0 {
    errs.Addf("mcpImports: phase-gated (Phase 4); must be empty in v0")
}

// Skills (Phase 3 active).
for i, sk := range s.Skills {
    validateKindPrefixedRef(&errs, fmt.Sprintf("skills[%d].ref", i), sk.Ref, "skill.")
}

// Output contract (Phase 3 active).
if s.OutputContract != nil {
    validateKindPrefixedRef(&errs, "outputContract.ref", s.OutputContract.Ref, "outputcontract.")
}
```

**Change B** — add the helper. Append after the existing `validateRef` function at the bottom of the file:

```go
// validateKindPrefixedRef requires the ref both to parse and to carry
// the expected kind prefix (e.g. "skill." for skills[], "outputcontract."
// for spec.outputContract.ref). Prefix mismatch errors guide the author
// to the correct registration namespace without leaking registry
// internals into validate.go.
func validateKindPrefixedRef(errs *Errors, field, ref, prefix string) {
	if ref == "" {
		errs.Addf("%s: required", field)
		return
	}
	if _, _, err := ParseID(ref); err != nil {
		errs.Addf("%s: %s", field, err.Error())
		return
	}
	if !strings.HasPrefix(ref, prefix) {
		errs.Addf("%s: ref %q must start with %q", field, ref, prefix)
	}
}
```

Ensure the imports at the top of the file include `strings`. Replace the existing import block if needed:

```go
import (
	"fmt"
	"regexp"
	"strings"
)
```

- [ ] **Step 4: Update the nolint comment to reflect active Phase 3 kinds**

In [`spec/validate.go:22`](../../../spec/validate.go#L22), the existing nolint comment says "linear list of Phase-1 invariants". Extend it:

```go
//nolint:gocyclo // linear list of invariants (header, refs, phase-gated fields, skills + outputContract prefix checks, duplicates); splitting into helpers scatters the invariant set without reducing complexity.
```

- [ ] **Step 5: Run the new tests to verify they pass**

Run: `go test ./spec/ -run 'TestValidate_(Accepts|Rejects)(Skills|Skill|OutputContract)' -v`

Expected: PASS — all 6 tests green.

- [ ] **Step 6: Run the full spec test suite**

Run: `go test ./spec/... -v && go vet ./spec/...`

Expected: all existing tests still pass. `TestValidate_RejectsExtends` and `TestValidate_RejectsMCP` still pass (those phase-gates are untouched). `TestLoadOverlay_InvalidFixtures` (which exercises `phase_gated_skills.yaml` as an overlay-level rejection) still passes.

- [ ] **Step 7: Commit**

```bash
git add spec/validate.go spec/validate_test.go
git commit -m "$(cat <<'EOF'
feat(spec): unlock skills and outputContract validation gates

Phase 3 allows non-empty spec.skills[] and non-nil spec.outputContract.
Each skill ref must be prefixed with "skill." and the output contract
ref with "outputcontract." — prefix mismatch yields a targeted error
that points the author at the right namespace.

mcpImports remains phase-gated until Phase 4. Overlay body (AgentOverlayBody)
still omits skills/outputContract — overlays cannot introduce Phase-3
fields; only the base spec or an extends parent can. This keeps the
existing overlay phase_gated_skills fixture meaningful.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

## Expected state after this task

- `spec/validate.go`: skills and outputContract are no longer phase-gated; they run through `validateKindPrefixedRef` which enforces id-parse + kind prefix.
- `spec/validate_test.go`: 6 new tests replace the 2 old rejection tests.
- `go test ./spec/...` green; `go vet` clean.
- One commit added to the branch.
