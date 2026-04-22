// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis-forge/factories/mcpbinding"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

func TestBuild_Pipeline_StampsMCPBindingOnManifest(t *testing.T) {
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterMCPBinding(mcpbinding.NewFactory("mcp.binding@1.0.0"))

	s := &spec.AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "a.mcp", Version: "0.1.0"},
		Provider:   spec.ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: "prompt.sys@1.0.0"}},
		MCPImports: []spec.ComponentRef{{
			Ref: "mcp.binding@1.0.0",
			Config: map[string]any{
				"id":         "fs",
				"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
				"trust":      map[string]any{"tier": "low", "owner": "demo"},
			},
		}},
	}

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var found bool
	for _, rc := range built.Manifest.Resolved {
		if rc.Kind == string(registry.KindMCPBinding) {
			found = true
			if rc.ID != "mcp.binding@1.0.0" {
				t.Fatalf("ID=%q want mcp.binding@1.0.0", rc.ID)
			}
		}
	}
	if !found {
		t.Fatalf("no mcp_binding in Resolved: %+v", built.Manifest.Resolved)
	}
}
