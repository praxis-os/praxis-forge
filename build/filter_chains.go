// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

// ErrFilterBlocked is returned by any filter chain when a stage is aborted
// by FilterActionBlock.
var ErrFilterBlocked = fmt.Errorf("filter: stage blocked")

func blocked(reason string) error {
	if reason == "" {
		return ErrFilterBlocked
	}
	return fmt.Errorf("%w: %s", ErrFilterBlocked, reason)
}

type preLLMFilterChain []hooks.PreLLMFilter

func (c preLLMFilterChain) Filter(ctx context.Context, msgs []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	var all []hooks.FilterDecision
	cur := msgs
	for _, f := range c {
		out, decs, err := f.Filter(ctx, cur)
		if err != nil {
			return nil, nil, err
		}
		all = append(all, decs...)
		if blockingDec(decs) != nil {
			return nil, all, blocked(blockingDec(decs).Reason)
		}
		cur = out
	}
	return cur, all, nil
}

type preToolFilterChain []hooks.PreToolFilter

func (c preToolFilterChain) Filter(ctx context.Context, call tools.ToolCall) (tools.ToolCall, []hooks.FilterDecision, error) {
	var all []hooks.FilterDecision
	cur := call
	for _, f := range c {
		out, decs, err := f.Filter(ctx, cur)
		if err != nil {
			return tools.ToolCall{}, nil, err
		}
		all = append(all, decs...)
		if blockingDec(decs) != nil {
			return tools.ToolCall{}, all, blocked(blockingDec(decs).Reason)
		}
		cur = out
	}
	return cur, all, nil
}

type postToolFilterChain []hooks.PostToolFilter

func (c postToolFilterChain) Filter(ctx context.Context, r tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	var all []hooks.FilterDecision
	cur := r
	for _, f := range c {
		out, decs, err := f.Filter(ctx, cur)
		if err != nil {
			return tools.ToolResult{}, nil, err
		}
		all = append(all, decs...)
		if blockingDec(decs) != nil {
			return tools.ToolResult{}, all, blocked(blockingDec(decs).Reason)
		}
		cur = out
	}
	return cur, all, nil
}

func blockingDec(decs []hooks.FilterDecision) *hooks.FilterDecision {
	for i := range decs {
		if decs[i].Action == hooks.FilterActionBlock {
			return &decs[i]
		}
	}
	return nil
}
