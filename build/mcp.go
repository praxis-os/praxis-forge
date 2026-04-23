// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

// ResolvedMCPBinding captures one built MCP binding plus the authoring
// ComponentRef so buildManifest can emit the full config verbatim.
type ResolvedMCPBinding struct {
	ID     registry.ID
	Config map[string]any
	Value  registry.MCPBinding
}

const (
	errCodeMCPUnresolvedFactory    = "mcp_unresolved_factory"
	errCodeMCPUnresolvedPolicy     = "mcp_unresolved_policy"
	errCodeMCPUnresolvedCredential = "mcp_unresolved_credential"
)

// resolveMCPBindings walks spec.mcpImports, resolves each binding's
// factory, builds it, and cross-validates that every Policy and Auth
// credential reference already exists in the component registry. It
// never touches the network: runtime (praxis) owns MCP I/O.
func resolveMCPBindings(
	ctx context.Context,
	s *spec.AgentSpec,
	r *registry.ComponentRegistry,
) ([]ResolvedMCPBinding, error) {
	if len(s.MCPImports) == 0 {
		return nil, nil
	}
	out := make([]ResolvedMCPBinding, 0, len(s.MCPImports))
	for i, ref := range s.MCPImports {
		fac, err := r.MCPBinding(registry.ID(ref.Ref))
		if err != nil {
			return nil, fmt.Errorf("%s: mcpImports[%d] %s: %w", errCodeMCPUnresolvedFactory, i, ref.Ref, err)
		}
		val, err := fac.Build(ctx, ref.Config)
		if err != nil {
			return nil, fmt.Errorf("build mcpImports[%d] %s: %w", i, ref.Ref, err)
		}
		for j, pid := range val.Policies {
			if _, err := r.PolicyPack(pid); err != nil {
				return nil, fmt.Errorf("%s: mcpImports[%d] %s: policies[%d] %s: %w",
					errCodeMCPUnresolvedPolicy, i, ref.Ref, j, pid, err)
			}
		}
		if val.Auth != nil {
			if _, err := r.CredentialResolver(val.Auth.CredentialRef); err != nil {
				return nil, fmt.Errorf("%s: mcpImports[%d] %s: auth.credentialRef %s: %w",
					errCodeMCPUnresolvedCredential, i, ref.Ref, val.Auth.CredentialRef, err)
			}
		}
		out = append(out, ResolvedMCPBinding{
			ID:     registry.ID(ref.Ref),
			Config: ref.Config,
			Value:  val,
		})
	}
	return out, nil
}
