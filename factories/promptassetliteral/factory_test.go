// SPDX-License-Identifier: Apache-2.0

package promptassetliteral

import (
	"context"
	"strings"
	"testing"
)

func TestFactory_BuildsText(t *testing.T) {
	f := NewFactory("prompt.sys@1.0.0")
	s, err := f.Build(context.Background(), map[string]any{"text": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello" {
		t.Fatalf("got=%q", s)
	}
}

func TestFactory_RejectsEmptyText(t *testing.T) {
	f := NewFactory("prompt.sys@1.0.0")
	_, err := f.Build(context.Background(), map[string]any{"text": ""})
	if err == nil || !strings.Contains(err.Error(), "text") {
		t.Fatalf("err=%v", err)
	}
}

func TestFactory_RejectsMissingText(t *testing.T) {
	f := NewFactory("prompt.sys@1.0.0")
	_, err := f.Build(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}
