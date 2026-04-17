// SPDX-License-Identifier: Apache-2.0

package registry

import "github.com/praxis-os/praxis-forge/spec"

// ID is a registered factory's stable address. Format: "<dotted>@<semver>".
type ID string

// ParseID validates the id string and returns it unchanged on success.
func ParseID(s string) (ID, error) {
	if _, _, err := spec.ParseID(s); err != nil {
		return "", err
	}
	return ID(s), nil
}
