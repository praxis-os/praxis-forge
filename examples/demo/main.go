//go:build integration

// SPDX-License-Identifier: Apache-2.0

// Command demo exercises the full praxis-forge Phase 1 path against a real
// Anthropic provider. Build-tagged "integration" so plain `go test ./...`
// stays offline.
//
// Usage:
//
//	ANTHROPIC_API_KEY=sk-... go run -tags=integration ./examples/demo
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"log/slog"
	"os"

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
	"github.com/praxis-os/praxis-forge/factories/provideranthropic"
	"github.com/praxis-os/praxis-forge/factories/telemetryprofileslog"
	"github.com/praxis-os/praxis-forge/factories/toolpackhttpget"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY not set")
	}
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	r := registry.NewComponentRegistry()
	must(r.RegisterProvider(provideranthropic.NewFactory("provider.anthropic@1.0.0", apiKey)))
	must(r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.demo-system@1.0.0")))
	must(r.RegisterToolPack(toolpackhttpget.NewFactory("toolpack.http-get@1.0.0")))
	must(r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0")))
	must(r.RegisterPreLLMFilter(filtersecretscrubber.NewFactory("filter.secret-scrubber@1.0.0")))
	must(r.RegisterPreToolFilter(filterpathescape.NewFactory("filter.path-escape@1.0.0")))
	must(r.RegisterPostToolFilter(filteroutputtruncate.NewFactory("filter.output-truncate@1.0.0")))
	must(r.RegisterBudgetProfile(budgetprofiledefault.NewFactory("budgetprofile.default-tier1@1.0.0")))
	must(r.RegisterTelemetryProfile(telemetryprofileslog.NewFactory("telemetryprofile.slog@1.0.0", slog.Default())))
	must(r.RegisterCredentialResolver(credresolverenv.NewFactory("credresolver.env@1.0.0")))
	must(r.RegisterIdentitySigner(identitysignered25519.NewFactory("identitysigner.ed25519@1.0.0", priv)))

	ctx := context.Background()
	s, err := forge.LoadSpec("examples/demo/agent.yaml")
	if err != nil {
		log.Fatalf("load spec: %v", err)
	}

	b, err := forge.Build(ctx, s, r)
	if err != nil {
		log.Fatalf("build: %v", err)
	}

	url := "https://raw.githubusercontent.com/praxis-os/praxis-forge/main/README.md"
	res, err := b.Invoke(ctx, praxis.InvocationRequest{
		Model:        "claude-sonnet-4-5",
		SystemPrompt: b.SystemPrompt(),
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Parts: []llm.MessagePart{{
				Type: llm.PartTypeText,
				Text: fmt.Sprintf("Fetch %s and summarize what praxis-forge does.", url),
			}},
		}},
	})
	if err != nil {
		log.Fatalf("invoke: %v", err)
	}

	if res.Response != nil {
		for _, p := range res.Response.Parts {
			if p.Type == llm.PartTypeText {
				fmt.Printf("response: %s\n", p.Text)
			}
		}
	}
	fmt.Printf("manifest components: %d\n", len(b.Manifest().Resolved))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
