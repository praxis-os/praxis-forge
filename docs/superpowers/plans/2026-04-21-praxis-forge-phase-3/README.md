# praxis-forge Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land Phase 3 of praxis-forge — skills and output contracts. Activate `KindSkill` and `KindOutputContract` in the registry, unlock the matching phase-gated fields in `AgentSpec`, add a skill-expansion stage to `Build` with strict conflict detection, stamp a post-expansion `ExpandedHash` and `InjectedBySkill` attribution onto the `Manifest`, and ship two vertical-slice factories (`skill.structured-output@1` + `outputcontract.json-schema@1`) that prove the path end-to-end.

**Architecture:** Two new factory kinds slot into the existing per-kind typed registry pattern (`registry/kind.go`, `registry/factories.go`, `registry/registry.go`). New value types `Skill`, `OutputContract`, `RequiredComponent`, `SkillDescriptor`, `OutputContractDescriptor` live in `registry/types.go`. A new `build/expand.go` module resolves skill factories, auto-injects contributions into effective tools/policies/output-contract sets, detects conflicts (version divergence, config divergence, multiple output contracts, user override, unresolved refs, empty contribution), and produces an `ExpandedSpec` consumed by the existing `resolve` + materialization path. Post-expansion canonical JSON + SHA-256 hash lives on the manifest as `ExpandedHash` (separate from Phase 2b's pre-expansion `NormalizedHash`). `ResolvedComponent.InjectedBySkill` records attribution when a skill drove inclusion. `Capabilities.Present`/`Skipped` gains `skill` and `output_contract`. Two concrete factories demonstrate the path: `skill.structured-output@1` (prompt fragment + required policy + required output contract) and `outputcontract.json-schema@1` (stores JSON Schema as `map[string]any` with structural-only validation, zero new deps).

**Tech Stack:** Go 1.26, stdlib `encoding/json` (canonical encoding via existing `spec.canonicalEncode` helper from Phase 2b), stdlib `crypto/sha256`, stdlib `regexp` for id-prefix validation, stdlib `testing` with table-driven and fixture-driven patterns, `gopkg.in/yaml.v3` via existing `spec.LoadSpec`. **No new third-party dependency.**

**Companion spec:** [`docs/superpowers/specs/2026-04-21-praxis-forge-phase-3-design.md`](../../specs/2026-04-21-praxis-forge-phase-3-design.md)

---

## File Structure

Each new file has one responsibility. Keep files under ~400 LOC; split if a file grows beyond that. Existing files are only touched where the design spec calls for a specific, localized change.

### `registry/` (changes + new entries in existing files)

- `registry/kind.go` — `KindSkill`, `KindOutputContract` move from (future) deferred block to the active const list
- `registry/types.go` — new types: `Skill`, `OutputContract`, `RequiredComponent`, `SkillDescriptor`, `OutputContractDescriptor`
- `registry/factories.go` — new interfaces: `SkillFactory`, `OutputContractFactory`
- `registry/registry.go` — new maps, `RegisterSkill`/`Skill`/`RegisterOutputContract`/`OutputContract`
- `registry/registry_test.go` — fakes + register/lookup/duplicate/frozen tests for both kinds

### `spec/` (changes to existing files)

- `spec/validate.go` — remove the `skills` and `outputContract` phase-gate blocks; add a referential check requiring `skill.*` / `outputcontract.*` id prefixes
- `spec/validate_test.go` — new tests for the referential check; rewire existing phase-gate tests
- `spec/testdata/overlay/invalid/` — remove `phase_gated_skills.*`; add `phase_gated_mcp_imports.*` if any gate test was using skills

### `build/` (new files + light edits)

- `build/expand.go` — new: `ExpandedSpec`, `expandSkills`, `resolveSkillsAndContract`, conflict-detection helpers
- `build/expand_test.go` — new: unit tests for every row of the expansion-semantics table
- `build/expanded_hash.go` — new: `computeExpandedHash` (reuses `spec.canonicalEncode`)
- `build/expanded_hash_test.go` — new: hash stability + determinism tests
- `build/build.go` — wire `expandSkills` into `Build`; extend `buildManifest` to emit skill rows + `InjectedBySkill` + `ExpandedHash`
- `build/capabilities.go` — add `skill` and `output_contract` to `Present`/`Skipped` logic
- `build/build_skills_test.go` — new: end-to-end integration tests using the two vertical-slice factories

### `manifest/` (changes to existing files)

- `manifest/manifest.go` — `ResolvedComponent` gains `InjectedBySkill`; `Manifest` gains `ExpandedHash`
- `manifest/manifest_test.go` — new tests for both fields (round-trip + omitempty)

### `factories/` (two new packages)

- `factories/outputcontractjsonschema/factory.go`
- `factories/outputcontractjsonschema/factory_test.go`
- `factories/skillstructuredoutput/factory.go`
- `factories/skillstructuredoutput/factory_test.go`

### `examples/demo/` (small extension)

- `examples/demo/main.go` — gains a `-structured` flag that registers both new factories and builds an agent that uses them

### `docs/` (amendments to existing files)

- `docs/design/agent-spec-v0.md` — remove `skills` and `outputContract` from §"Explicit deferrals"; mark the new kinds as active
- `docs/design/forge-overview.md` — mark Phase 3 as "(shipped)" in the roadmap

### `spec/testdata/skills/` (new fixture tree — used by `build/build_skills_test.go`)

Thirteen scenario directories — see [`08-integration-tests.md`](08-integration-tests.md).

---

## Task Breakdown

| Task | File | Scope |
|------|------|-------|
| 00 | [`00-registry-kinds-and-types.md`](00-registry-kinds-and-types.md) | Activate `KindSkill`/`KindOutputContract`; add `Skill`/`OutputContract`/`RequiredComponent` + descriptor types |
| 01 | [`01-registry-factories-and-registration.md`](01-registry-factories-and-registration.md) | `SkillFactory`/`OutputContractFactory` interfaces + `ComponentRegistry` register/lookup + tests |
| 02 | [`02-spec-validation-unlock.md`](02-spec-validation-unlock.md) | Remove `skills` and `outputContract` phase-gate; add id-prefix referential validation |
| 03 | [`03-expansion-core.md`](03-expansion-core.md) | `build/expand.go` — resolve skill + output-contract factories, auto-inject, conflict detection |
| 04 | [`04-expanded-hash.md`](04-expanded-hash.md) | `build/expanded_hash.go` — canonical JSON + SHA-256 over `ExpandedSpec.Spec` |
| 05 | [`05-manifest-extensions.md`](05-manifest-extensions.md) | `Manifest.ExpandedHash` + `ResolvedComponent.InjectedBySkill` + marshal/unmarshal tests |
| 06 | [`06-build-pipeline-wire.md`](06-build-pipeline-wire.md) | Wire expansion into `Build`; extend `buildManifest` and `computeCapabilities` |
| 07 | [`07-vertical-slice-factories.md`](07-vertical-slice-factories.md) | `outputcontract.json-schema@1` + `skill.structured-output@1` factories |
| 08 | [`08-integration-tests.md`](08-integration-tests.md) | Fixture tree + `build_skills_test.go` integration tests with golden canonical JSON + expanded-hash |
| 09 | [`09-demo-and-docs.md`](09-demo-and-docs.md) | `examples/demo -structured` flag + `agent-spec-v0.md` + `forge-overview.md` amendments |

Each task is self-contained: write failing test → run it (confirm failure) → minimal implementation → run tests (confirm pass) → commit. Every step states exact file paths, complete code, and the exact command to run with its expected output.

---

## Conventions

**Commit messages** follow the Phase 2b convention seen in recent git history:

- `feat(<area>): <imperative>` for functional additions
- `test(<area>): <imperative>` for test-only additions
- `docs: <imperative>` for doc-only changes
- `examples(<area>): <imperative>` for example code

**Go build** is verified at each commit via `go build ./...` + `go vet ./...` + `go test ./<touched-packages>/...`. The linter config in [`.golangci.yml`](../../../.golangci.yml) must pass; run `make lint` if the repo's Makefile exposes it.

**Path conventions** from the design spec are load-bearing:

- Skill factory ids must match `^skill\.[a-z0-9-]+@\d+(\.\d+){0,2}$`
- Output-contract factory ids must match `^outputcontract\.[a-z0-9-]+@\d+(\.\d+){0,2}$`
- Fixture directories under `spec/testdata/skills/<scenario>/` contain `spec.yaml` + (success cases) `want.expanded.json` and `want.expanded.hash`

**Hash goldens** are regenerated when the canonical form intentionally changes. The integration test in Task 08 includes a regen helper (documented inline) to rewrite all `want.expanded.*` files when `WRITE_GOLDEN=1` is set in the environment.
