// SPDX-License-Identifier: Apache-2.0

package filterpathescape

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/tools"
)

func TestFilter_BlocksPathEscape(t *testing.T) {
	f, _ := NewFactory("filter.path-escape@1.0.0").Build(context.Background(), nil)
	call := tools.ToolCall{Name: "read_file", ArgumentsJSON: []byte(`{"path":"../etc/passwd"}`)}
	_, decs, err := f.Filter(context.Background(), call)
	_ = err
	if len(decs) == 0 || decs[0].Action != hooks.FilterActionBlock {
		t.Fatalf("decs=%v", decs)
	}
}

func TestFilter_AllowsCleanPath(t *testing.T) {
	f, _ := NewFactory("filter.path-escape@1.0.0").Build(context.Background(), nil)
	call := tools.ToolCall{Name: "read_file", ArgumentsJSON: []byte(`{"path":"docs/readme.md"}`)}
	_, decs, err := f.Filter(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range decs {
		if d.Action == hooks.FilterActionBlock {
			t.Fatalf("unexpected block: %v", d)
		}
	}
}
