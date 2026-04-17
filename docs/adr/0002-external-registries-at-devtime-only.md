# ADR 0002 — External registries (MCP, skills) cross the boundary at dev time, not runtime

- **Status:** Accepted (Phase 0)
- **Date:** 2026-04-15
- **Deciders:** praxis-os maintainers
- **Supersedes:** —
- **Related:** [`design/external-registries.md`](../design/external-registries.md), [`design/registry-interfaces.md`](../design/registry-interfaces.md), [`design/default-toolpacks.md`](../design/default-toolpacks.md), [`adr/0001-praxis-forge-layering.md`](0001-praxis-forge-layering.md)

## Context

Two external ecosystems are in scope for forge:

- **MCP servers** — the Model Context Protocol has a reasonably
  convergent registry picture: an official registry maintained by the
  MCP project, plus third-party directories (Smithery, mcp.so,
  PulseMCP, Glama) sharing the same underlying manifest shape.
- **Agent skills** — fragmented. Anthropic Agent Skills is the
  dominant format in the Claude ecosystem; CrewAI, AutoGen,
  LangChain, Semantic Kernel each carry their own. No cross-vendor
  package manager exists as of Apr 2026.

Forge's existing constraints force a decision:

- ADR 0001 forbids runtime plugin systems. The `ComponentRegistry` is
  populated in Go at program start; no reflection, no dlopen, no WASM
  sandbox.
- `AgentSpec` v0 requires every `ref:` to resolve against a factory
  known at build time, with typed config, strict schema, deterministic
  output ([`design/agent-spec-v0.md`](../design/agent-spec-v0.md)
  invariants 1-4).
- `AgentSpec` v0 rejects arbitrary executable config (invariant 8),
  which rules out embedded registry-lookup closures or remote
  manifest-fetch hooks in the spec.

Without a decision, each ecosystem would invite its own runtime
surface: an MCP registry client per vendor, a skill-format adapter
layer loaded at build, cache invalidation concerns, trust bootstrap
problems. This would bypass ADR 0001 by the back door.

## Decision

Forge treats external ecosystems as **boundary artifacts crossed at
developer-tooling time**. The runtime surface stays narrow and typed.
The two ecosystems receive opposite strategies because they have
opposite shapes.

### For MCP: consume, don't mirror

- A single registry kind — `KindMCPBinding`
  ([`design/registry-interfaces.md:44-46`](../design/registry-interfaces.md#L44-L46))
  — represents one connection to one MCP server. Factories are
  registered in Go at program start, exactly like every other kind.
- A binding's spec config carries a concrete endpoint (transport, URL,
  auth scope). **Forge does not embed a registry client.** Registry
  lookup — turning a server ID into an endpoint — is a developer-
  tooling concern, resolved by a CLI (`forge mcp add <server-id>`)
  that writes concrete endpoint + scopes + lockfile hashes into the
  operator's files.
- The Phase-5 lockfile is authoritative for reproducibility. It pins
  server endpoint, manifest hash, tool list, tool schema hashes, and
  any trust attestation. At build time, lockfile drift fails the
  build with `mcp_manifest_drift`.
- Auth material never appears in the spec. Declared scopes resolve
  through the registered `credentials.Resolver` at runtime. `RiskTier`
  is set at registration, not inferred from the remote; overlays may
  only tighten.
- A `mcp-bridge@1` core ToolPack
  ([`design/default-toolpacks.md`](../design/default-toolpacks.md))
  ships from day one but fails closed with `mcp_not_yet_supported`
  until Phase 4 activates `KindMCPBinding`.

### For skills: native format + import adapters

- Forge defines its own skill format, registered via `KindSkill`. A
  skill factory produces typed prompt fragments, declared tool
  requirements, declared policy tags, and an optional output contract
  (the Phase-3 feature populating `AgentSpec.outputContract`).
- Skills are Go code. Assets (markdown, schema fragments) are
  `//go:embed`-ed into the factory package. No runtime file loading,
  no format parsing at build.
- **Forge does not consume Anthropic Skills — or any other foreign
  format — directly at runtime.** A one-way tooling path (`forge
  skills import anthropic <dir>`) reads the foreign format and emits
  a forge skill spec plus a Go stub, both of which the operator
  reviews and commits. Missing governance metadata surfaces as
  explicit TODOs in the generated artifact.
- Manifest records provenance: the original format + source reference
  for every imported skill, even after translation.

### Shared discipline

- No external-ecosystem material enters a running forge binary except
  through a factory explicitly registered in Go by the operator.
- CLI tooling that bridges ecosystems (`forge mcp add`, `forge skills
  import`) is out-of-band of the kernel and of the build layer. Its
  output is reviewed files, not runtime state.
- If a second-generation need emerges (multi-registry MCP discovery,
  forge-native skill marketplace), a new kind is added then —
  `KindMCPRegistryClient`, `KindSkillRegistryClient` — each resolved
  at dev time and pinned by lockfile.

## Consequences

### Positive

- ADR 0001's "no runtime plugin systems" invariant holds even as
  forge absorbs two external ecosystems. Each ecosystem is a
  compile-time surface, not a runtime surface.
- Builds stay deterministic. MCP manifests and imported skills are
  pinned by lockfile; remote drift fails the build instead of
  silently changing behavior.
- Format churn in external ecosystems (Anthropic Skills field
  changes, MCP manifest-schema evolution) is absorbed at the CLI
  adapter. Forge internals do not change when Claude Code ships a new
  skill frontmatter key.
- Governance metadata (`RiskTier`, `PolicyTags`,
  `RequiredPolicyPackID`, scope declarations) is always present on
  every registered component — foreign or native — because the
  adapter step is where missing metadata is forced to the operator's
  attention.

### Negative / accepted costs

- Operators pay an explicit step to adopt an external skill or MCP
  server: run a CLI, review output, register the factory, commit.
  There is no "paste a registry URL and go". This is deliberate but
  is friction.
- No runtime fallback when a lockfile entry is missing. Onboarding a
  new MCP server requires the dev-tool pass; forge cannot resolve
  blind.
- Forge ships no "import any agent framework's skills" universal
  adapter. Each foreign format gets its own subcommand
  (`anthropic`, potentially `crewai`, `autogen` later), each with its
  own lossy mapping table.

### Non-consequences (explicitly ruled out)

- **No registry client inside the kernel.** Neither for MCP nor for
  skills. A future `KindMCPRegistryClient` would be added only if a
  concrete multi-registry lookup need emerges; it is not in any
  current phase.
- **No runtime format translation.** The adapter is a CLI writing
  reviewable files; the running forge binary only sees forge-native
  types.
- **No implicit trust of remote manifests.** Default-deny remains; an
  MCP server that lacks a lockfile entry fails closed.

## Alternatives considered

1. **Embed per-vendor registry clients in forge (Smithery client, mcp.so
   client, ...).** Rejected: each new vendor becomes new Go code in
   the core, re-introducing the runtime plugin surface ADR 0001
   forbids, and couples forge releases to vendor API churn.
2. **Consume Anthropic Skills as a first-class registry kind.**
   Rejected: the format evolves with a product outside our release
   cadence, lacks the governance metadata forge needs
   (`RiskTier`, `PolicyTags`, declared tool/policy requirements,
   output contracts), and would force lossy runtime mapping every
   build. Coupling to a moving format is strictly worse than an
   operator-driven import step.
3. **Ignore external ecosystems entirely; require every tool and
   skill to be hand-written forge Go code.** Rejected: the adoption
   barrier is too high when the MCP ecosystem in particular already
   hosts a large usable inventory. The bridge is worth paying for;
   the argument is only *where* the bridge lives.
4. **Run the adapter at build time rather than dev time (CLI before
   `forge.Build` invocations).** Rejected: makes builds non-
   reproducible across environments where the foreign source has
   drifted, re-introduces the runtime translation surface, and loses
   the operator-review step that catches missing governance metadata.

## References

- Boundary design: [`design/external-registries.md`](../design/external-registries.md)
- Registry kinds: [`design/registry-interfaces.md:29-46`](../design/registry-interfaces.md#L29-L46)
- Core bridge pack: [`design/default-toolpacks.md`](../design/default-toolpacks.md) (`mcp-bridge@1`)
- Governing ADR: [`adr/0001-praxis-forge-layering.md`](0001-praxis-forge-layering.md)
- MCP seam in kernel: `praxis/mcp/invoker.go:54`
