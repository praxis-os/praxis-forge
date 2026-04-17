// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"errors"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

var (
	ErrToolNameCollision = errors.New("tool name collision across tool packs")
	ErrToolNotFound      = errors.New("tool not found in router")
)

type toolRouter struct {
	byName map[string]tools.Invoker
}

func newToolRouter(packs []registry.ToolPack) (*toolRouter, []llm.ToolDefinition, error) {
	r := &toolRouter{byName: map[string]tools.Invoker{}}
	var defs []llm.ToolDefinition
	for _, p := range packs {
		for _, def := range p.Definitions {
			if _, exists := r.byName[def.Name]; exists {
				return nil, nil, fmt.Errorf("%w: %s", ErrToolNameCollision, def.Name)
			}
			r.byName[def.Name] = p.Invoker
			defs = append(defs, def)
		}
	}
	return r, defs, nil
}

func (r *toolRouter) Invoke(ctx context.Context, ictx tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	inv, ok := r.byName[call.Name]
	if !ok {
		return tools.ToolResult{}, fmt.Errorf("%w: %s", ErrToolNotFound, call.Name)
	}
	return inv.Invoke(ctx, ictx, call)
}
