// SPDX-License-Identifier: Apache-2.0

package filteroutputtruncate

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/tools"
)

func TestFilter_Truncates(t *testing.T) {
	f, _ := NewFactory("filter.output-truncate@1.0.0").Build(context.Background(), map[string]any{"maxBytes": 16})
	r := tools.ToolResult{Content: strings.Repeat("a", 100)}
	out, decs, err := f.Filter(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Content) > 16 {
		t.Fatalf("len=%d", len(out.Content))
	}
	if len(decs) == 0 || decs[0].Action != hooks.FilterActionLog {
		t.Fatalf("decs=%v", decs)
	}
}

func TestFilter_PassThrough(t *testing.T) {
	f, _ := NewFactory("filter.output-truncate@1.0.0").Build(context.Background(), map[string]any{"maxBytes": 100})
	r := tools.ToolResult{Content: "short"}
	out, decs, err := f.Filter(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if out.Content != "short" {
		t.Fatalf("out=%q", out.Content)
	}
	if len(decs) > 0 && decs[0].Action == hooks.FilterActionLog {
		t.Fatal("unexpected log dec")
	}
}

func TestFilter_RequiresMaxBytes(t *testing.T) {
	_, err := NewFactory("filter.output-truncate@1.0.0").Build(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
