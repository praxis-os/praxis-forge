# SPDX-License-Identifier: Apache-2.0

.PHONY: test test-race lint fmt vet tidy bench cover integration banned-grep spdx-check check

# Single-module layout: everything under the root module.
MODULE_PATHS := ./...

# Run all tests with race detection (CI-equivalent).
test:
	go test -race -count=1 $(MODULE_PATHS)

# Alias kept for local habits.
test-race: test

# Run golangci-lint.
lint:
	golangci-lint run $(MODULE_PATHS)

# Run go vet.
vet:
	go vet $(MODULE_PATHS)

# Run gofmt check (fails if any files need formatting).
# Excludes .worktrees/ (git worktree checkouts of other branches) and vendor/.
fmt:
	@UNFORMATTED=$$(find . -name '*.go' \
	  -not -path './.worktrees/*' \
	  -not -path './vendor/*' \
	  | xargs gofmt -l); \
	if [ -n "$$UNFORMATTED" ]; then \
	  echo "Files need gofmt:"; echo "$$UNFORMATTED"; exit 1; \
	fi
	@echo "gofmt check: PASS"

# Tidy go.mod / go.sum.
tidy:
	go mod tidy

# Run benchmarks.
bench:
	go test -bench=. -benchmem -count=5 $(MODULE_PATHS)

# Generate coverage report (excludes examples/, matches CI behaviour).
cover:
	go test -race -coverprofile=coverage.out $$(go list $(MODULE_PATHS) | grep -v /examples/)
	go tool cover -func=coverage.out

# Run integration tests (live Anthropic provider, requires ANTHROPIC_API_KEY).
integration:
	go test -tags=integration ./examples/demo/...

# Check for banned identifiers (decoupling contract enforcement).
# Scope: .go and .md files. Excludes planning/tooling artefacts and files
# that legitimately discuss the rules themselves.
banned-grep:
	@echo "Checking for banned identifiers..."
	@BANNED='custos|reef|governance.event|governance_event'; \
	RESULT=$$(grep -rniw -E "$$BANNED" --include='*.go' --include='*.md' \
	  --exclude-dir='.worktrees' \
	  --exclude-dir=.claude \
	  --exclude-dir='docs/superpowers/plans' \
	  --exclude-dir='.git' \
	  --exclude='CLAUDE.md' \
	  --exclude='README.md' \
	  --exclude='REVIEW.md' \
	  . || true); \
	if [ -n "$$RESULT" ]; then \
	  echo "BANNED IDENTIFIER FOUND:"; echo "$$RESULT"; exit 1; \
	fi
	@echo "Checking for hardcoded identity attributes..."
	@ATTRS='org\.id|agent\.id|user\.id|tenant\.id'; \
	RESULT=$$(grep -rn -E "$$ATTRS" --include='*.go' . || true); \
	if [ -n "$$RESULT" ]; then \
	  echo "HARDCODED IDENTITY ATTRIBUTE FOUND:"; echo "$$RESULT"; exit 1; \
	fi
	@echo "Banned-identifier check: PASS"

# Verify SPDX headers on all Go files.
spdx-check:
	@missing=$$(find . -name '*.go' -not -path './vendor/*' -not -path './.worktrees/*' \
	  | xargs grep -L 'SPDX-License-Identifier: Apache-2.0'); \
	if [ -n "$$missing" ]; then \
	  echo "Missing SPDX header in:"; echo "$$missing"; exit 1; \
	fi
	@echo "SPDX check: PASS"

# Run all checks (CI gate).
check: lint test banned-grep spdx-check
	@echo "All checks passed."
