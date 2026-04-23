// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"context"
	"testing"
)

func TestMergeChain_MCPImportsPropagatesFromParent(t *testing.T) {
	parent := &AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   Metadata{ID: "parent", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
		MCPImports: []ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "parent"},
		}}},
	}
	child := &AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   Metadata{ID: "child", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
		Extends:    []string{"parent@0.1.0"},
	}
	store := MapSpecStore{"parent@0.1.0": parent}
	ns, err := Normalize(context.Background(), child, nil, store)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if len(ns.Spec.MCPImports) != 1 || ns.Spec.MCPImports[0].Ref != "mcp.binding@1.0.0" {
		t.Fatalf("MCPImports not propagated: %+v", ns.Spec.MCPImports)
	}
}

func TestMergeChain_MCPImportsChildWins(t *testing.T) {
	parent := &AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   Metadata{ID: "parent", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
		MCPImports: []ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "parent-fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "parent"},
		}}},
	}
	child := &AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   Metadata{ID: "child", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
		Extends:    []string{"parent@0.1.0"},
		MCPImports: []ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "child-fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "child"},
		}}},
	}
	store := MapSpecStore{"parent@0.1.0": parent}
	ns, err := Normalize(context.Background(), child, nil, store)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if len(ns.Spec.MCPImports) != 1 || ns.Spec.MCPImports[0].Config["id"] != "child-fs" { //nolint:forcetypeassert
		t.Fatalf("child did not win: %+v", ns.Spec.MCPImports)
	}
}
