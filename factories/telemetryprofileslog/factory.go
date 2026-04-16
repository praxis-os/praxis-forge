// SPDX-License-Identifier: Apache-2.0

// Package telemetryprofileslog is a telemetry_profile factory that emits one
// slog record per lifecycle event and extracts tenant/user attributes from
// context.
package telemetryprofileslog

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/telemetry"
)

type Factory struct {
	id  registry.ID
	log *slog.Logger
}

func NewFactory(id registry.ID, log *slog.Logger) *Factory {
	if log == nil {
		log = slog.Default()
	}
	return &Factory{id: id, log: log}
}

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "slog lifecycle emitter + enricher" }

type config struct {
	Level string
}

func decode(cfg map[string]any) (config, error) {
	c := config{Level: "info"}
	if raw, ok := cfg["level"]; ok {
		s, ok := raw.(string)
		if !ok {
			return c, fmt.Errorf("level: want string, got %T", raw)
		}
		switch s {
		case "debug", "info":
			c.Level = s
		default:
			return c, fmt.Errorf("level: want debug|info, got %q", s)
		}
	}
	return c, nil
}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.TelemetryProfile, error) {
	c, err := decode(cfg)
	if err != nil {
		return registry.TelemetryProfile{}, fmt.Errorf("%s: %w", f.id, err)
	}
	return registry.TelemetryProfile{
		Emitter:  &emitter{log: f.log, level: c.Level},
		Enricher: &enricher{},
	}, nil
}

type emitter struct {
	log   *slog.Logger
	level string
}

func (e *emitter) Emit(ctx context.Context, ev event.InvocationEvent) error {
	lvl := slog.LevelInfo
	if e.level == "debug" {
		lvl = slog.LevelDebug
	}
	e.log.Log(ctx, lvl, "invocation.event", "type", string(ev.Type))
	return nil
}

type enricher struct{}

// Keys the enricher reads from context. Callers set them via context.WithValue.
type ctxKey string

const (
	TenantKey ctxKey = "forge.tenant"
	UserKey   ctxKey = "forge.user"
)

func (e *enricher) Enrich(ctx context.Context) map[string]string {
	out := map[string]string{}
	if v, ok := ctx.Value(TenantKey).(string); ok && v != "" {
		out["tenant"] = v
	}
	if v, ok := ctx.Value(UserKey).(string); ok && v != "" {
		out["user"] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Compile-time interface checks.
var (
	_ telemetry.LifecycleEventEmitter = (*emitter)(nil)
	_ telemetry.AttributeEnricher     = (*enricher)(nil)
)
