// SPDX-License-Identifier: Apache-2.0

package outputcontractjsonschema

import (
	"context"
	"strings"
	"testing"
)

func TestFactory_BuildsWithValidSchema(t *testing.T) {
	oc, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{
		"schema": map[string]any{
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
			"properties": map[string]any{
				"answer": map[string]any{"type": "string"},
			},
			"required": []any{"answer"},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if oc.Schema["type"] != "object" {
		t.Errorf("Schema.type: %v", oc.Schema["type"])
	}
	if oc.Descriptor.Name == "" {
		t.Error("Descriptor.Name should be set")
	}
}

func TestFactory_RejectsMissingSchema(t *testing.T) {
	_, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "schema: required") {
		t.Fatalf("want 'schema: required', got %v", err)
	}
}

func TestFactory_RejectsNonMapSchema(t *testing.T) {
	_, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{
		"schema": "not-a-map",
	})
	if err == nil || !strings.Contains(err.Error(), "schema:") {
		t.Fatalf("want schema type error, got %v", err)
	}
}

func TestFactory_RejectsStructurallyEmptySchema(t *testing.T) {
	// Empty map is not a valid JSON Schema (must have at least one of
	// $schema/type/properties/$ref at the root).
	_, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{
		"schema": map[string]any{},
	})
	if err == nil || !strings.Contains(err.Error(), "schema:") {
		t.Fatalf("want structural error, got %v", err)
	}
}

func TestFactory_AcceptsTypeOnly(t *testing.T) {
	// A schema containing only "type" is the minimum acceptable form.
	oc, err := NewFactory("outputcontract.json-schema@1.0.0").Build(context.Background(), map[string]any{
		"schema": map[string]any{"type": "string"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if oc.Schema["type"] != "string" {
		t.Errorf("Schema.type: %v", oc.Schema["type"])
	}
}
