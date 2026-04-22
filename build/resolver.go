// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/llm"
)

// resolved holds every factory's built value for the materializer.
type resolved struct {
	provider     llm.Provider
	providerID   registry.ID
	providerCfg  map[string]any
	systemPrompt string
	promptID     registry.ID

	toolPacks    []registry.ToolPack
	toolPackIDs  []registry.ID
	toolPackCfgs []map[string]any

	policyHooks    []hooks.PolicyHook
	policyHookIDs  []registry.ID
	policyHookCfgs []map[string]any

	preLLMFilters []hooks.PreLLMFilter
	preLLMIDs     []registry.ID
	preLLMCfgs    []map[string]any

	preToolFilters []hooks.PreToolFilter
	preToolIDs     []registry.ID
	preToolCfgs    []map[string]any

	postToolFilters []hooks.PostToolFilter
	postToolIDs     []registry.ID
	postToolCfgs    []map[string]any

	budget          *registry.BudgetProfile
	budgetID        registry.ID
	budgetCfg       map[string]any
	budgetOverrides spec.BudgetOverrides

	telemetry    *registry.TelemetryProfile
	telemetryID  registry.ID
	telemetryCfg map[string]any

	credResolver    credentials.Resolver
	credResolverID  registry.ID
	credResolverCfg map[string]any

	identity    identity.Signer
	identityID  registry.ID
	identityCfg map[string]any

	skills            []registry.Skill
	skillIDs          []registry.ID
	skillCfgs         []map[string]any

	outputContract    *registry.OutputContract
	outputContractID  registry.ID
	outputContractCfg map[string]any

	specSnapshot *spec.AgentSpec
}

//nolint:gocyclo // linear dispatch over the 11 Phase-1 factory kinds; each block is a registry lookup + Build. Splitting into helpers scatters the kind list without reducing reader cognitive load.
func resolve(ctx context.Context, s *spec.AgentSpec, r *registry.ComponentRegistry) (*resolved, error) {
	out := &resolved{specSnapshot: s}

	// Provider (required).
	provFactory, err := r.Provider(registry.ID(s.Provider.Ref))
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)
	}
	prov, err := provFactory.Build(ctx, s.Provider.Config)
	if err != nil {
		return nil, fmt.Errorf("build provider %s: %w", s.Provider.Ref, err)
	}
	out.provider, out.providerID, out.providerCfg = prov, provFactory.ID(), s.Provider.Config

	// Prompt (required).
	promptFactory, err := r.PromptAsset(registry.ID(s.Prompt.System.Ref))
	if err != nil {
		return nil, fmt.Errorf("resolve prompt.system: %w", err)
	}
	text, err := promptFactory.Build(ctx, s.Prompt.System.Config)
	if err != nil {
		return nil, fmt.Errorf("build prompt %s: %w", s.Prompt.System.Ref, err)
	}
	out.systemPrompt, out.promptID = text, promptFactory.ID()

	// Tools.
	for i, ref := range s.Tools {
		f, err := r.ToolPack(registry.ID(ref.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve tools[%d]: %w", i, err)
		}
		pack, err := f.Build(ctx, ref.Config)
		if err != nil {
			return nil, fmt.Errorf("build tools[%d] %s: %w", i, ref.Ref, err)
		}
		out.toolPacks = append(out.toolPacks, pack)
		out.toolPackIDs = append(out.toolPackIDs, f.ID())
		out.toolPackCfgs = append(out.toolPackCfgs, ref.Config)
	}

	// Policies.
	for i, ref := range s.Policies {
		f, err := r.PolicyPack(registry.ID(ref.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve policies[%d]: %w", i, err)
		}
		pp, err := f.Build(ctx, ref.Config)
		if err != nil {
			return nil, fmt.Errorf("build policies[%d] %s: %w", i, ref.Ref, err)
		}
		out.policyHooks = append(out.policyHooks, pp.Hook)
		out.policyHookIDs = append(out.policyHookIDs, f.ID())
		out.policyHookCfgs = append(out.policyHookCfgs, ref.Config)
	}

	// PreLLM filters.
	for i, ref := range s.Filters.PreLLM {
		f, err := r.PreLLMFilter(registry.ID(ref.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve filters.preLLM[%d]: %w", i, err)
		}
		flt, err := f.Build(ctx, ref.Config)
		if err != nil {
			return nil, fmt.Errorf("build filters.preLLM[%d] %s: %w", i, ref.Ref, err)
		}
		out.preLLMFilters = append(out.preLLMFilters, flt)
		out.preLLMIDs = append(out.preLLMIDs, f.ID())
		out.preLLMCfgs = append(out.preLLMCfgs, ref.Config)
	}

	// PreTool filters.
	for i, ref := range s.Filters.PreTool {
		f, err := r.PreToolFilter(registry.ID(ref.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve filters.preTool[%d]: %w", i, err)
		}
		flt, err := f.Build(ctx, ref.Config)
		if err != nil {
			return nil, fmt.Errorf("build filters.preTool[%d] %s: %w", i, ref.Ref, err)
		}
		out.preToolFilters = append(out.preToolFilters, flt)
		out.preToolIDs = append(out.preToolIDs, f.ID())
		out.preToolCfgs = append(out.preToolCfgs, ref.Config)
	}

	// PostTool filters.
	for i, ref := range s.Filters.PostTool {
		f, err := r.PostToolFilter(registry.ID(ref.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve filters.postTool[%d]: %w", i, err)
		}
		flt, err := f.Build(ctx, ref.Config)
		if err != nil {
			return nil, fmt.Errorf("build filters.postTool[%d] %s: %w", i, ref.Ref, err)
		}
		out.postToolFilters = append(out.postToolFilters, flt)
		out.postToolIDs = append(out.postToolIDs, f.ID())
		out.postToolCfgs = append(out.postToolCfgs, ref.Config)
	}

	// Budget (optional).
	if s.Budget != nil {
		f, err := r.BudgetProfile(registry.ID(s.Budget.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve budget: %w", err)
		}
		bp, err := f.Build(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("build budget %s: %w", s.Budget.Ref, err)
		}
		out.budget = &bp
		out.budgetID = f.ID()
		out.budgetOverrides = s.Budget.Overrides
	}

	// Telemetry (optional).
	if s.Telemetry != nil {
		f, err := r.TelemetryProfile(registry.ID(s.Telemetry.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve telemetry: %w", err)
		}
		tp, err := f.Build(ctx, s.Telemetry.Config)
		if err != nil {
			return nil, fmt.Errorf("build telemetry %s: %w", s.Telemetry.Ref, err)
		}
		out.telemetry = &tp
		out.telemetryID = f.ID()
		out.telemetryCfg = s.Telemetry.Config
	}

	// Credentials (optional).
	if s.Credentials != nil {
		f, err := r.CredentialResolver(registry.ID(s.Credentials.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve credentials: %w", err)
		}
		cr, err := f.Build(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("build credentials %s: %w", s.Credentials.Ref, err)
		}
		out.credResolver = cr
		out.credResolverID = f.ID()
	}

	// Identity (optional).
	if s.Identity != nil {
		f, err := r.IdentitySigner(registry.ID(s.Identity.Ref))
		if err != nil {
			return nil, fmt.Errorf("resolve identity: %w", err)
		}
		signer, err := f.Build(ctx, s.Identity.Config)
		if err != nil {
			return nil, fmt.Errorf("build identity %s: %w", s.Identity.Ref, err)
		}
		out.identity = signer
		out.identityID = f.ID()
		out.identityCfg = s.Identity.Config
	}

	return out, nil
}
