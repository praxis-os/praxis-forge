// SPDX-License-Identifier: Apache-2.0

package fakeprovider

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/llm"
)

func TestProvider_CompleteReturnsCanned(t *testing.T) {
	canned := llm.LLMResponse{
		StopReason: llm.StopReasonEndTurn,
		Message: llm.Message{
			Role:  llm.RoleAssistant,
			Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: "hi"}},
		},
	}
	p := New(canned)
	resp, err := p.Complete(context.Background(), llm.LLMRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Message.Parts) == 0 || resp.Message.Parts[0].Text != "hi" {
		t.Fatalf("parts=%+v", resp.Message.Parts)
	}
}
