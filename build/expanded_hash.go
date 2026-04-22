// SPDX-License-Identifier: Apache-2.0

package build

import (
	"github.com/praxis-os/praxis-forge/spec"
)

// computeExpandedHash returns the SHA-256 of the canonical JSON encoding
// of the post-expansion AgentSpec. It reuses spec.NormalizedSpec's
// canonical encoder (single-sourced in the spec package) by wrapping
// the expanded AgentSpec in a throwaway NormalizedSpec and calling the
// memoized accessor.
//
// Semantics (design spec §"Manifest additions"):
//   - hash covers es.Spec only (skills are spec-level; their contributions
//     already rolled into Tools/Policies/OutputContract);
//   - stable across equivalent compositions (two specs that expand to
//     identical AgentSpec produce identical ExpandedHash);
//   - the value is a lowercase 64-char hex string.
func computeExpandedHash(es *ExpandedSpec) (string, error) {
	tmp := &spec.NormalizedSpec{Spec: es.Spec}
	return tmp.NormalizedHash()
}
