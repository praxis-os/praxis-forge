// SPDX-License-Identifier: Apache-2.0

// Package filtersecretscrubber is a pre_llm_filter that redacts common secret
// patterns (sk-*, ghp_*, AKIA AWS keys) from outbound LLM messages.
package filtersecretscrubber

import (
	"context"
	"regexp"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
)

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9]{16,}`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
}

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "redacts sk-*, ghp_*, AKIA patterns" }

func (f *Factory) Build(_ context.Context, _ map[string]any) (hooks.PreLLMFilter, error) {
	return &filter{}, nil
}

type filter struct{}

func (f *filter) Filter(_ context.Context, msgs []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	out := make([]llm.Message, len(msgs))
	var decs []hooks.FilterDecision
	for i, m := range msgs {
		newParts := make([]llm.MessagePart, len(m.Parts))
		msgHits := 0
		for j, p := range m.Parts {
			newParts[j] = p
			if p.Type == llm.PartTypeText {
				scrubbed, hits := scrub(p.Text)
				newParts[j].Text = scrubbed
				msgHits += hits
			}
		}
		out[i] = llm.Message{Role: m.Role, Parts: newParts}
		if msgHits > 0 {
			decs = append(decs, hooks.FilterDecision{
				Action: hooks.FilterActionRedact,
				Reason: "secret pattern redacted",
			})
		}
	}
	return out, decs, nil
}

func scrub(s string) (string, int) {
	out := s
	hits := 0
	for _, p := range patterns {
		loc := p.FindAllStringIndex(out, -1)
		if len(loc) > 0 {
			out = p.ReplaceAllString(out, "[REDACTED]")
			hits += len(loc)
		}
	}
	return out, hits
}
