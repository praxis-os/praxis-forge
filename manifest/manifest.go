// SPDX-License-Identifier: Apache-2.0

// Package manifest holds the inspectable build record for a BuiltAgent.
package manifest

import "time"

type Manifest struct {
	SpecID      string              `json:"specId"`
	SpecVersion string              `json:"specVersion"`
	BuiltAt     time.Time           `json:"builtAt"`
	Resolved    []ResolvedComponent `json:"resolved"`
}

type ResolvedComponent struct {
	Kind        string         `json:"kind"`
	ID          string         `json:"id"`
	Config      map[string]any `json:"config,omitempty"`
	Descriptors any            `json:"descriptors,omitempty"`
}
