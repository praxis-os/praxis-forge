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

// NormalizedSpec returns the canonical merge result that drove the build.
// It includes the flattened AgentSpec, resolved extends chain, overlay
// attribution, and per-field provenance. The returned pointer aliases
// internal state and should be treated as read-only.
func (b *BuiltAgent) NormalizedSpec() *spec.NormalizedSpec { return b.inner.NormalizedSpec }

// LoadSpec reads and decodes an AgentSpec YAML file.
func LoadSpec(path string) (*spec.AgentSpec, error) {
	return spec.LoadSpec(path)
}

// LoadOverlay reads and decodes an AgentOverlay YAML file. It is a
// re-export of spec.LoadOverlay for callers that import only the forge
// package.
func LoadOverlay(path string) (*spec.AgentOverlay, error) {
	return spec.LoadOverlay(path)
}

// LoadOverlays loads each overlay path in turn, returning a slice in the
// same order. If any load fails, returns the error with no partial result.
// This is a convenience helper for the common case of loading multiple
// overlays without writing the loop.
func LoadOverlays(paths ...string) ([]*spec.AgentOverlay, error) {
	overlays := make([]*spec.AgentOverlay, len(paths))
	for i, path := range paths {
		ov, err := LoadOverlay(path)
		if err != nil {
			return nil, err
		}
		overlays[i] = ov
	}
	return overlays, nil
}

// Build validates the spec, freezes the registry, resolves every component,
// composes the kernel options, and materializes a BuiltAgent.
func Build(ctx context.Context, s *spec.AgentSpec, r *registry.ComponentRegistry, opts ...Option) (*BuiltAgent, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	// Normalize: merge extends chain and apply overlays.
	ns, err := spec.Normalize(ctx, s, o.overlays, o.store)
	if err != nil {
		return nil, err
	}

	inner, err := build.Build(ctx, ns, r)
	if err != nil {
		return nil, err
	}
	return &BuiltAgent{inner: inner}, nil
}

// Option is a build-time knob for forge itself (distinct from kernel options).
type Option func(*options)

type options struct {
	overlays []*spec.AgentOverlay
	store    spec.SpecStore
}

// WithOverlays appends overlays to the build. Overlays are applied in slice
// order (last wins). Multiple WithOverlays calls accumulate. Passing nil or
// no overlays is a no-op.
func WithOverlays(overlays ...*spec.AgentOverlay) Option {
	return func(o *options) {
		o.overlays = append(o.overlays, overlays...)
	}
}

// WithSpecStore configures the SpecStore for extends resolution. Required
// whenever spec declares Extends. If Extends is present but no SpecStore
// is configured, Normalize returns ErrNoSpecStore.
func WithSpecStore(store spec.SpecStore) Option {
	return func(o *options) {
		o.store = store
	}
}
