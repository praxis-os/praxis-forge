// SPDX-License-Identifier: Apache-2.0

// Package fakeprovider is a deterministic llm.Provider for unit tests. It
// returns the canned LLMResponse passed to New on every Complete call.
package fakeprovider

import (
	"context"

	"github.com/praxis-os/praxis/llm"
)

type Provider struct{ resp llm.LLMResponse }

func New(resp llm.LLMResponse) *Provider { return &Provider{resp: resp} }

func (p *Provider) Name() string                    { return "fake" }
func (p *Provider) SupportsParallelToolCalls() bool { return false }
func (p *Provider) Capabilities() llm.Capabilities  { return llm.Capabilities{} }

func (p *Provider) Complete(_ context.Context, _ llm.LLMRequest) (llm.LLMResponse, error) {
	return p.resp, nil
}

func (p *Provider) Stream(_ context.Context, _ llm.LLMRequest) (<-chan llm.LLMStreamChunk, error) {
	ch := make(chan llm.LLMStreamChunk)
	close(ch)
	return ch, nil
}
