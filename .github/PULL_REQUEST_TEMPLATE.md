## Summary

<!-- What does this PR do? Why? One or two sentences. -->

## Type of change

<!-- Tick the box that applies. -->
- [ ] `feat` — new feature
- [ ] `fix` — bug fix
- [ ] `docs` — documentation only
- [ ] `refactor` — code change that neither fixes a bug nor adds a feature
- [ ] `test` — adding or updating tests
- [ ] `perf` — performance improvement
- [ ] `chore` / `ci` / `build` — tooling, dependencies, or release machinery

## Checklist

- [ ] All commits are signed off (`git commit -s`) — enforced by the `dco` check
- [ ] Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) — enforced by `commitsar`
- [ ] `make check` passes locally (lint + test + banned-grep + spdx-check)
- [ ] New `.go` files include the SPDX header `// SPDX-License-Identifier: Apache-2.0`
- [ ] Tests cover the new behaviour (unit tests; integration tests gated by `-tags=integration` where applicable)
- [ ] Exported symbols have godoc comments
- [ ] For breaking changes: the commit body contains a `BREAKING CHANGE:` footer

## Linked issues

<!-- `Closes #123`, `Refs #456`, or "n/a". -->
