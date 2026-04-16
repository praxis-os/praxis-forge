// SPDX-License-Identifier: Apache-2.0

package build

import (
	"errors"
	"testing"
	"time"

	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/budget"
)

func TestApplyBudgetOverrides_Tighten(t *testing.T) {
	defaults := budget.Config{
		MaxWallClock:    int64(30 * time.Second),
		MaxInputTokens:  50000,
		MaxOutputTokens: 10000,
		MaxToolCalls:    24,
	}
	cfg, err := applyBudgetOverrides(defaults, spec.BudgetOverrides{
		MaxWallClock: "15s",
		MaxToolCalls: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxWallClock != int64(15*time.Second) {
		t.Fatalf("wall=%v", cfg.MaxWallClock)
	}
	if cfg.MaxToolCalls != 12 {
		t.Fatalf("calls=%d", cfg.MaxToolCalls)
	}
	if cfg.MaxInputTokens != 50000 {
		t.Fatalf("input untouched expected: %d", cfg.MaxInputTokens)
	}
}

func TestApplyBudgetOverrides_RejectsLoosen(t *testing.T) {
	defaults := budget.Config{MaxToolCalls: 10}
	_, err := applyBudgetOverrides(defaults, spec.BudgetOverrides{MaxToolCalls: 100})
	if !errors.Is(err, ErrBudgetLoosening) {
		t.Fatalf("err=%v", err)
	}
}

func TestApplyBudgetOverrides_BadDuration(t *testing.T) {
	_, err := applyBudgetOverrides(budget.Config{MaxWallClock: int64(time.Minute)}, spec.BudgetOverrides{MaxWallClock: "banana"})
	if err == nil {
		t.Fatal("expected parse error")
	}
}
