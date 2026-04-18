# Contributing to praxis-forge

Thank you for your interest in contributing. This document covers everything
you need to get started: local setup, coding standards, commit conventions, and
the PR process.

praxis-forge is the build layer of the praxis stack — it turns typed factory
configuration into runnable praxis runtimes. The core runtime lives in
[`github.com/praxis-os/praxis`](https://github.com/praxis-os/praxis).

## Prerequisites

- Go 1.26 or later
- `make`
- `golangci-lint` (install: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)
- `commitsar` (install: `go install github.com/aevea/commitsar@latest`)

## Local setup

```sh
git clone https://github.com/praxis-os/praxis-forge.git
cd praxis-forge
go mod download
make check
```

`make check` runs lint, tests (with race detector), banned-identifier check,
and SPDX header verification. All checks must pass before opening a PR.

### Available make targets

| Target | What it does |
|---|---|
| `make test` | `go test -race -count=1 ./...` |
| `make lint` | `golangci-lint run ./...` |
| `make vet` | `go vet ./...` |
| `make fmt` | Verify `gofmt` formatting (fails if files need reformatting) |
| `make bench` | Run benchmarks with `-benchmem -count=5` |
| `make cover` | Generate coverage report (`coverage.out`, excludes `examples/`) |
| `make banned-grep` | Check for banned identifiers (decoupling contract) |
| `make spdx-check` | Verify every `.go` file has an SPDX header |
| `make check` | Run all CI gates: lint + test + banned-grep + spdx-check |
| `make tidy` | `go mod tidy` |
| `make integration` | Run integration tests (`-tags=integration`, needs `ANTHROPIC_API_KEY`) |

## Code style

- Run `gofmt -w .` before committing. The `make fmt` target fails CI if any
  files need reformatting.
- `golangci-lint` must pass without suppressions (`make lint`).
- Every `.go` file must begin with an SPDX header:
  ```go
  // SPDX-License-Identifier: Apache-2.0
  ```
  The `make spdx-check` target enforces this.
- Follow standard Go idioms: accept interfaces, return concrete types; explicit
  error handling; context propagation in all blocking calls.
- Document all exported symbols with godoc comments.

## Testing requirements

- Write table-driven tests with `t.Run` subtests.
- Tests that exercise concurrent code must pass the race detector (`make test`
  uses `-race` by default).
- Add benchmarks for hot paths (registry lookups, factory dispatch, spec
  validation). Name them `BenchmarkXxx` and run with `make bench`.
- Use `t.Helper()` in shared test utilities.
- Integration tests that hit live providers must be gated by the `integration`
  build tag and must not run by default.

## Commit message conventions

All commits must follow
[Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <description>
```

Allowed types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `perf`,
`ci`, `build`.

Scope is optional. When provided, it should be the package name — for example
`build`, `registry`, `spec`, `factories/provideranthropic`.

Breaking changes must be signalled with a `BREAKING CHANGE:` footer — not with
a `!` suffix:

```
feat(build): make resolver options required

BREAKING CHANGE: Build signature changed; callers must pass ResolverOptions.
```

`commitsar` validates commit format as a required CI check. Non-conforming
commits will fail the PR gate.

## DCO sign-off requirement

Every commit must carry a `Signed-off-by` line. Use `git commit -s` to add it
automatically:

```sh
git commit -s -m "feat(build): add per-factory budget pre-check"
```

This produces:

```
feat(build): add per-factory budget pre-check

Signed-off-by: Your Name <your@email.com>
```

By signing off you certify that you wrote the code or have the right to submit
it under the Apache 2.0 license, per the
[Developer Certificate of Origin v1.1](DCO).

The `probot/dco` GitHub App enforces this as a required check on all PRs
targeting `main`. PRs with unsigned commits will not be merged.

### Retrofitting sign-off on an existing branch

If you have in-flight commits on a feature branch made before the DCO
requirement landed, add sign-off retroactively with:

```sh
git rebase --signoff origin/main
git push --force-with-lease
```

This rewrites **your feature branch only** — never use it on `main`.

## Pull request process

1. Fork the repository and create a feature branch from `main`.
2. Make your changes, add tests, and verify `make check` passes locally.
3. Open a PR against `main`. Fill in the PR template.
4. All required CI checks must be green before review:
   - `lint` — golangci-lint
   - `test` — go test with race detector
   - `banned-grep` — decoupling contract
   - `spdx-check` — license headers
   - `commitsar` — conventional commits
   - `dco` — Signed-off-by on every commit
5. Review requirement: one maintainer approval is required during v0.x.
6. All PRs are squash-merged into `main`.

## Decoupling contract

praxis-forge is a generic library. The following identifiers are banned from
all `.go` and `.md` files outside of designated disclosure sections. The
`make banned-grep` target enforces this automatically.

Banned identifiers include consumer brand names and hardcoded identity
attributes (`org.id`, `agent.id`, `user.id`, `tenant.id`). Attribute
enrichment is always caller-provided — never baked into the framework.

## Questions and RFCs

For bugs and small improvements, open a GitHub issue. During v0.x, open an
issue to propose significant API changes before submitting a large PR.
