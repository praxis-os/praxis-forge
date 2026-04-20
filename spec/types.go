// SPDX-License-Identifier: Apache-2.0

// Package spec defines the AgentSpec declarative format plus a strict
// YAML loader and validator. Phase 1 shape: no overlays, no extends.
package spec

// AgentSpec is the top-level declarative agent definition.
type AgentSpec struct {
	APIVersion  string         `yaml:"apiVersion"                 json:"apiVersion"`
	Kind        string         `yaml:"kind"                       json:"kind"`
	Metadata    Metadata       `yaml:"metadata"                   json:"metadata"`
	Provider    ComponentRef   `yaml:"provider"                   json:"provider"`
	Prompt      PromptBlock    `yaml:"prompt"                     json:"prompt"`
	Tools       []ComponentRef `yaml:"tools,omitempty"            json:"tools,omitempty"`
	Policies    []ComponentRef `yaml:"policies,omitempty"         json:"policies,omitempty"`
	Filters     FilterBlock    `yaml:"filters,omitempty"          json:"filters,omitempty"`
	Budget      *BudgetRef     `yaml:"budget,omitempty"           json:"budget,omitempty"`
	Telemetry   *ComponentRef  `yaml:"telemetry,omitempty"        json:"telemetry,omitempty"`
	Credentials *CredRef       `yaml:"credentials,omitempty"      json:"credentials,omitempty"`
	Identity    *ComponentRef  `yaml:"identity,omitempty"         json:"identity,omitempty"`

	// Phase-gated: accepted by the parser but must be empty until the
	// corresponding phase ships.
	Extends        []string       `yaml:"extends,omitempty"          json:"extends,omitempty"`
	Skills         []ComponentRef `yaml:"skills,omitempty"           json:"skills,omitempty"`
	MCPImports     []ComponentRef `yaml:"mcpImports,omitempty"       json:"mcpImports,omitempty"`
	OutputContract *ComponentRef  `yaml:"outputContract,omitempty"   json:"outputContract,omitempty"`
}

type Metadata struct {
	ID          string            `yaml:"id"                    json:"id"`
	Version     string            `yaml:"version"               json:"version"`
	DisplayName string            `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Owners      []Owner           `yaml:"owners,omitempty"      json:"owners,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"      json:"labels,omitempty"`
}

type Owner struct {
	Team    string `yaml:"team,omitempty"    json:"team,omitempty"`
	Contact string `yaml:"contact,omitempty" json:"contact,omitempty"`
}

type ComponentRef struct {
	Ref    string         `yaml:"ref"              json:"ref"`
	Config map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

type PromptBlock struct {
	System *ComponentRef `yaml:"system"         json:"system,omitempty"`
	User   *ComponentRef `yaml:"user,omitempty" json:"user,omitempty"`
}

type FilterBlock struct {
	PreLLM   []ComponentRef `yaml:"preLLM,omitempty"   json:"preLLM,omitempty"`
	PreTool  []ComponentRef `yaml:"preTool,omitempty"  json:"preTool,omitempty"`
	PostTool []ComponentRef `yaml:"postTool,omitempty" json:"postTool,omitempty"`
}

type BudgetRef struct {
	Ref       string          `yaml:"ref"                 json:"ref"`
	Overrides BudgetOverrides `yaml:"overrides,omitempty" json:"overrides,omitempty"`
}

type BudgetOverrides struct {
	MaxWallClock        string `yaml:"maxWallClock,omitempty"        json:"maxWallClock,omitempty"` // duration, e.g. "30s"
	MaxInputTokens      int64  `yaml:"maxInputTokens,omitempty"      json:"maxInputTokens,omitempty"`
	MaxOutputTokens     int64  `yaml:"maxOutputTokens,omitempty"     json:"maxOutputTokens,omitempty"`
	MaxToolCalls        int64  `yaml:"maxToolCalls,omitempty"        json:"maxToolCalls,omitempty"`
	MaxCostMicrodollars int64  `yaml:"maxCostMicrodollars,omitempty" json:"maxCostMicrodollars,omitempty"`
}

type CredRef struct {
	Ref    string   `yaml:"ref"              json:"ref"`
	Scopes []string `yaml:"scopes,omitempty" json:"scopes,omitempty"`
}
