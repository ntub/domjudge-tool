# CLAUDE.md

Instructions for Claude Code working in this repository. This file is
loaded into context for every session — keep it short, concrete, and
actionable. Defer detail to `docs/`.

## Project

`domjudge-tool` is a Go-based CLI (and, eventually, desktop app) for
managing DOMjudge contest servers. It's a multi-phase rewrite of the
Python `ntub-inc/domjudge-tool-cli` 0.3.0. **Phase 1 is feature parity
with the Python CLI for DOMjudge 7.3.2.** See `docs/ROADMAP.md` for the
full phase plan.

## Tech stack

- **Go 1.23+** (declared in `go.mod`; local dev on newer versions is fine).
- **Cobra** (added in Phase 1) — CLI framework.
- **goquery** (added in Phase 1) — HTML scraping for the web UI gateway.
  **Pinned at v1.10.3** — v1.12+ bumps the Go directive to 1.25, breaking
  the `go 1.23` module declaration.
- **`net/http`**, **`encoding/json`**, **`encoding/csv`** — standard library.
- **just** — task runner. **lefthook** — git hooks. **golangci-lint** —
  linter. **git-cliff** — changelog from Conventional Commits.

## Environment quirks

- `lefthook` prints "Can't find lefthook in PATH" on every commit — expected,
  ignorable. Commits still succeed; hooks run where lefthook is installed.
- `golangci-lint` lives at `/opt/homebrew/bin/golangci-lint` on the dev
  machine (`just lint` also works).

## Common commands

| Command                      | What it does                                      |
| ---------------------------- | ------------------------------------------------- |
| `just`                       | List all tasks.                                   |
| `just setup`                 | `go mod download` + `lefthook install` + tool hints. |
| `just build`                 | Build `./bin/domjudge-tool`.                      |
| `just run -- <args>`         | Run the CLI from source.                          |
| `just test` / `just test-race` | Tests; race detector variant.                   |
| `just cover`                 | HTML coverage at `coverage.html`.                 |
| `just fmt`                   | Format with gofumpt (fallback: gofmt -s).         |
| `just vet` / `just lint`     | `go vet` / `golangci-lint run`.                   |
| `just privacy-check`         | TruffleHog secrets scan + project PII gates.      |
| `just check`                 | `fmt + vet + lint + privacy-check + test`. Used by CI. |
| `just changelog-unreleased`  | Preview pending changelog entries.                |

## Code conventions

- Errors wrap with `%w`. Sentinel errors use `errors.Is` / `errors.As`.
- I/O functions take `ctx context.Context` as the **first** parameter.
- No `init()` outside registry wiring.
- No `log.Fatal` in library code.
- Imports are grouped by `gci`: stdlib → third-party → `github.com/ntub/...`.
- Full rules: `docs/CONVENTIONS.md`.

## Testing

- Standard library `testing` only. **Do not add testify or ginkgo.**
- Table-driven, `t.Run(tc.name, ...)`.
- Fixtures in `testdata/<package>/<case>.{html,json,csv}`.
- `testdata/web/v7/` fixtures are sanitized snapshots from production DOMjudge
  DOMjudge 7.3.4. Do NOT re-capture or edit without re-running the 8 PII
  grep gates in `testdata/web/v7/CAPTURE.md`.
- **Never** hit a real DOMjudge from a test — always `httptest.Server`.
- Full rules: `docs/TESTING.md`.

## Privacy and secrets

Hard rule: **PII, secrets, credentials, raw production captures, and
environment-identifying details must not enter tracked files, docs,
fixtures, commit messages, tag messages, branch names, or git history.**

- Before committing any docs, scripts, fixtures, live-validation notes, or
  generated outputs, run `just privacy-check`.
- TruffleHog covers generic secrets/API keys/tokens. The project PII gate
  covers known DOMjudge hostnames, school/maintainer tokens, student ids,
  course/cohort tokens, private IPs, non-example emails, and unsanitized
  fixture content.
- Use aliases and reserved examples in committed text: `example.org`,
  `test-station.example.org`, `prod-station.example.org`, `192.0.2.1`,
  `203.0.113.10`, `user1@example.org`, `/path/to/domjudge-tool`.
- Keep raw captures, local configs, credentials, live-test outputs, and
  production-derived files only under ignored local paths such as `tmp/`,
  `tmp/raw_api_v4/`, or outside the repo.
- Do not commit real DOMjudge URLs, real public IPs, local absolute paths,
  personal usernames/emails in examples, passwords, API tokens, cookies,
  CSRF/session values, student/course data, or production contest names.
- `README.md` and `LICENSE` may retain the author's public attribution.

## Commit messages

Conventional Commits, enforced by `commit-msg` hook and CI. Cheat sheet:
`docs/COMMITS.md`. **Don't bypass the hook with `--no-verify`.**

TDD commit triplet per issue: `test(<scope>): red — <desc>` →
`feat(<scope>): green — <desc>` → optional `refactor(<scope>): <desc>`.

## Merging a PR

1. Squash: `git reset --soft main && git commit -m "feat(...): ..."`
2. Merge: `git checkout main && git merge --no-ff <branch> -m "Merge branch '<branch>' (PR-N)"`

## Documentation

- Update `README.md` only when user-visible features ship or install
  changes.
- Update `docs/ARCHITECTURE.md` when packages are added, removed, or
  reshaped.
- Add an ADR (`docs/adr/NNNN-...`) when a decision is non-obvious or
  hard to reverse. Template is in `docs/adr/template.md`.
- Meta-rules for docs: `docs/DOCUMENTATION.md`.

## Things to avoid

- **Don't add testify or ginkgo.** Standard library is enough.
- **Don't put business logic in `cmd/`.** `cmd/domjudge-tool/main.go` is
  intentionally tiny — version line and a hand-off to `internal/cli`.
- **Don't break `domserver.json` backward compatibility.** Field names,
  types, and defaults are user-facing. See `docs/ROADMAP.md` Phase 1
  notes.
- **Don't silently fix the v7 scraper quirks.** The Phase 1 plan locks
  in some Python behaviour bit-for-bit; see ROADMAP for the list.
- **Don't use `init()`** except for registry wiring.
- **Don't print from library packages.** `cmd/` and `internal/cli/` print;
  everything else returns data or logs via `slog`.
- **Don't use bare `gofmt`**; use `gofmt -s` (simplify flag). `just fmt`
  and CI both use `-s` — diverging causes a CI format failure.

## External docs (use `ctx7`)

Per the global `ctx7` rule: when working with any third-party library,
framework, SDK, CLI tool, or cloud service — fetch current docs via the
`ctx7` CLI rather than relying on training data. This matters most in
Phase 3 (Wails, PocketBase, Tailwind v4, shadcn/ui, TanStack), where
training data lags hardest, but the rule applies to Cobra and goquery in
Phase 1 too.

## Map of the repo

```
cmd/domjudge-tool/   thin entry point — version line; hand off to internal/cli
internal/            business logic; not importable outside this module
docs/                architecture, conventions, testing, roadmap, ADRs
testdata/            HTML / CSV / JSON fixtures (committed verbatim)
.claude/             Claude Code project config (hooks + permissions)
```

When in doubt, read `docs/ARCHITECTURE.md` before adding a new package.
