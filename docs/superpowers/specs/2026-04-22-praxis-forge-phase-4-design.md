# praxis-forge — Phase 4 design spec

**Date:** 2026-04-22
**Milestone:** Phase 4 — MCP consume (declarative binding contract)
**Status:** Approved (ready for planning)
**Scope chosen:** Runtime-first binding. Build time produces a governable contract; no network I/O, no tool-surface snapshot.

## Context

Phase 3 (shipped 2026-04-21, tag v0.5.0) activated skills and output
contracts, closing the build-time expansion story. Phase 4 activates
the last remaining phase-gated kind: `mcp_binding`.

MCP ([`docs/CONTEXT_SEED.md:91-100`](../../CONTEXT_SEED.md#L91-L100)) is
a binding/import/export layer. Today the spec accepts `mcpImports`
at the parser but fails validation when non-empty
([`spec/validate.go:84-85`](../../../spec/validate.go#L84-L85)), and
`KindMCPBinding` is still in the deferred block of
[`registry/kind.go`](../../../registry/kind.go). Phase 4 unlocks them.

### Architectural inversion

A naive reading of "consume MCP" suggests fetching remote tool
metadata at build time, normalizing it, and freezing it into the
manifest. That path collides with two load-bearing constraints:

1. **Phase 2b's determinism contract.** Same spec + same registry must
   produce the same `NormalizedHash` / `ExpandedHash`. A live fetch
   during `Build` violates this by definition.
2. **Runtime correctness.** Even if we froze the tool surface at build
   time, a subsequent change on the MCP server (Notion renames a tool,
   tightens a schema, removes a capability) breaks the agent at
   invoke time regardless of what the manifest says. Pinning the
   tool list creates the illusion of stability without delivering it.

Phase 4 therefore treats MCP as a **runtime binding**. The forge
produces a declarative, governable binding contract — connection
descriptor, auth reference, allow/deny patterns, policy chain, trust
metadata, on-new-tool policy — and stamps it into the manifest. The
runtime (praxis, future) opens MCP sessions, discovers tools live,
applies the binding's governance, and reacts to drift where it can
actually be acted on.

The tradeoff is explicit: we give up build-time verification that
declared tools exist on the server, and we gain a stable manifest, a
deterministic build, and a runtime story that can respond to reality.

## Scope decisions

### In scope (MVP)

- Generic `mcp.binding@1` factory in forge core.
- Both MCP transports declarable: `stdio` (locally-spawned) and
  streamable-HTTP (remote).
- Level 1 binding fields: `id`, `connection`, `auth` (optional),
  `allow`, `deny`, `policies`, `trust`, `onNewTool`.
- Build-time activation: kind flip, factory interface, registry
  wiring, spec validation, resolver cross-kind checks, manifest
  stamping, capability flag.
- Vertical slice: filesystem MCP server (stdio, no auth) + demo
  `-mcp` flag that prints the resolved binding contract.

### Out of scope (deferred)

- **Any network I/O during `Build`.** No subprocess spawn, no HTTP
  fetch, no tool listing. A separate `forge mcp inspect <ref>` dev
  CLI is possible but is not part of Phase 4 and does not participate
  in `Build`.
- **Build-time tool-surface snapshotting or pinning.** The manifest
  carries the contract, not the resolved tool list.
- **Per-tool overrides** (different policy chain for specific tools
  within a binding) — Phase 4.1+.
- **Rate or budget hints per binding** — Phase 4.1+.
- **Enum-enforced `Scheme`** with scheme-specific validation — v0
  keeps `Scheme` as a free-form tag.
- **Ecosystem named-server factories in forge core** (`mcp.notion@1`,
  `mcp.github@1`, ...). These are thin wrappers over the generic
  factory and belong outside forge core.
- **MCP expose** (forge projecting capabilities outward as an MCP
  server) — separate effort, out of Phase 4.
- **Lockfile integration** — Phase 5.
- **Runtime MCP client, session lifecycle, drift reconciliation** —
  praxis / Phase 6.
- **Skills declaring `RequiredMCPBindings`** — Phase 3 deferred this
  explicitly
  ([`docs/superpowers/specs/2026-04-21-praxis-forge-phase-3-design.md:62`](2026-04-21-praxis-forge-phase-3-design.md#L62));
  Phase 4 does not revisit.

## Data model

New types in [`registry/types.go`](../../../registry/types.go):

```go
// MCPBinding is the value produced by an MCP binding factory. It is a
// governance contract, not a resolved tool set. Runtime (praxis) opens
// the MCP session, discovers tools live, applies Allow/Deny + Policies,
// and enforces OnNewTool when the server surface changes.
type MCPBinding struct {
    ID         string                 // user-chosen, unique across this spec's mcpImports
    Connection MCPConnection
    Auth       *MCPAuth               // nil = no auth (common for local stdio servers)
    Allow      []string               // glob patterns matched against tool names
    Deny       []string               // glob patterns; deny beats allow on overlap
    Policies   []ID                   // policypack.* refs; applied to every MCP tool call at runtime
    Trust      MCPTrust
    OnNewTool  OnNewToolPolicy
    Descriptor MCPBindingDescriptor
}

type MCPTransport string

const (
    MCPTransportStdio MCPTransport = "stdio" // locally spawned subprocess
    MCPTransportHTTP  MCPTransport = "http"  // streamable-HTTP per MCP spec; SSE fallback is a runtime concern
)

type MCPConnection struct {
    Transport MCPTransport

    // Stdio-only fields.
    Command []string             // argv for spawned subprocess; required if Transport == stdio
    Env     map[string]string    // optional env overlay for the subprocess

    // HTTP-only fields.
    URL     string               // streamable-HTTP endpoint; required if Transport == http
    Headers map[string]string    // static, non-secret headers; secrets go through Auth
}

type MCPAuth struct {
    CredentialRef ID      // must resolve to a credential_resolver.* factory
    Scheme        string  // "bearer" | "api-key" | "oauth-token" (free-form tag in v0)
    HeaderName    string  // required iff Scheme == "api-key"
}

type MCPTrust struct {
    Tier  string   // "low" | "medium" | "high" | "untrusted"
    Owner string
    Tags  []string
}

type OnNewToolPolicy string

const (
    OnNewToolBlock            OnNewToolPolicy = "block"
    OnNewToolAllowIfMatch     OnNewToolPolicy = "allow-if-match-allowlist"
    OnNewToolRequireReapprove OnNewToolPolicy = "require-reapproval"
)

type MCPBindingDescriptor struct {
    Name    string
    Summary string
    Tags    []string
}
```

Design calls baked in:

- **`ID` is a user-chosen instance name**, unique within `spec.mcpImports`.
  Unlike factory IDs, it is not versioned — it names an instance, not
  a factory. Runtime uses it to attribute tool calls
  (`binding=fs, tool=read_file`).
- **`Policies` holds `registry.ID` refs** to existing `policypack.*`
  factories. The binding does not embed policy configs — it references
  them. Reuses Phase 1 policypack wiring.
- **`Auth` is optional.** Local stdio servers commonly have no auth.
  When present, only `CredentialRef` lives here; the secret itself is
  never in forge memory.

## Registry additions

Move `KindMCPBinding` from the deferred block to the active block in
[`registry/kind.go`](../../../registry/kind.go).

Add factory interface in [`registry/factories.go`](../../../registry/factories.go):

```go
type MCPBindingFactory interface {
    ID() ID
    Description() string
    Build(ctx context.Context, cfg map[string]any) (MCPBinding, error)
}
```

Extend [`registry/registry.go`](../../../registry/registry.go):

- map: `mcpBindings map[ID]MCPBindingFactory`
- registration: `RegisterMCPBinding(f MCPBindingFactory) error`
- lookup: `MCPBinding(id ID) (MCPBindingFactory, error)`
- `Each` enumerator visits the new kind after `KindOutputContract`.
- Frozen-after-`Build` protection applies (same guard as all other kinds).

## Spec changes

In [`spec/validate.go`](../../../spec/validate.go):

- Remove the `mcpImports` phase-gate block at
  [lines 84–85](../../../spec/validate.go#L84-L85).
- Add referential validation using the existing `validateKindPrefixedRef`
  helper: `spec.mcpImports[].ref` must match
  `^mcp\.[a-z0-9-]+@\d+(\.\d+){0,2}$`. Error: `mcp_invalid_ref`.
- Add well-formedness checks against each binding's decoded `config`:
  - Non-empty `id`; error `mcp_missing_id`.
  - Unique `id` across `spec.mcpImports[]`; error `mcp_duplicate_id`.
  - `connection.transport ∈ {stdio, http}`; error `mcp_transport_invalid`.
  - Transport-specific invariants: stdio requires `command` non-empty
    and rejects `url`; http requires `url` non-empty and rejects
    `command`. Error `mcp_transport_field_mismatch`.
  - `onNewTool ∈ {block, allow-if-match-allowlist, require-reapproval}`.
    Omitted value defaults to `"block"`. Error `mcp_on_new_tool_invalid`.
  - `auth.credentialRef` non-empty when `auth` is present; error
    `mcp_missing_credential_ref`.
  - `auth.scheme` non-empty when `auth` is present; error
    `mcp_missing_auth_scheme`.
  - `auth.headerName` non-empty when `scheme == "api-key"`; error
    `mcp_missing_header_name`.
  - Allow/deny entries are syntactically-legal globs (no unmatched
    brackets); error `mcp_invalid_glob`.

Note: validation operates on the already-decoded `config` because
`AgentSpec.MCPImports` fields are `ComponentRef`s with `Config map[string]any`.
The validator performs structural checks on the map; the factory's
typed decode (`factories/mcpbinding`) re-validates on `Build` as
defense in depth.

Remove any `spec/testdata/overlay/invalid/phase_gated_mcp*` fixtures
if they exist. Add valid fixtures under `spec/testdata/overlay/valid/mcp_*`.

Update [`docs/design/agent-spec-v0.md`](../../design/agent-spec-v0.md)
§"Explicit deferrals" to drop the MCP entry (mirrors the Phase 3
update for skills / output contracts).

No changes to `AgentSpec` — [`spec/types.go:26`](../../../spec/types.go#L26)
already carries `MCPImports []ComponentRef`.

## Build pipeline

Bindings flow through the existing resolve path — **no new expansion
stage**. Phase 3's `expandSkills` exists because skills inject into
other slots (Tools / Policies / OutputContract); bindings don't inject,
they're terminal components.

Post-Phase-4 pipeline (shape unchanged from Phase 3):

```
parse → validate → normalize
      → canonical/hash (NormalizedHash)
      → resolve skill + output-contract factories
      → EXPAND (Phase 3) → canonical/hash (ExpandedHash)
      → resolve remaining components (tools, policies, ..., mcp_binding)  ← NEW kind resolved here
      → compose → materialize → manifest
```

New file: [`build/mcp.go`](../../../build/mcp.go) with a pure resolve
step:

```go
// resolveMCPBindings resolves every entry of spec.mcpImports into a
// registry.MCPBinding via its factory, then cross-validates that each
// binding's Policies and Auth.CredentialRef resolve to components
// already registered. No network, no subprocess spawn.
func resolveMCPBindings(
    ctx context.Context,
    s *spec.AgentSpec,
    r *registry.ComponentRegistry,
) ([]registry.MCPBinding, error)
```

Cross-kind validation inside the resolver:

- Each `binding.Policies[i]` must resolve via `r.PolicyPack(id)`; error
  `mcp_unresolved_policy`.
- `binding.Auth.CredentialRef` (when `Auth != nil`) must resolve via
  `r.CredentialResolver(id)`; error `mcp_unresolved_credential`.

These checks reuse existing registry lookup paths — no duplicated
resolution logic.

Called from `Build` after component resolution and before `compose`.
Results flow into `buildManifest` alongside resolved tools / policies /
skills / output contract.

`ExpandedHash` is unaffected — MCP bindings are not part of expansion.
They participate in the resolved-components hash stream via their
`ResolvedComponent` entries, which is already covered by the canonical
manifest-serialization tests.

## Manifest additions

No new top-level Manifest fields. Each binding stamps one
`ResolvedComponent`:

```json
{
  "kind": "mcp_binding",
  "id": "mcp.binding@1",
  "config": {
    "id": "fs",
    "connection": {
      "transport": "stdio",
      "command": ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/data"]
    },
    "auth": null,
    "allow": ["read_*", "list_*"],
    "deny": ["write_*"],
    "policies": ["policypack.pii-redaction@1"],
    "trust": {"tier": "medium", "owner": "platform-team", "tags": ["local"]},
    "onNewTool": "block"
  },
  "descriptors": {
    "name": "generic MCP binding",
    "summary": "..."
  }
}
```

`Capabilities.Present` gains `"mcp_binding"` when `spec.mcpImports` is
non-empty; `Capabilities.Skipped` gains `"mcp_binding"` with reason
`"not_specified"` when empty.

Capability declaration order (extending Phase 3):

```
budget → telemetry → credential_resolver → identity_signer
       → skill → output_contract → mcp_binding
```

No `InjectedBySkill` attribution — skills cannot require MCP bindings
in v0 (Phase 3 deferred this;
[`2026-04-21-praxis-forge-phase-3-design.md:62`](2026-04-21-praxis-forge-phase-3-design.md#L62)).

## Auth resolution

- `Auth.CredentialRef` must resolve to a `credential_resolver.*`
  factory registered in the `ComponentRegistry`. Build-time lookup
  failure produces `mcp_unresolved_credential`.
- Forge **never reads the secret**. The resolver is called at runtime
  by praxis, not at build time.
- `Scheme` is a free-form tag in Phase 4 (`"bearer"`, `"api-key"`,
  `"oauth-token"` are documented but not enum-enforced). Phase 4.1+
  may formalize.
- When `Scheme == "api-key"`, `HeaderName` is required.
- Manifest stamps the descriptor (scheme, header name) but **not** the
  credential value. A manifest round-trip test asserts no secret
  material appears in the serialized form.

## Allow / deny semantics and namespace

- **Glob patterns** on MCP tool names: `*` matches any chars, `?`
  matches one char, literal otherwise. Single-segment (no `**`).
  Case-sensitive.
- **Resolution rule (runtime):** a tool is allowed iff at least one
  `Allow` pattern matches AND no `Deny` pattern matches. An omitted
  `Allow` and an explicit `Allow: []` are equivalent — both mean "no
  tools allowed" (strict default). Explicit `Allow: ["*"]` means all
  unless denied.
- **Syntax validation at build time; matching at runtime.** Forge
  validates patterns parse; it never enumerates actual tools.
- **Namespace reservation:** forge reserves the `mcp.<binding-id>.<tool>`
  naming space for praxis runtime tool projection. Static
  `toolpack.*` factories must not register tool names that collide.
  The tool router gains a build-time check: `tool_name_reserved_prefix`.

## Overlay + extends interaction

`spec.mcpImports` is a **replaceable list** under Phase 2a composition
(same semantics as `spec.tools` / `spec.policies`). Overlays can
append / replace / remove bindings via the existing `op: add/remove/replace`
mechanics. No locked subfields in v0 — operators legitimately tighten
`Deny` in prod vs dev, swap `CredentialRef` per environment, etc.
Per-field provenance tracks every change, same as other replaceable
lists.

Extends chains: child `mcpImports` merges with parent's; child-wins on
same `config.id`. Mirrors the existing merge rules.

## Vertical slice factory

[`factories/mcpbinding/`](../../../factories/mcpbinding/) →
`mcp.binding@1`:

```go
// Config is the typed configuration decoded from spec.mcpImports[].config.
type Config struct {
    ID         string            `json:"id"`
    Connection ConnectionConfig  `json:"connection"`
    Auth       *AuthConfig       `json:"auth,omitempty"`
    Allow      []string          `json:"allow,omitempty"`
    Deny       []string          `json:"deny,omitempty"`
    Policies   []string          `json:"policies,omitempty"`
    Trust      TrustConfig       `json:"trust"`
    OnNewTool  string            `json:"onNewTool,omitempty"` // default: "block"
    Name       string            `json:"name,omitempty"`
    Summary    string            `json:"summary,omitempty"`
    Tags       []string          `json:"tags,omitempty"`
}

type ConnectionConfig struct {
    Transport string            `json:"transport"` // "stdio" | "http"
    Command   []string          `json:"command,omitempty"`
    Env       map[string]string `json:"env,omitempty"`
    URL       string            `json:"url,omitempty"`
    Headers   map[string]string `json:"headers,omitempty"`
}

type AuthConfig struct {
    CredentialRef string `json:"credentialRef"`
    Scheme        string `json:"scheme"`
    HeaderName    string `json:"headerName,omitempty"`
}

type TrustConfig struct {
    Tier  string   `json:"tier"`
    Owner string   `json:"owner"`
    Tags  []string `json:"tags,omitempty"`
}
```

`Build(ctx, cfg)` decodes via `mapstructure` (consistent with existing
factories), re-validates internal consistency as defense in depth
(same checks `spec.validate` runs), and returns `registry.MCPBinding`.
`Config.Policies []string` is converted to `[]registry.ID` during
construction (one helper call, same pattern as skill `RequiredTools` /
`RequiredPolicies` conversion in Phase 3). Zero network, zero
subprocess.

### Demo update

[`examples/demo/`](../../../examples/demo/) gains a `-mcp` flag:

- Registers `mcp.binding@1` and the existing
  `policypack.pii-redaction@1`.
- Builds an agent with one filesystem-server binding
  (`command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp/demo"]`,
  `allow: ["read_*", "list_*"]`, `deny: ["write_*"]`,
  `policies: ["policypack.pii-redaction@1"]`,
  `trust: {tier: medium, owner: "demo"}`, `onNewTool: block`).
- Prints the manifest's `mcp_binding` resolved entry and the
  capabilities block so the user can see the governance contract.
- Does **not** open an MCP session. The demo output includes a
  prominent note: "binding is a contract; actual MCP invocation is a
  runtime concern."

## Test coverage

### Unit

- `registry/registry_test.go` — register / lookup / duplicate detection
  for MCP bindings; frozen-after-`Build`.
- `spec/validate_test.go` — prefix validation; duplicate `id`;
  transport invariants; `onNewTool` enum; glob syntax; auth
  well-formedness.
- `factories/mcpbinding/build_test.go` — config decode; transport-specific
  validation; auth / no-auth paths; default `onNewTool` resolution.
- `build/mcp_test.go` — resolve with missing policy ref (error);
  missing credential ref (error); happy path.

### Integration

- `build/build_mcp_test.go` — end-to-end YAML → `BuiltAgent` + manifest
  with `mcp_binding` entry in `Resolved`; capability flags set
  correctly; `NormalizedHash` and `ExpandedHash` stable across two
  builds of the same spec; manifest round-trip test asserts no
  credential value appears in the serialized form.

### Fixtures under `spec/testdata/mcp/`

- `minimal-stdio/` — one binding, stdio, no auth, small allow list.
- `stdio-with-env/` — stdio + `env`.
- `http-with-bearer/` — http transport, bearer auth.
- `http-with-apikey/` — http + api-key scheme + `HeaderName`.
- `multiple-bindings/` — two bindings with unique ids and distinct policy chains.
- `deny-beats-allow/` — overlapping patterns (syntax validation only;
  runtime matching is exercised at the unit level in a glob helper
  once praxis needs it).
- `unresolved-credential/` — build-time error (golden `err.txt`).
- `unresolved-policy/` — build-time error (golden `err.txt`).
- `duplicate-binding-id/` — build-time error.
- `transport-field-mismatch/` — stdio with `url`, and http with `command`.
- `on-new-tool-variants/` — three fixtures, one per enum value, all
  valid.

Each success fixture ships `spec.yaml` + `want.manifest.json` +
`want.expanded.hash` golden files, matching the Phase 2b / Phase 3
fixture pattern.

## Commit sequence

1. `feat(registry): activate KindMCPBinding`
2. `feat(registry): MCPBinding type + MCPBindingFactory interface`
3. `feat(registry): register + lookup for MCP bindings`
4. `feat(spec): unlock mcpImports validation gate; add ref / transport / onNewTool checks`
5. `feat(build): resolveMCPBindings with cross-kind validation (policies, credentials)`
6. `feat(build): wire MCP resolution into Build pipeline`
7. `feat(manifest): mcp_binding ResolvedComponent + capability flag`
8. `feat(factories): mcp.binding@1 generic factory (stdio + http transports)`
9. `test(spec+build): MCP fixtures and integration tests`
10. `examples(demo): -mcp flag exercising filesystem binding contract`
11. `docs: Phase 4 design doc + agent-spec-v0 update + forge-overview roadmap update`

## Risks and mitigations

| Risk | Mitigation |
|------|-----------|
| Runtime MCP drift breaks agents | Explicitly a runtime concern; `OnNewTool` gives operators a lever; `Trust.Tier` signals which bindings deserve stricter review. Documented as a first-class non-goal of Phase 4. |
| Users expect build-time tool introspection and are surprised it's runtime | Godoc on `MCPBinding` is explicit: "binding is a contract, not a snapshot." A separate `forge mcp inspect <ref>` dev CLI can cover exploratory UX later. |
| Over-permissive `Allow: ["*"]` ships to prod | Not prevented structurally; `Trust.Tier` + policy chain are the governance surface. Future `forge lint` can flag wildcard allow on `tier: high`. |
| Auth secrets leak into manifest | `MCPAuth` struct has no secret field by construction; only `CredentialRef` is stored. Manifest round-trip test asserts absence of credential values. |
| Multi-binding collision on `config.id` | `mcp_duplicate_id` at build time. |
| Static tool and MCP tool namespace collide at runtime | `mcp.*` prefix reserved; tool router rejects conflicting static tool names at build time (`tool_name_reserved_prefix`). |
| Phase 2a overlay replacing a binding's `CredentialRef` silently | Provenance tracks it (Phase 2a contract); operators review via `overlays` attribution in manifest. |
| `Scheme` free-form tag allows typos (`"bearer "` with trailing space, etc.) | Build-time trims and warns; Phase 4.1+ formalizes the enum once real runtime demands are clearer. |

## Out of scope (Phase 4.1 and beyond)

- Per-tool overrides (different policy chain for specific tools within
  the same binding).
- Rate / budget hints per binding.
- Enum-enforced `Scheme` with scheme-specific validation.
- Ecosystem named-server factories in forge core (`mcp.notion@1`,
  `mcp.github@1`, ...).
- MCP expose (forge projecting capabilities outward as an MCP server).
- Lockfile integration for MCP bindings (Phase 5).
- Runtime MCP client, session lifecycle, drift reconciliation
  (praxis / Phase 6).
- Skills declaring `RequiredMCPBindings` (Phase 3 deferred).
- `forge mcp inspect` dev CLI (separate effort).

## References

- [`docs/CONTEXT_SEED.md:91-100`](../../CONTEXT_SEED.md#L91-L100) — MCP as binding/import/export layer
- [`docs/CONTEXT_SEED.md:232-240`](../../CONTEXT_SEED.md#L232-L240) — Phase 4 charter
- [`docs/design/forge-overview.md:185-188`](../../design/forge-overview.md#L185-L188) — phase roadmap
- [`docs/design/registry-interfaces.md:44-46`](../../design/registry-interfaces.md#L44-L46) — reserved kinds
- [`docs/superpowers/specs/2026-04-21-praxis-forge-phase-3-design.md`](2026-04-21-praxis-forge-phase-3-design.md) — prior phase
- [`spec/types.go:26`](../../../spec/types.go#L26) — `MCPImports` field
- [`spec/validate.go:84-85`](../../../spec/validate.go#L84-L85) — current phase-gate enforcement
- [`registry/kind.go`](../../../registry/kind.go) — `KindMCPBinding` deferred block
