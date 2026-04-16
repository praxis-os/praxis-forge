// SPDX-License-Identifier: Apache-2.0

package provideranthropic

import (
	"context"
	"testing"
)

func TestFactory_Builds(t *testing.T) {
	f := NewFactory("provider.anthropic@1.0.0", "test-api-key")
	p, err := f.Build(context.Background(), map[string]any{
		"model":           "claude-sonnet-4-5",
		"maxOutputTokens": 2048,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("provider nil")
	}
}

func TestFactory_RejectsMissingModel(t *testing.T) {
	f := NewFactory("provider.anthropic@1.0.0", "test")
	_, err := f.Build(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFactory_RejectsMissingAPIKey(t *testing.T) {
	f := NewFactory("provider.anthropic@1.0.0", "")
	_, err := f.Build(context.Background(), map[string]any{"model": "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
