// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"errors"
	"testing"
)

func TestErrors_AppendAndError(t *testing.T) {
	var e Errors
	e.Addf("first problem")
	e.Addf("second %s", "problem")
	if len(e) != 2 {
		t.Fatalf("len=%d", len(e))
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
