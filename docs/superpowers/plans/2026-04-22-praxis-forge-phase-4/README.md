# praxis-forge Phase 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Activate MCP consume as a declarative runtime binding contract — forge validates and stamps a governable binding (connection, auth ref, allow/deny, policies, trust, onNewTool) into the manifest. No network I/O during `Build`.

**Architecture:** New `KindMCPBinding` activates in the registry with a generic `mcp.binding@1` factory. `spec.mcpImports` unlocks with structural validation. A new `build/mcp.go` resolves bindings with cross-kind validation (policies and credentials must already be registered). Resolved bindings flow through the existing `resolved` struct into manifest `ResolvedComponent` entries. No new hash; `ExpandedHash` semantics unchanged. Vertical slice: a filesystem-server binding exercised via a `-mcp` demo flag.

**Tech Stack:** Go 1.23, existing patterns — registry factories, canonical-JSON manifest, spec/validate error aggregation, TDD with golden hash files.

**Spec:** [`docs/superpowers/specs/2026-04-22-praxis-forge-phase-4-design.md`](../../specs/2026-04-22-praxis-forge-phase-4-design.md)

---

## File structure

### Modified

- [registry/kind.go](../../../../registry/kind.go) — move `KindMCPBinding` from deferred to active.
- [registry/kind_test.go](../../../../registry/kind_test.go) — assert `KindMCPBinding == "mcp_binding"`.
- [registry/types.go](../../../../registry/types.go) — add `MCPBinding`, `MCPConnection`, `MCPTransport`, `MCPAuth`, `MCPTrust`, `OnNewToolPolicy`, `MCPBindingDescriptor`.
- [registry/factories.go](../../../../registry/factories.go) — add `MCPBindingFactory` interface.
- [registry/registry.go](../../../../registry/registry.go) — add `mcpBindings` map, `RegisterMCPBinding`, `MCPBinding(id)` lookup.
- [registry/registry_test.go](../../../../registry/registry_test.go) — add `fakeMCPBindingFactory` + duplicate/lookup/frozen test.
- [spec/validate.go](../../../../spec/validate.go) — remove mcpImports phase-gate at lines 82–84; add ref + structural validation.
- [spec/validate_test.go](../../../../spec/validate_test.go) — MCP validation tests.
- [build/resolver.go](../../../../build/resolver.go) — add `mcpBindings` / `mcpBindingIDs` / `mcpBindingCfgs` fields to `resolved`.
- [build/build.go](../../../../build/build.go) — call `resolveMCPBindings` after `resolve`, stamp results onto `res`, emit manifest entries, include in capabilities.
- [build/capabilities.go](../../../../build/capabilities.go) — add `mcp_binding` to present/skipped lists.
- [build/tool_router.go](../../../../build/tool_router.go) — reserve `mcp.*` tool-name prefix; reject collisions.
- [examples/demo/main.go](../../../../examples/demo/main.go) — add `-mcp` flag and registration.
- [docs/design/agent-spec-v0.md](../../../design/agent-spec-v0.md) — drop `mcpImports` from deferred list.
- [docs/design/forge-overview.md](../../../design/forge-overview.md) — mark Phase 4 shipped.

### Created

- [build/mcp.go](../../../../build/mcp.go) — `resolveMCPBindings`, config decode, cross-kind validation.
- [build/mcp_test.go](../../../../build/mcp_test.go) — unit tests for `resolveMCPBindings`.
- [build/build_mcp_test.go](../../../../build/build_mcp_test.go) — end-to-end integration.
- [factories/mcpbinding/factory.go](../../../../factories/mcpbinding/factory.go) — `mcp.binding@1` generic factory.
- [factories/mcpbinding/factory_test.go](../../../../factories/mcpbinding/factory_test.go) — factory unit tests.
- `spec/testdata/mcp/minimal-stdio/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/stdio-with-env/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/http-with-bearer/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/http-with-apikey/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/multiple-bindings/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/on-new-tool-variants/spec.yaml` + `want.expanded.hash`
- `spec/testdata/mcp/unresolved-policy/spec.yaml` + `err.txt`
- `spec/testdata/mcp/unresolved-credential/spec.yaml` + `err.txt`
- `spec/testdata/mcp/duplicate-binding-id/spec.yaml` + `err.txt`
- `spec/testdata/mcp/transport-field-mismatch/spec.yaml` + `err.txt`
- [examples/demo/agent-mcp.yaml](../../../../examples/demo/agent-mcp.yaml) — demo spec.

Working directory for all commands: `/Users/francescofiore/Coding/praxis-os/praxis-forge`.
Baseline verification: `go test -race -count=1 ./...` must be green before starting.

---

## Task groups

Each task group lives in its own file. Each corresponds to one or more atomic commits.

- [`00-registry.md`](00-registry.md) — Tasks 1–4: activate `KindMCPBinding`, add types, factory interface, register/lookup methods
- [`01-spec-validation.md`](01-spec-validation.md) — Tasks 5–6.5: unlock `mcpImports` gate, structural validation, extends-chain merge
- [`02-factory.md`](02-factory.md) — Task 7: `mcp.binding@1` generic factory
- [`03-build-pipeline.md`](03-build-pipeline.md) — Tasks 8–11: `resolveMCPBindings`, Build wiring, capabilities flag, `mcp.*` prefix guard
- [`04-fixtures-integration.md`](04-fixtures-integration.md) — Task 12: fixtures + end-to-end integration tests
- [`05-demo-docs.md`](05-demo-docs.md) — Tasks 13–14: `-mcp` demo flag, docs amendments

---

## Final verification

After every task group is complete:

- [ ] **Step V1: Full test suite**

Run: `go test -race -count=1 ./...`
Expected: green, no skips beyond the existing integration-tagged package.

- [ ] **Step V2: Lint + format**

Run: `make fmt && make vet && make lint`
Expected: all green.

- [ ] **Step V3: Banned-grep**

Run: `make banned-grep`
Expected: clean.

- [ ] **Step V4: Smoke the demo (offline)**

Run: `ANTHROPIC_API_KEY=dummy go run -tags=integration ./examples/demo -mcp`
Expected: prints the `mcp_binding` contract JSON + NOTE line, exits cleanly before any LLM call.

- [ ] **Step V5: Inspect manifest shape**

```bash
go test -race -count=1 -run TestMCPFixtures_Success ./build/ -v
```

Expected: all fixture sub-tests pass (`minimal-stdio`, `stdio-with-env`, `http-with-bearer`, `http-with-apikey`, `multiple-bindings`, `on-new-tool-variants`).

- [ ] **Step V6: Final commit if docs/state drifted**

```bash
git status
```

Commit cleanly per-concern if any diff remains. Otherwise, nothing to do.

---

## Completion checklist

- [ ] `KindMCPBinding` active and visible via `Each` enumeration paths (registry).
- [ ] `spec.mcpImports` unlocked, prefix + structural validation live.
- [ ] `factories/mcpbinding/mcp.binding@1` ships and decodes both transports.
- [ ] `build/resolveMCPBindings` runs with cross-kind validation (policies + credentials).
- [ ] Manifest emits `mcp_binding` `ResolvedComponent`s; `Capabilities.Present/Skipped` includes `mcp_binding`.
- [ ] `mcp.` tool-name prefix reserved in the tool router.
- [ ] 10 fixtures under `spec/testdata/mcp/` cover happy paths + all declared error codes.
- [ ] `-mcp` demo flag exercises the path end-to-end without network.
- [ ] `docs/design/agent-spec-v0.md` and `docs/design/forge-overview.md` reflect Phase 4 shipped.
- [ ] `make fmt && make vet && make lint && make banned-grep` all green.
- [ ] `go test -race -count=1 ./...` green.
