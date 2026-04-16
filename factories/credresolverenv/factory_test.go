// SPDX-License-Identifier: Apache-2.0

package credresolverenv

import (
	"context"
	"os"
	"testing"
)

func TestResolver_FetchFromEnv(t *testing.T) {
	t.Setenv("FORGE_CRED_NET_HTTP", "secret-value")
	f := NewFactory("credresolver.env@1.0.0")
	r, err := f.Build(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.Fetch(context.Background(), "net:http")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if string(c.Value) != "secret-value" {
		t.Fatalf("got=%q", string(c.Value))
	}
}

func TestResolver_MissingEnvVar(t *testing.T) {
	os.Unsetenv("FORGE_CRED_NOPE")
	f := NewFactory("credresolver.env@1.0.0")
	r, _ := f.Build(context.Background(), nil)
	_, err := r.Fetch(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error")
	}
}
