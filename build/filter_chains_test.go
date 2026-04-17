// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

type fakePreLLM struct {
	action hooks.FilterAction
	reason string
	calls  *int
}

func (f fakePreLLM) Filter(_ context.Context, msgs []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	*f.calls++
	return msgs, []hooks.FilterDecision{{Action: f.action, Reason: f.reason}}, nil
}

func TestPreLLMFilterChain_PassPassBothRun(t *testing.T) {
	var a, b int
	chain := preLLMFilterChain{
		fakePreLLM{action: hooks.FilterActionPass, calls: &a},
		fakePreLLM{action: hooks.FilterActionPass, calls: &b},
	}
	_, decs, err := chain.Filter(context.Background(), []llm.Message{})
	if err != nil {
		t.Fatal(err)
	}
	if a != 1 || b != 1 || len(decs) != 2 {
		t.Fatalf("a=%d b=%d decs=%d", a, b, len(decs))
	}
}

func TestPreLLMFilterChain_BlockShortCircuits(t *testing.T) {
	var a, b int
	chain := preLLMFilterChain{
		fakePreLLM{action: hooks.FilterActionBlock, reason: "no", calls: &a},
		fakePreLLM{action: hooks.FilterActionPass, calls: &b},
	}
	_, decs, err := chain.Filter(context.Background(), []llm.Message{})
	if err == nil {
		t.Fatal("expected block error")
	}
	if b != 0 {
		t.Fatal("second filter should not run after block")
	}
	if len(decs) == 0 || decs[0].Action != hooks.FilterActionBlock {
		t.Fatalf("decs=%v", decs)
	}
}

type fakePreTool struct {
	action hooks.FilterAction
	calls  *int
}

func (f fakePreTool) Filter(_ context.Context, call tools.ToolCall) (tools.ToolCall, []hooks.FilterDecision, error) {
	*f.calls++
	return call, []hooks.FilterDecision{{Action: f.action}}, nil
}

func TestPreToolFilterChain_Block(t *testing.T) {
	var a, b int
	chain := preToolFilterChain{
		fakePreTool{action: hooks.FilterActionBlock, calls: &a},
		fakePreTool{action: hooks.FilterActionPass, calls: &b},
	}
	_, _, err := chain.Filter(context.Background(), tools.ToolCall{})
	if err == nil || b != 0 {
		t.Fatalf("err=%v b=%d", err, b)
	}
}

type fakePostTool struct {
	action hooks.FilterAction
	calls  *int
}

func (f fakePostTool) Filter(_ context.Context, r tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	*f.calls++
	return r, []hooks.FilterDecision{{Action: f.action}}, nil
}

func TestPostToolFilterChain_Block(t *testing.T) {
	var a, b int
	chain := postToolFilterChain{
		fakePostTool{action: hooks.FilterActionBlock, calls: &a},
		fakePostTool{action: hooks.FilterActionPass, calls: &b},
	}
	_, _, err := chain.Filter(context.Background(), tools.ToolResult{})
	if err == nil || b != 0 {
		t.Fatalf("err=%v b=%d", err, b)
	}
}
