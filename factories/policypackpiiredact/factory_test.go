// SPDX-License-Identifier: Apache-2.0

package policypackpiiredact

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
)

func msgInput(text string) hooks.PolicyInput {
	return hooks.PolicyInput{
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: text}},
		}},
	}
}

func TestPolicy_LogsOnEmailMedium(t *testing.T) {
	pp, err := NewFactory("policypack.pii-redaction@1.0.0").Build(context.Background(), map[string]any{"strictness": "medium"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := pp.Hook.Evaluate(context.Background(), hooks.PhasePreLLMInput, msgInput("contact me at foo@bar.com"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != hooks.VerdictLog {
		t.Fatalf("verdict=%v", d.Verdict)
	}
	if d.Metadata == nil || d.Metadata["pii.matches"] == nil {
		t.Fatalf("missing metadata: %+v", d.Metadata)
	}
}

func TestPolicy_DeniesSSNHigh(t *testing.T) {
	pp, _ := NewFactory("policypack.pii-redaction@1.0.0").Build(context.Background(), map[string]any{"strictness": "high"})
	d, _ := pp.Hook.Evaluate(context.Background(), hooks.PhasePreLLMInput, msgInput("SSN 123-45-6789"))
	if d.Verdict != hooks.VerdictDeny {
		t.Fatalf("verdict=%v", d.Verdict)
	}
}

func TestPolicy_AllowsClean(t *testing.T) {
	pp, _ := NewFactory("policypack.pii-redaction@1.0.0").Build(context.Background(), map[string]any{"strictness": "low"})
	d, _ := pp.Hook.Evaluate(context.Background(), hooks.PhasePreLLMInput, msgInput("just a normal sentence"))
	if d.Verdict != hooks.VerdictAllow {
		t.Fatalf("verdict=%v", d.Verdict)
	}
}

func TestPolicy_RejectsBadStrictness(t *testing.T) {
	_, err := NewFactory("policypack.pii-redaction@1.0.0").Build(context.Background(), map[string]any{"strictness": "nuclear"})
	if err == nil {
		t.Fatal("expected error")
	}
}
