# Task 04 — Expanded hash

Compute a canonical JSON + SHA-256 over the **post-expansion** AgentSpec (the `ExpandedSpec.Spec`), so the manifest can carry a deterministic hash of what the build actually composed. The pre-expansion hash (`NormalizedSpec.NormalizedHash`, Phase 2b) stays untouched.

## Files

- Create: `build/expanded_hash.go`
- Create: `build/expanded_hash_test.go`

## Background

Phase 2b's `NormalizedHash` is computed on `NormalizedSpec.Spec` ([`spec/canonical.go:243-254`](../../../spec/canonical.go#L243-L254)). It answers "what did the author write (after extends + overlays)". Phase 3 adds an orthogonal question: "what did the build actually compose (after skill expansion)". Two logically-equivalent specs with different skill orderings or different combinations of user-declared vs. skill-injected refs may expand to the same effective composition — the `ExpandedHash` makes that visible.

We reuse the `AgentSpec` canonicalization path by constructing a temporary `spec.NormalizedSpec{Spec: expanded.Spec}` and calling its `CanonicalJSON` + `NormalizedHash` methods. This keeps encoding logic single-sourced in the `spec` package.

## Steps

- [ ] **Step 1: Write failing tests**

Create `build/expanded_hash_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"regexp"
	"testing"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

var expandedHashRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestComputeExpandedHash_Format(t *testing.T) {
	s := baseSpec()
	s.Skills = []spec.ComponentRef{{Ref: "skill.a@1.0.0"}}
	r := regWithSkill(t, "skill.a@1.0.0", registry.Skill{PromptFragment: "x"})

	es, err := expandSkills(context.Background(), s, r)
	if err != nil {
		t.Fatalf("expandSkills: %v", err)
	}
	hash, err := computeExpandedHash(es)
	if err != nil {
		t.Fatalf("computeExpandedHash: %v", err)
	}
	if !expandedHashRE.MatchString(hash) {
		t.Errorf("expanded hash not 64-char lower hex: %q", hash)
	}
}

func TestComputeExpandedHash_Stable(t *testing.T) {
	build := func() string {
		s := baseSpec()
		s.Skills = []spec.ComponentRef{{Ref: "skill.a@1.0.0"}}
		r := regWithSkill(t, "skill.a@1.0.0", registry.Skill{
			PromptFragment: "x",
			RequiredTools:  []registry.RequiredComponent{{ID: "toolpack.http-get@1.0.0"}},
		})
		es, err := expandSkills(context.Background(), s, r)
		if err != nil {
			t.Fatal(err)
		}
		h, err := computeExpandedHash(es)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	if build() != build() {
		t.Error("expanded hash not stable across two computations")
	}
}

func TestComputeExpandedHash_EquivalentCompositionsMatch(t *testing.T) {
	// Spec A: user declares the tool.
	sA := baseSpec()
	sA.Tools = []spec.ComponentRef{{Ref: "toolpack.http-get@1.0.0"}}
	sA.Skills = []spec.ComponentRef{{Ref: "skill.a@1.0.0"}}
	rA := regWithSkill(t, "skill.a@1.0.0", registry.Skill{
		PromptFragment: "x",
		RequiredTools:  []registry.RequiredComponent{{ID: "toolpack.http-get@1.0.0"}},
	})
	esA, err := expandSkills(context.Background(), sA, rA)
	if err != nil {
		t.Fatal(err)
	}
	hA, _ := computeExpandedHash(esA)

	// Spec B: only the skill, with the same requirement. Expansion auto-adds the tool.
	sB := baseSpec()
	sB.Skills = []spec.ComponentRef{{Ref: "skill.a@1.0.0"}}
	rB := regWithSkill(t, "skill.a@1.0.0", registry.Skill{
		PromptFragment: "x",
		RequiredTools:  []registry.RequiredComponent{{ID: "toolpack.http-get@1.0.0"}},
	})
	esB, err := expandSkills(context.Background(), sB, rB)
	if err != nil {
		t.Fatal(err)
	}
	hB, _ := computeExpandedHash(esB)

	if hA != hB {
		t.Errorf("equivalent compositions should hash equal:\n  A=%s\n  B=%s", hA, hB)
	}
}

func TestComputeExpandedHash_ChangesOnDifferentComposition(t *testing.T) {
	s1 := baseSpec()
	s1.Skills = []spec.ComponentRef{{Ref: "skill.a@1.0.0"}}
	r1 := regWithSkill(t, "skill.a@1.0.0", registry.Skill{PromptFragment: "x"})
	es1, _ := expandSkills(context.Background(), s1, r1)
	h1, _ := computeExpandedHash(es1)

	s2 := baseSpec()
	s2.Skills = []spec.ComponentRef{{Ref: "skill.b@1.0.0"}}
	r2 := regWithSkill(t, "skill.b@1.0.0", registry.Skill{
		PromptFragment: "x",
		RequiredTools:  []registry.RequiredComponent{{ID: "toolpack.http-get@1.0.0"}},
	})
	es2, _ := expandSkills(context.Background(), s2, r2)
	h2, _ := computeExpandedHash(es2)

	if h1 == h2 {
		t.Errorf("different compositions should hash differently:\n  %s vs %s", h1, h2)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./build/ -run TestComputeExpandedHash -v`

Expected: FAIL — `undefined: computeExpandedHash`.

- [ ] **Step 3: Implement computeExpandedHash**

Create `build/expanded_hash.go`:

```go
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"github.com/praxis-os/praxis-forge/spec"
)

// computeExpandedHash returns the SHA-256 of the canonical JSON encoding
// of the post-expansion AgentSpec. It reuses spec.NormalizedSpec's
// canonical encoder (single-sourced in the spec package) by wrapping
// the expanded AgentSpec in a throwaway NormalizedSpec and calling the
// memoized accessor.
//
// Semantics (design spec §"Manifest additions"):
//   - hash covers es.Spec only (skills are spec-level; their contributions
//     already rolled into Tools/Policies/OutputContract);
//   - stable across equivalent compositions (two specs that expand to
//     identical AgentSpec produce identical ExpandedHash);
//   - the value is a lowercase 64-char hex string.
func computeExpandedHash(es *ExpandedSpec) (string, error) {
	tmp := &spec.NormalizedSpec{Spec: es.Spec}
	return tmp.NormalizedHash()
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./build/ -run TestComputeExpandedHash -v`

Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add build/expanded_hash.go build/expanded_hash_test.go
git commit -m "$(cat <<'EOF'
feat(build): computeExpandedHash over post-expansion AgentSpec

A second hash that answers "what did the build compose" vs Phase 2b's
NormalizedHash which answers "what did the author write". Reuses
spec.NormalizedSpec.CanonicalJSON by wrapping the expanded AgentSpec —
single-source canonical encoder, no duplicate logic.

Tests cover format, stability, equivalent-composition match, and
different-composition divergence.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

## Expected state after this task

- `build/expanded_hash.go`: `computeExpandedHash(es *ExpandedSpec) (string, error)`.
- `build/expanded_hash_test.go`: 4 tests covering format, stability, equivalence, divergence.
- One commit added.
