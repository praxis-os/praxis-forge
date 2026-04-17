// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"errors"
	"fmt"
	"strings"
)

// ErrValidation is the sentinel for any spec validation failure.
var ErrValidation = errors.New("spec validation failed")

// Errors aggregates one or more validation violations reported by Validate.
type Errors []string

func (e *Errors) Addf(format string, args ...any) {
	*e = append(*e, fmt.Sprintf(format, args...))
}

func (e Errors) OrNil() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

func (e Errors) Error() string {
	return fmt.Sprintf("%s: %s", ErrValidation.Error(), strings.Join(e, "; "))
}

func (e Errors) Is(target error) bool {
	return target == ErrValidation
}
