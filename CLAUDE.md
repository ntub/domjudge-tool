# Project Agent Instructions

Instructions for coding agents in this repository. `AGENTS.md` is a symlink to
this file so Codex and Claude Code share one source of truth.
Reply in zh-TW; the rest in en-US.

## Project

- `domjudge-tool` is a Go CLI, and later a desktop app, for managing DOMjudge
  contest servers.
- Current shipped milestone: Phase 1.5 (`v1.1.0`), API-first for DOMjudge
  7.3.x with scraper fallback. Phase 2 adds DOMjudge 8.3.1. Phase 3 adds the
  desktop app.
- Preserve Python CLI compatibility from Phase 1 unless the user explicitly
  asks to change behavior.

## First Reads

- Before adding, removing, or reshaping packages, read `docs/ARCHITECTURE.md`.
- Before style, test, commit, or docs changes, read the matching file under
  `docs/`: `CONVENTIONS.md`, `TESTING.md`, `COMMITS.md`, or
  `DOCUMENTATION.md`.
- For Phase 1.5 routing/API behavior, read
  `docs/adr/0006-api-first-with-scraper-fallback.md` and
  `docs/phase1.5/SPEC.md`.

## Workflow

- Start by checking `git status --short`; do not overwrite or revert changes
  you did not make.
- Prefer `rg` and `rg --files` for search.
- Keep edits narrow and aligned with existing package boundaries.
- Use structured parsers/APIs when available instead of ad hoc string parsing.
- When a third-party library, SDK, API, CLI, framework, or cloud service is
  involved, fetch current docs with Context7:
  `npx -y ctx7@latest library "<name>" "<question>"`, then
  `npx -y ctx7@latest docs <library-id> "<question>"`.
- Do not add dependencies unless they are necessary, compatible with Go 1.23,
  and consistent with this repo's phase plan.

## Commands

- `just` lists tasks.
- `just setup` installs project dependencies and hooks.
- `just build` builds `./bin/domjudge-tool`.
- `just test` runs the test suite; use `just test-race` for concurrency work.
- `just fmt` formats with gofumpt or `gofmt -s`.
- `just vet` and `just lint` run static checks.
- `just privacy-check` runs secret and PII gates.
- `just check` runs the full local gate used by CI.

## Architecture

- `cmd/domjudge-tool` stays tiny: version wiring and handoff to `internal/cli`.
- `internal/cli` owns Cobra command wiring, flag parsing, and user-facing
  output.
- Use-case packages (`internal/importer`, `internal/exporter`,
  `internal/email`, `internal/scoreboard`) orchestrate behavior.
- DOMjudge REST API code lives under `internal/client/api/v4`; HTML scraping
  lives under `internal/client/web`.
- `internal/client/httpx` is the shared place for HTTP transport concerns.
- `internal/model`, `internal/config`, and `internal/util` are leaf packages.
- Library packages return data/errors or use structured `slog`; they do not
  print directly.

## Go Rules

- Go 1.23+ is the module target. Do not upgrade the module directive casually.
- `goquery` stays pinned at v1.10.3 unless an ADR changes the Go-version
  tradeoff.
- Wrap errors with `%w`; use `errors.Is` / `errors.As` for sentinel errors.
- I/O functions take `ctx context.Context` as the first parameter.
- Do not use `init()` except for registry wiring.
- Imports follow `gci`: standard library, third-party, then
  `github.com/ntub/domjudge-tool/...`.
- Tests use only the standard library `testing` package. Do not add testify or
  ginkgo.
- Prefer table-driven tests with `t.Run(tc.name, ...)`.

## Compatibility Locks

- Do not break `domserver.json` field names, types, or defaults.
- Preserve existing CLI command names, flag names, and output file names.
- Preserve `import-users-teams.csv` column order.
- Do not silently fix locked v7 scraper quirks. In particular, `DeleteUsers`
  and `DeleteTeams` preserve the known inverted-filter behavior; changing it
  requires tests and an ADR.
- Never hit a real DOMjudge, SMTP server, or production endpoint from tests.
  Use `httptest.Server` and sanitized fixtures.

## Privacy

- PII, secrets, raw production captures, credentials, real DOMjudge URLs,
  private IPs, personal emails, student/course identifiers, cookies, and CSRF
  values must not enter tracked files or git history.
- Use reserved examples in committed text: `example.org`,
  `test-station.example.org`, `prod-station.example.org`, `192.0.2.1`,
  `203.0.113.10`, `user1@example.org`, and `/path/to/domjudge-tool`.
- Keep raw captures, local configs, live-test outputs, and generated private
  data in ignored locations such as `tmp/`, `tmp/raw_api_v4/`, or outside the
  repo.
- Before committing docs, scripts, fixtures, live-validation notes, or
  generated outputs, run `just privacy-check`.

## Documentation And Commits

- Update `README.md` only for user-visible features or setup/install changes.
- Update `docs/ARCHITECTURE.md` when packages are added, removed, or reshaped.
- Add an ADR for decisions that are non-obvious, hard to reverse, or cross
  package boundaries.
- Use Conventional Commits. Do not bypass hooks with `--no-verify`.

## Claude Code

- Shared Claude Code settings live in `.claude/settings.json`; personal or
  machine-specific permissions stay in `.claude/settings.local.json`.
- Project hooks live under `.claude/hooks/` and should be referenced with
  `$CLAUDE_PROJECT_DIR` for portability.
- After changing `.claude/settings.json` or hook scripts, restart Claude Code
  or review `/hooks` so the running session uses the new hook snapshot.

## Repository Map

```text
cmd/domjudge-tool/   thin entry point
internal/            business logic, clients, CLI wiring
docs/                architecture, conventions, roadmap, ADRs
testdata/            sanitized fixtures
.claude/             Claude Code shared settings and hooks
.codex/              Codex project settings
```
