> Part of [praxis-forge Phase 1 Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-15-praxis-forge-phase-1-design.md`](../../specs/2026-04-15-praxis-forge-phase-1-design.md).

## Task group 0 — Repo prep

### Task 0.1: Dev tooling bootstrap

**Files:**
- Create: `Makefile`
- Create: `.golangci.yml` (copy from `../praxis/.golangci.yml` if present)
- Modify: `go.mod` (add `gopkg.in/yaml.v3`)

- [ ] **Step 1: Inspect praxis lint config**

Run: `ls ../praxis/.golangci* 2>/dev/null && cat ../praxis/.golangci.yml`
If present, copy to `./.golangci.yml` verbatim. If absent, create a minimal one (see Step 3).

- [ ] **Step 2: Add YAML dep**

Run: `go get gopkg.in/yaml.v3 && go mod tidy`
Expected: `go.mod` gains `require gopkg.in/yaml.v3 v3.x.y`; `go.sum` created.

- [ ] **Step 3: Create Makefile**

```make
.PHONY: test test-race lint fmt tidy integration

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

integration:
	go test -tags=integration ./examples/demo/...
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum Makefile .golangci.yml
git commit -m "chore: bootstrap dev tooling (Make, lint, yaml dep)"
```

---

