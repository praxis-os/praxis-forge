// SPDX-License-Identifier: Apache-2.0

package registry

import "errors"

var (
	ErrRegistryFrozen = errors.New("registry: frozen, cannot register after Build")
	ErrDuplicate      = errors.New("registry: duplicate (kind, id)")
	ErrNotFound       = errors.New("registry: factory not found")
	ErrKindMismatch   = errors.New("registry: id registered under different kind")
)
