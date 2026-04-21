// SPDX-License-Identifier: Apache-2.0

// Package skillstructuredoutput is the Phase-3 vertical-slice skill
// factory. It contributes:
//
//   - a fixed prompt fragment instructing the model to emit JSON only
//     matching the required schema, no surrounding prose;
//   - a required policy pack (policypack.pii-redaction@1.0.0) so the
//     structured output path defaults to PII-scrubbed logging;
//   - a required output contract (outputcontract.json-schema@1.0.0)
//     which the consumer supplies with their schema via config.
package skillstructuredoutput

import (
	"context"

	"github.com/praxis-os/praxis-forge/registry"
)

type Factory struct{ id registry.ID }

// NewFactory constructs the structured-output skill factory. The id
// must match `skill.<name>@<semver>`.
func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "forces structured JSON output matching a supplied schema" }

const promptFragment = "Respond with JSON matching the required schema. Do not include prose outside the JSON."

func (f *Factory) Build(_ context.Context, _ map[string]any) (registry.Skill, error) {
	return registry.Skill{
		PromptFragment: promptFragment,
		RequiredPolicies: []registry.RequiredComponent{
			{ID: "policypack.pii-redaction@1.0.0", Config: map[string]any{"strictness": "medium"}},
		},
		RequiredOutputContract: &registry.RequiredComponent{
			ID: "outputcontract.json-schema@1.0.0",
			// No Config: the consumer supplies the schema by declaring
			// spec.outputContract directly, or via another skill layer.
		},
		Descriptor: registry.SkillDescriptor{
			Name:    "structured-output",
			Owner:   "core",
			Summary: "Emit JSON matching a schema; default PII-redaction policy.",
			Tags:    []string{"structured", "json", "governance"},
		},
	}, nil
}
