// SPDX-License-Identifier: Apache-2.0

// Package filterpathescape is a pre_tool_filter that blocks any tool call
// whose serialized JSON arguments contain "../" (parent-dir traversal).
package filterpathescape

import (
	"bytes"
	"context"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/tools"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "blocks ../ path traversal in tool args" }

func (f *Factory) Build(_ context.Context, _ map[string]any) (hooks.PreToolFilter, error) {
	return &filter{}, nil
}

type filter struct{}

var needle = []byte("../")

func (f *filter) Filter(_ context.Context, call tools.ToolCall) (tools.ToolCall, []hooks.FilterDecision, error) {
	if bytes.Contains(call.ArgumentsJSON, needle) {
		return call, []hooks.FilterDecision{{
			Action: hooks.FilterActionBlock,
			Reason: "path traversal '../' in tool arguments",
		}}, nil
	}
	return call, nil, nil
}
