// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

const yamlNullTag = "!!null"

// RefList wraps []ComponentRef so the YAML decoder can distinguish
// "field absent" from "field present (set to anything, including null
// or empty)".
//
//	tools:                              → RefList absent in body          → Set=false
//	tools:                              → explicit null after the colon    → Set=true,  Items=nil
//	tools: []                           → explicit empty list               → Set=true,  Items=[]
//	tools: [{ref: ...}]                 → populated                         → Set=true,  Items=[...]
//
// Merge semantics on the apply path: if Set is false the base list is
// preserved; if Set is true the wrapper replaces the base list verbatim
// (including nil and empty cases).
type RefList struct {
	Set   bool
	Items []ComponentRef

	// Line is the 1-based line number of the field in the source YAML, or
	// 0 if the wrapper was constructed in Go.
	Line int
}

// UnmarshalYAML records that the field was present and decodes its
// contents into Items. An explicit null (e.g. `tools:` followed by
// nothing) leaves Items nil but flips Set to true.
func (r *RefList) UnmarshalYAML(node *yaml.Node) error {
	r.Set = true
	r.Line = node.Line

	switch node.Kind {
	case yaml.SequenceNode:
		var items []ComponentRef
		if err := node.Decode(&items); err != nil {
			return fmt.Errorf("decode RefList at line %d: %w", node.Line, err)
		}
		r.Items = items
		return nil
	case yaml.ScalarNode:
		// Only an explicit null is legal as a scalar here.
		if node.Tag == "!!null" || node.Value == "" {
			r.Items = nil
			return nil
		}
		return fmt.Errorf("RefList at line %d: expected sequence or null, got scalar %q", node.Line, node.Value)
	default:
		return fmt.Errorf("RefList at line %d: expected sequence or null, got %s", node.Line, nodeKindName(node.Kind))
	}
}

func nodeKindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return fmt.Sprintf("kind(%d)", k)
	}
}

// AgentOverlay is the top-level overlay document. The strict YAML
// decoder ensures unknown keys at any depth fail to parse.
type AgentOverlay struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   OverlayMeta      `yaml:"metadata"`
	Spec       AgentOverlayBody `yaml:"spec"`

	// File is the source path passed to LoadOverlay, or empty for in-Go
	// constructed overlays. Populated by LoadOverlay; surfaced through
	// OverlayAttribution and Provenance.
	File string `yaml:"-"`
}

// OverlayMeta carries attribution-only metadata for an overlay file.
// metadata.name surfaces in error messages and the manifest.
type OverlayMeta struct {
	Name string `yaml:"name"`
}

// AgentOverlayBody mirrors AgentSpec but every field is optional and
// each replaceable list uses the RefList tri-state wrapper. Phase-gated
// AgentSpec fields (extends, skills, mcpImports, outputContract) are
// deliberately absent so the strict decoder rejects them at parse time.
type AgentOverlayBody struct {
	Metadata    *OverlayMetadata `yaml:"metadata,omitempty"`
	Provider    *ComponentRef    `yaml:"provider,omitempty"`
	Prompt      *PromptBlock     `yaml:"prompt,omitempty"`
	Tools       RefList          `yaml:"tools,omitempty"`
	Policies    RefList          `yaml:"policies,omitempty"`
	Filters     *FilterOverlay   `yaml:"filters,omitempty"`
	Budget      *BudgetRef       `yaml:"budget,omitempty"`
	Telemetry   *ComponentRef    `yaml:"telemetry,omitempty"`
	Credentials *CredRef         `yaml:"credentials,omitempty"`
	Identity    *ComponentRef    `yaml:"identity,omitempty"`
}

// OverlayMetadata mirrors Metadata but every field is optional. ID and
// Version are accepted at parse time and rejected at apply time by
// validateLocked when they would change the merged result.
type OverlayMetadata struct {
	ID          string            `yaml:"id,omitempty"`
	Version     string            `yaml:"version,omitempty"`
	DisplayName string            `yaml:"displayName,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Owners      []Owner           `yaml:"owners,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// UnmarshalYAML for OverlayMetadata enforces strict field checking.
func (m *OverlayMetadata) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("OverlayMetadata: expected mapping, got %s", nodeKindName(node.Kind))
	}

	allowedFields := map[string]bool{
		"id":          true,
		"version":     true,
		"displayName": true,
		"description": true,
		"owners":      true,
		"labels":      true,
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if !allowedFields[keyNode.Value] {
			return fmt.Errorf("field %q not found in type spec.OverlayMetadata", keyNode.Value)
		}
	}

	type rawMetadata struct {
		ID          string            `yaml:"id,omitempty"`
		Version     string            `yaml:"version,omitempty"`
		DisplayName string            `yaml:"displayName,omitempty"`
		Description string            `yaml:"description,omitempty"`
		Owners      []Owner           `yaml:"owners,omitempty"`
		Labels      map[string]string `yaml:"labels,omitempty"`
	}

	raw := &rawMetadata{}
	if err := node.Decode(raw); err != nil {
		return err
	}

	m.ID = raw.ID
	m.Version = raw.Version
	m.DisplayName = raw.DisplayName
	m.Description = raw.Description
	m.Owners = raw.Owners
	m.Labels = raw.Labels

	return nil
}

// FilterOverlay wraps each filter slice in its own RefList so each
// stage can be replaced or cleared independently.
type FilterOverlay struct {
	PreLLM   RefList `yaml:"preLLM,omitempty"`
	PreTool  RefList `yaml:"preTool,omitempty"`
	PostTool RefList `yaml:"postTool,omitempty"`
}

// decodeRefList is a helper to decode a RefList from a YAML node,
// handling the case where the node is a null scalar (which doesn't
// trigger UnmarshalYAML in yaml.v3).
func decodeRefList(node *yaml.Node) (RefList, error) {
	r := RefList{}

	// Explicitly handle null scalars, which don't trigger UnmarshalYAML.
	if node.Kind == yaml.ScalarNode && node.Tag == yamlNullTag {
		r.Set = true
		r.Items = nil
		r.Line = node.Line
		return r, nil
	}

	// For all other node types, call UnmarshalYAML.
	if err := node.Decode(&r); err != nil {
		return RefList{}, err
	}

	// If Decode didn't set Set (meaning it was absent or null but we didn't
	// reach the null-scalar case above), we need to detect null scalars.
	// Actually, null scalars would have been handled above, so if we're here
	// and Set is still false, the field was absent.
	return r, nil
}

// UnmarshalYAML for AgentOverlayBody manually processes RefList fields
// to ensure null values are properly distinguished from absent fields.
// It also validates that no unknown fields are present (strict mode).
func (b *AgentOverlayBody) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("AgentOverlayBody: expected mapping, got %s", nodeKindName(node.Kind))
	}

	// Allowed field names in AgentOverlayBody
	allowedFields := map[string]bool{
		"metadata":    true,
		"provider":    true,
		"prompt":      true,
		"tools":       true,
		"policies":    true,
		"filters":     true,
		"budget":      true,
		"telemetry":   true,
		"credentials": true,
		"identity":    true,
	}

	// First pass: check for unknown fields
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if !allowedFields[keyNode.Value] {
			return fmt.Errorf("field %q not found in type spec.AgentOverlayBody", keyNode.Value)
		}
	}

	type rawBody struct {
		Metadata    *OverlayMetadata `yaml:"metadata,omitempty"`
		Provider    *ComponentRef    `yaml:"provider,omitempty"`
		Prompt      *PromptBlock     `yaml:"prompt,omitempty"`
		Filters     *FilterOverlay   `yaml:"filters,omitempty"`
		Budget      *BudgetRef       `yaml:"budget,omitempty"`
		Telemetry   *ComponentRef    `yaml:"telemetry,omitempty"`
		Credentials *CredRef         `yaml:"credentials,omitempty"`
		Identity    *ComponentRef    `yaml:"identity,omitempty"`
	}

	// Second pass: decode all non-RefList fields
	raw := &rawBody{}
	if err := node.Decode(raw); err != nil {
		return err
	}

	b.Metadata = raw.Metadata
	b.Provider = raw.Provider
	b.Prompt = raw.Prompt
	b.Filters = raw.Filters
	b.Budget = raw.Budget
	b.Telemetry = raw.Telemetry
	b.Credentials = raw.Credentials
	b.Identity = raw.Identity

	// Third pass: manually handle RefList fields by checking the YAML node directly
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		switch keyNode.Value {
		case "tools":
			r, err := decodeRefList(valueNode)
			if err != nil {
				return err
			}
			b.Tools = r
		case "policies":
			r, err := decodeRefList(valueNode)
			if err != nil {
				return err
			}
			b.Policies = r
		}
	}

	return nil
}

// decodeFilterOverlay is a helper to decode filter fields with RefList support.
func decodeFilterRefList(node *yaml.Node) (RefList, error) {
	return decodeRefList(node)
}

// UnmarshalYAML for FilterOverlay manually processes RefList fields.
func (f *FilterOverlay) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("FilterOverlay: expected mapping, got %s", nodeKindName(node.Kind))
	}

	// Allowed field names in FilterOverlay
	allowedFields := map[string]bool{
		"preLLM":   true,
		"preTool":  true,
		"postTool": true,
	}

	// First pass: check for unknown fields
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if !allowedFields[keyNode.Value] {
			return fmt.Errorf("field %q not found in type spec.FilterOverlay", keyNode.Value)
		}
	}

	// Second pass: decode RefList fields
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		switch keyNode.Value {
		case "preLLM":
			r, err := decodeFilterRefList(valueNode)
			if err != nil {
				return err
			}
			f.PreLLM = r
		case "preTool":
			r, err := decodeFilterRefList(valueNode)
			if err != nil {
				return err
			}
			f.PreTool = r
		case "postTool":
			r, err := decodeFilterRefList(valueNode)
			if err != nil {
				return err
			}
			f.PostTool = r
		}
	}

	return nil
}
