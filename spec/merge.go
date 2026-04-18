// SPDX-License-Identifier: Apache-2.0

package spec

// merge.go centralizes the field-shape helpers used by both mergeChain
// (parents → base) and applyOverlays (overlays → merged). The two
// callers differ only in how they detect "set" — mergeChain checks
// nil/zero on the AgentSpec field; applyOverlays checks the
// AgentOverlayBody's pointer field plus, for replaceable lists, the
// RefList.Set flag.

// scalarString picks the child value if non-empty; otherwise the parent.
func scalarString(parent, child string) string {
	if child != "" {
		return child
	}
	return parent
}

// pointerStruct returns child if non-nil; otherwise parent. Generic over
// any pointer type the spec uses (*ComponentRef, *BudgetRef, *CredRef).
func pointerStruct[T any](parent, child *T) *T {
	if child != nil {
		return child
	}
	return parent
}

// sliceReplaceIfSet returns child if it is non-nil; otherwise parent.
// Used for typed slices on AgentSpec where "no value" is nil.
func sliceReplaceIfSet[T any](parent, child []T) []T {
	if child != nil {
		return child
	}
	return parent
}

// mapStringStringReplace returns child if non-nil; otherwise parent.
// Per design, Metadata.Labels uses replace semantics rather than merge.
func mapStringStringReplace(parent, child map[string]string) map[string]string {
	if child != nil {
		return child
	}
	return parent
}
