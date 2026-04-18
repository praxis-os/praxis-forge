# Task group 8 — Phase 0/1 doc amendments

> Part of [praxis-forge Phase 2a Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md).

**Commit (atomic):** `docs: amend Phase 0/1 design docs for Phase 2a`

**Scope:** documentation-only commit that reconciles the Phase 0/1 architecture docs with the actual Phase 2a implementation. Two files change:

1. `docs/design/forge-overview.md` — drop the planned positional `overlays []forge.AgentOverlay` slot in the `Build` signature; explain the functional-option entry path (`forge.WithOverlays`).
2. `docs/design/agent-spec-v0.md` — replace the speculative `patch:`-style overlay sketch with the actual `kind: AgentOverlay` envelope; clarify that overlays are sibling files passed to `Build`, not embedded inside an `AgentSpec`; document the `extends:` chain semantics, depth cap, cycle detection, and locked-field rules.

Also lands one ADR amendment file recording the rationale.

---

## Task 8.1: Amend `forge-overview.md` — Build signature

**Files:**
- Modify: `docs/design/forge-overview.md`

- [ ] **Step 1: Replace the Phase-2 comment in the facade snippet**

Locate the code block starting at [`forge-overview.md:112`](../../design/forge-overview.md#L112). The current snippet ends with:

```go
)
// Phase 2 grows this signature with an `overlays []forge.AgentOverlay`
// parameter between spec and registry.
if err != nil { ... }
```

Replace those three lines with:

```go
    forge.WithOverlays(prodOverlay, regionOverlay),  // Phase 2a
    forge.WithSpecStore(spec.MapSpecStore{...}),     // Phase 2a, required if spec uses extends:
)
// Build signature is stable across phases: the positional slots
// remain (ctx, spec, registry); every Phase 2+ input enters via
// forge.With*(...) functional options. See ADR 0004.
if err != nil { ... }
```

- [ ] **Step 2: Add a brief subsection under "Phase roadmap" reflecting the Phase 2a → 2b split**

Locate the "Phase roadmap" section starting around line 150. Find the bullet for **Phase 2 — composition depth.** and replace it with:

```markdown
- **Phase 2 — composition depth.** Split into two cuts.
  - **Phase 2a (shipped):** `forge.WithOverlays` + `forge.WithSpecStore`
    options, declarative `AgentOverlay` (typed Go struct mirror of
    AgentSpec, all fields optional, tri-state `RefList` wrapper for
    replaceable lists), `extends:` chain resolution (depth ≤ 8, cycle
    detection, root-first merge with child-wins), per-field provenance
    tracking (`NormalizedSpec.Provenance(fieldPath)`), locked-field
    protection (apiVersion, kind, metadata.id, metadata.version cannot
    drift through extends or overlays). Manifest gains `extendsChain`
    and `overlays` attribution fields. See
    [`docs/superpowers/specs/2026-04-18-praxis-forge-phase-2a-design.md`](../superpowers/specs/2026-04-18-praxis-forge-phase-2a-design.md).
  - **Phase 2b (next):** canonical JSON ordering of `NormalizedSpec`,
    stable hash on `Manifest` (`normalizedHash`), richer inspection
    surfaces (capability flags, dependency graph export).
```

- [ ] **Step 3: Build doc check (no test runner needed; this is markdown)**

Run: `git diff docs/design/forge-overview.md`
Expected: only the two blocks above change. No accidental edits.

---

## Task 8.2: Amend `agent-spec-v0.md` — overlays + extends semantics

**Files:**
- Modify: `docs/design/agent-spec-v0.md`

- [ ] **Step 1: Update the `extends:` comment in the example spec**

Around line 28, change:

```yaml
extends:                          # optional; acyclic; Phase 2
  - acme.base-agent@2.0.0
```

to:

```yaml
extends:                          # optional; acyclic; depth ≤ 8; Phase 2a
  - acme.base-agent@2.0.0         # resolved via SpecStore passed to Build
```

- [ ] **Step 2: Remove the misleading `overlays: []` line inside the example AgentSpec**

Around line 90 (in the same example spec block) the line:

```yaml
overlays: []                      # siblings, applied in order; see below
```

is wrong: overlays are **not** embedded in an AgentSpec; they are sibling YAML files passed to `Build` via `forge.WithOverlays(...)`. Delete the line entirely.

- [ ] **Step 3: Replace the "## Overlays" section**

Locate the `## Overlays` section header (around line 93) and replace the entire section (header + body, up to but not including the next `##` header `## Referenced kinds`) with:

```markdown
## Overlays

Overlays are typed YAML documents whose body mirrors `AgentSpec` with
every field optional. They are applied **after** `extends:` resolution
and **before** validation of the merged result. Overlays are sibling
files passed to `forge.Build` via `forge.WithOverlays(...)`; they are
not embedded inside an `AgentSpec`.

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentOverlay
metadata:
  name: prod-override            # attribution label, surfaced in errors + manifest
spec:
  provider:                      # any subset of AgentSpec fields
    ref: provider.anthropic@1.0.0
    config:
      model: claude-opus-4-7
  policies:                      # replace by default
    - ref: policypack.staging-observer@1.0.0
  tools: []                      # explicit empty clears the base list
```

Phase 2a merge semantics:

- **Lists** (`tools`, `policies`, `filters.preLLM`, etc.) replace the
  base list when set. The `RefList` wrapper distinguishes "absent"
  (preserve base), "explicit null/empty" (clear), and "populated"
  (replace).
- **`Config map[string]any`** blocks replace the entire map (no deep
  merge across opaque schemas).
- **`metadata.labels`** replaces, not merges.
- **Scalars** that are zero/empty in the overlay preserve the base.

Locked fields (`apiVersion`, `kind`, `metadata.id`, `metadata.version`)
cannot be touched by an overlay; attempting to do so produces
`spec.ErrLockedFieldOverride` with attribution to the source overlay.

Phase-gated AgentSpec fields (`extends`, `skills`, `mcpImports`,
`outputContract`) are deliberately absent from the overlay body, so
the strict YAML decoder rejects them at parse time.

Overlay count is bounded by `spec.MaxOverlayCount` (16); exceeding the
bound returns `spec.ErrCompositionLimit`.
```

- [ ] **Step 4: Tighten the `extends:` invariants**

Around line 158, the invariant currently reads:

```markdown
5. **Acyclic `extends:` chain.** Cycle detection runs before
   normalization. Depth is capped (suggested: 8) to catch pathological
   inheritance.
```

Replace with:

```markdown
5. **Acyclic `extends:` chain.** Cycle detection runs before
   normalization. Depth is capped at `spec.MaxExtendsDepth` (8) to
   catch pathological inheritance. Both violations surface as
   `spec.ErrExtendsInvalid` with a typed `*spec.ExtendsError` carrying
   the resolution chain and a `Reason` of `"cycle"` or `"depth"`.
   Parents resolve through an injectable `spec.SpecStore`; if the
   spec declares `Extends` and `Build` was not given a SpecStore via
   `forge.WithSpecStore(...)`, it returns `spec.ErrNoSpecStore`.
```

- [ ] **Step 5: Tighten the canonicalization invariant**

Around line 173, the invariant currently reads:

```markdown
9. **Deterministic canonicalization.** After extends + overlays, the
   normalizer produces a canonical ordering (alphabetical keys,
   normalized list ordering for commutative fields, version numbers
   fully qualified). Two semantically equivalent specs must hash
   identically.
```

Replace with:

```markdown
9. **Deterministic merge order.** Phase 2a fixes the merge field-iteration
   order to match `AgentSpec`'s declaration order in `spec/types.go`.
   Phase 2b layers a canonical JSON serialization plus a stable hash
   on top of this — two semantically equivalent specs must hash
   identically. Reordering the `AgentSpec` struct fields changes the
   hash by design (the hash is bound to the struct shape).
```

- [ ] **Step 6: Diff check**

Run: `git diff docs/design/agent-spec-v0.md`
Expected: only the five blocks above change.

---

## Task 8.3: Add ADR 0004 — overlays as functional option

**Files:**
- Create: `docs/adr/0004-overlays-as-functional-option.md`

- [ ] **Step 1: Write the ADR**

```markdown
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
```

- [ ] **Step 2: Verify the ADR renders in the index**

Run: `ls docs/adr/`
Expected: `0001-praxis-forge-layering.md`, `0002-external-registries-at-devtime-only.md`, `0003-memory-strategy-across-three-levels.md`, `0004-overlays-as-functional-option.md`.

---

## Task 8.4: Lint + commit task group 8

- [ ] **Step 1: Render the diff one last time**

Run: `git status && git diff --stat`
Expected: three modified files (`docs/design/forge-overview.md`, `docs/design/agent-spec-v0.md`) + one new file (`docs/adr/0004-overlays-as-functional-option.md`).

- [ ] **Step 2: Commit**

```bash
git add docs/design/forge-overview.md docs/design/agent-spec-v0.md docs/adr/0004-overlays-as-functional-option.md
git commit -m "docs: amend Phase 0/1 design docs for Phase 2a

Reconciles forge-overview.md and agent-spec-v0.md with what Phase 2a
actually shipped:

- forge-overview.md: drops the planned positional overlays []slot from
  the Build signature; documents the functional-option entry path
  (forge.WithOverlays + forge.WithSpecStore). Rewrites the Phase 2
  roadmap bullet into a 2a/2b split.
- agent-spec-v0.md: removes the misleading 'overlays: []' line from
  the example AgentSpec (overlays are sibling files, not embedded);
  replaces the speculative patch:-style overlay sketch with the
  actual kind: AgentOverlay envelope and Phase 2a merge rules;
  tightens the extends: invariants (depth cap, sentinel names,
  SpecStore requirement) and the canonicalization invariant
  (declaration-order merge in 2a, canonical JSON + hash in 2b).
- adr/0004: records the supersession of the positional-slot plan.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Final verification (run once after task group 8 commits)

After all nine commits land, run the full verification block from
[`README.md`](README.md):

- [ ] `make test-race` — clean
- [ ] `make lint` — zero reports
- [ ] `go vet ./...` — clean
- [ ] `go test ./... -count=1` — all packages green
- [ ] Round-trip extends + overlays via `TestForge_ExtendsAndOverlays_Offline`
- [ ] Locked-field violation surfaces via `TestForge_LockedFieldOverrideSurfaces`
- [ ] `TestForge_FullSlice_Offline` (Phase 1 path) still passes unchanged

If every step passes: Phase 2a is complete and ready for Phase 2b planning (canonical JSON ordering + stable hash + richer inspection).
