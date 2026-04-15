# Claude Code Prompt — Praxis Forge

Read this document carefully and use it as the primary implementation brief.

Your task is to design and implement **`praxis-forge`** as the **agent definition, composition, packaging, and materialization layer** between **`praxis`** and the future **`praxis-os`**.

## Core positioning

Treat the architecture as three distinct layers:

* **`praxis`** = invocation kernel
* **`praxis-forge`** = agent definition/composition/materialization layer
* **`praxis-os`** = orchestration and control-plane layer for multi-agent systems

Do not blur these boundaries.

`praxis-forge` must not become a mini orchestration framework and must not introduce multi-agent semantics.

## What `praxis-forge` is

`praxis-forge` is **not** a simple YAML loader and **not** a thin prompt wrapper.

It is the layer that:

* defines agents declaratively
* resolves registered components
* composes prompts, skills, tools, policies, and MCP bindings
* validates the resulting configuration strictly
* materializes a reproducible, governable, invocable runtime unit backed by `praxis`
* produces stable units that the future `praxis-os` can orchestrate

## What it must not do

Do **not** introduce any of the following into `praxis-forge`:

* multi-agent coordination
* routing/delegation between agents
* planners/supervisors
* workflow graphs
* long-lived session orchestration
* team semantics
* distributed memory systems
* operator control-plane concerns that belong to `praxis-os`
* runtime plugin systems
* reflection-heavy magic
* arbitrary executable config behavior

Config must **select registered behavior**, never define arbitrary runtime behavior.

## Design principles

Implement and evaluate every decision against these principles:

* explicit over magical
* typed contracts over generic maps
* strict validation over permissive fallback behavior
* deterministic builds over implicit resolution
* governability and inspectability by default
* build-time extensibility over dynamic plugin systems
* strong separation between kernel, definition layer, and orchestration layer

## Conceptual model

Model `praxis-forge` around these artifacts:

* **`AgentSpec`**: user-authored declarative agent definition
* **`AgentOverlay`**: environment-specific overrides
* **`BuiltAgent`**: materialized runtime unit backed by `praxis`
* **`AgentBundle`**: packageable/distributable representation
* **`Lockfile`**: deterministic resolution record
* **`ComponentRegistry`**: typed registry resolving providers, policies, filters, tools, MCP bindings, and skills

## How to model core concepts

### Tools

Treat tools as governed runtime capabilities, not ad hoc functions.

Each tool or tool pack should be modeled with metadata such as:

* stable id
* input/output schema
* description
* auth requirements
* risk tier
* policy tags
* source/owner metadata
* timeout/retry hints
* audit metadata

### MCP

Treat MCP primarily as a **binding/import/export layer** for capabilities.

`praxis-forge` should eventually support:

1. **consume MCP**: import remote capabilities from MCP servers
2. **expose as MCP**: project agent or pack capabilities outward through MCP

For now, design for both, but do not overbuild. The important part is that MCP capabilities are normalized into forge-managed, governable runtime capabilities.

### Skills

Treat skills as **composable higher-level packages**, not mere prompt snippets.

A skill may contribute:

* prompt fragments
* examples/few-shots
* required tools
* required policies
* required MCP bindings
* output contracts
* model preferences
* safety constraints
* validation rules

Skills should expand an agent definition at build time and participate in dependency/conflict validation.

### Policies and filters

Policies and filters must remain first-class because `praxis` itself is centered on policy enforcement, filter chains, budget control, observability, and trust boundaries.

Forge should compose these explicitly and validate compatibility.

## Shape of the full AgentSpec

Design toward a complete spec that can eventually express:

* metadata and versioning
* inheritance / extension from base specs
* provider/model configuration
* prompt assets and templates
* skills
* tool packs
* MCP imports
* policy packs
* filters
* budget profiles and overrides
* telemetry profiles
* credentials/auth requirements
* output contracts
* environment overlays

Important: keep the spec declarative and strict. Avoid fields that imply arbitrary code execution.

## Required architecture boundaries

These invariants must hold:

1. `praxis` must remain unaware of `praxis-forge`
2. `praxis-forge` may depend on `praxis`, never the reverse
3. `praxis-forge` must stop before orchestration semantics begin
4. `praxis-os` should eventually consume stable agent-level units produced by `praxis-forge`, not raw kernel wiring
5. skills, tools, policies, and MCP bindings must resolve through typed registries/factories
6. build outputs must be deterministic and inspectable

## Internal architecture direction

Design `praxis-forge` around these internal areas:

* **spec layer**: parsing, schema validation, normalization, overlays, versioning
* **registry layer**: typed resolution of providers, tools, policies, filters, MCP bindings, skills
* **build layer**: dependency resolution, compatibility checks, expansion, materialization into `BuiltAgent`
* **packaging layer**: bundles, lockfiles, hashes, reproducibility
* **runtime facade**: Go API for loading/building/invoking agents backed by `praxis`

Do not work on CLI right now.

## What I want from you first

Before implementing major code, do the following in order:

1. inspect the current public/runtime surface of `praxis`
2. identify which seams already exist and can be reused directly
3. write an ADR introducing `praxis-forge`
4. write a design doc for the full target shape of `praxis-forge`
5. define the initial `AgentSpec` model and its invariants
6. explicitly list any mismatch between the proposed forge model and the real `praxis` runtime seams

If you find architectural mismatches, surface them explicitly before expanding the code.

## Implementation roadmap

### Phase 0 — architecture and contracts

Produce:

* ADR for `praxis -> praxis-forge -> praxis-os`
* design doc for forge
* initial spec design
* registry interface design
* invariants and non-goals

### Phase 1 — minimum vertical slice

Implement only the minimum useful non-CLI slice:

* load an `AgentSpec`
* strict schema validation
* typed component registry
* resolve provider/model
* resolve at least one policy pack
* resolve at least one tool pack
* materialize a `BuiltAgent` backed by `praxis`
* minimal Go API to build and invoke
* tests and small examples

### Phase 2 — deeper composition model

Then expand toward:

* overlays
* inheritance/extends
* normalized manifest
* lockfile
* deterministic resolution
* richer inspection/explainability
* pack metadata and compatibility checks

### Phase 3 — skills model

Add:

* skill registry
* skill expansion rules
* dependency/conflict validation
* prompt fragment merge rules
* required capability enforcement
* output contract support

### Phase 4 — MCP consume model

Add:

* MCP import bindings
* normalization of remote capability metadata
* auth and trust metadata
* allowlist/denylist behavior
* projection into forge-managed runtime capabilities

### Phase 5 — packaging/distribution

Add:

* bundle format
* stable lockfile
* artifact metadata
* reproducibility guarantees
* export/import model
* package integrity checks

### Phase 6 — handoff contract for `praxis-os`

Define the stable contract that the future orchestration layer can consume:

* invoke surface
* capability metadata surface
* governance metadata surface
* runtime identity
* inspectable manifest/bundle metadata

## Acceptance criteria

A good implementation should make all of the following true:

* an agent can be defined declaratively without writing low-level runtime wiring
* the resulting built agent is backed by `praxis`, not by a parallel runtime
* tools, skills, policies, and MCP bindings are composable but governed
* the build is deterministic and inspectable
* the resulting unit is suitable for future orchestration by `praxis-os`
* no orchestration concerns leak into forge
* the kernel remains clean and independent

## Additional constraints

* prefer small, load-bearing abstractions
* avoid premature generalization
* avoid framework creep
* keep naming precise
* be explicit about trust boundaries
* preserve observability, auditability, budget control, and policy enforcement
* surface hidden coupling immediately
* challenge the design if it starts duplicating `praxis-os` concerns

## Delivery expectation

Do not jump straight into broad implementation.

Start with:

1. architecture review of current `praxis`
2. ADR
3. design doc
4. initial `AgentSpec` proposal
5. mismatch analysis

Only then propose the first implementation slice.
