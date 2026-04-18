// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/llm"
)

type provFac struct{ id registry.ID }

func (f provFac) ID() registry.ID                                             { return f.id }
func (f provFac) Description() string                                         { return "p" }
func (f provFac) Build(context.Context, map[string]any) (llm.Provider, error) { return nil, nil }

type promptFac struct{ id registry.ID }

func (f promptFac) ID() registry.ID                                       { return f.id }
func (f promptFac) Description() string                                   { return "p" }
func (f promptFac) Build(context.Context, map[string]any) (string, error) { return "hi", nil }

func TestResolveProviderAndPrompt(t *testing.T) {
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(provFac{id: "provider.fake@1.0.0"})
	_ = r.RegisterPromptAsset(promptFac{id: "prompt.sys@1.0.0"})

	s := &spec.AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "a.b", Version: "0.1.0"},
		Provider:   spec.ComponentRef{Ref: "provider.fake@1.0.0"},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: "prompt.sys@1.0.0"}},
	}
	res, err := resolve(context.Background(), s, r)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.providerID != "provider.fake@1.0.0" {
		t.Fatal("provider ID mismatch")
	}
	if res.systemPrompt != "hi" {
		t.Fatalf("prompt=%s", res.systemPrompt)
	}
}
