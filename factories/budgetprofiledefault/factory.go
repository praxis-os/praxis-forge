// SPDX-License-Identifier: Apache-2.0

// Package budgetprofiledefault provides a conservative default-tier-1 budget
// profile: 30s wall, 50k in, 10k out, 24 tool calls, 500k microdollars.
package budgetprofiledefault

import (
	"context"
	"time"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/budget"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "default tier-1 budget profile" }

func (f *Factory) Build(_ context.Context, _ map[string]any) (registry.BudgetProfile, error) {
	return registry.BudgetProfile{
		Guard: budget.NullGuard{},
		DefaultConfig: budget.Config{
			MaxWallClock:        int64(30 * time.Second),
			MaxInputTokens:      50_000,
			MaxOutputTokens:     10_000,
			MaxToolCalls:        24,
			MaxCostMicrodollars: 500_000,
		},
	}, nil
}
