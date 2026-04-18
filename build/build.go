// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"
	"time"

	"github.com/praxis-os/praxis-forge/manifest"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/orchestrator"
)

// BuiltAgent is a stateless wiring + metadata bundle. Per-turn state lives in
// the embedded Orchestrator; conversation history is the caller's concern.
type BuiltAgent struct {
	Orchestrator  *orchestrator.Orchestrator
	Manifest      manifest.Manifest
	SystemPrompt  string
	ToolDefs      []llm.ToolDefinition
	NormalizedSpec *spec.NormalizedSpec
}

// Build validates the spec, resolves every component through the registry,
// composes chains, and materializes a *orchestrator.Orchestrator.
func Build(ctx context.Context, ns *spec.NormalizedSpec, r *registry.ComponentRegistry) (*BuiltAgent, error) {
	r.Freeze()

	res, err := resolve(ctx, &ns.Spec, r)
	if err != nil {
		return nil, err
	}

	var opts []orchestrator.Option
	var toolDefs []llm.ToolDefinition

	// Tools.
	if len(res.toolPacks) > 0 {
		router, defs, err := newToolRouter(res.toolPacks)
		if err != nil {
			return nil, fmt.Errorf("tool router: %w", err)
		}
		opts = append(opts, orchestrator.WithToolInvoker(router))
		toolDefs = defs
	}

	// Policy chain.
	if len(res.policyHooks) > 0 {
		opts = append(opts, orchestrator.WithPolicyHook(policyChain(res.policyHooks)))
	}

	// Filter chains.
	if len(res.preLLMFilters) > 0 {
		opts = append(opts, orchestrator.WithPreLLMFilter(preLLMFilterChain(res.preLLMFilters)))
	}
	if len(res.preToolFilters) > 0 {
		opts = append(opts, orchestrator.WithPreToolFilter(preToolFilterChain(res.preToolFilters)))
	}
	if len(res.postToolFilters) > 0 {
		opts = append(opts, orchestrator.WithPostToolFilter(postToolFilterChain(res.postToolFilters)))
	}

	// Budget.
	if res.budget != nil {
		_, err := applyBudgetOverrides(res.budget.DefaultConfig, res.budgetOverrides)
		if err != nil {
			return nil, err
		}
		opts = append(opts, orchestrator.WithBudgetGuard(res.budget.Guard))
	}

	// Telemetry.
	if res.telemetry != nil {
		opts = append(opts, orchestrator.WithLifecycleEmitter(res.telemetry.Emitter))
		opts = append(opts, orchestrator.WithAttributeEnricher(res.telemetry.Enricher))
	}

	// Credentials.
	if res.credResolver != nil {
		opts = append(opts, orchestrator.WithCredentialResolver(res.credResolver))
	}

	// Identity.
	if res.identity != nil {
		opts = append(opts, orchestrator.WithIdentitySigner(res.identity))
	}

	orch, err := orchestrator.New(res.provider, opts...)
	if err != nil {
		return nil, fmt.Errorf("orchestrator.New: %w", err)
	}

	return &BuiltAgent{
		Orchestrator:  orch,
		Manifest:      buildManifest(&ns.Spec, res, ns),
		SystemPrompt:  res.systemPrompt,
		ToolDefs:      toolDefs,
		NormalizedSpec: ns,
	}, nil
}

func buildManifest(s *spec.AgentSpec, res *resolved, ns *spec.NormalizedSpec) manifest.Manifest {
	m := manifest.Manifest{
		SpecID:      s.Metadata.ID,
		SpecVersion: s.Metadata.Version,
		BuiltAt:     time.Now().UTC(),
	}

	if len(ns.ExtendsChain) > 0 {
		m.ExtendsChain = append([]string(nil), ns.ExtendsChain...)
	}
	if len(ns.Overlays) > 0 {
		m.Overlays = make([]manifest.OverlayAttribution, 0, len(ns.Overlays))
		for _, o := range ns.Overlays {
			m.Overlays = append(m.Overlays, manifest.OverlayAttribution{
				Name: o.Name,
				File: o.File,
			})
		}
	}

	m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
		Kind:   string(registry.KindProvider),
		ID:     string(res.providerID),
		Config: res.providerCfg,
	})
	m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
		Kind: string(registry.KindPromptAsset),
		ID:   string(res.promptID),
	})
	for i, id := range res.toolPackIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:   string(registry.KindToolPack),
			ID:     string(id),
			Config: res.toolPackCfgs[i],
		})
	}
	for i, id := range res.policyHookIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:   string(registry.KindPolicyPack),
			ID:     string(id),
			Config: res.policyHookCfgs[i],
		})
	}
	for i, id := range res.preLLMIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindPreLLMFilter), ID: string(id), Config: res.preLLMCfgs[i],
		})
	}
	for i, id := range res.preToolIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindPreToolFilter), ID: string(id), Config: res.preToolCfgs[i],
		})
	}
	for i, id := range res.postToolIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindPostToolFilter), ID: string(id), Config: res.postToolCfgs[i],
		})
	}
	if res.budget != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindBudgetProfile), ID: string(res.budgetID), Config: res.budgetCfg,
		})
	}
	if res.telemetry != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindTelemetryProfile), ID: string(res.telemetryID), Config: res.telemetryCfg,
		})
	}
	if res.credResolver != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindCredentialResolver), ID: string(res.credResolverID), Config: res.credResolverCfg,
		})
	}
	if res.identity != nil {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind: string(registry.KindIdentitySigner), ID: string(res.identityID), Config: res.identityCfg,
		})
	}
	return m
}
