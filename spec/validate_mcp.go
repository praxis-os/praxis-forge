// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"path/filepath"
)

// validateMCPImportsStructure checks the typed-map shape of each
// spec.mcpImports[].config. It operates on the decoded map rather than
// a typed struct because AgentSpec.MCPImports is []ComponentRef for
// uniformity with other factory kinds. The factory's own Build method
// re-validates as defense in depth (factories/mcpbinding).
func validateMCPImportsStructure(errs *Errors, imports []ComponentRef) {
	seenID := map[string]int{}
	for i, ref := range imports {
		field := fmt.Sprintf("mcpImports[%d].config", i)
		cfg := ref.Config
		if cfg == nil {
			errs.Addf("%s: mcp_missing_id: config required", field)
			continue
		}

		id := stringAt(cfg, "id")
		if id == "" {
			errs.Addf("%s.id: mcp_missing_id", field)
		} else if prev, dup := seenID[id]; dup {
			errs.Addf("%s.id %q: mcp_duplicate_id of mcpImports[%d]", field, id, prev)
		} else {
			seenID[id] = i
		}

		validateMCPConnection(errs, field+".connection", mapAt(cfg, "connection"))
		validateMCPOnNewTool(errs, field+".onNewTool", stringAt(cfg, "onNewTool"))
		validateMCPAuth(errs, field+".auth", mapAt(cfg, "auth"))
		validateGlobList(errs, field+".allow", anyListAt(cfg, "allow"))
		validateGlobList(errs, field+".deny", anyListAt(cfg, "deny"))
	}
}

func validateMCPConnection(errs *Errors, field string, conn map[string]any) {
	if conn == nil {
		errs.Addf("%s: mcp_transport_invalid: connection required", field)
		return
	}
	transport := stringAt(conn, "transport")
	switch transport {
	case "stdio":
		if len(anyListAt(conn, "command")) == 0 {
			errs.Addf("%s: mcp_transport_field_mismatch: stdio requires command", field)
		}
		if stringAt(conn, "url") != "" {
			errs.Addf("%s: mcp_transport_field_mismatch: stdio must not set url", field)
		}
	case "http":
		if stringAt(conn, "url") == "" {
			errs.Addf("%s: mcp_transport_field_mismatch: http requires url", field)
		}
		if len(anyListAt(conn, "command")) > 0 {
			errs.Addf("%s: mcp_transport_field_mismatch: http must not set command", field)
		}
	default:
		errs.Addf("%s.transport %q: mcp_transport_invalid (want stdio|http)", field, transport)
	}
}

func validateMCPOnNewTool(errs *Errors, field, v string) {
	if v == "" {
		return
	}
	switch v {
	case "block", "allow-if-match-allowlist", "require-reapproval":
		return
	default:
		errs.Addf("%s %q: mcp_on_new_tool_invalid (want block|allow-if-match-allowlist|require-reapproval)", field, v)
	}
}

func validateMCPAuth(errs *Errors, field string, auth map[string]any) {
	if auth == nil {
		return
	}
	if stringAt(auth, "credentialRef") == "" {
		errs.Addf("%s.credentialRef: mcp_missing_credential_ref", field)
	}
	scheme := stringAt(auth, "scheme")
	if scheme == "" {
		errs.Addf("%s.scheme: mcp_missing_auth_scheme", field)
	}
	if scheme == "api-key" && stringAt(auth, "headerName") == "" {
		errs.Addf("%s.headerName: mcp_missing_header_name (required for scheme api-key)", field)
	}
}

func validateGlobList(errs *Errors, field string, patterns []any) {
	for i, p := range patterns {
		s, ok := p.(string)
		if !ok {
			errs.Addf("%s[%d]: mcp_invalid_glob: want string, got %T", field, i, p)
			continue
		}
		if _, err := filepath.Match(s, ""); err != nil {
			errs.Addf("%s[%d] %q: mcp_invalid_glob: %s", field, i, s, err.Error())
		}
	}
}

// --- small helpers ---

func stringAt(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

func mapAt(m map[string]any, k string) map[string]any {
	v, _ := m[k].(map[string]any)
	return v
}

func anyListAt(m map[string]any, k string) []any {
	switch v := m[k].(type) {
	case []any:
		return v
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out
	default:
		return nil
	}
}
