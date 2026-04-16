// SPDX-License-Identifier: Apache-2.0

// Package filteroutputtruncate is a post_tool_filter that truncates tool
// output (Content field) to a configured maxBytes and emits a Log filter
// decision whenever truncation occurs.
package filteroutputtruncate

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/tools"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "truncates tool output to maxBytes" }

func (f *Factory) Build(_ context.Context, cfg map[string]any) (hooks.PostToolFilter, error) {
	n, err := requireInt(cfg, "maxBytes")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.id, err)
	}
	if n <= 0 {
		return nil, fmt.Errorf("%s: maxBytes must be > 0", f.id)
	}
	return &filter{maxBytes: n}, nil
}

func requireInt(cfg map[string]any, key string) (int, error) {
	raw, ok := cfg[key]
	if !ok {
		return 0, fmt.Errorf("%s: required", key)
	}
	switch x := raw.(type) {
	case int:
		return x, nil
	case int64:
		return int(x), nil
	case float64:
		return int(x), nil
	default:
		return 0, fmt.Errorf("%s: want int, got %T", key, raw)
	}
}

type filter struct{ maxBytes int }

func (f *filter) Filter(_ context.Context, r tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	if len(r.Content) <= f.maxBytes {
		return r, nil, nil
	}
	orig := len(r.Content)
	r.Content = r.Content[:f.maxBytes]
	return r, []hooks.FilterDecision{{
		Action: hooks.FilterActionLog,
		Reason: fmt.Sprintf("truncated tool output from %d to %d bytes", orig, f.maxBytes),
	}}, nil
}
