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
	Orchestrator   *orchestrator.Orchestrator
	Manifest       manifest.Manifest
	SystemPrompt   string
	ToolDefs       []llm.ToolDefinition
	NormalizedSpec *spec.NormalizedSpec
}

// Build validates the spec, resolves every component through the registry,
// composes chains, and materializes a *orchestrator.Orchestrator.
//nolint:gocyclo
func Build(ctx context.Context, ns *spec.NormalizedSpec, r *registry.ComponentRegistry) (*BuiltAgent, error) {
	r.Freeze()

	expanded, err := expandSkills(ctx, &ns.Spec, r)
	if err != nil {
		return nil, err
	}
	if err := resolveOutputContract(ctx, expanded, r); err != nil {
		return nil, err
	}

	res, err := resolve(ctx, &expanded.Spec, r)
	if err != nil {
		return nil, err
	}
	// Stamp expansion artefacts on res so buildManifest can attribute them.
	res.skills = make([]registry.Skill, 0, len(expanded.Skills))
	res.skillIDs = make([]registry.ID, 0, len(expanded.Skills))
	res.skillCfgs = make([]map[string]any, 0, len(expanded.Skills))
	for _, rs := range expanded.Skills {
		res.skills = append(res.skills, rs.Value)
		res.skillIDs = append(res.skillIDs, rs.ID)
		res.skillCfgs = append(res.skillCfgs, rs.Config)
	}
	if expanded.ResolvedOutputContract != nil {
		oc := expanded.ResolvedOutputContract.Value
		res.outputContract = &oc
		res.outputContractID = expanded.ResolvedOutputContract.ID
		res.outputContractCfg = expanded.ResolvedOutputContract.Config
	}
	mcpBinds, err := resolveMCPBindings(ctx, &expanded.Spec, r)
	if err != nil {
		return nil, err
	}
	for _, b := range mcpBinds {
		res.mcpBindings = append(res.mcpBindings, b.Value)
		res.mcpBindingIDs = append(res.mcpBindingIDs, b.ID)
		res.mcpBindingCfgs = append(res.mcpBindingCfgs, b.Config)
	}

	// Append skill prompt fragments to the base system prompt (design
	// §"Prompt fragment merge"): declaration order, "\n\n" separator,
	// byte-identical dedupe.
	res.systemPrompt = appendSkillFragments(res.systemPrompt, expanded.Skills)

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
		Orchestrator:   orch,
		Manifest:       buildManifest(&ns.Spec, res, ns, expanded),
		SystemPrompt:   res.systemPrompt,
		ToolDefs:       toolDefs,
		NormalizedSpec: ns,
	}, nil
}

// appendSkillFragments appends each skill's PromptFragment to base.
// Order: skills[] declaration order. Separator: "\n\n". Byte-identical
// fragments deduplicate silently (audit still shows each contributing
// skill in the manifest Resolved list).
func appendSkillFragments(base string, skills []ResolvedSkill) string {
	if len(skills) == 0 {
		return base
	}
	seen := map[string]bool{}
	out := base
	for _, rs := range skills {
		frag := rs.Value.PromptFragment
		if frag == "" {
			continue
		}
		if seen[frag] {
			continue
		}
		seen[frag] = true
		if out != "" {
			out += "\n\n"
		}
		out += frag
	}
	return out
}

//nolint:gocyclo
func buildManifest(s *spec.AgentSpec, res *resolved, ns *spec.NormalizedSpec, expanded *ExpandedSpec) manifest.Manifest {
	hash, _ := ns.NormalizedHash() // error impossible: ns passed validation
	m := manifest.Manifest{
		SpecID:         s.Metadata.ID,
		SpecVersion:    s.Metadata.Version,
		BuiltAt:        time.Now().UTC(),
		NormalizedHash: hash,
		Capabilities:   computeCapabilities(s, res, expanded),
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

	// Phase 3: expanded hash emitted only when skills[] was non-empty.
	if len(s.Skills) > 0 {
		eh, _ := computeExpandedHash(expanded)
		m.ExpandedHash = eh
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
			Kind:            string(registry.KindToolPack),
			ID:              string(id),
			Config:          res.toolPackCfgs[i],
			InjectedBySkill: lookupInjector(expanded, "tool_pack", string(id)),
		})
	}
	for i, id := range res.policyHookIDs {
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:            string(registry.KindPolicyPack),
			ID:              string(id),
			Config:          res.policyHookCfgs[i],
			InjectedBySkill: lookupInjector(expanded, "policy_pack", string(id)),
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
	// Skills and output contract (Phase 3).
	for i, id := range res.skillIDs {
		desc := res.skills[i].Descriptor
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:        string(registry.KindSkill),
			ID:          string(id),
			Config:      res.skillCfgs[i],
			Descriptors: desc,
		})
	}
	if res.outputContract != nil {
		desc := res.outputContract.Descriptor
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:            string(registry.KindOutputContract),
			ID:              string(res.outputContractID),
			Config:          res.outputContractCfg,
			Descriptors:     desc,
			InjectedBySkill: lookupInjector(expanded, "output_contract", string(res.outputContractID)),
		})
	}
	// MCP bindings (Phase 4).
	for i, id := range res.mcpBindingIDs {
		desc := res.mcpBindings[i].Descriptor
		m.Resolved = append(m.Resolved, manifest.ResolvedComponent{
			Kind:        string(registry.KindMCPBinding),
			ID:          string(id),
			Config:      res.mcpBindingCfgs[i],
			Descriptors: desc,
		})
	}
	return m
}

// lookupInjector returns the skill id that drove inclusion of a
// specific (kindLabel, id) pair, or empty string if the component was
// user-declared or there was no expansion.
func lookupInjector(expanded *ExpandedSpec, kindLabel, id string) string {
	if expanded == nil || expanded.InjectedBy == nil {
		return ""
	}
	return string(expanded.InjectedBy[kindLabel+":"+id])
}
