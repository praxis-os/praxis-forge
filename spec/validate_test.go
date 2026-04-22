// SPDX-License-Identifier: Apache-2.0

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

func TestValidate_RejectsExtends(t *testing.T) {
	s := baseValidSpec()
	s.Extends = []string{"acme.base@1.0.0"}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "extends") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_AcceptsSkills(t *testing.T) {
	s := baseValidSpec()
	s.Skills = []ComponentRef{{Ref: "skill.structured-output@1.0.0"}}
	if err := s.Validate(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidate_RejectsSkillWrongPrefix(t *testing.T) {
	s := baseValidSpec()
	s.Skills = []ComponentRef{{Ref: "toolpack.http-get@1.0.0"}}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "skill.") || !strings.Contains(err.Error(), "must start with") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_RejectsSkillEmptyRef(t *testing.T) {
	s := baseValidSpec()
	s.Skills = []ComponentRef{{Ref: ""}}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "skills[0].ref") || !strings.Contains(err.Error(), "required") {
		t.Fatalf("err=%v", err)
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
	s.OutputContract = &ComponentRef{Ref: "contract.foo@1.0.0"}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "outputcontract.") || !strings.Contains(err.Error(), "must start with") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_RejectsOutputContractEmptyRef(t *testing.T) {
	s := baseValidSpec()
	s.OutputContract = &ComponentRef{Ref: ""}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "outputContract.ref") || !strings.Contains(err.Error(), "required") {
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
