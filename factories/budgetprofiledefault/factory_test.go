// SPDX-License-Identifier: Apache-2.0

package budgetprofiledefault

import (
	"context"
	"testing"
	"time"
)

func TestFactory_Defaults(t *testing.T) {
	f := NewFactory("budgetprofile.default-tier1@1.0.0")
	bp, err := f.Build(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bp.Guard == nil {
		t.Fatal("guard nil")
	}
	if bp.DefaultConfig.MaxWallClock != int64(30*time.Second) {
		t.Fatalf("wall=%v", bp.DefaultConfig.MaxWallClock)
	}
	if bp.DefaultConfig.MaxToolCalls != 24 {
		t.Fatalf("calls=%d", bp.DefaultConfig.MaxToolCalls)
	}
}
