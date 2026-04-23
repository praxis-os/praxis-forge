// SPDX-License-Identifier: Apache-2.0

package mcpbinding

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis-forge/registry"
)

const factoryID registry.ID = "mcp.binding@1.0.0"

func TestFactory_Stdio_Minimal(t *testing.T) {
	b, err := NewFactory(factoryID).Build(context.Background(), map[string]any{
		"id": "fs",
		"connection": map[string]any{
			"transport": "stdio",
			"command":   []any{"/bin/true"},
		},
		"trust": map[string]any{"tier": "low", "owner": "demo"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if b.ID != "fs" || b.Connection.Transport != registry.MCPTransportStdio {
		t.Fatalf("unexpected binding: %+v", b)
	}
	if b.OnNewTool != registry.OnNewToolBlock {
		t.Fatalf("OnNewTool=%q, want block", b.OnNewTool)
	}
}

func TestFactory_HTTPWithBearer(t *testing.T) {
	b, err := NewFactory(factoryID).Build(context.Background(), map[string]any{
		"id": "notion",
		"connection": map[string]any{
			"transport": "http",
			"url":       "https://api.example.com/mcp",
		},
		"auth": map[string]any{
			"credentialRef": "credresolver.env@1.0.0",
			"scheme":        "bearer",
		},
		"trust":    map[string]any{"tier": "medium", "owner": "platform"},
		"policies": []any{"policypack.pii-redaction@1.0.0"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if b.Auth == nil || b.Auth.CredentialRef != "credresolver.env@1.0.0" || b.Auth.Scheme != "bearer" {
		t.Fatalf("Auth=%+v", b.Auth)
	}
	if len(b.Policies) != 1 || b.Policies[0] != "policypack.pii-redaction@1.0.0" {
		t.Fatalf("Policies=%+v", b.Policies)
	}
}

func TestFactory_RejectsTransportFieldMismatch(t *testing.T) {
	_, err := NewFactory(factoryID).Build(context.Background(), map[string]any{
		"id": "fs",
		"connection": map[string]any{
			"transport": "stdio",
			"url":       "https://wrong",
		},
		"trust": map[string]any{"tier": "low", "owner": "demo"},
	})
	if err == nil || !strings.Contains(err.Error(), "transport") {
		t.Fatalf("want transport mismatch, got %v", err)
	}
}

func TestFactory_RejectsMissingID(t *testing.T) {
	_, err := NewFactory(factoryID).Build(context.Background(), map[string]any{
		"connection": map[string]any{"transport": "stdio", "command": []any{"/bin/true"}},
		"trust":      map[string]any{"tier": "low", "owner": "demo"},
	})
	if err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("want id error, got %v", err)
	}
}
