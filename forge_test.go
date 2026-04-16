// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	praxis "github.com/praxis-os/praxis"

	forge "github.com/praxis-os/praxis-forge"
	"github.com/praxis-os/praxis-forge/factories/budgetprofiledefault"
	"github.com/praxis-os/praxis-forge/factories/credresolverenv"
	"github.com/praxis-os/praxis-forge/factories/filteroutputtruncate"
	"github.com/praxis-os/praxis-forge/factories/filterpathescape"
	"github.com/praxis-os/praxis-forge/factories/filtersecretscrubber"
	"github.com/praxis-os/praxis-forge/factories/identitysignered25519"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
	"github.com/praxis-os/praxis-forge/factories/promptassetliteral"
	"github.com/praxis-os/praxis-forge/factories/telemetryprofileslog"
	"github.com/praxis-os/praxis-forge/internal/testutil/fakeprovider"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
)

// fakeProviderFactory wraps fakeprovider as a forge Provider factory.
type fakeProviderFactory struct {
	id   registry.ID
	resp llm.LLMResponse
}

func (f fakeProviderFactory) ID() registry.ID     { return f.id }
func (f fakeProviderFactory) Description() string { return "fake" }
func (f fakeProviderFactory) Build(context.Context, map[string]any) (llm.Provider, error) {
	return fakeprovider.New(f.resp), nil
}

func TestForge_FullSlice_Offline(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	canned := llm.LLMResponse{
		StopReason: llm.StopReasonEndTurn,
		Message: llm.Message{
			Role:  llm.RoleAssistant,
			Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: "hi"}},
		},
	}

	r := registry.NewComponentRegistry()
	mustRegister(t, r.RegisterProvider(fakeProviderFactory{
		id:   "provider.fake@1.0.0",
		resp: canned,
	}))
	mustRegister(t, r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.sys@1.0.0")))
	mustRegister(t, r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0")))
	mustRegister(t, r.RegisterPreLLMFilter(filtersecretscrubber.NewFactory("filter.secret-scrubber@1.0.0")))
	mustRegister(t, r.RegisterPreToolFilter(filterpathescape.NewFactory("filter.path-escape@1.0.0")))
	mustRegister(t, r.RegisterPostToolFilter(filteroutputtruncate.NewFactory("filter.output-truncate@1.0.0")))
	mustRegister(t, r.RegisterBudgetProfile(budgetprofiledefault.NewFactory("budgetprofile.default-tier1@1.0.0")))
	mustRegister(t, r.RegisterTelemetryProfile(telemetryprofileslog.NewFactory("telemetryprofile.slog@1.0.0", nil)))
	mustRegister(t, r.RegisterCredentialResolver(credresolverenv.NewFactory("credresolver.env@1.0.0")))
	mustRegister(t, r.RegisterIdentitySigner(identitysignered25519.NewFactory("identitysigner.ed25519@1.0.0", priv)))

	s, err := forge.LoadSpec("testdata/agent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	b, err := forge.Build(context.Background(), s, r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Manifest should include all 10 non-tool kinds.
	m := b.Manifest()
	got := map[string]bool{}
	for _, rc := range m.Resolved {
		got[rc.Kind] = true
	}
	wantKinds := []string{
		"provider", "prompt_asset", "policy_pack",
		"pre_llm_filter", "pre_tool_filter", "post_tool_filter",
		"budget_profile", "telemetry_profile", "credential_resolver", "identity_signer",
	}
	for _, k := range wantKinds {
		if !got[k] {
			t.Errorf("manifest missing kind %q: %+v", k, m.Resolved)
		}
	}

	// System prompt resolved through prompt.literal@1.
	if b.SystemPrompt() != "You are a test agent." {
		t.Errorf("systemPrompt=%q", b.SystemPrompt())
	}

	// Registry now frozen.
	if err := r.RegisterProvider(fakeProviderFactory{id: "provider.other@1.0.0"}); err == nil {
		t.Fatal("expected registry frozen after Build")
	}

	// Invoke round-trip through the fake provider.
	res, err := b.Invoke(context.Background(), praxis.InvocationRequest{
		Model:        "fake",
		SystemPrompt: b.SystemPrompt(),
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: "ping"}},
		}},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
}

func mustRegister(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("register: %v", err)
	}
}
