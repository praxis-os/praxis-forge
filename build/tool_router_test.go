// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"errors"
	"testing"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

type canned struct{}

func (c canned) Invoke(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	return tools.ToolResult{Status: tools.ToolStatusSuccess, Content: call.Name}, nil
}

func TestToolRouter_DispatchByName(t *testing.T) {
	packA := registry.ToolPack{
		Invoker:     canned{},
		Definitions: []llm.ToolDefinition{{Name: "a"}},
	}
	packB := registry.ToolPack{
		Invoker:     canned{},
		Definitions: []llm.ToolDefinition{{Name: "b"}},
	}
	r, defs, err := newToolRouter([]registry.ToolPack{packA, packB})
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 2 {
		t.Fatalf("defs=%d", len(defs))
	}
	out, err := r.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Content != "a" {
		t.Fatalf("content=%s", out.Content)
	}
}

func TestToolRouter_Collision(t *testing.T) {
	pack := registry.ToolPack{
		Invoker:     canned{},
		Definitions: []llm.ToolDefinition{{Name: "dup"}},
	}
	_, _, err := newToolRouter([]registry.ToolPack{pack, pack})
	if !errors.Is(err, ErrToolNameCollision) {
		t.Fatalf("err=%v", err)
	}
}

func TestToolRouter_Unknown(t *testing.T) {
	pack := registry.ToolPack{
		Invoker:     canned{},
		Definitions: []llm.ToolDefinition{{Name: "a"}},
	}
	r, _, err := newToolRouter([]registry.ToolPack{pack})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{Name: "missing"})
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("err=%v", err)
	}
}
