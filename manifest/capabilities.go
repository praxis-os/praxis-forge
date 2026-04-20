// SPDX-License-Identifier: Apache-2.0

package manifest

// Capabilities summarizes which registry kinds contributed to the build
// and which optional kinds were absent from the spec.
//
// Required kinds (provider, prompt_asset) are always in Present. Optional
// kinds (budget_profile, telemetry_profile, credential_resolver,
// identity_signer) appear in Present when the spec referenced them, or in
// Skipped with reason "not_specified" when they were nil on the spec.
type Capabilities struct {
	// Present is a lexicographically-sorted list of kind slugs that
	// contributed at least one resolved component to the build.
	Present []string `json:"present"`

	// Skipped lists optional kinds that were not specified by the spec.
	// Absent when all optional kinds were used.
	Skipped []CapabilitySkip `json:"skipped,omitempty"`
}

// CapabilitySkip records an optional kind that was absent from the spec.
type CapabilitySkip struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"` // "not_specified" in Phase 2b
}
