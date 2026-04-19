// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"testing"
)

func TestMergeChain_NoParents(t *testing.T) {
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "my.agent", Version: "1.0.0"},
		Provider:   ComponentRef{Ref: "my.provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "my.system"}},
	}
	var prov provenanceFields
	result := mergeChain(nil, base, &prov)

	if result.APIVersion != base.APIVersion {
		t.Errorf("apiVersion: want %q, got %q", base.APIVersion, result.APIVersion)
	}
	if result.Metadata.ID != base.Metadata.ID {
		t.Errorf("id: want %q, got %q", base.Metadata.ID, result.Metadata.ID)
	}
	if prov.Provider.Role != RoleBase {
		t.Errorf("provider role: want RoleBase, got %v", prov.Provider.Role)
	}
}

func TestMergeChain_ExtendsIsCleared(t *testing.T) {
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "my.agent", Version: "1.0.0"},
		Provider:   ComponentRef{Ref: "provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt"}},
		Extends:    []string{"some.parent"},
	}
	var prov provenanceFields
	result := mergeChain(nil, base, &prov)

	if len(result.Extends) != 0 {
		t.Errorf("Extends should be cleared, got %v", result.Extends)
	}
}

func TestMergeChain_ChildWinsOverParent(t *testing.T) {
	parent := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "parent.agent", Version: "1.0.0"},
		Provider:   ComponentRef{Ref: "parent.provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "parent.prompt"}},
		Tools:      []ComponentRef{{Ref: "parent.tool"}},
	}
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "base.agent", Version: "2.0.0"},
		Provider:   ComponentRef{Ref: "base.provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "base.prompt"}},
		Tools:      []ComponentRef{{Ref: "base.tool"}},
	}
	var prov provenanceFields
	result := mergeChain([]*AgentSpec{parent}, base, &prov)

	// Base should win on all fields.
	if result.Metadata.ID != base.Metadata.ID {
		t.Errorf("id: want %q, got %q", base.Metadata.ID, result.Metadata.ID)
	}
	if result.Metadata.Version != base.Metadata.Version {
		t.Errorf("version: want %q, got %q", base.Metadata.Version, result.Metadata.Version)
	}
	if result.Provider.Ref != base.Provider.Ref {
		t.Errorf("provider: want %q, got %q", base.Provider.Ref, result.Provider.Ref)
	}
	if len(result.Tools) != 1 || result.Tools[0].Ref != base.Tools[0].Ref {
		t.Errorf("tools: want [%q], got %+v", base.Tools[0].Ref, result.Tools)
	}

	// Provenance should show base as the source.
	if prov.Provider.Role != RoleBase {
		t.Errorf("provider provenance: want RoleBase, got %v", prov.Provider.Role)
	}
}

func TestMergeChain_LinearChainProvenance(t *testing.T) {
	grandparent := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "grandparent", Version: "1.0.0"},
		Provider:   ComponentRef{Ref: "gp.provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "gp.prompt"}},
	}
	parent := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "parent", Version: "1.0.0"},
		Provider:   ComponentRef{Ref: "p.provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "p.prompt"}},
		Tools:      []ComponentRef{{Ref: "p.tool"}},
	}
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "base", Version: "1.0.0"},
		Provider:   ComponentRef{Ref: "b.provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "b.prompt"}},
	}
	var prov provenanceFields
	_ = mergeChain([]*AgentSpec{grandparent, parent}, base, &prov)

	// Base should win on Provider.
	if prov.Provider.Role != RoleBase {
		t.Errorf("provider provenance: want RoleBase, got %v", prov.Provider.Role)
	}
	// Parent should win on Tools.
	if prov.Tools.Role != RoleParent || prov.Tools.Step != 1 {
		t.Errorf("tools provenance: want RoleParent Step=1, got %v Step=%d", prov.Tools.Role, prov.Tools.Step)
	}
}

func TestMergeChain_ParentStepNumbers(t *testing.T) {
	// Create a 3-level chain and verify step numbers are correct.
	grandparent := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "gp", Version: "1.0.0"},
		Provider:   ComponentRef{Ref: "gp.provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt"}},
	}
	parent := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "parent", Version: "1.0.0"},
		Provider:   ComponentRef{Ref: "parent.provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt"}},
	}
	base := &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "base", Version: "1.0.0"},
		Provider:   ComponentRef{Ref: "base.provider"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt"}},
	}

	var prov provenanceFields
	_ = mergeChain([]*AgentSpec{grandparent, parent}, base, &prov)

	// Provider comes from base, so Step should be 0.
	if prov.Provider.Step != 0 {
		t.Errorf("provider step: want 0, got %d", prov.Provider.Step)
	}
}

func TestMergeOne_FiltersReplaceWhole(t *testing.T) {
	merged := &AgentSpec{}
	child := &AgentSpec{
		Filters: FilterBlock{
			PreLLM:  []ComponentRef{{Ref: "filter1"}},
			PreTool: []ComponentRef{{Ref: "filter2"}},
		},
	}
	var prov provenanceFields
	mergeOne(merged, child, Provenance{Role: RoleBase}, &prov)

	if len(merged.Filters.PreLLM) != 1 || merged.Filters.PreLLM[0].Ref != "filter1" {
		t.Errorf("filters.preLLM: want [filter1], got %+v", merged.Filters.PreLLM)
	}
	if len(merged.Filters.PreTool) != 1 || merged.Filters.PreTool[0].Ref != "filter2" {
		t.Errorf("filters.preTool: want [filter2], got %+v", merged.Filters.PreTool)
	}
}

func TestMergeMetadata_ScalarFields(t *testing.T) {
	merged := Metadata{
		ID:          "merged.id",
		Version:     "1.0.0",
		DisplayName: "Original Name",
		Description: "Original Description",
	}
	child := Metadata{
		DisplayName: "New Name",
		Description: "New Description",
	}
	result := mergeMetadata(merged, child)

	// ID and Version should come from merged (unchanged).
	if result.ID != "merged.id" {
		t.Errorf("id: want merged.id, got %q", result.ID)
	}
	if result.Version != "1.0.0" {
		t.Errorf("version: want 1.0.0, got %q", result.Version)
	}
	// DisplayName and Description should come from child.
	if result.DisplayName != "New Name" {
		t.Errorf("displayName: want New Name, got %q", result.DisplayName)
	}
	if result.Description != "New Description" {
		t.Errorf("description: want New Description, got %q", result.Description)
	}
}

func TestMergeMetadata_LabelsReplace(t *testing.T) {
	merged := Metadata{
		Labels: map[string]string{"old": "label"},
	}
	child := Metadata{
		Labels: map[string]string{"new": "label"},
	}
	result := mergeMetadata(merged, child)

	// Labels should be completely replaced.
	if len(result.Labels) != 1 || result.Labels["new"] != "label" {
		t.Errorf("labels: want {new: label}, got %+v", result.Labels)
	}
	if _, ok := result.Labels["old"]; ok {
		t.Errorf("labels: should not contain old key")
	}
}

func TestScalarString(t *testing.T) {
	if scalarString("parent", "child") != "child" {
		t.Errorf("child non-empty should win")
	}
	if scalarString("parent", "") != "parent" {
		t.Errorf("parent should win when child empty")
	}
}

func TestPointerStruct(t *testing.T) {
	childRef := &ComponentRef{Ref: "child"}
	parentRef := &ComponentRef{Ref: "parent"}

	result := pointerStruct(parentRef, childRef)
	if result != childRef {
		t.Errorf("child non-nil should win")
	}

	result = pointerStruct(parentRef, nil)
	if result != parentRef {
		t.Errorf("parent should win when child nil")
	}
}

func TestSliceReplaceIfSet(t *testing.T) {
	parent := []ComponentRef{{Ref: "parent"}}
	child := []ComponentRef{{Ref: "child"}}

	result := sliceReplaceIfSet(parent, child)
	if len(result) != 1 || result[0].Ref != "child" {
		t.Errorf("child non-nil should win")
	}

	result = sliceReplaceIfSet(parent, nil)
	if len(result) != 1 || result[0].Ref != "parent" {
		t.Errorf("parent should win when child nil")
	}
}

func TestMapStringStringReplace(t *testing.T) {
	const labelValue = "label"
	parent := map[string]string{"parent": labelValue}
	child := map[string]string{"child": labelValue}

	result := mapStringStringReplace(parent, child)
	if len(result) != 1 || result["child"] != labelValue {
		t.Errorf("child non-nil should win")
	}

	result = mapStringStringReplace(parent, nil)
	if len(result) != 1 || result["parent"] != labelValue {
		t.Errorf("parent should win when child nil")
	}
}

func TestIsZeroMetadata(t *testing.T) {
	if !isZeroMetadata(Metadata{}) {
		t.Errorf("empty metadata should be zero")
	}
	if isZeroMetadata(Metadata{ID: "test"}) {
		t.Errorf("metadata with ID should not be zero")
	}
	if isZeroMetadata(Metadata{DisplayName: "test"}) {
		t.Errorf("metadata with DisplayName should not be zero")
	}
}
