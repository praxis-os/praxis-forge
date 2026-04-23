// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis-forge/factories/credresolverenv"
	"github.com/praxis-os/praxis-forge/factories/mcpbinding"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

func baseMCPRegistry(t *testing.T) *registry.ComponentRegistry {
	t.Helper()
	r := registry.NewComponentRegistry()
	_ = r.RegisterMCPBinding(mcpbinding.NewFactory("mcp.binding@1.0.0"))
	_ = r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0"))
	_ = r.RegisterCredentialResolver(credresolverenv.NewFactory("credresolver.env@1.0.0"))
	return r
}

func TestResolveMCPBindings_Happy_StdioNoAuth(t *testing.T) {
	s := &spec.AgentSpec{
		MCPImports: []spec.ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "demo"},
			"policies":   []any{"policypack.pii-redaction@1.0.0"},
		}}},
	}
	r := baseMCPRegistry(t)

	res, err := resolveMCPBindings(context.Background(), s, r)
	if err != nil {
		t.Fatalf("resolveMCPBindings: %v", err)
	}
	if len(res) != 1 || res[0].Value.ID != "fs" {
		t.Fatalf("res=%+v", res)
	}
	if len(res[0].Value.Policies) != 1 {
		t.Fatalf("Policies=%+v", res[0].Value.Policies)
	}
}

func TestResolveMCPBindings_UnresolvedPolicy(t *testing.T) {
	s := &spec.AgentSpec{
		MCPImports: []spec.ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "fs",
			"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
			"trust":      map[string]any{"tier": "low", "owner": "demo"},
			"policies":   []any{"policypack.missing@9.9.9"},
		}}},
	}
	r := baseMCPRegistry(t)

	_, err := resolveMCPBindings(context.Background(), s, r)
	if err == nil || !strings.Contains(err.Error(), "mcp_unresolved_policy") {
		t.Fatalf("want mcp_unresolved_policy, got %v", err)
	}
}

func TestResolveMCPBindings_UnresolvedCredential(t *testing.T) {
	s := &spec.AgentSpec{
		MCPImports: []spec.ComponentRef{{Ref: "mcp.binding@1.0.0", Config: map[string]any{
			"id":         "notion",
			"connection": map[string]any{"transport": "http", "url": "https://e/mcp"},
			"trust":      map[string]any{"tier": "medium", "owner": "demo"},
			"auth":       map[string]any{"credentialRef": "credresolver.missing@9.9.9", "scheme": "bearer"},
		}}},
	}
	r := baseMCPRegistry(t)

	_, err := resolveMCPBindings(context.Background(), s, r)
	if err == nil || !strings.Contains(err.Error(), "mcp_unresolved_credential") {
		t.Fatalf("want mcp_unresolved_credential, got %v", err)
	}
}

func TestResolveMCPBindings_FactoryMissing(t *testing.T) {
	s := &spec.AgentSpec{
		MCPImports: []spec.ComponentRef{{Ref: "mcp.notregistered@1.0.0", Config: map[string]any{}}},
	}
	r := baseMCPRegistry(t)

	_, err := resolveMCPBindings(context.Background(), s, r)
	if err == nil || !strings.Contains(err.Error(), "mcp_unresolved_factory") {
		t.Fatalf("want mcp_unresolved_factory, got %v", err)
	}
}

func TestResolveMCPBindings_EmptyImportsReturnsNil(t *testing.T) {
	s := &spec.AgentSpec{}
	r := baseMCPRegistry(t)

	res, err := resolveMCPBindings(context.Background(), s, r)
	if err != nil {
		t.Fatalf("resolveMCPBindings: %v", err)
	}
	if res != nil {
		t.Fatalf("res=%+v, want nil", res)
	}
}
