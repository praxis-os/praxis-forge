// SPDX-License-Identifier: Apache-2.0

// Package outputcontractjsonschema is the Phase-3 vertical-slice
// output-contract factory. It accepts a JSON Schema document as a
// decoded Go map and stamps it into OutputContract.Schema. Structural
// well-formedness is checked at build time (non-nil map, at least one
// of $schema/type/properties/$ref at the root). Semantic validation of
// LLM outputs against the schema is explicitly out of scope —
// Phase 3 stays zero-dep and hands the raw schema to consumers.
package outputcontractjsonschema

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis-forge/registry"
)

type Factory struct{ id registry.ID }

// NewFactory constructs a JSON-Schema output-contract factory with the
// given registry id. The id must match `outputcontract.<name>@<semver>`.
func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "JSON Schema output contract (structural validation only)" }

// allowedRootKeys are the keys a valid JSON Schema must expose at least
// one of at the root. Bare validation — no library dependency.
var allowedRootKeys = []string{"$schema", "type", "properties", "$ref"}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.OutputContract, error) {
	raw, ok := cfg["schema"]
	if !ok {
		return registry.OutputContract{}, fmt.Errorf("%s: schema: required", f.id)
	}
	schema, ok := raw.(map[string]any)
	if !ok {
		return registry.OutputContract{}, fmt.Errorf("%s: schema: want map, got %T", f.id, raw)
	}
	if len(schema) == 0 {
		return registry.OutputContract{}, fmt.Errorf("%s: schema: empty; need at least one of %v at the root", f.id, allowedRootKeys)
	}
	// Require at least one well-known root keyword.
	hasKey := false
	for _, k := range allowedRootKeys {
		if _, ok := schema[k]; ok {
			hasKey = true
			break
		}
	}
	if !hasKey {
		return registry.OutputContract{}, fmt.Errorf("%s: schema: must contain at least one of %v at the root", f.id, allowedRootKeys)
	}
	return registry.OutputContract{
		Schema: schema,
		Descriptor: registry.OutputContractDescriptor{
			Name:    "json-schema",
			Owner:   "core",
			Summary: "structural JSON Schema; semantic validation deferred",
		},
	}, nil
}
