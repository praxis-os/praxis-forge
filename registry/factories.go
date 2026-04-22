// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"

	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/llm"
)

// ProviderFactory builds an llm.Provider.
type ProviderFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (llm.Provider, error)
}

// PromptAssetFactory builds the string body of a registered prompt asset.
type PromptAssetFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (string, error)
}

// ToolPackFactory builds a governed tool pack.
type ToolPackFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (ToolPack, error)
}

// PolicyPackFactory builds a hooks.PolicyHook plus governance metadata.
type PolicyPackFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (PolicyPack, error)
}

// PreLLMFilterFactory builds a hooks.PreLLMFilter.
type PreLLMFilterFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (hooks.PreLLMFilter, error)
}

// PreToolFilterFactory builds a hooks.PreToolFilter.
type PreToolFilterFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (hooks.PreToolFilter, error)
}

// PostToolFilterFactory builds a hooks.PostToolFilter.
type PostToolFilterFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (hooks.PostToolFilter, error)
}

// BudgetProfileFactory builds a budget guard + default config.
type BudgetProfileFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (BudgetProfile, error)
}

// TelemetryProfileFactory builds emitter + enricher.
type TelemetryProfileFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (TelemetryProfile, error)
}

// CredentialResolverFactory builds a credentials.Resolver.
type CredentialResolverFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (credentials.Resolver, error)
}

// IdentitySignerFactory builds an identity.Signer.
type IdentitySignerFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (identity.Signer, error)
}

// SkillFactory builds a Skill.
type SkillFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (Skill, error)
}

// OutputContractFactory builds an OutputContract.
type OutputContractFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (OutputContract, error)
}

// MCPBindingFactory builds an MCPBinding.
type MCPBindingFactory interface {
	ID() ID
	Description() string
	Build(ctx context.Context, cfg map[string]any) (MCPBinding, error)
}
