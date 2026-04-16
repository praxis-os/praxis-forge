// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"regexp"
)

const (
	expectedAPIVersion = "forge.praxis-os.dev/v0"
	expectedKind       = "AgentSpec"
)

var (
	metadataIDRegexp = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z0-9]+)*(-[a-z0-9.]+)?$`)
	semverRegexp     = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
)

// Validate runs every Phase 1 invariant in a fixed order, aggregating failures.
func (s *AgentSpec) Validate() error {
	var errs Errors

	// Header.
	if s.APIVersion != expectedAPIVersion {
		errs.Addf("apiVersion: want %q, got %q", expectedAPIVersion, s.APIVersion)
	}
	if s.Kind != expectedKind {
		errs.Addf("kind: want %q, got %q", expectedKind, s.Kind)
	}
	if !metadataIDRegexp.MatchString(s.Metadata.ID) {
		errs.Addf("metadata.id %q: must be dotted-lowercase", s.Metadata.ID)
	}
	if !semverRegexp.MatchString(s.Metadata.Version) {
		errs.Addf("metadata.version %q: must be semver MAJOR.MINOR.PATCH", s.Metadata.Version)
	}

	// Required top-level refs.
	validateRef(&errs, "provider.ref", s.Provider.Ref)
	if s.Prompt.System == nil || s.Prompt.System.Ref == "" {
		errs.Addf("prompt.system: required")
	} else {
		validateRef(&errs, "prompt.system.ref", s.Prompt.System.Ref)
	}

	// Optional component refs.
	for i, t := range s.Tools {
		validateRef(&errs, fmt.Sprintf("tools[%d].ref", i), t.Ref)
	}
	for i, p := range s.Policies {
		validateRef(&errs, fmt.Sprintf("policies[%d].ref", i), p.Ref)
	}
	for i, f := range s.Filters.PreLLM {
		validateRef(&errs, fmt.Sprintf("filters.preLLM[%d].ref", i), f.Ref)
	}
	for i, f := range s.Filters.PreTool {
		validateRef(&errs, fmt.Sprintf("filters.preTool[%d].ref", i), f.Ref)
	}
	for i, f := range s.Filters.PostTool {
		validateRef(&errs, fmt.Sprintf("filters.postTool[%d].ref", i), f.Ref)
	}
	if s.Budget != nil {
		validateRef(&errs, "budget.ref", s.Budget.Ref)
	}
	if s.Telemetry != nil {
		validateRef(&errs, "telemetry.ref", s.Telemetry.Ref)
	}
	if s.Credentials != nil {
		validateRef(&errs, "credentials.ref", s.Credentials.Ref)
	}
	if s.Identity != nil {
		validateRef(&errs, "identity.ref", s.Identity.Ref)
	}

	// Phase-gated fields: must be empty in v0.
	if len(s.Extends) > 0 {
		errs.Addf("extends: phase-gated (Phase 2); must be empty in v0")
	}
	if len(s.Skills) > 0 {
		errs.Addf("skills: phase-gated (Phase 3); must be empty in v0")
	}
	if len(s.MCPImports) > 0 {
		errs.Addf("mcpImports: phase-gated (Phase 4); must be empty in v0")
	}
	if s.OutputContract != nil {
		errs.Addf("outputContract: phase-gated (Phase 3); must be empty in v0")
	}

	// Duplicate tool refs.
	seen := map[string]int{}
	for i, t := range s.Tools {
		if prev, ok := seen[t.Ref]; ok {
			errs.Addf("tools[%d]: duplicate of tools[%d] (ref=%s)", i, prev, t.Ref)
		} else {
			seen[t.Ref] = i
		}
	}

	return errs.OrNil()
}

func validateRef(errs *Errors, field, ref string) {
	if ref == "" {
		errs.Addf("%s: required", field)
		return
	}
	if _, _, err := ParseID(ref); err != nil {
		errs.Addf("%s: %s", field, err.Error())
	}
}
