// SPDX-License-Identifier: Apache-2.0

// Package policypackpiiredact is a policy_pack that scans message text for
// PII. Strictness "low" logs email/phone; "medium" logs email/phone/SSN/CC;
// "high" denies on SSN/CC.
package policypackpiiredact

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "regex PII detector with tiered strictness" }

type strictness string

const (
	strictLow    strictness = "low"
	strictMedium strictness = "medium"
	strictHigh   strictness = "high"
)

var (
	reEmail = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	rePhone = regexp.MustCompile(`\b\d{3}[-.\s]\d{3}[-.\s]\d{4}\b`)
	reSSN   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	reCC    = regexp.MustCompile(`\b(?:\d[ -]?){13,16}\b`)
)

func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.PolicyPack, error) {
	s, ok := cfg["strictness"].(string)
	if !ok {
		return registry.PolicyPack{}, fmt.Errorf("%s: strictness: required string", f.id)
	}
	var mode strictness
	switch s {
	case "low", "medium", "high":
		mode = strictness(s)
	default:
		return registry.PolicyPack{}, fmt.Errorf("%s: strictness: want low|medium|high, got %q", f.id, s)
	}
	return registry.PolicyPack{
		Hook: &hook{mode: mode},
		Descriptors: []registry.PolicyDescriptor{{
			Name: "pii-redaction", Source: string(f.id), PolicyTags: []string{"pii", "privacy"},
		}},
	}, nil
}

type hook struct{ mode strictness }

func (h *hook) Evaluate(_ context.Context, _ hooks.Phase, in hooks.PolicyInput) (hooks.Decision, error) {
	text := extractText(in)

	var matches []string
	if reEmail.MatchString(text) {
		matches = append(matches, "email")
	}
	if rePhone.MatchString(text) {
		matches = append(matches, "phone")
	}
	if reSSN.MatchString(text) {
		matches = append(matches, "ssn")
	}
	if reCC.MatchString(text) {
		matches = append(matches, "credit_card")
	}

	if len(matches) == 0 {
		return hooks.Decision{Verdict: hooks.VerdictAllow}, nil
	}

	if h.mode == strictHigh {
		for _, m := range matches {
			if m == "ssn" || m == "credit_card" {
				return hooks.Decision{
					Verdict:  hooks.VerdictDeny,
					Reason:   "high-risk PII detected: " + m,
					Metadata: map[string]any{"pii.matches": matches},
				}, nil
			}
		}
	}

	return hooks.Decision{
		Verdict:  hooks.VerdictLog,
		Reason:   "PII detected",
		Metadata: map[string]any{"pii.matches": matches},
	}, nil
}

// extractText flattens every text part of every message in the policy input
// into a newline-joined string the regex detectors can scan.
func extractText(in hooks.PolicyInput) string {
	var b strings.Builder
	if in.SystemPrompt != "" {
		b.WriteString(in.SystemPrompt)
		b.WriteByte('\n')
	}
	for _, m := range in.Messages {
		for _, p := range m.Parts {
			if p.Type == llm.PartTypeText {
				b.WriteString(p.Text)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}
