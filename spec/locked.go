// SPDX-License-Identifier: Apache-2.0

package spec

import "fmt"

// scanLockedDrift inspects every overlay supplied to Normalize for
// any locked-field value that differs from base. Each drift produces a
// wrapped ErrLockedFieldOverride entry on the aggregator carrying the
// offending overlay's name and index.
//
// Locked fields:
//   - APIVersion (overlay envelope; should match base)
//   - Metadata.ID
//   - Metadata.Version
//
// (Kind on AgentOverlay is "AgentOverlay", not "AgentSpec"; it is not
// a base-comparable field, so it is intentionally skipped.)
//
// Parents are not scanned: extends parents are separate agents with
// their own identities. The child-wins merge rule already ensures
// base.metadata.id/version win over any parent's, so a parent's
// differing locked fields are silently overridden by base — no
// rebrand of the child agent occurs.
//
// scanLockedDrift is a pure function over the inputs; reusable from
// any future composition path. It deliberately does not diff base
// against the merged result (that diff is always zero under the
// base-wins-last merge rule we use). It also does not live as a
// method on *AgentSpec — that would couple AgentSpec to the merge
// concern.
func scanLockedDrift(base *AgentSpec, overlays []*AgentOverlay, errs *Errors) {
	for i, ov := range overlays {
		if ov == nil {
			continue
		}
		// AgentOverlay.APIVersion drift would have been rejected at
		// LoadOverlay time (envelope check); programmatic overlays
		// constructed in Go bypass that envelope, so re-check here.
		if ov.APIVersion != "" && ov.APIVersion != base.APIVersion {
			errs.Wrap(ErrLockedFieldOverride, "%s",
				lockedMsg("apiVersion", i, ov.Metadata.Name, ov.APIVersion, base.APIVersion))
		}
		if ov.Spec.Metadata == nil {
			continue
		}
		if ov.Spec.Metadata.ID != "" && ov.Spec.Metadata.ID != base.Metadata.ID {
			errs.Wrap(ErrLockedFieldOverride, "%s",
				lockedMsg("metadata.id", i, ov.Metadata.Name, ov.Spec.Metadata.ID, base.Metadata.ID))
		}
		if ov.Spec.Metadata.Version != "" && ov.Spec.Metadata.Version != base.Metadata.Version {
			errs.Wrap(ErrLockedFieldOverride, "%s",
				lockedMsg("metadata.version", i, ov.Metadata.Name, ov.Spec.Metadata.Version, base.Metadata.Version))
		}
	}
}

// lockedMsg formats a uniform error string. Examples:
//
//	"metadata.id: locked, overlay #0 (prod-override) set \"acme.other\" (base = \"acme.demo\")"
//	"metadata.id: locked, overlay #1 set \"acme.other\" (base = \"acme.demo\")"  (no name)
func lockedMsg(field string, idx int, name, got, base string) string {
	if name != "" {
		return fmt.Sprintf("%s: locked, overlay #%d (%s) set %q (base = %q)", field, idx, name, got, base)
	}
	return fmt.Sprintf("%s: locked, overlay #%d set %q (base = %q)", field, idx, got, base)
}
