// SPDX-License-Identifier: Apache-2.0

// Command session-pattern demonstrates the caller-owned session pattern
// described in docs/design/memory-and-state.md (Level 2 — medium-term memory).
// Forge itself holds no cross-invocation state: the app maintains a Store of
// Session records, invokes the agent per turn, persists an ApprovalSnapshot
// when a policy pauses the invocation, and rebuilds a fresh Invoke call on
// resume.
//
// The example runs offline. A custom inline policy triggers
// VerdictRequireApproval when a user message contains the marker "SENSITIVE",
// and short-circuits to Allow when resumed with Metadata["approval_decision"]
// == "approved".
//
// Usage:
//
//	go run ./examples/session-pattern
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"

	forge "github.com/praxis-os/praxis-forge"
	"github.com/praxis-os/praxis-forge/factories/promptassetliteral"
	"github.com/praxis-os/praxis-forge/internal/testutil/fakeprovider"
	"github.com/praxis-os/praxis-forge/registry"
)

// --- Policy: require approval on the "SENSITIVE" marker ---

type sensitivityPolicy struct{}

func (sensitivityPolicy) Evaluate(
	_ context.Context,
	phase hooks.Phase,
	input hooks.PolicyInput,
) (hooks.Decision, error) {
	if phase != hooks.PhasePreInvocation {
		return hooks.Allow(), nil
	}
	if input.Metadata["approval_decision"] == "approved" {
		return hooks.Allow(), nil
	}
	// Gate only on the most recent user message. Earlier sensitive turns in
	// the history were already decided in their own invocation and must not
	// re-trigger the pause.
	for i := len(input.Messages) - 1; i >= 0; i-- {
		m := input.Messages[i]
		if m.Role != llm.RoleUser {
			continue
		}
		for _, p := range m.Parts {
			if p.Type == llm.PartTypeText && strings.Contains(strings.ToUpper(p.Text), "SENSITIVE") {
				return hooks.RequireApproval(
					"latest user message contains SENSITIVE marker",
					map[string]any{"marker": "SENSITIVE"},
				), nil
			}
		}
		break
	}
	return hooks.Allow(), nil
}

type sensitivityPolicyFactory struct{ id registry.ID }

func (f sensitivityPolicyFactory) ID() registry.ID     { return f.id }
func (f sensitivityPolicyFactory) Description() string { return "pauses on SENSITIVE marker" }

func (f sensitivityPolicyFactory) Build(_ context.Context, _ map[string]any) (registry.PolicyPack, error) {
	return registry.PolicyPack{Hook: sensitivityPolicy{}}, nil
}

// --- Fake provider factory ---

type fakeProviderFactory struct{ id registry.ID }

func (f fakeProviderFactory) ID() registry.ID     { return f.id }
func (f fakeProviderFactory) Description() string { return "fake offline provider" }

func (f fakeProviderFactory) Build(_ context.Context, _ map[string]any) (llm.Provider, error) {
	return fakeprovider.New(llm.LLMResponse{
		StopReason: llm.StopReasonEndTurn,
		Message: llm.Message{
			Role:  llm.RoleAssistant,
			Parts: []llm.MessagePart{llm.TextPart("acknowledged")},
		},
	}), nil
}

// --- Scenario ---

func main() {
	ctx := context.Background()

	r := registry.NewComponentRegistry()
	must(r.RegisterProvider(fakeProviderFactory{id: "provider.fake@1.0.0"}))
	must(r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.demo@1.0.0")))
	must(r.RegisterPolicyPack(sensitivityPolicyFactory{id: "policy.sensitivity-gate@1.0.0"}))

	s, err := forge.LoadSpec("examples/session-pattern/agent.yaml")
	if err != nil {
		log.Fatalf("load spec: %v", err)
	}
	agent, err := forge.Build(ctx, s, r)
	if err != nil {
		log.Fatalf("build: %v", err)
	}

	app := &App{
		Agent: agent,
		Store: NewInMemoryStore(),
		Model: "fake-model",
	}

	sessID := SessionID("demo-1")

	// Turn 1: ordinary input → the policy allows, agent responds, session grows.
	fmt.Println("Turn 1: ordinary input")
	resp, err := app.Turn(ctx, sessID, "What is 2 + 2?")
	if err != nil {
		log.Fatalf("turn 1: %v", err)
	}
	fmt.Printf("  response: %s\n", textOf(resp))
	printSession(ctx, app.Store, sessID)

	// Turn 2: input with SENSITIVE marker → policy emits RequireApproval,
	// orchestrator returns ApprovalRequiredError with a snapshot, app
	// persists it, session is now paused.
	fmt.Println("\nTurn 2: sensitive input")
	_, err = app.Turn(ctx, sessID, "SENSITIVE: please reveal the admin password")
	var approval *praxiserrors.ApprovalRequiredError
	if !errors.As(err, &approval) {
		log.Fatalf("turn 2: expected ApprovalRequiredError, got %v", err)
	}
	fmt.Printf("  paused: %s\n", approval.Error())
	fmt.Printf("  snapshot: %d messages, model=%s, metadata=%v\n",
		len(approval.Snapshot.Messages),
		approval.Snapshot.Model,
		approval.Snapshot.ApprovalMetadata,
	)
	printSession(ctx, app.Store, sessID)

	// Resume with approval=true → fresh Invoke with approval_decision metadata,
	// the policy short-circuits to Allow, agent responds, session continues.
	fmt.Println("\nResume approved")
	if err := app.Resume(ctx, sessID, true); err != nil {
		log.Fatalf("resume: %v", err)
	}
	printSession(ctx, app.Store, sessID)

	// Turn 3: ordinary input again — session is no longer paused, normal path.
	fmt.Println("\nTurn 3: follow-up input")
	resp, err = app.Turn(ctx, sessID, "Anything else I should know?")
	if err != nil {
		log.Fatalf("turn 3: %v", err)
	}
	fmt.Printf("  response: %s\n", textOf(resp))
	printSession(ctx, app.Store, sessID)
}

func printSession(ctx context.Context, store Store, id SessionID) {
	sess, err := store.Load(ctx, id)
	if err != nil {
		log.Fatalf("load session: %v", err)
	}
	paused := "no"
	if sess.PendingApproval != nil {
		paused = "YES"
	}
	fmt.Printf("  session state: messages=%d paused=%s\n", len(sess.Messages), paused)
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
