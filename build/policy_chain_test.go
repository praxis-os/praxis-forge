// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
)

type fixedHook struct {
	decision hooks.Decision
	calls    *int
}

func (f fixedHook) Evaluate(_ context.Context, _ hooks.Phase, _ hooks.PolicyInput) (hooks.Decision, error) {
	*f.calls++
	return f.decision, nil
}

func TestPolicyChain_AllAllowContinues(t *testing.T) {
	var a, b int
	chain := policyChain{
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &a},
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &b},
	}
	d, err := chain.Evaluate(context.Background(), hooks.PhasePreInvocation, hooks.PolicyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != hooks.VerdictAllow {
		t.Fatalf("verdict=%v", d.Verdict)
	}
	if a != 1 || b != 1 {
		t.Fatalf("calls a=%d b=%d", a, b)
	}
}

func TestPolicyChain_DenyShortCircuits(t *testing.T) {
	var a, b int
	chain := policyChain{
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictDeny, Reason: "nope"}, calls: &a},
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &b},
	}
	d, err := chain.Evaluate(context.Background(), hooks.PhasePreInvocation, hooks.PolicyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != hooks.VerdictDeny {
		t.Fatalf("verdict=%v", d.Verdict)
	}
	if b != 0 {
		t.Fatal("second hook should not run after deny")
	}
}

func TestPolicyChain_RequireApprovalShortCircuits(t *testing.T) {
	var a, b int
	chain := policyChain{
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictRequireApproval}, calls: &a},
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &b},
	}
	d, _ := chain.Evaluate(context.Background(), hooks.PhasePreInvocation, hooks.PolicyInput{})
	if d.Verdict != hooks.VerdictRequireApproval || b != 0 {
		t.Fatalf("verdict=%v b=%d", d.Verdict, b)
	}
}

func TestPolicyChain_LogContinues(t *testing.T) {
	var a, b int
	chain := policyChain{
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictLog}, calls: &a},
		fixedHook{decision: hooks.Decision{Verdict: hooks.VerdictAllow}, calls: &b},
	}
	d, _ := chain.Evaluate(context.Background(), hooks.PhasePreInvocation, hooks.PolicyInput{})
	if d.Verdict != hooks.VerdictAllow || a != 1 || b != 1 {
		t.Fatalf("verdict=%v a=%d b=%d", d.Verdict, a, b)
	}
}
