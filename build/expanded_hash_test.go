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
