// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadSpec reads and decodes an AgentSpec YAML file with strict unknown-field
// rejection. It does not run validation; call (*AgentSpec).Validate separately.
func LoadSpec(path string) (*AgentSpec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spec: %w", err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var s AgentSpec
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("decode spec %s: %w", path, err)
	}
	return &s, nil
}
