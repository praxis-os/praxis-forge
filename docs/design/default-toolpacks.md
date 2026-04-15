# Design — Default ToolPacks

Which ToolPacks ship inside `praxis-forge` itself, which live in the
`praxis-toolpacks` satellite repo, and why. Phase 0 proposal describing
the **target default set**; phasing below clarifies what actually lands
when.

## Phasing

Phase 1 is a vertical slice and ships **one ToolPack only** —
`toolpack.http-get@1` — to prove the declarative → runtime path
end-to-end alongside one factory per other kernel seam (see the
Phase 1 design spec under `docs/superpowers/specs/`). The full 10-pack
default set described here is the target that fills in across
Phase 1.x and Phase 2. `mcp-bridge@1` remains Phase 4 regardless.

| Milestone | ToolPacks shipped |
|-----------|-------------------|
| Phase 1   | `toolpack.http-get@1` only (vertical slice) |
| Phase 1.x | remaining low/moderate packs: `clock`, `id`, `json-xform`, `text`, `fs-read`, split of `http-get` into `http-fetch`/`http-post` |
| Phase 2   | destructive packs: `fs-write`, `shell-exec` (with companion policy-packs — see [`default-policypacks.md`](default-policypacks.md)) |
| Phase 4   | `mcp-bridge@1` becomes functional when `KindMCPBinding` activates |

Phase 1's `toolpack.http-get@1` is the ancestor of `http-fetch@1` in
the table below: same surface, narrower capability (GET only), no
`http-post` split yet. The split lands when POST / PUT / PATCH / DELETE
is actually needed.

## Why this split matters

Every `ref: toolpack.foo@N` in an `AgentSpec`
([`agent-spec-v0.md`](agent-spec-v0.md)) has to resolve to a Go factory
registered in the `ComponentRegistry` at program start
([`registry-interfaces.md`](registry-interfaces.md)). So "which
ToolPacks exist" is a **distribution** question — not a configuration
question. Forge needs a clear policy on what it bundles vs. what the
user pulls from elsewhere, because:

- bundling too much drags vendor SDKs, auth flows, and API churn into
  the core module's dependency graph
- bundling too little forces every consumer to re-invent primitives
  that have no vendor semantics (clock, id, json, regex)
- the governance story (`ToolDescriptor` metadata, required policy
  companions) is only credible if the curated default set lives under
  one roof with one owner

## Criterion boundary

A ToolPack lives **in `praxis-forge`** when *all* of these hold:

- zero external credentials, zero SaaS dependency
- primitive used by the broad majority of agents
- API semantics stable (not tied to a vendor release cycle)
- useful as reference implementation + fixture for the build layer's
  tests

A ToolPack lives **in `praxis-toolpacks`** (satellite monorepo) when
*any* of these hold:

- requires auth scopes resolved via `credentials.Resolver`
- versioned against an external API (GitHub, Slack, Jira, ...)
- non-trivial dependency graph (vendor SDKs)
- org-specific fork/extension is likely

The non-goal "no runtime plugin systems" from the seed applies to
*loading*, not to *distribution*. Satellite packs are plain Go modules
imported and registered from the consumer's `main`.

## Core ToolPacks — target set

Ten packs, chosen to be the minimum viable default set. The `Phase`
column lines up with the phasing table above: Phase 1 ships only
`http-get@1` as vertical slice; the rest accrues later.

| ID                 | Phase   | RiskTier     | Required companion policy-pack       | Notes |
|--------------------|---------|--------------|--------------------------------------|-------|
| `http-get@1`       | **1**   | moderate     | —                                    | GET-only ancestor of `http-fetch@1`; host allowlist, timeout, size cap |
| `clock@1`          | 1.x     | low          | —                                    | `now`, `parse`, `format`, `since`; tz-aware |
| `id@1`             | 1.x     | low          | —                                    | UUID v4/v7, ULID, nanoid; stateless |
| `json-xform@1`     | 1.x     | low          | —                                    | parse / stringify / `jq`-subset query / JSON Patch |
| `text@1`           | 1.x     | low          | —                                    | regex, unicode normalize, diff; operates on strings passed in |
| `fs-read@1`        | 1.x     | moderate     | —                                    | read files within a sandbox root; bounded glob |
| `http-fetch@1`     | 1.x     | moderate     | —                                    | supersedes `http-get@1`; still GET / HEAD only |
| `http-post@1`      | 2       | high         | `http-body-scrubber@1`               | POST / PUT / PATCH / DELETE; body scrubbed by policy |
| `fs-write@1`       | 2       | destructive  | `fs-guard@1`                         | write / delete within sandbox root |
| `shell-exec@1`     | 2       | destructive  | `shell-guard@1`                      | argv form only (never shell string); allowlist, timeout, cwd sandbox |
| `mcp-bridge@1`     | 4       | inherited    | —                                    | dormant skeleton shipped earlier if convenient; functional only when `KindMCPBinding` activates |

### Why `http-fetch` is split

A GET with a host allowlist is cheap to reason about: it leaks input
through the URL but cannot mutate remote state. A POST is a different
beast — the body can exfiltrate anything the model touched, and the
remote effect is generally not idempotent. Splitting into two packs
lets the `RiskTier`, policy companion, and manifest audit trail differ
naturally. A spec that only needs to fetch pages declares the cheap
pack and skips the body-scrubbing policy altogether.

### Why `mcp-bridge` ships in core but dormant

Forge should not have to grow a new seam in Phase 4. The factory is
registered from day one; its `Build` method returns
`mcp_not_yet_supported` until `KindMCPBinding` is activated
([`registry-interfaces.md:44-46`](registry-interfaces.md#L44-L46),
[`mismatches.md`](mismatches.md) Mismatch 4). When Phase 4 lands, the
change is an unlock in the registry — not a new integration point in
every consumer's `main`.

## The "required companion policy-pack" rule

Three of the core packs are unsafe without a policy peer. Rather than
document this as a convention, the build layer enforces it as a
capability constraint:

```go
// Pseudocode; real implementation lives in forge/build.
// The companion declaration is carried as data on each ToolDescriptor,
// not as a hard-coded map inside forge.
type ToolDescriptor struct {
    // ... existing fields from registry-interfaces.md ...
    RequiredPolicyPackID registry.ID // e.g. "policypack.fs-guard@1"
}

// During compatibility checks: every ToolPack ref in the spec whose
// descriptor carries RequiredPolicyPackID must have that policy-pack
// present in spec.policies[]. Missing → build fails with
// `missing_required_policy_companion` and the offending pair.
```

Three consequences worth calling out:

1. **No warnings.** Missing companion aborts the build, consistent with
   the "strict validation over permissive fallback" principle
   ([`forge-overview.md:170-173`](forge-overview.md#L170-L173)).
2. **Overlays cannot unbind.** An overlay that removes a required
   policy-pack fails validation after composition, before the registry
   is consulted.
3. **The rule is data, not code.** Because `RequiredPolicyPackID` lives
   on the ToolDescriptor, third-party ToolPacks can declare their own
   companions without touching forge internals.

The three companion policy-packs (`fs-guard@1`, `shell-guard@1`,
`http-body-scrubber@1`) ship inside `praxis-forge` alongside their
ToolPacks. They are *part of the core bundle*, not user homework.

## Satellite repo — `praxis-toolpacks`

A single Go module with one sub-package per ToolPack:

```
github.com/praxis-os/praxis-toolpacks/
├── go.mod
├── git/        # local git operations (log, diff, status, commit)
├── github/     # PR / issue / review
├── search/     # single-vendor in v0 (see Open questions)
├── sql/        # postgres + sqlite
├── jira/
├── slack/
├── notion/
├── gcal/
├── gmail/
├── confluence/
└── code/       # tree-sitter / LSP queries
```

Consumer usage:

```go
import (
    "github.com/praxis-os/praxis-toolpacks/git"
    "github.com/praxis-os/praxis-toolpacks/github"
)

r := registry.NewComponentRegistry()
must(r.RegisterToolPack(git.NewFactory("toolpack.git@1")))
must(r.RegisterToolPack(github.NewFactory("toolpack.github@1")))
```

Each sub-package owns: the factory, its JSON Schema, its typed config,
its integration tests. No cross-package imports between satellites —
they compose only through the registry.

### Priority tiers

**Tier 1** — first milestone after core ships:

- `git` (local git only, no remote push)
- `github` (PR / issue / review via `go-github` or `gh` shell-out)
- `search` (one of Exa / Brave / Tavily — pick one; see Open questions)
- `sql` (postgres read + sqlite read; write gated behind policy peer)

**Tier 2** — office integrations, ordered by user pull:

- `jira`, `confluence`, `slack`, `notion`, `gcal`, `gmail`

**Tier 3** — specialized:

- `code` (tree-sitter / LSP queries)
- `browser` (Playwright headless — evaluate before committing; heavy
  dependency, licensing considerations)
- `cloud` (AWS / GCP SDK subsets)

### Naming convention

- Factory IDs use a flat dotted name: `toolpack.git@1`,
  `toolpack.github@1`, `toolpack.search.exa@1` (vendor suffix when a
  slot has multiple implementations).
- Package import path mirrors the ID root:
  `praxis-toolpacks/git`, `praxis-toolpacks/search/exa`.
- Required policy companions (if any) live in a parallel
  `praxis-policypacks` satellite with matching naming.

## Companion repo — `praxis-policypacks`

Mentioned here for completeness; full design lives in a sibling
document. The core forge bundles only the three policy-packs required
by its core ToolPacks. Everything else (PII redaction, jailbreak
guard, secret scrubber, org-specific tag rules) goes into
`praxis-policypacks` on the same monorepo pattern.

## Open questions

- **`go.mod` granularity.** Single module at `praxis-toolpacks` root is
  simpler to release but forces every consumer to inherit the union of
  sub-package dependencies. Nested modules (one `go.mod` per sub-dir)
  isolate heavy deps (Slack SDK, Playwright) at the cost of release
  tooling. Proposed default: **single module**, promote a sub-package
  to nested module only when its dependency footprint forces it (rule
  of thumb: >20 MB download, CGO requirement, or incompatible license
  surface).
- **Search vendor for v0.** Exa (semantic-focused, generous free tier),
  Brave (cheaper, web-index-driven), Tavily (agent-optimized).
  Decision pending — pick one for v0 and document the others as Tier-3
  follow-ups.
- **`shell-guard@1` policy surface.** Needs a minimum useful allowlist
  shape — literal command list? argv shape predicates? regex on full
  argv? RFC in Phase 1 before implementation.
- **`http-fetch@1` response decoding.** Does the pack auto-decode JSON
  / HTML-to-text, or hand the raw bytes to the model? v0 leans raw;
  `json-xform@1` composes on top.
- **`fs-read@1` / `fs-write@1` sandbox root.** Passed via factory
  config at registration time, or per-invocation context? v0 leans
  **per-invocation** so different agents can scope differently without
  re-registering the factory.
- **`mcp-bridge@1` tool naming.** Keep the remote tool's own name, or
  prefix with the binding ID (`{binding}__{tool}`, as
  `praxis/mcp.Invoker` already does)? Existing praxis convention wins
  by default; confirm in Phase 4.
- **Phase 1 scope.** [`forge-overview.md:148-152`](forge-overview.md#L148-L152)
  currently describes the Phase 1 minimum slice as "one tool pack".
  This document expands that to ten core ToolPacks + three companion
  policy-packs. The overview's Phase 1 bullet needs a corresponding
  update before Phase 1 planning.
