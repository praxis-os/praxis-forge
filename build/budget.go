// SPDX-License-Identifier: Apache-2.0

package build

import (
	"errors"
	"fmt"
	"time"

	"github.com/praxis-os/praxis-forge/spec"
	"github.com/praxis-os/praxis/budget"
)

// ErrBudgetLoosening is returned when an override would loosen a profile
// ceiling beyond its registered default.
var ErrBudgetLoosening = errors.New("budget override loosens a profile ceiling")

// applyBudgetOverrides returns the default config with tightening overrides
// applied. Any override that would loosen a ceiling (increase max*) fails.
// Zero-valued override fields mean "unset, keep default".
func applyBudgetOverrides(defaults budget.Config, ov spec.BudgetOverrides) (budget.Config, error) {
	out := defaults

	if ov.MaxWallClock != "" {
		d, err := time.ParseDuration(ov.MaxWallClock)
		if err != nil {
			return budget.Config{}, fmt.Errorf("budget.overrides.maxWallClock: %w", err)
		}
		nanos := int64(d)
		if defaults.MaxWallClock > 0 && nanos > defaults.MaxWallClock {
			return budget.Config{}, fmt.Errorf("%w: maxWallClock %v > default %v", ErrBudgetLoosening, d, time.Duration(defaults.MaxWallClock))
		}
		out.MaxWallClock = nanos
	}
	if err := tightenInt64("maxInputTokens", ov.MaxInputTokens, defaults.MaxInputTokens, &out.MaxInputTokens); err != nil {
		return budget.Config{}, err
	}
	if err := tightenInt64("maxOutputTokens", ov.MaxOutputTokens, defaults.MaxOutputTokens, &out.MaxOutputTokens); err != nil {
		return budget.Config{}, err
	}
	if err := tightenInt64("maxToolCalls", ov.MaxToolCalls, defaults.MaxToolCalls, &out.MaxToolCalls); err != nil {
		return budget.Config{}, err
	}
	if err := tightenInt64("maxCostMicrodollars", ov.MaxCostMicrodollars, defaults.MaxCostMicrodollars, &out.MaxCostMicrodollars); err != nil {
		return budget.Config{}, err
	}
	return out, nil
}

func tightenInt64(field string, override, def int64, dst *int64) error {
	if override == 0 {
		return nil
	}
	if def > 0 && override > def {
		return fmt.Errorf("%w: %s %d > default %d", ErrBudgetLoosening, field, override, def)
	}
	*dst = override
	return nil
}
