# praxis-forge — Phase 2b design spec

**Date:** 2026-04-20
**Milestone:** Phase 2b — deterministic build (canonical JSON, stable hash, capabilities)
**Status:** Shipped
**Scope chosen:** Option B (canonical JSON + stable hash + capability flags; dependency graph deferred)

## Context

Phase 2a shipped extends-chain resolution, `AgentOverlay`, per-field
provenance tracking, and locked-field protection. Phase 2b completes the
"deterministic build" story deferred from Phase 2a (spec section 10):

1. **Canonical JSON** — a stable byte sequence for `NormalizedSpec`
   independent of map insertion order or YAML authoring quirks.
2. **NormalizedHash** — a SHA-256 digest of the canonical form, stamped
   on `Manifest.NormalizedHash` after every build.
3. **Capabilities** — `Manifest.Capabilities` records which registry kinds
   contributed (`Present`) and which optional kinds were absent from the
   spec (`Skipped`).

`spec.Graph()` (dependency graph export) was explicitly deferred; it has
no dependencies on this work and warrants its own spec.

## Brainstorm decisions

1. **Canonical encoding strategy: round-trip + sorted keys, no library.**
   `json.Marshal` (uses the new `json:` tags on `AgentSpec`) → unmarshal
   into `any` tree via `json.Decoder.UseNumber()` → `canonicalEncode`
   (recursive, sorts map keys with `sort.Strings`). No external deps.
   Alternative approaches (full custom `MarshalJSON`, RFC 8785 JCS
   library, intermediate AST) rejected as higher maintenance cost.

2. **Empty-collection normalization.** A pre-walk (`pruneEmpty`) drops
   empty maps and empty slices before canonical encoding, so YAML
   authoring quirks (explicit `{}` vs absent Config) do not perturb the
   hash. Intentional: two specs that are logically equivalent produce
   the same hash.

3. **Memoization via sync.Once.** `NormalizedSpec` is effectively immutable
   after `Normalize` returns. Both `CanonicalJSON()` and `NormalizedHash()`
   memoize their results via embedded `memoCanonical` / `memoHash` structs
   (each wrapping `sync.Once`). Repeated calls are free.

4. **Hash format: bare lowercase hex (SHA-256, 64 chars).** No prefix.
   The godoc for `NormalizedHash()` makes explicit: hash covers
   `NormalizedSpec.Spec` only; `Manifest.BuiltAt`, `Resolved`, and
   `Capabilities` are not in scope. Hash churn on schema additions is
   intentional — it signals the logical spec changed.

5. **Capabilities shape: Present + Skipped.** `Contributed []...` was
   considered but rejected as redundant with `Manifest.Resolved`. The
   useful information that has no existing home: which optional kinds
   were absent. `Missing` (resolver returns ErrNotFound) dropped — the
   resolver fails fast, so a successful build never has missing entries.
   `Present` is lexicographically sorted for stability. `Skipped` follows
   registry declaration order (budget → telemetry → credential_resolver →
   identity_signer).

6. **JSON tags on AgentSpec (commit 1 pre-work).** `AgentSpec` and all
   nested types previously had only `yaml:` tags. Phase 2b adds matching
   `json:` tags with the same field names and `omitempty` policy as the
   YAML tags. This enables standard `json.Marshal` as the first step of
   canonical encoding.

## Architecture

### New files

| File | Role |
|------|------|
| `spec/canonical.go` | `CanonicalJSON()`, `NormalizedHash()`, `canonicalEncode`, `pruneEmpty`, `memoCanonical`, `memoHash` |
| `spec/canonical_test.go` | Unit tests: map order stability, config map stability, empty vs nil equivalence, memoization, valid JSON output, hash format/stability/determinism |
| `manifest/capabilities.go` | `Capabilities`, `CapabilitySkip` types |
| `build/capabilities.go` | `computeCapabilities(s, res)` — populates Present/Skipped from resolved struct + spec |
| `build/build_hash_test.go` | Integration: hash format, stability, spec-change detection, capabilities shape, JSON round-trip |
| `spec/testdata/normalize/canonical-stable/` | Fixture scenario with two YAMLs differing only in map key order; golden `want.canonical.json` + `want.hash` |

### Modified files

| File | Change |
|------|--------|
| `spec/types.go` | Add `json:"..."` tags to `AgentSpec`, `Metadata`, `Owner`, `ComponentRef`, `PromptBlock`, `FilterBlock`, `BudgetRef`, `BudgetOverrides`, `CredRef` |
| `spec/provenance.go` | Add `canonicalMemo memoCanonical` and `hashMemo memoHash` fields to `NormalizedSpec` |
| `spec/normalize_integration_test.go` | `checkFixtureSuccess` extended with `checkFixtureCanonical` helper for optional `want.canonical.json` / `want.hash` checks |
| `manifest/manifest.go` | Add `NormalizedHash string` and `Capabilities Capabilities` fields (after `BuiltAt`, before `ExtendsChain`) |
| `manifest/manifest_test.go` | Two new tests: round-trip for new fields, `Skipped` omitted when empty |
| `build/build.go` | `buildManifest` calls `ns.NormalizedHash()` and `computeCapabilities` |
| `docs/design/forge-overview.md` | Phase 2b roadmap line updated to "(shipped)" |

### Public API additions

```go
// spec package

// CanonicalJSON returns the compact, deterministic JSON encoding of the
// normalized spec. Memoized; repeated calls return the same backing slice.
func (ns *NormalizedSpec) CanonicalJSON() ([]byte, error)

// NormalizedHash returns the lowercase hex-encoded SHA-256 of CanonicalJSON.
// Hash covers NormalizedSpec.Spec only. Memoized.
func (ns *NormalizedSpec) NormalizedHash() (string, error)

// manifest package

type Capabilities struct {
    Present []string         `json:"present"`           // sorted kind slugs
    Skipped []CapabilitySkip `json:"skipped,omitempty"`
}

type CapabilitySkip struct {
    Kind   string `json:"kind"`
    Reason string `json:"reason"` // "not_specified"
}

// Manifest gains two new fields:
NormalizedHash string       `json:"normalizedHash"`
Capabilities   Capabilities `json:"capabilities"`
```

## Risks and mitigations

| Risk | Mitigation |
|------|-----------|
| Non-string map keys in `ComponentRef.Config` from yaml.v3 | `canonicalEncode` fallback branch coerces via `fmt.Sprintf("%v", val)` for unexpected types |
| Large integers in budget fields | `json.Decoder.UseNumber()` preserves int fidelity without float64 rounding |
| Hash churn on schema evolution | Intentional; documented in godoc; callers MUST NOT treat hash as stable across agent schema versions |
| `sync.Once` copy via NormalizedSpec value copy | `memoCanonical`/`memoHash` embed `noCopy` (Lock/Unlock no-ops); `go vet` catches erroneous copies |

## Test coverage

- `spec/canonical_test.go`: 9 tests (encoder stability × 3, hash format/stability/change/mapOrder/memoized, valid JSON)
- `spec/normalize_integration_test.go`: `TestNormalize_CanonicalStable` + `checkFixtureCanonical` extension
- `spec/testdata/normalize/canonical-stable/`: golden canonical JSON + hash locked against regression
- `manifest/manifest_test.go`: 2 new tests (round-trip, skipped-omitted)
- `build/build_hash_test.go`: 6 tests (hash format, stability, spec-change, capabilities minimal, capabilities with budget, manifest JSON round-trip)

## Commit sequence

1. `feat(spec): add JSON tags to AgentSpec for canonical serialization`
2. `feat(spec): canonical JSON encoder for NormalizedSpec`
3. `feat(spec): stable SHA-256 hash accessor on NormalizedSpec`
4. `test(spec): determinism fixture with permuted map order`
5. `feat(manifest): NormalizedHash + Capabilities fields`
6. `feat(build): stamp NormalizedHash and Capabilities onto Manifest`
7. `docs: Phase 2b design doc` (this file)

## Out of scope (Phase 3 and beyond)

- `spec.Graph()` dependency graph export
- Lockfile, bundle, signing
- Schema-agnostic hash versioning (a NormalizedHashV2 if ever needed)
- Per-element provenance for collection items
- Skills resolution / expansion rules
- MCP imports
