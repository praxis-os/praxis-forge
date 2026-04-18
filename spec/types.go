// SPDX-License-Identifier: Apache-2.0

// Package spec defines the AgentSpec declarative format plus a strict
// YAML loader and validator. Phase 1 shape: no overlays, no extends.
package spec

// AgentSpec is the top-level declarative agent definition.
type AgentSpec struct {
	APIVersion  string         `yaml:"apiVersion"`
	Kind        string         `yaml:"kind"`
	Metadata    Metadata       `yaml:"metadata"`
	Provider    ComponentRef   `yaml:"provider"`
	Prompt      PromptBlock    `yaml:"prompt"`
	Tools       []ComponentRef `yaml:"tools,omitempty"`
	Policies    []ComponentRef `yaml:"policies,omitempty"`
	Filters     FilterBlock    `yaml:"filters,omitempty"`
	Budget      *BudgetRef     `yaml:"budget,omitempty"`
	Telemetry   *ComponentRef  `yaml:"telemetry,omitempty"`
	Credentials *CredRef       `yaml:"credentials,omitempty"`
	Identity    *ComponentRef  `yaml:"identity,omitempty"`

	// Phase-gated: accepted by the parser but must be empty until the
	// corresponding phase ships.
	Extends        []string       `yaml:"extends,omitempty"`
	Skills         []ComponentRef `yaml:"skills,omitempty"`
	MCPImports     []ComponentRef `yaml:"mcpImports,omitempty"`
	OutputContract *ComponentRef  `yaml:"outputContract,omitempty"`
}

type Metadata struct {
	ID          string            `yaml:"id"`
	Version     string            `yaml:"version"`
	DisplayName string            `yaml:"displayName,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Owners      []Owner           `yaml:"owners,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

type Owner struct {
	Team    string `yaml:"team,omitempty"`
	Contact string `yaml:"contact,omitempty"`
}

type ComponentRef struct {
	Ref    string         `yaml:"ref"`
	Config map[string]any `yaml:"config,omitempty"`
}

type PromptBlock struct {
	System *ComponentRef `yaml:"system"`
	User   *ComponentRef `yaml:"user,omitempty"`
}

type FilterBlock struct {
	PreLLM   []ComponentRef `yaml:"preLLM,omitempty"`
	PreTool  []ComponentRef `yaml:"preTool,omitempty"`
	PostTool []ComponentRef `yaml:"postTool,omitempty"`
}

type BudgetRef struct {
	Ref       string          `yaml:"ref"`
	Overrides BudgetOverrides `yaml:"overrides,omitempty"`
}

type BudgetOverrides struct {
	MaxWallClock        string `yaml:"maxWallClock,omitempty"` // duration, e.g. "30s"
	MaxInputTokens      int64  `yaml:"maxInputTokens,omitempty"`
	MaxOutputTokens     int64  `yaml:"maxOutputTokens,omitempty"`
	MaxToolCalls        int64  `yaml:"maxToolCalls,omitempty"`
	MaxCostMicrodollars int64  `yaml:"maxCostMicrodollars,omitempty"`
}

type CredRef struct {
	Ref    string   `yaml:"ref"`
	Scopes []string `yaml:"scopes,omitempty"`
}
