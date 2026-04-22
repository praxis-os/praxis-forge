// SPDX-License-Identifier: Apache-2.0

// Package mcpbinding is the Phase-4 vertical-slice MCP binding factory.
// It decodes a typed config describing an MCP server connection, allow
// / deny policy, attached forge policy packs, trust metadata, and
// on-new-tool behaviour, and produces a registry.MCPBinding value.
// Build is pure: no network, no subprocess. Runtime (praxis) opens the
// session, lists tools live, applies Allow/Deny + Policies, and enforces
// OnNewTool when the server surface changes.
package mcpbinding

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/praxis-os/praxis-forge/registry"
)

// Factory is the generic MCP binding factory.
type Factory struct{ id registry.ID }

// NewFactory constructs the generic MCP binding factory.
func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "generic MCP binding contract (stdio or streamable-HTTP)" }

// Build decodes cfg into registry.MCPBinding, re-validating as defense in depth.
func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.MCPBinding, error) {
	if cfg == nil {
		return registry.MCPBinding{}, fmt.Errorf("%s: config required", f.id)
	}

	id, _ := cfg["id"].(string)
	if id == "" {
		return registry.MCPBinding{}, fmt.Errorf("%s: id: required", f.id)
	}

	conn, err := decodeConnection(f.id, cfg["connection"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	auth, err := decodeAuth(f.id, cfg["auth"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	allow, err := decodeGlobList(f.id, "allow", cfg["allow"])
	if err != nil {
		return registry.MCPBinding{}, err
	}
	deny, err := decodeGlobList(f.id, "deny", cfg["deny"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	policies, err := decodePolicyIDs(f.id, cfg["policies"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	trust, err := decodeTrust(f.id, cfg["trust"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	onNewTool, err := decodeOnNewTool(f.id, cfg["onNewTool"])
	if err != nil {
		return registry.MCPBinding{}, err
	}

	return registry.MCPBinding{
		ID:         id,
		Connection: conn,
		Auth:       auth,
		Allow:      allow,
		Deny:       deny,
		Policies:   policies,
		Trust:      trust,
		OnNewTool:  onNewTool,
		Descriptor: registry.MCPBindingDescriptor{
			Name:    stringOr(cfg, "name", "generic-mcp-binding"),
			Summary: stringOr(cfg, "summary", "generic MCP binding contract"),
			Tags:    stringListOr(cfg, "tags"),
		},
	}, nil
}

func decodeConnection(id registry.ID, raw any) (registry.MCPConnection, error) {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return registry.MCPConnection{}, fmt.Errorf("%s: connection: required", id)
	}
	transport, _ := m["transport"].(string)
	switch transport {
	case "stdio":
		cmd, err := stringList(m["command"])
		if err != nil || len(cmd) == 0 {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.command: stdio transport requires non-empty command", id)
		}
		if s, _ := m["url"].(string); s != "" {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.url: stdio transport must not set url", id)
		}
		env, err := stringMap(m["env"])
		if err != nil {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.env: %w", id, err)
		}
		return registry.MCPConnection{Transport: registry.MCPTransportStdio, Command: cmd, Env: env}, nil
	case "http":
		url, _ := m["url"].(string)
		if url == "" {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.url: http transport requires url", id)
		}
		if cmd, _ := stringList(m["command"]); len(cmd) > 0 {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.command: http transport must not set command", id)
		}
		headers, err := stringMap(m["headers"])
		if err != nil {
			return registry.MCPConnection{}, fmt.Errorf("%s: connection.headers: %w", id, err)
		}
		return registry.MCPConnection{Transport: registry.MCPTransportHTTP, URL: url, Headers: headers}, nil
	default:
		return registry.MCPConnection{}, fmt.Errorf("%s: connection.transport %q: want stdio|http", id, transport)
	}
}

func decodeAuth(id registry.ID, raw any) (*registry.MCPAuth, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: auth: want map, got %T", id, raw)
	}
	credRef, _ := m["credentialRef"].(string)
	if credRef == "" {
		return nil, fmt.Errorf("%s: auth.credentialRef: required", id)
	}
	scheme, _ := m["scheme"].(string)
	if scheme == "" {
		return nil, fmt.Errorf("%s: auth.scheme: required", id)
	}
	headerName, _ := m["headerName"].(string)
	if scheme == "api-key" && headerName == "" {
		return nil, fmt.Errorf("%s: auth.headerName: required when scheme == api-key", id)
	}
	parsed, err := registry.ParseID(credRef)
	if err != nil {
		return nil, fmt.Errorf("%s: auth.credentialRef: %w", id, err)
	}
	return &registry.MCPAuth{CredentialRef: parsed, Scheme: scheme, HeaderName: headerName}, nil
}

func decodeGlobList(id registry.ID, field string, raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	list, err := stringList(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", id, field, err)
	}
	for i, p := range list {
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("%s: %s[%d] %q: %w", id, field, i, p, err)
		}
	}
	return list, nil
}

func decodePolicyIDs(id registry.ID, raw any) ([]registry.ID, error) {
	if raw == nil {
		return nil, nil
	}
	list, err := stringList(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: policies: %w", id, err)
	}
	out := make([]registry.ID, 0, len(list))
	for i, s := range list {
		parsed, err := registry.ParseID(s)
		if err != nil {
			return nil, fmt.Errorf("%s: policies[%d] %q: %w", id, i, s, err)
		}
		out = append(out, parsed)
	}
	return out, nil
}

func decodeTrust(id registry.ID, raw any) (registry.MCPTrust, error) {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return registry.MCPTrust{}, fmt.Errorf("%s: trust: required", id)
	}
	tier, _ := m["tier"].(string)
	owner, _ := m["owner"].(string)
	if tier == "" || owner == "" {
		return registry.MCPTrust{}, fmt.Errorf("%s: trust.tier and trust.owner: required", id)
	}
	tags, err := stringList(m["tags"])
	if err != nil {
		return registry.MCPTrust{}, fmt.Errorf("%s: trust.tags: %w", id, err)
	}
	return registry.MCPTrust{Tier: tier, Owner: owner, Tags: tags}, nil
}

func decodeOnNewTool(id registry.ID, raw any) (registry.OnNewToolPolicy, error) {
	s, ok := raw.(string)
	if !ok || s == "" {
		return registry.OnNewToolBlock, nil
	}
	switch s {
	case "block":
		return registry.OnNewToolBlock, nil
	case "allow-if-match-allowlist":
		return registry.OnNewToolAllowIfMatch, nil
	case "require-reapproval":
		return registry.OnNewToolRequireReapprove, nil
	default:
		return "", fmt.Errorf("%s: onNewTool %q: want block|allow-if-match-allowlist|require-reapproval", id, s)
	}
}

func stringList(raw any) ([]string, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case []any:
		out := make([]string, 0, len(v))
		for i, el := range v {
			s, ok := el.(string)
			if !ok {
				return nil, fmt.Errorf("[%d]: want string, got %T", i, el)
			}
			out = append(out, s)
		}
		return out, nil
	case []string:
		return append([]string(nil), v...), nil
	default:
		return nil, fmt.Errorf("want []string or []any, got %T", raw)
	}
}

func stringMap(raw any) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("want map[string]any, got %T", raw)
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%q: want string, got %T", k, v)
		}
		out[k] = s
	}
	return out, nil
}

func stringOr(m map[string]any, key, fallback string) string {
	if s, ok := m[key].(string); ok && s != "" {
		return s
	}
	return fallback
}

func stringListOr(m map[string]any, key string) []string {
	list, _ := stringList(m[key])
	return list
}
