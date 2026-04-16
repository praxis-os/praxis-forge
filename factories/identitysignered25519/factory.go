// SPDX-License-Identifier: Apache-2.0

// Package identitysignered25519 wraps praxis identity.NewEd25519Signer. The
// private key is supplied at factory construction time; spec config carries
// the issuer and token lifetime.
package identitysignered25519

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/identity"
)

type Factory struct {
	id   registry.ID
	priv ed25519.PrivateKey
}

func NewFactory(id registry.ID, priv ed25519.PrivateKey) *Factory {
	return &Factory{id: id, priv: priv}
}

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "Ed25519 JWT identity signer" }

type config struct {
	Issuer   string
	Lifetime time.Duration
}

func decode(cfg map[string]any) (config, error) {
	var c config
	iss, ok := cfg["issuer"].(string)
	if !ok || iss == "" {
		return c, fmt.Errorf("issuer: required string")
	}
	c.Issuer = iss

	lifetime, err := toInt(cfg["tokenLifetimeSeconds"])
	if err != nil {
		return c, fmt.Errorf("tokenLifetimeSeconds: %w", err)
	}
	if lifetime < 5 || lifetime > 300 {
		return c, fmt.Errorf("tokenLifetimeSeconds: want 5..300, got %d", lifetime)
	}
	c.Lifetime = time.Duration(lifetime) * time.Second
	return c, nil
}

func toInt(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		return int64(x), nil
	default:
		return 0, fmt.Errorf("want integer, got %T", v)
	}
}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (identity.Signer, error) {
	c, err := decode(cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.id, err)
	}
	signer, err := identity.NewEd25519Signer(
		f.priv,
		identity.WithIssuer(c.Issuer),
		identity.WithTokenLifetime(c.Lifetime),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.id, err)
	}
	return signer, nil
}
