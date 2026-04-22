# Task group 5 — Demo flag `-mcp` and docs amendments

> Part of [praxis-forge Phase 4 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-22-praxis-forge-phase-4-design.md`](../../specs/2026-04-22-praxis-forge-phase-4-design.md).

**Commits (2 atomic):**
1. `examples(demo): -mcp flag exercising filesystem binding contract`
2. `docs: record Phase 4 shipped (MCP consume as runtime binding contract)`

---

### Task 13: Demo flag `-mcp`

**Files:**
- Modify: `examples/demo/main.go`
- Create: `examples/demo/agent-mcp.yaml`

- [ ] **Step 1: Write demo spec**

Create [examples/demo/agent-mcp.yaml](../../../../examples/demo/agent-mcp.yaml):

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: demo.mcp
  version: 0.1.0
  displayName: mcp-binding demo
provider:
  ref: provider.anthropic@1.0.0
  config:
    model: claude-sonnet-4-5
    maxTokens: 1024
prompt:
  system:
    ref: prompt.demo-system@1.0.0
policies:
  - ref: policypack.pii-redaction@1.0.0
    config:
      strictness: medium
mcpImports:
  - ref: mcp.binding@1.0.0
    config:
      id: fs
      connection:
        transport: stdio
        command:
          - npx
          - -y
          - "@modelcontextprotocol/server-filesystem"
          - /tmp/demo
      allow:
        - "read_*"
        - "list_*"
      deny:
        - "write_*"
      policies:
        - policypack.pii-redaction@1.0.0
      trust:
        tier: medium
        owner: demo
      onNewTool: block
```

- [ ] **Step 2: Add `-mcp` flag + registration**

In [examples/demo/main.go](../../../../examples/demo/main.go), add a new flag alongside the `structured` one:

```go
	structured := flag.Bool("structured", false, "use the Phase-3 structured-output skill path")
	mcpDemo := flag.Bool("mcp", false, "use the Phase-4 MCP binding demo spec (filesystem server)")
	flag.Parse()
```

Add the import: `"github.com/praxis-os/praxis-forge/factories/mcpbinding"`.

After the `if *structured { ... }` block (around line 67), add:

```go
	if *mcpDemo {
		must(r.RegisterMCPBinding(mcpbinding.NewFactory("mcp.binding@1.0.0")))
	}
```

Extend the spec-path selection around line 73:

```go
	specPath := "examples/demo/agent.yaml"
	switch {
	case *structured:
		specPath = "examples/demo/agent-structured.yaml"
	case *mcpDemo:
		specPath = "examples/demo/agent-mcp.yaml"
	}
```

At the end of `main` (after `Build` succeeds and the existing output prints), add a dedicated MCP-demo print when `*mcpDemo` is true. Read the current tail of [examples/demo/main.go](../../../../examples/demo/main.go) first, then insert:

```go
	if *mcpDemo {
		fmt.Println("--- MCP binding contract ---")
		for _, rc := range built.Manifest.Resolved {
			if rc.Kind == "mcp_binding" {
				raw, _ := json.MarshalIndent(rc, "", "  ")
				fmt.Println(string(raw))
			}
		}
		fmt.Println("NOTE: binding is a contract; actual MCP invocation is a runtime concern.")
		return
	}
```

Add `"encoding/json"` and `"fmt"` to imports if not present. Verify that `fmt` and `json` are already imported and reuse the existing pattern if so.

- [ ] **Step 3: Run the integration build**

Run: `go build -tags=integration ./examples/demo/`
Expected: compiles clean.

Run: `ANTHROPIC_API_KEY=dummy go run -tags=integration ./examples/demo -mcp`
Expected: prints the `mcp_binding` resolved entry JSON, ending with the NOTE line, then exits before any LLM call. If it errors on Anthropic API key, that's fine — we only need to reach the MCP branch.

- [ ] **Step 4: Verify plain path still works**

Run: `go build -tags=integration ./examples/demo/`
Expected: still clean.

Run: `go test -race -count=1 ./...`
Expected: green.

- [ ] **Step 5: Commit**

```bash
git add examples/demo/main.go examples/demo/agent-mcp.yaml
git commit -m "examples(demo): -mcp flag exercising filesystem binding contract"
```

---

### Task 14: Docs amendments

**Files:**
- Modify: `docs/design/agent-spec-v0.md`
- Modify: `docs/design/forge-overview.md`

- [ ] **Step 1: Drop `mcpImports` from the deferred list**

In [docs/design/agent-spec-v0.md](../../../design/agent-spec-v0.md), find the §"Explicit deferrals" / deferred-fields section. Remove any bullet referencing `mcpImports (Phase 4)` (mirroring the way Phase 3 dropped `skills` and `outputContract` bullets). If there is a "Phase 4" reserved line, replace with a sentence acknowledging Phase 4 shipped: `mcpImports: active (Phase 4, landed 2026-04-22) — generic mcp.binding@1 factory, runtime binding contract.`

Search for the exact string using: `grep -n "mcpImports" docs/design/agent-spec-v0.md` first.

- [ ] **Step 2: Mark Phase 4 shipped in the roadmap**

In [docs/design/forge-overview.md](../../../design/forge-overview.md), locate the Phase 4 bullet at lines 185–188. Replace:

```markdown
- **Phase 4 — MCP consume.** MCP imports, remote metadata normalization,
  auth/trust metadata, allowlist/denylist, projection into forge tool
  namespace.
```

with:

```markdown
- **Phase 4 (shipped):** MCP imports activated as a declarative runtime
  binding contract. Generic `mcp.binding@1` factory, stdio + streamable-HTTP
  transports, allow/deny globs, policy chain, trust metadata, on-new-tool
  policy. Build time produces a governable contract; runtime (praxis)
  owns session establishment and drift handling. See
  [`docs/superpowers/specs/2026-04-22-praxis-forge-phase-4-design.md`](../superpowers/specs/2026-04-22-praxis-forge-phase-4-design.md).
```

- [ ] **Step 3: Verify links resolve and docs render**

Run: `go test -race -count=1 ./...`
Expected: unchanged / green (docs don't affect tests).

- [ ] **Step 4: Lint + format gate**

Run: `make fmt && make vet && make lint`
Expected: all three pass.

Also run: `make banned-grep`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add docs/design/agent-spec-v0.md docs/design/forge-overview.md
git commit -m "docs: record Phase 4 shipped (MCP consume as runtime binding contract)"
```
