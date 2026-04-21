# Task 09 — Demo and documentation

Extend [`examples/demo/main.go`](../../../examples/demo/main.go) with a `-structured` flag that registers the two Phase-3 vertical-slice factories and builds an agent that uses them. Amend the two design docs that referenced skills/outputContract as deferred to mark Phase 3 as shipped.

## Files

- Create: `examples/demo/agent-structured.yaml`
- Modify: `examples/demo/main.go`
- Modify: [`docs/design/agent-spec-v0.md`](../../../docs/design/agent-spec-v0.md)
- Modify: [`docs/design/forge-overview.md`](../../../docs/design/forge-overview.md)
- Modify: [`README.md`](../../../README.md) — status section reflects Phase 3

## Steps

### Part A — demo -structured flag

- [ ] **Step 1: Create the structured demo agent spec**

Create `examples/demo/agent-structured.yaml`:

```yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: demo.structured
  version: 0.1.0
  displayName: structured-output demo
provider:
  ref: provider.anthropic@1.0.0
  config:
    model: claude-sonnet-4-5
    maxTokens: 1024
prompt:
  system:
    ref: prompt.demo-structured-system@1.0.0
policies:
  - ref: policypack.pii-redaction@1.0.0
    config:
      strictness: medium
budget:
  ref: budgetprofile.default-tier1@1.0.0
telemetry:
  ref: telemetryprofile.slog@1.0.0
outputContract:
  ref: outputcontract.json-schema@1.0.0
  config:
    schema:
      $schema: https://json-schema.org/draft/2020-12/schema
      type: object
      properties:
        summary:
          type: string
        url:
          type: string
      required:
        - summary
skills:
  - ref: skill.structured-output@1.0.0
```

- [ ] **Step 2: Extend main.go with a -structured flag**

Edit [`examples/demo/main.go`](../../../examples/demo/main.go). Replace the existing `main` function. The change: add a `-structured` flag, register the two new factories when it's set, and pick the spec file accordingly. Full replacement of `main` (from line 41 through line 96):

```go
func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY not set")
	}
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	structured := flag.Bool("structured", false, "use the Phase-3 structured-output skill path")
	flag.Parse()

	r := registry.NewComponentRegistry()
	must(r.RegisterProvider(provideranthropic.NewFactory("provider.anthropic@1.0.0", apiKey)))
	must(r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.demo-system@1.0.0")))
	must(r.RegisterPromptAsset(promptassetliteral.NewFactory("prompt.demo-structured-system@1.0.0")))
	must(r.RegisterToolPack(toolpackhttpget.NewFactory("toolpack.http-get@1.0.0")))
	must(r.RegisterPolicyPack(policypackpiiredact.NewFactory("policypack.pii-redaction@1.0.0")))
	must(r.RegisterPreLLMFilter(filtersecretscrubber.NewFactory("filter.secret-scrubber@1.0.0")))
	must(r.RegisterPreToolFilter(filterpathescape.NewFactory("filter.path-escape@1.0.0")))
	must(r.RegisterPostToolFilter(filteroutputtruncate.NewFactory("filter.output-truncate@1.0.0")))
	must(r.RegisterBudgetProfile(budgetprofiledefault.NewFactory("budgetprofile.default-tier1@1.0.0")))
	must(r.RegisterTelemetryProfile(telemetryprofileslog.NewFactory("telemetryprofile.slog@1.0.0", slog.Default())))
	must(r.RegisterCredentialResolver(credresolverenv.NewFactory("credresolver.env@1.0.0")))
	must(r.RegisterIdentitySigner(identitysignered25519.NewFactory("identitysigner.ed25519@1.0.0", priv)))
	if *structured {
		must(r.RegisterSkill(skillstructuredoutput.NewFactory("skill.structured-output@1.0.0")))
		must(r.RegisterOutputContract(outputcontractjsonschema.NewFactory("outputcontract.json-schema@1.0.0")))
	}

	ctx := context.Background()
	specPath := "examples/demo/agent.yaml"
	if *structured {
		specPath = "examples/demo/agent-structured.yaml"
	}
	s, err := forge.LoadSpec(specPath)
	if err != nil {
		log.Fatalf("load spec: %v", err)
	}

	b, err := forge.Build(ctx, s, r)
	if err != nil {
		log.Fatalf("build: %v", err)
	}

	var prompt string
	if *structured {
		prompt = "Summarize the praxis-forge README in one paragraph (≤80 words). Respond with JSON {\"summary\": string, \"url\": string} matching the output schema."
	} else {
		url := "https://raw.githubusercontent.com/praxis-os/praxis-forge/main/README.md"
		prompt = fmt.Sprintf("Fetch %s and summarize what praxis-forge does.", url)
	}
	res, err := b.Invoke(ctx, praxis.InvocationRequest{
		Model:        "claude-sonnet-4-5",
		SystemPrompt: b.SystemPrompt(),
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: prompt}},
		}},
	})
	if err != nil {
		log.Fatalf("invoke: %v", err)
	}

	if res.Response != nil {
		for _, p := range res.Response.Parts {
			if p.Type == llm.PartTypeText {
				fmt.Printf("response: %s\n", p.Text)
			}
		}
	}
	fmt.Printf("manifest components: %d\n", len(b.Manifest().Resolved))
	if *structured {
		fmt.Printf("expanded hash: %s\n", b.Manifest().ExpandedHash)
	}
}
```

Add the `flag` import to the imports block, plus the two Phase-3 factory packages:

```go
import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"

	praxis "github.com/praxis-os/praxis"

	forge "github.com/praxis-os/praxis-forge"
	"github.com/praxis-os/praxis-forge/factories/budgetprofiledefault"
	"github.com/praxis-os/praxis-forge/factories/credresolverenv"
	"github.com/praxis-os/praxis-forge/factories/filteroutputtruncate"
	"github.com/praxis-os/praxis-forge/factories/filterpathescape"
	"github.com/praxis-os/praxis-forge/factories/filtersecretscrubber"
	"github.com/praxis-os/praxis-forge/factories/identitysignered25519"
	"github.com/praxis-os/praxis-forge/factories/outputcontractjsonschema"
	"github.com/praxis-os/praxis-forge/factories/policypackpiiredact"
	"github.com/praxis-os/praxis-forge/factories/promptassetliteral"
	"github.com/praxis-os/praxis-forge/factories/provideranthropic"
	"github.com/praxis-os/praxis-forge/factories/skillstructuredoutput"
	"github.com/praxis-os/praxis-forge/factories/telemetryprofileslog"
	"github.com/praxis-os/praxis-forge/factories/toolpackhttpget"
	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
)
```

- [ ] **Step 3: Verify the demo compiles (integration tag)**

Run: `go build -tags=integration ./examples/demo/`

Expected: compiles without error. (Running the demo requires a real API key and is out of scope for this step.)

- [ ] **Step 4: Commit demo change**

```bash
git add examples/demo/agent-structured.yaml examples/demo/main.go
git commit -m "$(cat <<'EOF'
examples(demo): -structured flag exercising Phase-3 skill path

A second spec (agent-structured.yaml) declares skill.structured-output,
an outputcontract.json-schema with a concrete Q&A schema, and the
PII-redaction policy at strictness medium. When -structured is passed
the demo registers the two new factories and prints both the JSON
response and the ExpandedHash so the reader sees the attribution flow
end-to-end.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Part B — Documentation amendments

- [ ] **Step 5: Amend agent-spec-v0.md**

Edit [`docs/design/agent-spec-v0.md`](../../../docs/design/agent-spec-v0.md).

In the "Explicit deferrals" section, **remove** the first two bullet points ([`docs/design/agent-spec-v0.md:209-219`](../../../docs/design/agent-spec-v0.md#L209-L219)):

```markdown
- **Skills (Phase 3).** `skills:` is present in the schema but must be
  empty in v0. A non-empty list fails validation with a
  `skills_not_yet_supported` error.
```

and

```markdown
- **Output contracts.** Declared only when a skill contributes one;
  v0 without skills has no output contract field.
```

Replace the "Phase-gated AgentSpec fields" sentence ([`docs/design/agent-spec-v0.md:129-131`](../../../docs/design/agent-spec-v0.md#L129-L131)):

```markdown
Phase-gated AgentSpec fields (`extends`, `skills`, `mcpImports`,
`outputContract`) are deliberately absent from the overlay body, so
the strict YAML decoder rejects them at parse time.
```

With (removes `skills`, `outputContract` since they are active at the spec level, but the overlay body still rejects them — the text needs to acknowledge that distinction):

```markdown
Overlay bodies omit phase-gated AgentSpec fields — `extends`,
`skills`, `mcpImports`, `outputContract` — so the strict YAML decoder
rejects them at parse time. For `skills` and `outputContract`
(Phase 3 active), this is a deliberate scope choice: overlays never
introduce Phase-3 composition; only the base spec or an extends
parent can. The overlay phase-gate check therefore predates the
spec-level phase unlocking and remains load-bearing.
```

In the "Deferred kinds" paragraph ([`docs/design/agent-spec-v0.md:158-159`](../../../docs/design/agent-spec-v0.md#L158-L159)), change:

```markdown
Deferred kinds: `skill` (Phase 3), `mcp_binding` (Phase 4), `output_contract`
(Phase 3, driven by skills).
```

To:

```markdown
Active kinds added in Phase 3: `skill`, `output_contract`. Deferred
kinds remaining: `mcp_binding` (Phase 4).
```

- [ ] **Step 6: Amend forge-overview.md**

Edit [`docs/design/forge-overview.md`](../../../docs/design/forge-overview.md). In the phase roadmap, change the Phase 3 bullet (around [`docs/design/forge-overview.md:184-185`](../../../docs/design/forge-overview.md#L184-L185)):

```markdown
- **Phase 3 — skills.** Skill registry, expansion rules, prompt-fragment
  merge, dependency/conflict validation, output contracts.
```

To:

```markdown
- **Phase 3 (shipped):** skill + output-contract registry kinds, skill
  expansion with strict conflict detection (version divergence, config
  divergence, multi-contract, user override, empty contribution),
  prompt-fragment append with byte-identical dedupe,
  `Manifest.ExpandedHash` alongside Phase 2b's `NormalizedHash`,
  `ResolvedComponent.InjectedBySkill` attribution, two vertical-slice
  factories (`skill.structured-output@1.0.0`,
  `outputcontract.json-schema@1.0.0`). See
  [`docs/superpowers/specs/2026-04-21-praxis-forge-phase-3-design.md`](../superpowers/specs/2026-04-21-praxis-forge-phase-3-design.md).
```

- [ ] **Step 7: Amend README.md status section**

Edit [`README.md`](../../../README.md). Find the status paragraph at around [`README.md:28-36`](../../../README.md#L28-L36) and replace "Phase 1 — minimum vertical slice." with a Phase 3 summary. Replace this block:

```markdown
## Status

**Phase 1 — minimum vertical slice.** Spec loader, typed
`ComponentRegistry`, 11 factory kinds, composition adapters, and
materialization into a real `*orchestrator.Orchestrator`. 11 concrete
factories ship alongside the kernel wiring; see
[`docs/superpowers/specs/`](docs/superpowers/specs/) for the design
spec and [`docs/superpowers/plans/`](docs/superpowers/plans/) for the
task-by-task implementation plan.
```

With:

```markdown
## Status

**Phase 3 — skills and output contracts.** Adds `KindSkill` and
`KindOutputContract` to the registry, unlocks `spec.skills[]` and
`spec.outputContract`, and introduces a skill-expansion build stage
with strict conflict detection, prompt-fragment append, and a
post-expansion `Manifest.ExpandedHash`. 13 factory kinds + two new
vertical-slice factories. Phase 2a composition depth (extends chains,
overlays, provenance) and Phase 2b determinism (canonical JSON +
stable hash + capabilities) both in place. See
[`docs/superpowers/specs/`](docs/superpowers/specs/) for design docs
and [`docs/superpowers/plans/`](docs/superpowers/plans/) for
task-by-task implementation plans.
```

- [ ] **Step 8: Run lint + full tests**

Run: `go vet ./... && go test ./... -count=1`

Expected: all tests green, vet silent.

If the repo exposes `make lint`, also run:

```bash
make lint
```

Expected: clean.

- [ ] **Step 9: Commit docs**

```bash
git add docs/design/agent-spec-v0.md docs/design/forge-overview.md README.md
git commit -m "$(cat <<'EOF'
docs: Phase 3 amendments (skills + output contracts active)

agent-spec-v0.md: drop skills/outputContract from explicit deferrals;
update the deferred-kinds paragraph to list only mcp_binding as
remaining; reword the overlay body explanation to distinguish
spec-level activation from the overlay-body boundary (which still
rejects Phase-3 fields by design).

forge-overview.md: Phase 3 marked (shipped) with a summary of what
landed and a link to the design doc.

README.md: status section rewritten for Phase 3.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

## Expected state after this task

- `examples/demo/agent-structured.yaml` + extended `main.go` with `-structured` flag.
- `docs/design/agent-spec-v0.md` amended — skills/outputContract active.
- `docs/design/forge-overview.md` amended — Phase 3 shipped.
- `README.md` amended — Phase 3 status.
- Full `go test ./...` green, `make lint` clean.
- Two commits added.
