// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestManifest_MarshalStableOrder(t *testing.T) {
	m := Manifest{
		SpecID:      "acme.demo",
		SpecVersion: "0.1.0",
		BuiltAt:     time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		Resolved: []ResolvedComponent{
			{Kind: "provider", ID: "provider.anthropic@1.0.0", Config: map[string]any{"model": "x"}},
			{Kind: "tool_pack", ID: "toolpack.http-get@1.0.0", Config: map[string]any{"timeoutMs": 5000}},
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"specId":"acme.demo"`) {
		t.Fatalf("specId missing: %s", s)
	}
	if strings.Index(s, `"provider.anthropic@1.0.0"`) > strings.Index(s, `"toolpack.http-get@1.0.0"`) {
		t.Fatal("resolved order not preserved")
	}
}
