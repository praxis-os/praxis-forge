# Design — `praxis-forge` overview

Target shape of the module across all phases. Phase 0 ships only this
document and its siblings; Phase 1 begins the code.

## Goals

- Let a team define an agent **declaratively**, strictly validate it,
  and materialize it as a reproducible runtime unit backed by
  [`praxis`](../../../praxis).
- Compose tools, policies, filters, budgets, telemetry, credentials,
  identity, and MCP bindings under one governed build step.
- Produce a unit (`BuiltAgent` + `Manifest`) that `praxis-os` can
  schedule and observe without reaching into the kernel.

## Non-goals (never cross these lines)

Copied from the source brief, load-bearing:

- multi-agent coordination, routing, delegation
- planners, supervisors, workflow graphs
- long-lived sessions, conversation history, team semantics
- distributed memory, operator control-plane
- runtime plugin systems, reflection-heavy magic, arbitrary executable config
- session-resume state (approval, multi-turn history) — those cross
  orchestration's boundary and belong to the caller or `praxis-os`

## Internal layering

Five areas inside the module:

```
┌─────────────────────────────────────────────────────────────────┐
│ forge (top-level facade)                                        │
│   Build(ctx, spec, registry, opts) (*BuiltAgent, …)             │
└────────────▲───────────────▲──────────────▲────────────────────┘
             │               │              │
    ┌────────┴─────┐ ┌───────┴──────┐ ┌─────┴──────┐
    │  spec/       │ │  registry/   │ │  build/    │
    │  parse       │ │  typed       │ │  resolve   │
    │  validate    │ │  factories   │ │  compose   │
    │  normalize   │ │  ID → Factory│ │  materialize│
    │  overlay     │ │              │ │ (→ praxis) │
    └──────────────┘ └──────────────┘ └─────┬──────┘
                                            │
                                     ┌──────┴──────┐
                                     │  bundle/    │
                                     │  manifest   │
                                     │  lockfile   │
                                     │  hash/pack  │
                                     └─────────────┘
```

### `spec/`

- Typed Go structs for `AgentSpec`, `AgentOverlay`, embedded sub-specs
  (provider, prompt, tool packs, policy packs, filters, budget profile,
  telemetry profile, credentials, skills, MCP imports, output contract).
- Parser (YAML → structs) with **strict unknown-field rejection**.
- Normalizer: merges overlays, resolves `extends:` chains, canonicalizes
  field ordering for hashing.
- Schema validation: structural, referential (every `ref:` resolvable),
  acyclic (extends), version-gated.

### `registry/`

- `ComponentRegistry` indexing typed factories by `(kind, id, version)`.
- Kinds: `provider`, `tool_pack`, `policy_pack`, `pre_llm_filter`,
  `pre_tool_filter`, `post_tool_filter`, `mcp_binding`, `skill`,
  `budget_profile`, `telemetry_profile`, `credential_resolver`,
  `identity_signer`.
- Factories are Go code registered at program start (no runtime plugin
  loading). Each factory exposes its own config JSON Schema used by spec
  validation at build time.

### `build/`

- Dependency resolution: walk the normalized spec, fetch each referenced
  factory, collect required capabilities (e.g. a skill declaring
  required tools/policies), surface conflicts.
- Compatibility checks: provider capability vs. tool shape, policy risk
  tier vs. tool risk tier, budget profile vs. request ceilings.
- Composition adapters: multi-policy → single `hooks.PolicyHook`,
  multi-filter → single `hooks.PreLLMFilter` / `PreToolFilter` /
  `PostToolFilter`, multi-tool-pack → single `tools.Invoker` (routing
  by name prefix), MCP bindings → projected into the tool namespace.
- Materialization: call `orchestrator.New(provider, opts...)` with the
  assembled options and wrap the resulting `*orchestrator.Orchestrator`
  in a `BuiltAgent` along with its `Manifest`.

### `bundle/`

- `Manifest`: inspectable, machine-readable record of what was built —
  every resolved factory id + version + config hash, capability list,
  governance tags, signing info.
- `Lockfile`: pinned resolution of factories + external references
  (MCP servers, credential scopes) with cryptographic hashes.
- `Bundle`: zipped AgentSpec + overlays + lockfile + manifest for
  distribution. Integrity check verifies bundle hashes match manifest.
- Phase-5 concerns. Phase 1 ships only the manifest (no lockfile, no
  bundle).

### Top-level `forge` facade

Single import for consumers:

```go
built, err := forge.Build(ctx,
    spec,          // loaded via forge.LoadSpec
    registry,      // *forge.ComponentRegistry
    forge.WithTelemetry(t),
    forge.WithLogger(l),
)
// Phase 2 grows this signature with an `overlays []forge.AgentOverlay`
// parameter between spec and registry.
if err != nil { ... }

result, err := built.Invoke(ctx, praxis.InvocationRequest{ ... })
```

`BuiltAgent.Invoke` is a thin, stateless pass-through to the embedded
`*orchestrator.Orchestrator`. Per-turn state stays inside praxis. No
conversation memory is stored in forge.

## Data flow

```
AgentSpec (yaml) ─▶ parse ─▶ validate ─▶ normalize ─▶ + overlays ─▶ canonical spec
                                                                         │
          registry (ID → Factory) ──────────────────────────────────────┤
                                                                         ▼
                                                           resolve + compose
                                                                         │
                                   compatibility + governance checks ◀──┤
                                                                         ▼
                                             orchestrator.New(provider, opts...)
                                                                         │
                                                                         ▼
                                                    BuiltAgent + Manifest
                                                                         │
                                                                         ▼
                                                           (optional) Bundle + Lockfile
```

## Phase roadmap (from seed, with module-internal notes)

- **Phase 0 — contracts.** This document and siblings. No code.
- **Phase 1 — minimum vertical slice.** `spec/` loader + strict
  validation, one typed registry, one factory per kernel seam (11
  kinds including `prompt_asset`), composition adapters, materialization
  into a real `*orchestrator.Orchestrator`, minimal Go API, one
  realistic demo, unit + offline integration tests. Detailed scope
  in `docs/superpowers/specs/2026-04-15-praxis-forge-phase-1-design.md`.
  No overlays in the `Build` signature; added in Phase 2. The broader
  default component set described in
  [`default-toolpacks.md`](default-toolpacks.md) and
  [`default-policypacks.md`](default-policypacks.md) is a target that
  accrues across Phase 1.x → Phase 2; Phase 1 itself ships one factory
  per seam.
- **Phase 2 — composition depth.** `Build` signature grows
  `overlays []AgentOverlay`. Overlays, `extends:`, canonical manifest,
  deterministic build (stable hashing), richer inspection.
- **Phase 3 — skills.** Skill registry, expansion rules, prompt-fragment
  merge, dependency/conflict validation, output contracts.
- **Phase 4 — MCP consume.** MCP imports, remote metadata normalization,
  auth/trust metadata, allowlist/denylist, projection into forge tool
  namespace.
- **Phase 5 — packaging.** Bundle format, lockfile, artifact metadata,
  reproducibility guarantees, integrity checks.
- **Phase 6 — `praxis-os` handoff contract.** Freeze the invoke/capability/
  governance/identity/manifest surfaces; document stability guarantees.

## Design principles (applied to every PR)

- **Explicit over magical.** No reflection-driven wiring beyond what Go
  stdlib `encoding/json` + `yaml` provide for typed decoding.
- **Typed contracts over generic maps.** Every factory config is a typed
  struct with its own JSON Schema. No `map[string]any` leaking into
  downstream code.
- **Strict validation over permissive fallback.** Unknown fields,
  unresolved refs, cyclic `extends`, capability mismatches — all fail
  the build.
- **Deterministic builds.** Canonical ordering, stable hashing. A given
  spec + overlays + registry must produce an identical manifest.
- **Governability and inspectability.** Every `BuiltAgent` carries a
  manifest that answers "what is inside this agent and where did it
  come from" without source access.
- **Build-time extensibility, not runtime plugins.** New capabilities
  arrive via new factories in Go, not via dlopen or WASM sandboxes.
- **Strong separation.** Kernel ↔ definition ↔ orchestration. Any PR
  that blurs these must be rejected.
