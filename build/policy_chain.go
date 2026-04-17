// SPDX-License-Identifier: Apache-2.0

// Package build resolves a validated AgentSpec against a ComponentRegistry,
// composes multi-component chains into praxis single-instance hooks, and
// materializes a *BuiltAgent backed by a *orchestrator.Orchestrator.
package build

import (
	"context"

	"github.com/praxis-os/praxis/hooks"
)

// policyChain fans a single praxis PolicyHook call across multiple hooks with
// short-circuit semantics. VerdictDeny / VerdictRequireApproval return at
// once; VerdictLog records and continues; VerdictAllow / VerdictContinue
// continue without recording.
type policyChain []hooks.PolicyHook

func (c policyChain) Evaluate(ctx context.Context, phase hooks.Phase, in hooks.PolicyInput) (hooks.Decision, error) {
	for _, h := range c {
		d, err := h.Evaluate(ctx, phase, in)
		if err != nil {
			return hooks.Decision{}, err
		}
		switch d.Verdict {
		case hooks.VerdictDeny, hooks.VerdictRequireApproval:
			return d, nil
		case hooks.VerdictLog, hooks.VerdictAllow, hooks.VerdictContinue:
			// keep going
		default:
			return d, nil
		}
	}
	return hooks.Decision{Verdict: hooks.VerdictAllow}, nil
}
