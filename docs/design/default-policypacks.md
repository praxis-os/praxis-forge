# Design — Default Policy-packs

Which policy-packs and filters ship inside `praxis-forge` itself,
which live in the `praxis-policypacks` satellite repo, and why.
Phase 0 proposal describing the **target governance-layer set**;
phasing below clarifies what lands when.

## Phasing

Phase 1 is a vertical slice and ships **one factory per governance
seam** — not the full target set. Per the Phase 1 design spec:

| Kind              | Phase 1 factory                  |
|-------------------|----------------------------------|
| `policy_pack`     | `policypack.pii-redaction@1`     |
| `pre_llm_filter`  | `filter.secret-scrubber@1`       |
| `pre_tool_filter` | `filter.path-escape@1`           |
| `post_tool_filter`| `filter.output-truncate@1`       |

The three required companions — `fs-guard@1`, `shell-guard@1`,
`http-body-scrubber@1` — arrive in Phase 2 **together with their
guarded ToolPacks** (`fs-write@1`, `shell-exec@1`, `http-post@1`).
There is no companion constraint to enforce in Phase 1 because Phase 1
ships no destructive ToolPack.

The four Phase 1 factories listed above are genuine default
components, not throwaway demo code. They live in core forge, not in
the satellite. The satellite repo below hosts everything *beyond* the
Phase 1 seed + the three Phase 2 companions.

This document is the governance-layer sibling of the ToolPack doc. It
covers two registry kinds that share the same "bundle minimally, let
users opt-in" ethos:

- `KindPolicyPack` — produces `hooks.PolicyHook` (allow / deny / log /
  require-approval decisions)
- `KindPreLLMFilter`, `KindPreToolFilter`, `KindPostToolFilter` —
  produce per-stage filters (pass / redact / log / block transforms)

## Why this split matters

Policy-packs and filters sit on the control path of every turn. A
bundled default set has to be:

- *safe* — a missing guard must not silently become a shipped agent
- *minimal* — the fewer rules in core, the fewer surprises an operator
  cannot opt out of
- *governance-grade* — each pack carries manifest descriptors that tie
  it to the ToolPacks it protects

The satellite repo is where opinion lives: PII redaction rule sets,
jailbreak heuristics, compliance tag vocabularies, org-specific
policies. Those evolve faster than the core can release, and different
orgs want different defaults.

## Criterion boundary

A policy-pack (or filter) lives **in `praxis-forge`** when *all* of
these hold:

- required companion for a core ToolPack (see "required companion"
  rule in [`default-toolpacks.md`](default-toolpacks.md#the-required-companion-policy-pack-rule)),
  or structurally foundational (no plausible agent runs without it)
- zero external services, zero credentials
- rule set stable enough that changes are a breaking semver bump, not a
  weekly update
- small enough that the whole rule table reads in one sitting

A pack lives **in `praxis-policypacks`** (satellite monorepo) when
*any* of these hold:

- rule set evolves with regulatory or threat landscape (PII,
  jailbreaks, prompt injection, GDPR/HIPAA/SOC2 tagging)
- depends on a maintained pattern / signature database
- org-specific customization is expected
- targets a specific vendor ecosystem

## Core governance-layer factories

Seven factories across two groups. All live in `praxis-forge`; all
ship eventually as part of the core bundle. The phase column tracks
when each lands.

### Phase 1 baseline — one factory per governance seam

| ID                            | Kind               | Decision shape | Notes |
|-------------------------------|--------------------|----------------|-------|
| `policypack.pii-redaction@1`  | `policy_pack`      | deny / log     | Regex bank; `strictness` config (low / medium / high). `high` denies on SSN / CC. |
| `filter.secret-scrubber@1`    | `pre_llm_filter`   | redact         | Patterns for `sk-*`, `ghp_*`, AWS keys. |
| `filter.path-escape@1`        | `pre_tool_filter`  | block          | Blocks `../` traversal in tool args. |
| `filter.output-truncate@1`    | `post_tool_filter` | log            | `maxBytes` cap; truncate + log. |

These are not demo throwaways — they are the permanent default
factories for their respective seams. The composition adapters live
alongside them in `build/` ([`registry-interfaces.md:156-194`](registry-interfaces.md#L156-L194)).

### Phase 2 companions — one per risky ToolPack

| ID                           | Companion to             | Decision shape | Notes |
|------------------------------|--------------------------|----------------|-------|
| `policypack.fs-guard@1`      | `toolpack.fs-write@1`    | deny / require-approval | Path allowlist + write-depth cap + optional approval threshold for deletes |
| `policypack.shell-guard@1`   | `toolpack.shell-exec@1`  | deny / log              | Command allowlist (argv-shape predicates), env scrubbing, cwd containment |
| `policypack.http-body-scrubber@1` | `toolpack.http-post@1` | redact / log           | Structurally a policy-pack, not a filter: emits auditable decisions even when redaction is successful |

Each arrives in lock-step with its guarded ToolPack: adding
`toolpack.fs-write@1` to the registry without also registering
`policypack.fs-guard@1` fails the build (see the required-companion
rule below).

### Why `http-body-scrubber` is a policy-pack and not a filter

Filters transform data silently (pass / redact / block). Policy-packs
emit auditable decisions. For outbound HTTP bodies both concerns
matter: the body *is* transformed, but the operator also needs a record
of *what was scrubbed and why*. v0 implements this as a policy-pack
that carries the redaction as part of its `Decision` payload, so the
manifest / telemetry path sees one object per scrub event.

## The required-companion rule, restated from the policy side

Every policy-pack factory declares — in its `PolicyDescriptor` — the
ToolPack ID it guards (if any). The build layer walks the spec's
policy list at compatibility-check time and verifies:

- each core ToolPack that declares a `RequiredPolicyPackID` has its
  companion present
- conversely, a companion policy-pack without its guarded ToolPack is
  *allowed* (operator may want the guard pre-armed even if the tool is
  conditionally loaded later — overlay-driven)

This is the same rule as in the ToolPack doc, phrased from the policy
side. It lives as data on descriptors; the build layer enforces it
without hardcoded maps.

## Satellite repo — `praxis-policypacks`

Monorepo, same shape as `praxis-toolpacks`:

```
github.com/praxis-os/praxis-policypacks/
├── go.mod
├── pii-redaction-plus/        # extended rule sets beyond the core regex bank
├── jailbreak-guard/           # prompt jailbreak heuristics
├── prompt-injection-guard/    # indirect injection defenses
├── secret-scrubber-plus/      # extended patterns + entropy heuristics
├── compliance-gdpr/           # tag vocabulary + region routing rules
├── compliance-hipaa/
├── compliance-soc2/
└── org-observer/              # generic audit-only pack for staging
```

Consumer usage:

```go
import (
    "github.com/praxis-os/praxis-policypacks/jailbreak-guard"
    "github.com/praxis-os/praxis-policypacks/compliance-gdpr"
)

must(r.RegisterPolicyPack(jailbreakguard.NewFactory("policypack.jailbreak-guard@1")))
must(r.RegisterPolicyPack(compliancegdpr.NewFactory("policypack.compliance-gdpr@1")))
```

The satellite deliberately does not duplicate the four Phase 1
baseline factories: it extends them. `pii-redaction-plus` and
`secret-scrubber-plus` are *additive* — they compose with their core
counterparts in a chain, they don't replace them.

### Priority tiers

**Tier 1** — threat-model packs, first satellite release:

- `jailbreak-guard` (prompt jailbreak heuristics)
- `prompt-injection-guard` (indirect injection defenses)
- `pii-redaction-plus` (extended detector set, composes with the
  core regex bank)

**Tier 2** — compliance tag vocabularies:

- `compliance-gdpr`, `compliance-hipaa`, `compliance-soc2`
- `org-observer` (audit-only, staging / shadow mode)

**Tier 3** — extended filters:

- `secret-scrubber-plus` (entropy-based detectors on top of the core
  pattern set)

### Naming convention

- Factory IDs: `policypack.<name>@<major>` and `filter.<name>@<major>`
- Package path mirrors the ID root: `praxis-policypacks/pii-redaction`,
  `praxis-policypacks/secret-scrubber`
- Cross-kind naming is by prefix (`policypack.` vs `filter.`), not by
  sub-directory — keeps the satellite flat and scannable

## Open questions

- **`shell-guard@1` rule-language.** Literal command allowlist is
  brittle; full regex is unsafe. Proposed middle path: argv-shape
  predicates (`{cmd: "git", args: ["log", "--oneline", "<any>"]}`).
  Decision pending before Phase 1 implementation.
- **`fs-guard@1` approval threshold.** Should deletes always require
  operator approval, or only deletes above a configurable depth / file
  count? v0 leans "always require approval for delete"; overlay can
  relax for trusted agents.
- **`http-body-scrubber@1` pattern source.** Ship with a small built-in
  pattern set (AWS keys, Stripe keys, bearer tokens) and let config
  extend? Or empty by default and require operator-declared patterns?
  v0 leans *built-in minimum set, extensible by config*.
- **Filter composition ordering.** When multiple filters at a stage
  both redact, does the second see the first's redaction? Forge's
  chain adapter currently runs them in declared order; confirm this is
  the right semantics before freezing.
- **Manifest schema for policy decisions.** `http-body-scrubber@1`
  needs a structured decision payload (what was scrubbed, which
  pattern matched, which bytes). Locking this format in Phase 1
  constrains future policy-packs that want similar audit richness.
