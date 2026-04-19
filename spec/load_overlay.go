// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadOverlay reads and decodes an AgentOverlay YAML file with strict
// unknown-field rejection at every depth. It validates only the
// envelope (apiVersion + kind); body validation happens during
// Normalize when the overlay is applied.
func LoadOverlay(path string) (*AgentOverlay, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open overlay: %w", err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var ov AgentOverlay
	if err := dec.Decode(&ov); err != nil {
		return nil, fmt.Errorf("decode overlay %s: %w", path, err)
	}
	if ov.APIVersion != expectedAPIVersion {
		return nil, fmt.Errorf("overlay %s: apiVersion: want %q, got %q",
			path, expectedAPIVersion, ov.APIVersion)
	}
	if ov.Kind != "AgentOverlay" {
		return nil, fmt.Errorf("overlay %s: kind: want %q, got %q",
			path, "AgentOverlay", ov.Kind)
	}
	ov.File = path
	return &ov, nil
}
