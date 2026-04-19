// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"testing"
	"time"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/llm"
)

// minProvider implements llm.Provider for build tests (never invoked).
type minProvider struct{}

func (minProvider) Name() string                    { return "min" }
func (minProvider) SupportsParallelToolCalls() bool { return false }
func (minProvider) Capabilities() llm.Capabilities  { return llm.Capabilities{} }
func (minProvider) Complete(_ context.Context, _ llm.LLMRequest) (llm.LLMResponse, error) {
	return llm.LLMResponse{}, nil
}
func (minProvider) Stream(_ context.Context, _ llm.LLMRequest) (<-chan llm.LLMStreamChunk, error) {
	ch := make(chan llm.LLMStreamChunk)
	close(ch)
	return ch, nil
}

type minProvFac struct{ id registry.ID }

func (f minProvFac) ID() registry.ID     { return f.id }
func (f minProvFac) Description() string { return "" }
func (f minProvFac) Build(context.Context, map[string]any) (llm.Provider, error) {
	return minProvider{}, nil
}

type minPromptFac struct{ id registry.ID }

func (f minPromptFac) ID() registry.ID                                       { return f.id }
func (f minPromptFac) Description() string                                   { return "" }
func (f minPromptFac) Build(context.Context, map[string]any) (string, error) { return "hi", nil }

type minBudgetFac struct{ id registry.ID }

func (f minBudgetFac) ID() registry.ID     { return f.id }
func (f minBudgetFac) Description() string { return "" }
func (f minBudgetFac) Build(context.Context, map[string]any) (registry.BudgetProfile, error) {
	return registry.BudgetProfile{
		Guard:         budget.NullGuard{},
		DefaultConfig: budget.Config{MaxWallClock: int64(30 * time.Second), MaxToolCalls: 10},
	}, nil
}

func TestBuild_MinimalSpec(t *testing.T) {
	r := registry.NewComponentRegistry()
	_ = r.RegisterProvider(minProvFac{id: "provider.min@1.0.0"})
	_ = r.RegisterPromptAsset(minPromptFac{id: "prompt.sys@1.0.0"})
	_ = r.RegisterBudgetProfile(minBudgetFac{id: "budgetprofile.default@1.0.0"})

	s := &spec.AgentSpec{
		APIVersion: "forge.praxis-os.dev/v0",
		Kind:       "AgentSpec",
		Metadata:   spec.Metadata{ID: "a.b", Version: "0.1.0"},
		Provider:   spec.ComponentRef{Ref: "provider.min@1.0.0"},
		Prompt:     spec.PromptBlock{System: &spec.ComponentRef{Ref: "prompt.sys@1.0.0"}},
		Budget:     &spec.BudgetRef{Ref: "budgetprofile.default@1.0.0"},
	}

	ns, err := spec.Normalize(context.Background(), s, nil, nil)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	built, err := Build(context.Background(), ns, r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if built.Orchestrator == nil {
		t.Fatal("orchestrator nil")
	}
	m := built.Manifest
	if m.SpecID != "a.b" || m.SpecVersion != "0.1.0" {
		t.Fatalf("manifest=%+v", m)
	}
	// Resolved components: provider, prompt, budget.
	if len(m.Resolved) != 3 {
		t.Fatalf("resolved=%d", len(m.Resolved))
	}

	// Registry should now be frozen.
	err = r.RegisterProvider(minProvFac{id: "provider.other@1.0.0"})
	if err == nil {
		t.Fatal("expected registry frozen")
	}
}
