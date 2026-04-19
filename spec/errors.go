// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"errors"
	"fmt"
	"strings"
)

// ErrValidation is the sentinel for any spec validation failure.
var ErrValidation = errors.New("spec validation failed")

// Phase 2a sentinels. All are matched through errors.Is on the
// aggregated Errors value; callers do not need to unwrap individual
// violations to discriminate.
var (
	ErrNoSpecStore         = errors.New("forge: extends present but no SpecStore configured")
	ErrSpecNotFound        = errors.New("spec store: ref not found")
	ErrExtendsInvalid      = errors.New("extends: invalid")
	ErrLockedFieldOverride = errors.New("locked field overridden")
	ErrPhaseGatedInOverlay = errors.New("phase-gated field in overlay")
	ErrCompositionLimit    = errors.New("composition limit exceeded")
)

// Errors aggregates one or more validation violations reported by Validate
// or Normalize. It records both formatted messages (in declaration order)
// and any sentinels supplied via Wrap, so callers can both render the full
// human-facing string and discriminate via errors.Is.
type Errors struct {
	msgs      []string
	sentinels []error
}

// Addf records a plain formatted message. Use Wrap when the message is
// associated with a sentinel that callers might match on.
func (e *Errors) Addf(format string, args ...any) {
	e.msgs = append(e.msgs, fmt.Sprintf(format, args...))
}

// Wrap records a formatted message *and* tracks the sentinel so the
// aggregated Errors value reports true for errors.Is(err, sentinel).
func (e *Errors) Wrap(sentinel error, format string, args ...any) {
	e.msgs = append(e.msgs, fmt.Sprintf(format, args...))
	e.sentinels = append(e.sentinels, sentinel)
}

// Len reports how many violations have been recorded.
func (e Errors) Len() int { return len(e.msgs) }

// OrNil returns nil if no violation was recorded; otherwise it returns
// e itself (which satisfies the error interface).
func (e Errors) OrNil() error {
	if len(e.msgs) == 0 {
		return nil
	}
	return e
}

func (e Errors) Error() string {
	return fmt.Sprintf("%s: %s", ErrValidation.Error(), strings.Join(e.msgs, "; "))
}

// Is matches ErrValidation always (the default sentinel), plus any
// sentinel wrapped via Wrap.
func (e Errors) Is(target error) bool {
	if target == ErrValidation {
		return true
	}
	for _, s := range e.sentinels {
		if errors.Is(s, target) {
			return true
		}
	}
	return false
}
