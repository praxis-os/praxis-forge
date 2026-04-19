// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"errors"
	"strings"
	"testing"
)

func TestErrors_AppendAndError(t *testing.T) {
	var e Errors
	e.Addf("first problem")
	e.Addf("second %s", "problem")
	if e.Len() != 2 {
		t.Fatalf("len=%d", e.Len())
	}
	msg := e.Error()
	if msg == "" {
		t.Fatal("empty error message")
	}
	if !errors.Is(e, ErrValidation) {
		t.Fatal("Errors should match ErrValidation via Is")
	}
}

func TestErrors_OrNilWhenEmpty(t *testing.T) {
	var e Errors
	if e.OrNil() != nil {
		t.Fatal("empty Errors.OrNil should be nil")
	}
}

func TestErrors_WrapMatchesSentinel(t *testing.T) {
	var e Errors
	e.Wrap(ErrLockedFieldOverride, "metadata.id: changed by overlay %q", "prod")
	if !errors.Is(e, ErrValidation) {
		t.Fatal("aggregator should still match ErrValidation")
	}
	if !errors.Is(e, ErrLockedFieldOverride) {
		t.Fatal("aggregator should match the wrapped sentinel")
	}
	if errors.Is(e, ErrNoSpecStore) {
		// negative control — must not match unrelated sentinel
		t.Fatal("aggregator must not match unrelated sentinel")
	}
	if !strings.Contains(e.Error(), "prod") {
		t.Fatalf("formatted message lost: %v", e)
	}
}

func TestErrors_WrapMultipleSentinels(t *testing.T) {
	var e Errors
	e.Wrap(ErrLockedFieldOverride, "metadata.id changed")
	e.Wrap(ErrCompositionLimit, "overlay count > 16")
	if !errors.Is(e, ErrLockedFieldOverride) {
		t.Fatal("first sentinel lost")
	}
	if !errors.Is(e, ErrCompositionLimit) {
		t.Fatal("second sentinel lost")
	}
}

func TestErrors_AddfDoesNotRecordSentinel(t *testing.T) {
	var e Errors
	e.Addf("plain message")
	if errors.Is(e, ErrLockedFieldOverride) {
		t.Fatal("plain Addf must not match an unrelated sentinel")
	}
}
