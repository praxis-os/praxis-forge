// SPDX-License-Identifier: Apache-2.0

package filtersecretscrubber

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
)

func TestFilter_RedactsAnthropicKey(t *testing.T) {
	f, _ := NewFactory("filter.secret-scrubber@1.0.0").Build(context.Background(), nil)
	msgs := []llm.Message{{
		Role: llm.RoleUser,
		Parts: []llm.MessagePart{{
			Type: llm.PartTypeText,
			Text: "please call sk-abc123xyz456789012345678",
		}},
	}}
	out, decs, err := f.Filter(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out[0].Parts[0].Text, "sk-abc123") {
		t.Fatalf("leaked: %s", out[0].Parts[0].Text)
	}
	if len(decs) == 0 || decs[0].Action != hooks.FilterActionRedact {
		t.Fatalf("decs=%v", decs)
	}
}

func TestFilter_PassesCleanMessages(t *testing.T) {
	f, _ := NewFactory("filter.secret-scrubber@1.0.0").Build(context.Background(), nil)
	msgs := []llm.Message{{
		Role:  llm.RoleUser,
		Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: "hello world"}},
	}}
	_, decs, err := f.Filter(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range decs {
		if d.Action == hooks.FilterActionRedact {
			t.Fatalf("unexpected redact: %v", d)
		}
	}
}
