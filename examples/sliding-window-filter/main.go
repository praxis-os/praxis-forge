// SPDX-License-Identifier: Apache-2.0

// Command sliding-window-filter demonstrates caller-owned context-window
// management on top of praxis-forge. It implements a hooks.PreLLMFilter that
// keeps the last N user/assistant/tool messages (system messages are always
// preserved), registers it as a factory, and builds an agent that uses it.
//
// The example stays offline by using the fakeprovider under internal/testutil
// as the llm.Provider. This is the Level-1 pattern described in
// docs/design/memory-and-state.md — forge owns no short-term memory itself;
// users plug the policy in via KindPreLLMFilter.
//
// Usage:
//
//	go run ./examples/sliding-window-filter
package main

import (
	"context"
	"fmt"
	"log"

	praxis "github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"

	forge "github.com/praxis-os/praxis-forge"
	"github.com/praxis-os/praxis-forge/factories/promptassetliteral"
	"github.com/praxis-os/praxis-forge/internal/testutil/fakeprovider"
	"github.com/praxis-os/praxis-forge/registry"
)

// --- Sliding-window PreLLMFilter ---

// slidingWindowFilter keeps only the last MaxTurns non-system messages.
// System messages are always preserved (dropping them would break the agent's
// instructions). The filter is deterministic and allocation-bounded by the
// window size.
type slidingWindowFilter struct{ MaxTurns int }

func (f *slidingWindowFilter) Filter(
	_ context.Context,
	messages []llm.Message,
) ([]llm.Message, []hooks.FilterDecision, error) {
	var system, convo []llm.Message
	for _, m := range messages {
		if m.Role == llm.RoleSystem {
			system = append(system, m)
			continue
		}
		convo = append(convo, m)
	}
	if len(convo) <= f.MaxTurns {
		return messages, nil, nil
	}
	kept := convo[len(convo)-f.MaxTurns:]
	out := make([]llm.Message, 0, len(system)+len(kept))
	out = append(out, system...)
	out = append(out, kept...)
	dropped := len(convo) - len(kept)
	return out, []hooks.FilterDecision{{
		Action: hooks.FilterActionRedact,
		Field:  "messages",
		Reason: fmt.Sprintf("sliding-window truncation: dropped %d oldest turns", dropped),
	}}, nil
}

// slidingWindowFactory adapts slidingWindowFilter to the forge registry.
type slidingWindowFactory struct{ id registry.ID }

func (f slidingWindowFactory) ID() registry.ID     { return f.id }
func (f slidingWindowFactory) Description() string { return "sliding-window filter: keep last N non-system turns" }

func (f slidingWindowFactory) Build(_ context.Context, cfg map[string]any) (hooks.PreLLMFilter, error) {
	n, _ := cfg["maxTurns"].(int)
	if n <= 0 {
		n = 10
	}
	return &slidingWindowFilter{MaxTurns: n}, nil
}

// --- Fake provider factory (offline) ---

type fakeProviderFactory struct{ id registry.ID }

func (f fakeProviderFactory) ID() registry.ID     { return f.id }
func (f fakeProviderFactory) Description() string { return "fake offline provider" }

func (f fakeProviderFactory) Build(_ context.Context, _ map[string]any) (llm.Provider, error) {
	return fakeprovider.New(llm.LLMResponse{
		StopReason: llm.StopReasonEndTurn,
		Message: llm.Message{
			Role:  llm.RoleAssistant,
			Parts: []llm.MessagePart{llm.TextPart("ok")},
		},
	}), nil
}

// --- main ---

func main() {
	ctx := context.Background()

	// 1. Build the registry with our sliding-window filter + the minimum
	//    factories the spec references.
	r := registry.NewComponentRegistry()
	must(r.RegisterProvider(fakeProviderFactory{id: "provider.fake@1.0.0"}))
	must(r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.demo@1.0.0")))
	must(r.RegisterPreLLMFilter(slidingWindowFactory{id: "filter.sliding-window@1.0.0"}))

	// 2. Load the spec that wires the filter into the agent.
	s, err := forge.LoadSpec("examples/sliding-window-filter/agent.yaml")
	if err != nil {
		log.Fatalf("load spec: %v", err)
	}

	// 3. Build. The manifest records the filter as a governed component.
	b, err := forge.Build(ctx, s, r)
	if err != nil {
		log.Fatalf("build: %v", err)
	}
	fmt.Println("manifest components:")
	for _, rc := range b.Manifest().Resolved {
		fmt.Printf("  - %-18s %s\n", rc.Kind, rc.ID)
	}

	// 4. Demonstrate filter behavior directly on a synthetic 25-turn
	//    conversation. This is what forge's preLLMFilterChain would apply
	//    before every LLM call.
	fmt.Println("\nfilter behavior on a synthetic 25-turn conversation:")
	f := &slidingWindowFilter{MaxTurns: 10}
	msgs := synthConversation(25)
	filtered, decisions, err := f.Filter(ctx, msgs)
	if err != nil {
		log.Fatalf("filter: %v", err)
	}
	fmt.Printf("  input:  %d messages\n", len(msgs))
	fmt.Printf("  output: %d messages\n", len(filtered))
	for _, d := range decisions {
		fmt.Printf("  decision: %s %s — %s\n", d.Action, d.Field, d.Reason)
	}

	// 5. Run a real Invoke through the built agent. With the fake provider
	//    we just get "ok" back, but the path exercises the full filter chain.
	res, err := b.Invoke(ctx, praxis.InvocationRequest{
		Model:        "fake-model",
		SystemPrompt: b.SystemPrompt(),
		Messages:     synthConversation(25),
	})
	if err != nil {
		log.Fatalf("invoke: %v", err)
	}
	fmt.Printf("\ninvoke response: %s\n", textOf(res.Response))
}

// synthConversation builds n alternating user/assistant text messages.
func synthConversation(n int) []llm.Message {
	out := make([]llm.Message, 0, n)
	for i := 0; i < n; i++ {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		out = append(out, llm.Message{
			Role:  role,
			Parts: []llm.MessagePart{llm.TextPart(fmt.Sprintf("turn %d", i))},
		})
	}
	return out
}

func textOf(m *llm.Message) string {
	if m == nil {
		return "(nil)"
	}
	for _, p := range m.Parts {
		if p.Type == llm.PartTypeText {
			return p.Text
		}
	}
	return "(no text)"
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
