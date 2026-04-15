# praxis-forge Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the first vertical slice of praxis-forge: parse + validate a declarative `AgentSpec` YAML, resolve every component through a typed `ComponentRegistry`, materialize a stateless `BuiltAgent` backed by `*orchestrator.Orchestrator`, with one concrete factory per kernel seam (11 Kinds) and a runnable realistic demo.

**Architecture:** Module-cohesive Go layout. `spec/` parses + validates YAML; `registry/` holds typed per-Kind factories; `build/` resolves refs, composes multi-component chains into praxis single-instance hooks, and materializes via `orchestrator.New`. `manifest/` records the resolved build. 11 concrete factories live under `factories/<kind>/`. Top-level `forge.go` is a thin facade. No overlays, no `extends`, no skills, no MCP, no lockfile in Phase 1.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3` (strict decode), `log/slog`, `crypto/ed25519`, `net/http`. Depends on `github.com/praxis-os/praxis` via local `replace` to `../praxis`. Tests use stdlib `testing` with table-driven patterns. Lint: `golangci-lint` (reuse praxis config). No external validation library.

**Companion spec:** [`docs/superpowers/specs/2026-04-15-praxis-forge-phase-1-design.md`](../specs/2026-04-15-praxis-forge-phase-1-design.md)

---

## File Structure

Each file has one responsibility. Keep files under ~400 LOC; split if a file grows beyond that.

### `spec/` (Task group 1)

- `spec/ids.go` — `ParseID`, regex, error types
- `spec/types.go` — all YAML-mapped structs (`AgentSpec`, `Metadata`, `ComponentRef`, etc.)
- `spec/load.go` — `LoadSpec(path)` using strict `yaml.v3` decoder
- `spec/validate.go` — `(*AgentSpec).Validate()` running invariants in fixed order
- `spec/errors.go` — `ValidationError`, `Errors` aggregator
- `spec/testdata/valid/*.yaml`, `spec/testdata/invalid/*.yaml`, `*.err.txt`

### `registry/` (Task group 2)

- `registry/kind.go` — `Kind` string type + 11 constants
- `registry/id.go` — `ID` type (re-exports `spec.ParseID` semantics)
- `registry/types.go` — result structs: `ToolPack`, `PolicyPack`, `BudgetProfile`, `TelemetryProfile`, `ToolDescriptor`, `PolicyDescriptor`, `RiskTier`
- `registry/factories.go` — 11 factory interfaces
- `registry/registry.go` — `ComponentRegistry` + per-kind `Register*` / lookup methods + `Freeze`
- `registry/errors.go` — `ErrRegistryFrozen`, `ErrDuplicate`, `ErrNotFound`, `ErrKindMismatch`

### `build/` + `manifest/` (Task group 3)

- `manifest/manifest.go` — `Manifest`, `ResolvedComponent`, JSON marshal
- `build/resolver.go` — walk spec, resolve refs
- `build/policy_chain.go` — `policyChain` adapter
- `build/filter_chains.go` — three filter-stage chain adapters
- `build/tool_router.go` — `toolRouter`
- `build/budget.go` — `applyBudgetOverrides`
- `build/build.go` — top-level `Build` function

### `factories/<kind>/` (Task group 4)

One leaf package per factory, each with `factory.go` + `factory_test.go`.

### Facade + demo (Task group 5)

- `forge.go` — re-exports + `Option` type
- `forge_test.go` — offline integration test
- `internal/testutil/fakeprovider/fakeprovider.go`
- `examples/demo/main.go`, `examples/demo/agent.yaml`
- Root `README.md` update

### Commit 6 (Task group 6)

- Phase 0 doc amendments.

---

## Conventions used throughout

- **TDD**: every task is test-first. Write the failing test, run it, implement, rerun.
- **Error style**: return errors wrapping `fmt.Errorf("%w", sentinel)` for typed matching via `errors.Is`.
- **Imports**: standard Go ordering — stdlib, then external, then this module, separated by blank lines.
- **License header**: first line of every `.go` file is `// SPDX-License-Identifier: Apache-2.0` matching [doc.go](../../../doc.go).
- **Package docs**: each non-leaf package has a `doc.go` with a package comment.
- **Commits**: one commit per task unless the task explicitly says otherwise. Conventional-commit format: `feat(pkg): short line`.

---

## Task groups

Each task group lives in its own file. Task numbering within files is
preserved from the original flat plan (e.g. `Task 1.3.2` stays as
`Task 1.3.2` in `01-spec.md`). The numbered commit order listed under
"Delivery shape" in the companion design spec maps 1:1 to these files.

- [`00-repo-prep.md`](00-repo-prep.md) — Task group 0: Repo prep
- [`01-spec.md`](01-spec.md) — Task group 1: `spec/` package
- [`02-registry.md`](02-registry.md) — Task group 2: `registry/` package
- [`03-build-manifest.md`](03-build-manifest.md) — Task group 3: `manifest/` + `build/`
- [`04-factories.md`](04-factories.md) — Task group 4: 11 concrete factories
- [`05-facade-demo.md`](05-facade-demo.md) — Task group 5: facade + fakeprovider + integration + demo
- [`06-doc-amendments.md`](06-doc-amendments.md) — Task group 6: Phase 0 doc amendments

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
Expected: `forge_test.go` exercises all 10 non-tool kinds; passes.

- [ ] **Step V5: Live Anthropic round-trip (manual)**

Run: `ANTHROPIC_API_KEY=$KEY go run -tags=integration ./examples/demo`
Expected: demo fetches the allowed URL, returns a non-empty response, logs show one slog line per lifecycle event.

- [ ] **Step V6: Manifest inspection**

In a scratch test, assert `b.Manifest().Resolved` contains one entry per referenced component with the exact config the spec declared.

- [ ] **Step V7: Registry freeze sanity**

Assert `r.RegisterProvider(...)` after `forge.Build` returns `ErrRegistryFrozen`.

---

## Self-review notes (for plan author)

Coverage matrix of the design spec → task mapping:

| Spec section | Task(s) |
|--------------|---------|
| Public API + layering | 5.1 |
| `spec/` contract | 1.1–1.8 |
| `registry/` contract | 2.1–2.5 |
| `build/` pipeline + chains + router + budget | 3.1–3.8 |
| 11 concrete factories | 4.1–4.11 |
| Testing strategy (unit + offline integration + tagged live) | 5.3, 5.4, per-factory tests |
| Delivery shape (5 atomic commit tracks) | 1.x, 2.x, 3.x, 4.x, 5.x task groups |
| Phase 0 doc amendments | 6.1 |
| Verification | Final verification block |
| Out of scope | Documented in plan context; not implemented |

Red-flag re-check — searched this plan for each phrase from the "No Placeholders" list:
- No "TBD"/"TODO"/"implement later".
- Every step that changes code shows the code.
- Expected command output is named (PASS / FAIL / clean build).
- Every referenced type is introduced in an earlier task or imported from praxis/praxis-forge.

Type consistency checks:
- `registry.BudgetProfile.Guard` and `.DefaultConfig` used identically in Tasks 3.5, 3.7, 3.8, 4.2, 5.3.
- `hooks.FilterActionBlock` / `FilterActionRedact` / `FilterActionLog` / `FilterActionPass` used with matching spelling in Tasks 3.3, 4.6, 4.7, 4.8.
- `tools.ToolCall.ArgumentsJSON` and `tools.ToolResult.Output` appear in Tasks 4.7, 4.8, 4.10 — flagged as "verify against real praxis type" in each task. Engineer must reconcile before writing the test.
- Factory ID strings (e.g. `provider.anthropic@1.0.0`) consistent between test fixtures, integration test, and demo YAML.

Known items the engineer must verify against the real praxis source before implementing (flagged inline in the relevant tasks):
1. `tools.ToolCall` arguments field name (`ArgumentsJSON` vs `Arguments`).
2. `tools.ToolResult` content field name (`Output` vs `Content`).
3. `llm.Message.Content` type (plain string vs multimodal blocks).
4. `event.InvocationEvent` field name (`Kind` vs `Name`).
5. `anthropic.New` signature and functional options.
6. `identity.NewEd25519Signer` functional options (`WithIssuer`, `WithTokenLifetime`).
7. `credentials.Credential` interface method names.
8. `hooks.PolicyInput` shape for `PhasePreLLMInput`.
9. `budget.NullGuard` vs `NewInMemoryGuard` availability.

These are unavoidable — the plan cannot pin names it has not confirmed. Each is flagged at the point of use so the engineer resolves it in context.
