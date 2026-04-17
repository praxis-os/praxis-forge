// SPDX-License-Identifier: Apache-2.0

// Package promptassetliteral is the simplest prompt_asset factory: it returns
// the literal `text` string from its config. Register one Factory per prompt
// id your application needs.
package promptassetliteral

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "literal prompt asset" }

func (f *Factory) Build(_ context.Context, cfg map[string]any) (string, error) {
	raw, ok := cfg["text"]
	if !ok {
		return "", fmt.Errorf("%s: config.text: required", f.id)
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s: config.text: want string, got %T", f.id, raw)
	}
	if s == "" {
		return "", fmt.Errorf("%s: config.text: must be non-empty", f.id)
	}
	return s, nil
}
