// SPDX-License-Identifier: Apache-2.0

// Package provideranthropic wraps praxis's anthropic provider. The API key is
// injected at factory construction time (never via spec); spec config carries
// model and maxOutputTokens. Temperature is not yet supported by the praxis
// anthropic options in v0.9.0 — accepted in config for forward compat but
// currently ignored.
package provideranthropic

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
)

type Factory struct {
	id     registry.ID
	apiKey string
}

func NewFactory(id registry.ID, apiKey string) *Factory {
	return &Factory{id: id, apiKey: apiKey}
}

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "Anthropic Claude provider" }

type config struct {
	Model           string
	MaxOutputTokens int
}

func decode(cfg map[string]any) (config, error) {
	var c config
	model, ok := cfg["model"].(string)
	if !ok || model == "" {
		return c, fmt.Errorf("model: required string")
	}
	c.Model = model
	if raw, ok := cfg["maxOutputTokens"]; ok {
		switch x := raw.(type) {
		case int:
			c.MaxOutputTokens = x
		case int64:
			c.MaxOutputTokens = int(x)
		case float64:
			c.MaxOutputTokens = int(x)
		default:
			return c, fmt.Errorf("maxOutputTokens: want int, got %T", raw)
		}
	}
	return c, nil
}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (llm.Provider, error) {
	if f.apiKey == "" {
		return nil, fmt.Errorf("%s: api key not set at factory construction", f.id)
	}
	c, err := decode(cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.id, err)
	}
	opts := []anthropic.Option{anthropic.WithModel(c.Model)}
	if c.MaxOutputTokens > 0 {
		opts = append(opts, anthropic.WithMaxTokens(c.MaxOutputTokens))
	}
	return anthropic.New(f.apiKey, opts...), nil
}
