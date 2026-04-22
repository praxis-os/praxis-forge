// SPDX-License-Identifier: Apache-2.0

// Package manifest holds the inspectable build record for a BuiltAgent.
package manifest

import "time"

// Manifest is the build record returned alongside every BuiltAgent. It
// is JSON-serializable so callers can persist it for audit, diff, and
// inspection workflows.
type Manifest struct {
	SpecID         string               `json:"specId"`
	SpecVersion    string               `json:"specVersion"`
	BuiltAt        time.Time            `json:"builtAt"`
	NormalizedHash string               `json:"normalizedHash"`
	// ExpandedHash is the SHA-256 of the canonical JSON of the
	// post-skill-expansion AgentSpec. Emitted when spec.skills[] was
	// non-empty (Phase 3). Omitted when no skill expansion ran.
	ExpandedHash   string               `json:"expandedHash,omitempty"`
	Capabilities   Capabilities         `json:"capabilities"`
	ExtendsChain   []string             `json:"extendsChain,omitempty"`
	Overlays       []OverlayAttribution `json:"overlays,omitempty"`
	Resolved       []ResolvedComponent  `json:"resolved"`
}

// OverlayAttribution identifies one overlay that contributed to the
// build. Mirror of spec.OverlayAttribution; duplicated here so the
// manifest package keeps zero internal dependencies.
type OverlayAttribution struct {
	Name string `json:"name"`
	File string `json:"file,omitempty"`
}

type ResolvedComponent struct {
	Kind        string         `json:"kind"`
	ID          string         `json:"id"`
	Config      map[string]any `json:"config,omitempty"`
	Descriptors any            `json:"descriptors,omitempty"`
	// InjectedBySkill is the skill id that drove inclusion of this
	// component via Phase 3 expansion. Empty for user-declared or
	// for the skills themselves (Kind == "skill").
	InjectedBySkill string `json:"injectedBySkill,omitempty"`
}
