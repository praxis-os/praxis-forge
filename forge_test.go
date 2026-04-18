// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
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
	"github.com/praxis-os/praxis-forge/spec"
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

// TestForge_FullSlice_Offline tests the Phase 1 path without overlays/extends.
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

// nolint:gocyclo // integration test with many assertions
func TestForge_ExtendsAndOverlays_Offline(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	canned := llm.LLMResponse{
		StopReason: llm.StopReasonEndTurn,
		Message: llm.Message{
			Role:  llm.RoleAssistant,
			Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: "response"}},
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

	// Load base spec that extends parent-direct
	base, err := forge.LoadSpec("testdata/extends_overlays/base.yaml")
	if err != nil {
		t.Fatalf("load base spec: %v", err)
	}

	// Load parent specs and create MapSpecStore
	parentDirect, err := forge.LoadSpec("testdata/extends_overlays/parent-direct.yaml")
	if err != nil {
		t.Fatalf("load parent-direct spec: %v", err)
	}
	parentGrand, err := forge.LoadSpec("testdata/extends_overlays/parent-grand.yaml")
	if err != nil {
		t.Fatalf("load parent-grand spec: %v", err)
	}

	store := spec.MapSpecStore{
		"forge.test.grandparent@1.0.0":   parentGrand,
		"forge.test.parent-direct@1.0.0": parentDirect,
	}

	// Load overlays
	overlays, err := forge.LoadOverlays(
		"testdata/extends_overlays/overlay-1.yaml",
		"testdata/extends_overlays/overlay-2.yaml",
	)
	if err != nil {
		t.Fatalf("load overlays: %v", err)
	}

	// Build with extends and overlays
	built, err := forge.Build(context.Background(), base, r,
		forge.WithSpecStore(store),
		forge.WithOverlays(overlays...),
	)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Assert manifest carries both parents in root-first order
	m := built.Manifest()
	if len(m.ExtendsChain) != 2 {
		t.Errorf("ExtendsChain length: want 2, got %d: %v", len(m.ExtendsChain), m.ExtendsChain)
	}
	if len(m.ExtendsChain) >= 1 && m.ExtendsChain[0] != "forge.test.grandparent@1.0.0" {
		t.Errorf("ExtendsChain[0]: want grandparent, got %s", m.ExtendsChain[0])
	}
	if len(m.ExtendsChain) >= 2 && m.ExtendsChain[1] != "forge.test.parent-direct@1.0.0" {
		t.Errorf("ExtendsChain[1]: want parent-direct, got %s", m.ExtendsChain[1])
	}

	// Assert manifest carries 2 overlay attributions
	if len(m.Overlays) != 2 {
		t.Errorf("Overlays length: want 2, got %d: %v", len(m.Overlays), m.Overlays)
	}
	if len(m.Overlays) >= 1 && m.Overlays[0].Name != "overlay-1-provider-replacement" {
		t.Errorf("Overlays[0].Name: want overlay-1-provider-replacement, got %s", m.Overlays[0].Name)
	}
	if len(m.Overlays) >= 2 && m.Overlays[1].Name != "overlay-2-budget-modification" {
		t.Errorf("Overlays[1].Name: want overlay-2-budget-modification, got %s", m.Overlays[1].Name)
	}

	// Assert NormalizedSpec accessor is present and returns the spec used in build
	ns := built.NormalizedSpec()
	if ns == nil {
		t.Fatal("NormalizedSpec() returned nil")
	}

	// Assert NormalizedSpec carries the merged provider from overlays
	if ns.Spec.Provider.Ref != "provider.fake@1.0.0" {
		t.Errorf("Provider.Ref: want provider.fake@1.0.0, got %s", ns.Spec.Provider.Ref)
	}

	// Assert NormalizedSpec carries the overlay-modified budget
	if ns.Spec.Budget == nil {
		t.Fatal("Budget is nil in NormalizedSpec")
	}
	if ns.Spec.Budget.Overrides.MaxToolCalls != 10 {
		t.Errorf("Budget.Overrides.MaxToolCalls: want 10, got %d", ns.Spec.Budget.Overrides.MaxToolCalls)
	}
	if ns.Spec.Budget.Overrides.MaxInputTokens != 50000 {
		t.Errorf("Budget.Overrides.MaxInputTokens: want 50000, got %d", ns.Spec.Budget.Overrides.MaxInputTokens)
	}

	// Assert ExtendsChain is empty on the merged spec (flattened)
	if len(ns.Spec.Extends) != 0 {
		t.Errorf("Extends should be empty after merge: %v", ns.Spec.Extends)
	}

	// Assert overlays are recorded in NormalizedSpec
	if len(ns.Overlays) != 2 {
		t.Errorf("NormalizedSpec.Overlays length: want 2, got %d", len(ns.Overlays))
	}

	// Negative test: attempting to build without specifying WithSpecStore when extends present
	base2, err := forge.LoadSpec("testdata/extends_overlays/base.yaml")
	if err != nil {
		t.Fatalf("load base spec for negative test: %v", err)
	}
	_, err = forge.Build(context.Background(), base2, r)
	if err == nil {
		t.Fatal("expected error when Extends present but no SpecStore provided")
	}
	if !errors.Is(err, spec.ErrNoSpecStore) {
		t.Errorf("expected ErrNoSpecStore, got: %v", err)
	}
}

func mustRegister(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("register: %v", err)
	}
}
