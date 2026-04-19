# praxis-forge Phase 2a Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land Phase 2a of praxis-forge — composition depth: extends-chain resolution, declarative `AgentOverlay` overlays, per-field provenance tracking, and locked-field protection — without breaking the Phase 1 `Build` signature and without yet introducing canonical hashing or richer manifest inspection (those land in Phase 2b).

**Architecture:** All new logic stays inside the `spec/` package as additive files; `forge.go` gains two functional options (`WithOverlays`, `WithSpecStore`) plus two file loaders (`LoadOverlay`, `LoadOverlays`); `build/build.go` gains a single call site for `spec.Normalize` before the existing resolver; `manifest/Manifest` gains two `omitempty` fields. Overlays are typed Go structs (mirroring `AgentSpec` with optional fields and a tri-state `RefList` wrapper). Extends-chain parents load through an injectable `SpecStore` interface, with `FilesystemSpecStore` and `MapSpecStore` shipping. Provenance is a typed mirror struct embedded in `NormalizedSpec`, exposed via `Provenance(fieldPath)` accessor. Validation runs only on the merged result; per-fragment validation is intentionally skipped because parents are legitimately partial.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3` (strict decode at top level for both spec and overlay), `errors`/`errors.Is` semantics for sentinel + typed-error matching, stdlib `testing` with table-driven and fixture-driven patterns, no new third-party dependency.

**Companion spec:** [`docs/superpowers/specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md)

---

## File Structure

Each new file has one responsibility. Keep files under ~400 LOC; split if a file grows beyond that. No file under `factories/`, `registry/`, `internal/testutil/` is touched.

### `spec/` (new files)

- `spec/store.go` — `SpecStore` interface, `FilesystemSpecStore`, `MapSpecStore`, `ErrSpecNotFound`
- `spec/store_test.go`
- `spec/overlay.go` — `AgentOverlay`, `OverlayMeta`, `AgentOverlayBody`, `OverlayMetadata`, `FilterOverlay`, `RefList` (tri-state wrapper) + `UnmarshalYAML`
- `spec/load_overlay.go` — `LoadOverlay(path)` strict YAML decoder
- `spec/overlay_test.go`
- `spec/testdata/overlay/valid/*.yaml`, `spec/testdata/overlay/invalid/*.yaml` + `.err.txt`
- `spec/provenance.go` — `Provenance`, `Role` enum, `provenanceFields` mirror struct, `OverlayAttribution`, `NormalizedSpec`, `NormalizedSpec.Provenance(path)`
- `spec/provenance_test.go`
- `spec/normalize.go` — `Normalize(ctx, s, overlays, store)`, `resolveExtendsChain`, `mergeChain`, `applyOverlays`
- `spec/normalize_test.go`
- `spec/testdata/normalize/<scenario>/{base,parent-N,overlay-N}.yaml` + `want.json` / `want.err`
- `spec/locked.go` — `validateLocked` pure function
- `spec/locked_test.go`

### `spec/` (modified)

- `spec/errors.go` — add new sentinels (`ErrNoSpecStore`, `ErrSpecNotFound` ref, `ErrExtendsInvalid`, `ErrLockedFieldOverride`, `ErrPhaseGatedInOverlay`, `ErrCompositionLimit`); add `Errors.Wrap` helper plus sibling sentinel slice; extend `Errors.Is` to match recorded sentinels.

### `manifest/` (modified)

- `manifest/manifest.go` — add `ExtendsChain []string`, `Overlays []OverlayAttribution`, type `OverlayAttribution`. Both new fields use `omitempty`.

### `forge.go` + `build/build.go` (modified)

- `forge.go` — add `LoadOverlay`, `LoadOverlays`, `WithOverlays`, `WithSpecStore`; extend `options` struct; add `BuiltAgent.NormalizedSpec()`; replace direct `build.Build` call with the normalized path.
- `build/build.go` — switch from `s.Validate()` to `spec.Normalize(...)`; pass `*spec.NormalizedSpec` (or its inner `*AgentSpec`) into `resolve()`; populate manifest's `ExtendsChain` + `Overlays`.

### `forge_test.go` (modified)

- Existing `TestForge_FullSlice_Offline` keeps the same call shape (`forge.Build(ctx, s, r)` with no extra args). New test `TestForge_ExtendsAndOverlays` exercises a 2-deep chain + 2 overlays via `MapSpecStore`, asserts manifest carries `ExtendsChain` + `Overlays` and `BuiltAgent.NormalizedSpec()` returns the merged result. Negative test asserts `ErrNoSpecStore` when extends is non-empty without `WithSpecStore`.

---

## Conventions used throughout

- **TDD**: every task is test-first. Write the failing test, run it, implement, rerun.
- **Error style**: return errors wrapping `fmt.Errorf("%w", sentinel)` for typed matching via `errors.Is`. Aggregate via `spec.Errors`.
- **Imports**: standard Go ordering — stdlib, then external, then this module, separated by blank lines.
- **License header**: first line of every `.go` file is `// SPDX-License-Identifier: Apache-2.0` matching [doc.go](../../../doc.go).
- **Package docs**: every new file lives in an existing package; no new `doc.go` needed.
- **Commits**: one commit per task group (the 9 commits in the spec's Delivery shape) unless a single task explicitly says otherwise. Conventional-commit format: `feat(pkg): short line` or `test(pkg): short line` for pure-test additions.
- **Fixtures**: YAML fixtures live under `spec/testdata/<area>/<scenario>/`. Tests glob over directories so adding a fixture adds a subtest.

---

## Task groups

Each task group lives in its own file. Numbering matches the commit number in the design spec's Delivery shape table (1:1).

- [`00-spec-store.md`](00-spec-store.md) — Task group 1 (commit 1): `SpecStore` interface + filesystem and map impls
- [`01-overlay.md`](01-overlay.md) — Task group 2 (commit 2): `AgentOverlay` + `RefList` tri-state + `LoadOverlay`
- [`02-provenance.md`](02-provenance.md) — Task group 3 (commit 3): provenance type + `NormalizedSpec` wrapper
- [`03-extends-resolver.md`](03-extends-resolver.md) — Task group 4 (commit 4): extends chain resolver
- [`04-overlay-merger-locked.md`](04-overlay-merger-locked.md) — Task group 5 (commit 5): overlay merger + locked-field validation + `Normalize` entry point
- [`05-forge-options.md`](05-forge-options.md) — Task group 6 (commit 6): `WithOverlays` + `WithSpecStore` + Build wiring
- [`06-manifest.md`](06-manifest.md) — Task group 7 (commit 7): `Manifest.ExtendsChain` + `Manifest.Overlays`
- [`07-integration.md`](07-integration.md) — Task group 8 (commit 8): `NormalizedSpec` accessor + `LoadOverlays` helper + integration test
- [`08-doc-amendments.md`](08-doc-amendments.md) — Task group 9 (commit 9): Phase 0/1 doc amendments

---

## Final verification

After every task group is complete:

- [ ] **Step V1: Full test pass**

Run: `make test-race`
Expected: all packages green, no race warnings.

- [ ] **Step V2: Lint**

Run: `make lint`
Expected: zero reports.

- [ ] **Step V3: Vet**

Run: `go vet ./...`
Expected: clean.

- [ ] **Step V4: Offline integration**

Run: `go test ./... -count=1`
Expected: all packages green; `forge_test.go` exercises both Phase 1 path and the new extends+overlays scenario.

- [ ] **Step V5: Round-trip extends + overlays**

Inspect a `BuiltAgent` produced from a 2-deep chain + 2 overlays: assert `b.Manifest().ExtendsChain` carries the two parents root-first, `b.Manifest().Overlays` carries both overlay names, and `b.NormalizedSpec().Spec.Provider.Ref` resolves to the overlay's provider rather than the base's.

- [ ] **Step V6: Locked-field violation surfaces**

Build the same scenario with an overlay that sets `metadata.id`. Expected: `errors.Is(err, spec.ErrLockedFieldOverride)` is true and the error string carries the overlay file/name and the path `metadata.id`.

- [ ] **Step V7: No regressions in Phase 1**

Existing `TestForge_FullSlice_Offline` passes unchanged (no overlays, no extends, no `WithSpecStore`).

---

## Self-review notes (for plan author)

Coverage matrix of the design spec → task mapping:

| Spec section | Task group(s) |
|--------------|---------------|
| §1 Public API + layering | 5, 7, 8 |
| §2 `AgentOverlay` shape (incl. `RefList` tri-state) | 1 (RefList), 2 |
| §3 `SpecStore` | 0, 5 |
| §4 Provenance | 2 |
| §5 Normalize pipeline (chain + merge + apply) | 3, 4 |
| §6 Locked-field validation | 4 |
| §7 Error model (sentinels + `ExtendsError` + bounds + aggregation) | 0 (ErrSpecNotFound), 3 (ErrExtendsInvalid + ExtendsError), 4 (ErrLockedFieldOverride, ErrCompositionLimit) |
| §8 Manifest extensions | 6 |
| §9 Testing strategy | distributed across 0–7 |
| §10 Out of scope | n/a (deliberately excluded) |
| Delivery shape (9 atomic commits) | 0–8 task groups (1:1) |
| Verification | Final verification block |

Type consistency checks (cross-task):

- `RefList` declared in task 1, consumed in tasks 2 (provenance), 4 (overlay merger), 7 (integration test).
- `SpecStore.Load(ctx, ref) (*AgentSpec, error)` defined in task 0, consumed in tasks 3 (resolveExtendsChain), 4 (Normalize), 5 (forge options), 7 (integration test).
- `Provenance{File, Line, Role, Step}` defined in task 2, written in tasks 3 (parent attribution), 4 (overlay attribution), and read in task 4 (locked field error message), task 7 (integration assertion).
- `ErrLockedFieldOverride` declared in task 0 (errors.go), wrapped in task 4 (validateLocked), asserted in task 7 (integration).
- `Errors.Wrap(sentinel, format, args...)` declared in task 0, used in tasks 3, 4.
- `NormalizedSpec` declared in task 2, populated in task 4 (Normalize), exposed in task 7 (forge accessor).

Open items the engineer must verify against the live tree:

1. `*yaml.Node.Line` field on `yaml.v3` for line-number capture in `RefList.UnmarshalYAML`. Spot-checked at planning time but version-sensitive; if the field name has shifted, capture the line through `node.Line` documented helper.
2. Whether `errors.Join` (Go 1.20+) yields a cleaner aggregator than the bespoke `Errors.Wrap` proposed here. Plan uses the bespoke variant because Phase 1 already extends `Errors`; switching to `errors.Join` is acceptable as long as the public contract (`errors.Is(err, ErrLockedFieldOverride)` etc.) holds.
3. `FilesystemSpecStore` Windows behavior (path separators, symlink semantics). Plan keeps the impl `filepath.Clean`-based; engineer adds a `runtime.GOOS == "windows"` guard if a real Windows test reveals divergence.
