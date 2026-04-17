// SPDX-License-Identifier: Apache-2.0

package telemetryprofileslog

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/event"
)

func TestFactory_EmitsSlog(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	f := NewFactory("telemetryprofile.slog@1.0.0", log)
	tp, err := f.Build(context.Background(), map[string]any{"level": "info"})
	if err != nil {
		t.Fatal(err)
	}
	ev := event.InvocationEvent{Type: "invocation.started"}
	if err := tp.Emitter.Emit(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "invocation.started") {
		t.Fatalf("buf=%q", buf.String())
	}
}

func TestEnricher_ReadsContext(t *testing.T) {
	f := NewFactory("telemetryprofile.slog@1.0.0", nil)
	tp, _ := f.Build(context.Background(), nil)
	ctx := context.WithValue(context.Background(), TenantKey, "acme")
	attrs := tp.Enricher.Enrich(ctx)
	if attrs["tenant"] != "acme" {
		t.Fatalf("attrs=%+v", attrs)
	}
}

func TestFactory_RejectsBadLevel(t *testing.T) {
	f := NewFactory("telemetryprofile.slog@1.0.0", nil)
	_, err := f.Build(context.Background(), map[string]any{"level": "trace"})
	if err == nil {
		t.Fatal("expected error")
	}
}
