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
