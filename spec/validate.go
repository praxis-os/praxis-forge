// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"regexp"
	"strings"
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
//
//nolint:gocyclo // linear list of Phase-1 invariants (header, refs, phase-gated fields, skills + outputContract prefix checks, duplicates); splitting into helpers scatters the invariant set without reducing complexity.
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

	// Phase-gated fields: extends remains gated; skills, outputContract, and mcpImports now validated by prefix + structure.
	if len(s.Extends) > 0 {
		errs.Addf("extends: phase-gated (Phase 2); must be empty in v0")
	}

	// MCP imports validation (Phase 4): each ref must be prefixed with "mcp.".
	for i, mi := range s.MCPImports {
		validateKindPrefixedRef(&errs, fmt.Sprintf("mcpImports[%d].ref", i), mi.Ref, "mcp.")
	}

	// Skills validation: each ref must be prefixed with "skill.".
	for i, skill := range s.Skills {
		validateKindPrefixedRef(&errs, fmt.Sprintf("skills[%d].ref", i), skill.Ref, "skill.")
	}

	// OutputContract validation: ref must be prefixed with "outputcontract.".
	if s.OutputContract != nil {
		validateKindPrefixedRef(&errs, "outputContract.ref", s.OutputContract.Ref, "outputcontract.")
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

func validateKindPrefixedRef(errs *Errors, field, ref, prefix string) {
	if ref == "" {
		errs.Addf("%s: required", field)
		return
	}
	if _, _, err := ParseID(ref); err != nil {
		errs.Addf("%s: %s", field, err.Error())
		return
	}
	if !strings.HasPrefix(ref, prefix) {
		errs.Addf("%s: must start with %q", field, prefix)
	}
}
