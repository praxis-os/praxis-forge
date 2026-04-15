> Part of [praxis-forge Phase 1 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-15-praxis-forge-phase-1-design.md`](../../specs/2026-04-15-praxis-forge-phase-1-design.md).

## Task group 6 — Phase 0 doc amendments

### Task 6.1: Update design docs to match locked decisions

**Files:**
- Modify: `docs/design/registry-interfaces.md`
- Modify: `docs/design/forge-overview.md`
- Modify: `docs/design/agent-spec-v0.md`

- [ ] **Step 1: `registry-interfaces.md`**

In the `Factory — one interface, one Build method per kind` section, remove `ConfigSchema() json.RawMessage` from every factory interface sketch. Add a note:

> **Phase 1 note.** Factory interfaces carry only `ID`, `Description`, and
> `Build(ctx, cfg map[string]any)`. Each factory decodes `cfg` into its own
> typed struct. JSON Schema introspection is deferred to Phase 2 when
> lockfile/manifest tooling needs it.

In the `Kind` enum listing, add `KindPromptAsset` after `KindProvider`.

Add a `PromptAssetFactory` sketch alongside the other factory-per-kind examples:

```go
type PromptAssetFactory interface {
    ID() ID
    Description() string
    Build(ctx context.Context, cfg map[string]any) (string, error)
}
```

- [ ] **Step 2: `forge-overview.md`**

In the `Top-level forge facade` section, change the example from:

```go
built, err := forge.Build(ctx, spec, overlays, registry, ...)
```

to:

```go
built, err := forge.Build(ctx, spec, registry, opts...) // Phase 2 adds overlays
```

In `Phase roadmap` under Phase 1, add to the bullet list: "no overlays in the Build signature; added Phase 2". Under Phase 2: "Build signature grows `overlays []AgentOverlay`".

- [ ] **Step 3: `agent-spec-v0.md`**

In the `Referenced kinds` table, add a row:

| Slot               | Kind           | Resolves to                  |
|--------------------|----------------|------------------------------|
| `prompt.system`    | `prompt_asset` | a string injected per-invoke |

Under `Open questions (raised for Phase 1 review)`, replace the prompt question with a resolution note:

> **Resolved in Phase 1.** Prompts flow through registered `prompt_asset`
> factories. The simplest factory (`prompt.literal@1`) returns its configured
> `text` verbatim. Inline literal prompts in the spec remain prohibited for
> tighter governance.

- [ ] **Step 4: Verify no broken cross-links**

Run: `grep -rn "overlays \[\]AgentOverlay" docs/` and inspect each hit.
Run: `grep -rn "ConfigSchema" docs/` and confirm no surviving references imply Phase 1 support.

- [ ] **Step 5: Commit**

```bash
git add docs/design
git commit -m "docs: amend Phase 0 designs — drop ConfigSchema, add prompt_asset, defer overlays"
```

---

