// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"regexp"
)

var idRegexp = regexp.MustCompile(`^([a-z][a-z0-9.-]*)@(\d+\.\d+\.\d+)$`)

// ParseID splits a component reference `<dotted>@<semver>` into its name and
// version parts. Returns an error if the string does not match.
func ParseID(s string) (name, version string, err error) {
	m := idRegexp.FindStringSubmatch(s)
	if m == nil {
		return "", "", fmt.Errorf("invalid component id %q: want <dotted-name>@<semver>", s)
	}
	return m[1], m[2], nil
}
