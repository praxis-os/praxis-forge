// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"

	praxis "github.com/praxis-os/praxis"

	"github.com/praxis-os/praxis-forge/build"
	"github.com/praxis-os/praxis-forge/manifest"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis-forge/spec"
)

// BuiltAgent is the result of a successful Build. Its Invoke is a stateless
// pass-through to the embedded Orchestrator; conversation history lives in
// the caller.
type BuiltAgent struct {
	inner *build.BuiltAgent
}

func (b *BuiltAgent) Invoke(ctx context.Context, req praxis.InvocationRequest) (*praxis.InvocationResult, error) {
	return b.inner.Orchestrator.Invoke(ctx, req)
}

func (b *BuiltAgent) Manifest() manifest.Manifest { return b.inner.Manifest }

// SystemPrompt returns the resolved system prompt text produced by the
// registered prompt_asset factory. Callers may pass it as
// InvocationRequest.SystemPrompt.
func (b *BuiltAgent) SystemPrompt() string { return b.inner.SystemPrompt }

// LoadSpec reads and decodes an AgentSpec YAML file.
func LoadSpec(path string) (*spec.AgentSpec, error) {
	return spec.LoadSpec(path)
}

// Build validates the spec, freezes the registry, resolves every component,
// composes the kernel options, and materializes a BuiltAgent.
func Build(ctx context.Context, s *spec.AgentSpec, r *registry.ComponentRegistry, opts ...Option) (*BuiltAgent, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	inner, err := build.Build(ctx, s, r)
	if err != nil {
		return nil, err
	}
	return &BuiltAgent{inner: inner}, nil
}

// Option is a build-time knob for forge itself (distinct from kernel options).
type Option func(*options)

type options struct {
	// Reserved for Phase 2. Phase 1 has no knobs but keeps the type shape
	// stable so callers can adopt now.
}
