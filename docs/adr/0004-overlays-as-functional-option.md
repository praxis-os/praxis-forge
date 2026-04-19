# ADR 0004 — Overlays enter `forge.Build` as a functional option

**Status:** Accepted
**Date:** 2026-04-18
**Supersedes:** the planned positional-slot signature sketched in
[`docs/design/forge-overview.md`](../design/forge-overview.md) prior to
Phase 2a (line ~119, "Phase 2 grows this signature with an
`overlays []forge.AgentOverlay` parameter between spec and registry").

## Context

Phase 0 sketched a `forge.Build` signature that would grow over time:
each new phase appended a positional parameter (overlays in Phase 2,
hashing inputs in Phase 2b, and so on). The intent was that the call
site visibly carry every input.

By the time Phase 2a was being planned, `forge.Build` already had the
shape:

```go
Build(ctx context.Context, s *spec.AgentSpec, r *registry.ComponentRegistry, opts ...Option) (*BuiltAgent, error)
```

shipped in Phase 1. Inserting a positional `overlays []*spec.AgentOverlay`
slot between `s` and `r` would have broken every Phase 1 caller for
the first incremental extension — a cost out of proportion with the
visibility benefit, especially given that the codebase had already
committed to the functional-options idiom in `forge.Option` /
`forge.options`.

## Decision

Overlays enter `forge.Build` via `forge.WithOverlays(...)`. The
positional shape `Build(ctx, s, r, opts...)` is preserved across
phases. Future Phase 2+ inputs will likewise enter through new `With*`
options.

The `SpecStore` configured for `extends:` resolution enters the same
way (`forge.WithSpecStore(...)`) for symmetry.

## Consequences

- Phase 1 callers compile and run unchanged when no overlays or
  extends are in play.
- Read-at-call-site visibility of "this build has overlays / a spec
  store" comes from the `WithOverlays` / `WithSpecStore` lines, not
  from a positional slot. Acceptable trade.
- The composition-depth concept stays first-class in the public
  surface (its own option, not buried in a `forge.WithExtras` bag);
  this preserves the brainstorm intent of treating overlays as a
  first-class input rather than a tuning knob.
- ADR 0001's layering invariants are unchanged: this is a packaging
  decision about the facade, not a layering one.
