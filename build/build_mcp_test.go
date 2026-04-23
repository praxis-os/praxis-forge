// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/praxis-os/praxis-forge/factories/credresolverenv"
	"github.com/praxis-os/praxis-forge/factories/mcpbinding"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
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

func buildFromFixture(t *testing.T, path string) (*BuiltAgent, error) {
	t.Helper()
	s, err := spec.LoadSpec(path)
	if err != nil {
		return nil, err
	}
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterCredentialResolver(credresolverenv.NewFactory("credresolver.env@1.0.0"))
	_ = r.RegisterMCPBinding(mcpbinding.NewFactory("mcp.binding@1.0.0"))
	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		return nil, err
	}
	return Build(context.Background(), ns, r)
}

func TestMCPFixtures_Success(t *testing.T) {
	cases := []string{
		"minimal-stdio",
		"stdio-with-env",
		"http-with-bearer",
		"http-with-apikey",
		"multiple-bindings",
		"on-new-tool-variants",
	}
	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			built, err := buildFromFixture(t, "../spec/testdata/mcp/"+name+"/spec.yaml")
			if err != nil {
				t.Fatalf("Build %s: %v", name, err)
			}
			built2, err := buildFromFixture(t, "../spec/testdata/mcp/"+name+"/spec.yaml")
			if err != nil {
				t.Fatalf("second Build %s: %v", name, err)
			}
			if built.Manifest.NormalizedHash != built2.Manifest.NormalizedHash {
				t.Errorf("NormalizedHash drift: %s vs %s",
					built.Manifest.NormalizedHash, built2.Manifest.NormalizedHash)
			}
			raw, err := json.Marshal(built.Manifest)
			if err != nil {
				t.Fatalf("marshal manifest: %v", err)
			}
			if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "password") {
				t.Errorf("manifest contains secret-looking field: %s", raw)
			}
		})
	}
}

func TestMCPFixtures_BuildErrors(t *testing.T) {
	cases := map[string]string{
		"unresolved-policy":     "mcp_unresolved_policy",
		"unresolved-credential": "mcp_unresolved_credential",
	}
	for name, marker := range cases {
		name, marker := name, marker
		t.Run(name, func(t *testing.T) {
			_, err := buildFromFixture(t, "../spec/testdata/mcp/"+name+"/spec.yaml")
			if err == nil || !strings.Contains(err.Error(), marker) {
				t.Fatalf("want marker %q in error, got %v", marker, err)
			}
		})
	}
}

func TestMCPFixtures_ValidateErrors(t *testing.T) {
	cases := map[string]string{
		"duplicate-binding-id":     "mcp_duplicate_id",
		"transport-field-mismatch": "mcp_transport_field_mismatch",
	}
	for name, marker := range cases {
		name, marker := name, marker
		t.Run(name, func(t *testing.T) {
			s, err := spec.LoadSpec("../spec/testdata/mcp/" + name + "/spec.yaml")
			if err != nil {
				t.Fatalf("LoadSpec: %v", err)
			}
			err = s.Validate()
			if err == nil || !strings.Contains(err.Error(), marker) {
				t.Fatalf("want marker %q in Validate error, got %v", marker, err)
			}
		})
	}
}
