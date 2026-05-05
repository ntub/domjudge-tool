# Contributing to domjudge-tool

Thanks for working on this project. This guide covers the contributor
workflow: setup, branching, commits, tests, PRs, and releases.

## Prerequisites

| Tool                 | Version            | Required?                        |
| -------------------- | ------------------ | -------------------------------- |
| Go                   | ≥ 1.23             | Yes                              |
| just                 | latest             | Yes (task runner)                |
| git                  | ≥ 2.30             | Yes                              |
| lefthook             | latest             | Strongly recommended (git hooks) |
| gofumpt              | latest             | Recommended (formatter)          |
| golangci-lint        | latest             | Recommended (linter)             |
| TruffleHog           | latest             | Recommended (secret scanner)     |
| git-cliff            | latest             | Only for releases                |

Install missing tools:

```bash
brew install just lefthook gofumpt golangci-lint git-cliff   # macOS
# or:
go install github.com/evilmartians/lefthook@latest
go install mvdan.cc/gofumpt@latest
# golangci-lint: see https://golangci-lint.run/welcome/install/
# trufflehog:    curl -sSfL https://raw.githubusercontent.com/trufflesecurity/trufflehog/main/scripts/install.sh | sh -s -- -b /usr/local/bin
# git-cliff:     see https://github.com/orhun/git-cliff#installation
```

## First-time setup

```bash
git clone https://github.com/ntub/domjudge-tool.git
cd domjudge-tool
just setup            # go mod download + lefthook install
```

`just setup` is idempotent — re-run it whenever dependencies change.

## Daily workflow

1. **Branch.** Branch off `main`. Naming: `feat/<slug>`, `fix/<slug>`,
   `docs/<slug>`, `chore/<slug>`.
2. **Code + test.** Keep changes scoped. New behaviour gets a test;
   bug fixes get a regression test that reproduces the bug first.
3. **Run the gate locally.** `just check` runs
   `fmt + vet + lint + privacy-check + test`.
   The lefthook pre-commit hook does formatting + lint on staged files;
   the pre-push hook runs the test suite.
4. **Commit.** Conventional Commits, one logical change per commit. See
   [`docs/COMMITS.md`](docs/COMMITS.md).
5. **Open a PR.** Target `main`. Fill in the description: what + why.
   Keep PRs reviewable (≤ ~400 lines of code change is the soft cap).

## Testing

- Standard library `testing` only — no testify or ginkgo.
- Table-driven tests with `t.Run(...)` subtests.
- Fixtures in `testdata/<package>/<case>.{html,json,csv}`.
- Never hit a real DOMjudge — use `httptest.NewServer`.
- Run `just privacy-check` before committing fixture or documentation
  changes. TruffleHog catches secrets; the project PII gate catches known
  DOMjudge host, school, student, and fixture sanitization leaks.
- Full guidance: [`docs/TESTING.md`](docs/TESTING.md).

## Code style

- `just fmt` before committing (the hook does it for you).
- Errors are wrapped with `%w`; no `log.Fatal` in libraries.
- I/O takes `ctx context.Context` first.
- Full guidance: [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md).

## Documentation changes

- User-facing change → also update `README.md` and (if it ships in a
  release) `CHANGELOG.md` will pick it up automatically from the
  Conventional Commit.
- Architectural change → update `docs/ARCHITECTURE.md`.
- Significant decision → add an ADR via `docs/adr/template.md`.

## Releases

Releases are manual and infrequent. The flow:

1. Make sure `main` is green.
2. From a clean checkout: `just release X.Y.Z`. This tags and regenerates
   `CHANGELOG.md`.
3. Review the regenerated `CHANGELOG.md`. Commit it as
   `chore(release): vX.Y.Z` (this commit is filtered out of the next
   changelog by `cliff.toml`).
4. `git push --follow-tags`.

Phase 0 has no released versions yet.

## Reporting issues

Open a GitHub issue. Include:

- What you tried (the exact command).
- What you expected.
- What happened (full output, no truncation).
- Versions: `go version`, `domjudge-tool` version (when there is one).

## Git hook bypass

Don't pass `--no-verify`. If a hook fails, fix the underlying issue. CI
runs the same checks and will reject the PR anyway.
