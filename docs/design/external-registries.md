# Design — External registries (MCP and skills)

How `praxis-forge` consumes ecosystem registries without breaking its
"no runtime plugin systems" non-goal
([`forge-overview.md`](forge-overview.md)). Phase 0 proposal; the
MCP-side work lands in Phase 4, the skills-side work in Phase 3.

## The two worlds

Forge has a typed, in-process `ComponentRegistry`
([`registry-interfaces.md`](registry-interfaces.md)). The world
outside has its own registries — human-curated directories of MCP
servers, vendor-specific skill repositories, community package
indexes. The question this doc answers: *how do those external
registries relate to forge's internal one, and which boundary
artifacts bridge them.*

Two external ecosystems are in scope:

- **MCP servers** — reasonably convergent (the MCP project publishes an
  official registry; third-party directories follow the same manifest
  shape)
- **Agent skills** — fragmented (Anthropic has one format, other
  frameworks have others; no cross-vendor package manager exists as of
  Apr 2026)

Forge takes opposite strategies for the two: **consume** external MCP
registries via a single well-scoped factory kind; **define its own
format** for skills with an adapter path for imports.

## MCP registries — consume, don't mirror

### What exists

- An official registry maintained by the MCP project, listing servers
  by capability, transport, auth requirements, and tool manifests
- Third-party directories (Smithery, mcp.so, PulseMCP, Glama) that
  curate the same underlying manifests with their own UX / trust
  layer
- Each MCP server exposes a programmatic manifest at handshake time —
  the tool list + schemas are self-describing

### Forge's Phase 4 surface

A new registry kind already reserved in the enum
([`registry-interfaces.md:44-46`](registry-interfaces.md#L44-L46)):

```go
KindMCPBinding Kind = "mcp_binding"
```

An `mcp_binding` factory's `Build` method produces a connected
`mcp.Invoker` ([`praxis/mcp/invoker.go`](../../../praxis/mcp/invoker.go))
plus normalized `ToolDescriptor` entries, one per remote tool. The
`mcp-bridge@1` core ToolPack
([`default-toolpacks.md`](default-toolpacks.md)) wraps the resulting
invoker so remote tools appear in the same tool namespace as local
ones, prefixed per the existing praxis convention
(`{binding}__{tool}`, [Mismatch 4](mismatches.md)).

### Two explicit choices

**Choice 1 — forge does not embed a registry client.** An MCP binding
takes its server endpoint (transport, URL, auth scope) from spec
config, not from a remote lookup. If an operator wants registry-backed
discovery, that's a developer-tooling concern resolved at spec-author
time (e.g. `forge mcp add <server-id>` CLI resolves the registry entry
into concrete endpoint + auth scope + lockfile hash).

Rationale: a registry client inside forge would be a second runtime
plugin surface (every new registry vendor = new Go code). Keeping
resolution outside forge means the kernel stays deterministic: a spec
either references a concrete endpoint or it fails.

**Choice 2 — manifests are pinned by lockfile, not fetched on every
build.** Phase 5 lockfile ([`forge-overview.md:93-100`](forge-overview.md#L93-L100))
records for each MCP binding:

- server endpoint + auth scope
- manifest hash at lockfile creation time
- tool list + tool schema hashes
- trust attestation (signer, if any)

At build time forge may re-fetch the manifest, but **the lockfile is
authoritative**: divergence fails the build with
`mcp_manifest_drift`. This makes MCP-backed agents as reproducible as
the rest of the bundle.

### Trust model

- Default deny: empty allowlist of MCP servers in a fresh registry;
  every binding is an explicit `RegisterMCPBinding` call in consumer
  Go code.
- Auth credentials never live in the spec. `mcp_binding.config.scopes`
  declares required scopes; the registered `credentials.Resolver`
  supplies tokens at runtime. Forge itself never touches credential
  material.
- `ToolDescriptor.RiskTier` inherits from the binding's declared trust
  level (set at registration, not by the remote). Overlays can only
  tighten, never loosen — same invariant as budget overrides
  ([`agent-spec-v0.md:177-179`](agent-spec-v0.md#L177-L179)).

### What forge does not add

- No registry-vendor-specific adapters (`registry.smithery@1` etc.) in
  the core. If multi-registry lookup becomes a felt need, add *one*
  `KindMCPRegistryClient` and support it there — deferred until a
  concrete use case forces the surface.
- No automatic tool projection from unpinned servers. A binding that
  lacks a lockfile entry fails closed after the `mcp-bridge@1` Phase 4
  unlock.

## Agent skills — forge-native format, import-only adapters

### What exists

- **Anthropic Agent Skills** — the dominant format in the Claude
  ecosystem (directory of `SKILL.md` + supporting files, frontmatter
  for metadata, prompt fragments as markdown). Widely adopted in
  Claude Code, Claude Agent SDK, Claude Skills marketplace.
- **Framework-specific formats** — CrewAI, AutoGen, LangChain,
  Semantic Kernel each carry their own "skill / ability / plugin"
  shape. None interoperate directly.
- No cross-vendor package manager for skills exists. The equivalent of
  PyPI for agent skills is not a solved problem.

### Forge's Phase 3 surface

Forge defines its own skill format, registered via `KindSkill`
([`registry-interfaces.md:44`](registry-interfaces.md#L44)) like
every other component. A `skill_factory` produces:

- typed prompt fragments (system / user / context slots)
- declared tool requirements (a set of `toolpack.<id>` references the
  skill needs — build fails if spec doesn't supply them)
- declared policy tags (matched against active policy-packs)
- an optional output contract (the Phase 3 feature that finally
  populates `AgentSpec.outputContract`)

Skills are Go code, registered at program start with
`r.RegisterSkill(foo.NewFactory(...))`. The skill's assets (prompt
markdown, schema fragments) are Go `//go:embed`-ed resources, not
runtime-loaded files. This is consistent with the no-reflection, no-
dlopen principle.

### Why not consume Anthropic skills directly

Three reasons:

1. **Format is a moving target.** Anthropic skills evolve with the
   Claude Code product — frontmatter keys change, tool namespace
   conventions drift. Coupling forge's `KindSkill` semantics to that
   format would make every Claude release a potential forge churn.
2. **Missing governance metadata.** Anthropic skills don't carry
   `RiskTier`, `PolicyTags`, `RequiredToolPacks`, or declared output
   contracts in the structured form forge needs. Mapping to those
   fields is lossy in the general case.
3. **Adapter path is more resilient than coupling.** A `forge skills
   import <path>` CLI that reads an Anthropic skill directory and
   emits a forge skill spec (with explicit holes where metadata is
   missing) is a one-way, operator-driven step. That surface absorbs
   format churn without changing forge internals.

### The adapter path (Phase 3.x tooling)

A tooling subcommand — not a runtime concern:

```
forge skills import anthropic ./skills/my-skill/
  → writes ./skills/my-skill.forge.yaml
  → flags missing governance metadata as TODO comments
  → emits a Go stub the operator fills in + registers
```

Output is review-able by the operator, committed to the repo, and
becomes a normal `KindSkill` factory registration. There is no
"automatic import at build time". This is the same pattern as MCP
binding resolution above: external artifacts cross the boundary via
developer tooling, never at runtime.

### What about community skill marketplaces

If a forge-native skill marketplace emerges later, it can be supported
the same way MCP registries are: a single new kind (say
`KindSkillRegistryClient`), resolved at dev-tool time, pinned by
lockfile. Not planned for Phase 3.

## Auth, trust, and the lockfile — shared surface

Both external worlds share the same boundary artifacts:

- **CredentialResolver** supplies secrets for remote calls (MCP auth
  tokens, skill-backing service calls if a skill happens to wrap one).
- **Lockfile (Phase 5)** pins everything crossable — MCP server
  endpoints, manifest hashes, imported skill versions, adapter
  provenance.
- **Manifest (Phase 2)** records what was resolved: for MCP the server
  id + scope; for skills the factory id + version + source provenance
  (native vs imported from which format).
- **Identity signer** can co-sign the manifest so a downstream
  `praxis-os` can verify the bundle integrity without recomputing
  hashes.

## Summary table

| Axis                    | MCP                                         | Skills                                      |
|-------------------------|---------------------------------------------|---------------------------------------------|
| External registry shape | Convergent (official + third-party mirrors) | Fragmented (per-vendor formats)             |
| Forge strategy          | Consume; resolve endpoint via dev tooling    | Define native format; adapter on import     |
| Runtime kind            | `KindMCPBinding` (Phase 4)                  | `KindSkill` (Phase 3)                       |
| Registry client in core | No                                          | No                                          |
| Reproducibility surface | Lockfile entry per binding                  | Lockfile entry per imported skill           |
| Auth path               | `CredentialResolver` + declared scopes      | `CredentialResolver` if the skill needs it  |
| Adapter step            | `forge mcp add` CLI (dev-time)              | `forge skills import <format>` CLI (dev-time) |

## Open questions

- **Official MCP registry dependency.** Should the Phase 4
  `mcp_binding` spec field accept a shortcut `registry: mcp-official`
  for readability, even if the actual resolution happens at dev time?
  Tradeoff: readability vs. no-registry-client purity. Leans *no* for
  v0.
- **Skill format version pinning.** Does a forge skill spec declare
  the skill-format version (`skillFormat: forge.v0`) so forge can evolve
  the format without breaking old specs? Proposed: yes, mandatory
  from day one.
- **Imported skill provenance.** Should the manifest record the
  original format + source path (`imported from anthropic-skill at
  <sha>`) even after import? Proposed: yes — provenance is cheap and
  audit-relevant.
- **Multi-source skills.** Can a single forge skill be composed from
  fragments of two imported sources? Probably no in Phase 3; revisit
  if operators request composition.
- **MCP server trust attestation.** When a remote server advertises a
  signature, does forge verify it at build or delegate to the operator?
  Leans "verify if present, require-operator-flag if absent". Decision
  in Phase 4.
