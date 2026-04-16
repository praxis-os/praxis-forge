// SPDX-License-Identifier: Apache-2.0

package identitysignered25519

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestFactory_Signs(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := NewFactory("identitysigner.ed25519@1.0.0", priv)
	s, err := f.Build(context.Background(), map[string]any{
		"issuer":               "test",
		"tokenLifetimeSeconds": 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	tok, err := s.Sign(context.Background(), map[string]any{"sub": "agent-x"})
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}
}

func TestFactory_RejectsInvalidLifetime(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := NewFactory("identitysigner.ed25519@1.0.0", priv)
	_, err := f.Build(context.Background(), map[string]any{"issuer": "t", "tokenLifetimeSeconds": 2})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFactory_RejectsMissingIssuer(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := NewFactory("identitysigner.ed25519@1.0.0", priv)
	_, err := f.Build(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}
