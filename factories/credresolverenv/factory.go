// SPDX-License-Identifier: Apache-2.0

// Package credresolverenv provides a credential resolver factory that reads
// scope-named secrets from environment variables. Scope "net:http" maps to
// FORGE_CRED_NET_HTTP. Colons and dashes become underscores; letters become
// uppercase. Intended for dev and tests; production deployments should use a
// real secret store resolver.
package credresolverenv

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/credentials"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "env-var credential resolver" }

func (f *Factory) Build(_ context.Context, _ map[string]any) (credentials.Resolver, error) {
	return &resolver{}, nil
}

type resolver struct{}

func (r *resolver) Fetch(_ context.Context, scope string) (credentials.Credential, error) {
	envName := scopeToEnv(scope)
	v := os.Getenv(envName)
	if v == "" {
		return credentials.Credential{}, fmt.Errorf("credresolver.env: %s not set", envName)
	}
	return credentials.Credential{Value: []byte(v)}, nil
}

func scopeToEnv(scope string) string {
	s := strings.ToUpper(scope)
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return "FORGE_CRED_" + s
}
