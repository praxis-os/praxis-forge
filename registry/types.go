// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"time"

	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/telemetry"
	"github.com/praxis-os/praxis/tools"
)

// RiskTier categorises tools and policies for governance tagging.
type RiskTier string

const (
	RiskLow         RiskTier = "low"
	RiskModerate    RiskTier = "moderate"
	RiskHigh        RiskTier = "high"
	RiskDestructive RiskTier = "destructive"
)

// ToolDescriptor is forge-managed metadata added on top of llm.ToolDefinition.
type ToolDescriptor struct {
	Name        string
	Owner       string
	RiskTier    RiskTier
	PolicyTags  []string
	AuthScopes  []string
	TimeoutHint time.Duration
	Source      string // factory ID
}

// PolicyDescriptor is forge-managed metadata for a policy pack.
type PolicyDescriptor struct {
	Name       string
	Owner      string
	PolicyTags []string
	Source     string
}

// ToolPack is what a ToolPackFactory produces.
type ToolPack struct {
	Invoker     tools.Invoker
	Definitions []llm.ToolDefinition
	Descriptors []ToolDescriptor
}

// PolicyPack is what a PolicyPackFactory produces.
type PolicyPack struct {
	Hook        hooks.PolicyHook
	Descriptors []PolicyDescriptor
}

// BudgetProfile is what a BudgetProfileFactory produces.
type BudgetProfile struct {
	Guard         budget.Guard
	DefaultConfig budget.Config
}

// TelemetryProfile is what a TelemetryProfileFactory produces.
type TelemetryProfile struct {
	Emitter  telemetry.LifecycleEventEmitter
	Enricher telemetry.AttributeEnricher
}

// RequiredComponent represents a required dependency with optional configuration.
type RequiredComponent struct {
	ID     ID
	Config map[string]interface{}
}

// SkillDescriptor is forge-managed metadata for a skill.
type SkillDescriptor struct {
	Name    string
	Owner   string
	Summary string
	Tags    []string
}

// Skill is what a SkillFactory produces.
type Skill struct {
	PromptFragment         string
	RequiredTools          []RequiredComponent
	RequiredPolicies       []RequiredComponent
	RequiredOutputContract *RequiredComponent
	Descriptor             SkillDescriptor
}

// OutputContractDescriptor is forge-managed metadata for an output contract.
type OutputContractDescriptor struct {
	Name    string
	Owner   string
	Summary string
}

// OutputContract is what an OutputContractFactory produces.
type OutputContract struct {
	Schema     map[string]interface{}
	Descriptor OutputContractDescriptor
}

// MCPBinding is the value produced by an MCPBindingFactory. It is a
// governance contract, not a resolved tool set. Runtime (praxis) opens
// the MCP session, discovers tools live, applies Allow/Deny + Policies,
// and enforces OnNewTool when the server surface changes.
type MCPBinding struct {
	ID         string
	Connection MCPConnection
	Auth       *MCPAuth
	Allow      []string
	Deny       []string
	Policies   []ID
	Trust      MCPTrust
	OnNewTool  OnNewToolPolicy
	Descriptor MCPBindingDescriptor
}

// MCPTransport enumerates the declarable MCP connection transports.
type MCPTransport string

const (
	MCPTransportStdio MCPTransport = "stdio"
	MCPTransportHTTP  MCPTransport = "http"
)

// MCPConnection holds transport-specific connection details. Only the
// fields matching Transport are meaningful; the others are ignored.
type MCPConnection struct {
	Transport MCPTransport
	Command   []string
	Env       map[string]string
	URL       string
	Headers   map[string]string
}

// MCPAuth describes how the runtime authenticates against the MCP
// server. Forge never reads the secret; only CredentialRef is stored.
type MCPAuth struct {
	CredentialRef ID
	Scheme        string
	HeaderName    string
}

// MCPTrust holds governance-level metadata for a binding.
type MCPTrust struct {
	Tier  string
	Owner string
	Tags  []string
}

// OnNewToolPolicy governs how the runtime reacts when the remote MCP
// server exposes a tool that was not observed when the agent was built.
type OnNewToolPolicy string

const (
	OnNewToolBlock            OnNewToolPolicy = "block"
	OnNewToolAllowIfMatch     OnNewToolPolicy = "allow-if-match-allowlist"
	OnNewToolRequireReapprove OnNewToolPolicy = "require-reapproval"
)

// MCPBindingDescriptor is forge-managed metadata for an MCP binding.
type MCPBindingDescriptor struct {
	Name    string
	Summary string
	Tags    []string
}
