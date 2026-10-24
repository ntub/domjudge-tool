# Project Agent Instructions

Instructions for coding agents in this repository. `AGENTS.md` is a symlink to
this file so Codex and Claude Code share one source of truth.
Reply in zh-TW; the rest in en-US.

## Project

- `domjudge-tool` provides a Go CLI and local web UI (`domjudge-tool web`)
  for managing DOMjudge contest servers.
- Current CLI milestone: supporting DOMjudge 7.3.x and 8.3.1. The local web
  UI uses templ, Datastar, and pure-Go GORM/SQLite, served directly by the CLI
  with optional browser opening, authentication, and TLS.
- Preserve Python CLI compatibility from Phase 1 unless the user explicitly
  asks to change behavior.

## First Reads

- Before adding, removing, or reshaping packages, read `docs/ARCHITECTURE.md`.
- Before style, test, commit, or docs changes, read the matching file under
  `docs/`: `CONVENTIONS.md`, `TESTING.md`, `COMMITS.md`, or
  `DOCUMENTATION.md`.
- For transport routing and API behavior, read
  `docs/adr/0006-api-first-with-scraper-fallback.md`,
  `docs/phase1.5/CONTRACT.md`, and `docs/phase2/CONTRACT.md`.

## Workflow

- Start by checking `git status --short`; do not overwrite or revert changes
  you did not make.
- Prefer `rg` and `rg --files` for search.
- Keep edits narrow and aligned with existing package boundaries.
- Use structured parsers/APIs when available instead of ad hoc string parsing.
- When coordinating parallel subagents, prefer separate `git worktree`
  checkouts on dedicated feature branches. Give each subagent an explicit
  scope, required docs, test expectations, and compatibility locks before it
  implements anything.
- For implementation work, write or update the relevant design/spec/docs first,
  then develop with TDD: add the failing test or regression coverage, implement
  the smallest compatible change, and keep code plus docs in sync.
- After implementation, run a code review pass before accepting the result:
  evaluate behavioral risk, compatibility, privacy, missing tests, and
  documentation gaps; fix the findings before moving on.
- Finish by updating all related documentation according to
  `docs/DOCUMENTATION.md`.
- When a third-party library, SDK, API, CLI, framework, or cloud service is
  involved, fetch current docs with Context7:
  `npx -y ctx7@latest library "<name>" "<question>"`, then
  `npx -y ctx7@latest docs <library-id> "<question>"`.
- Do not add dependencies unless they are necessary, compatible with Go 1.27.3,
  and consistent with this repo's phase plan.

## Commands

- `mise.toml` pins every runtime and tool; `mise.lock` records checksums.
  Never install tools another way. After changing a pin, run `mise lock`.
- `mise install` installs the pinned toolchain; `mise tasks` lists tasks.
- `mise run setup` installs project dependencies and hooks.
- `mise run build` builds `./bin/domjudge-tool`.
- `mise run build-css` rebuilds `./internal/webui/assets/tailwind.css`.
- `mise run generate-templates` regenerates templ Go files from `*.templ` sources.
- `mise run user-guide-check` checks paired manuals and their local references.
- `mise run test` runs the test suite; use `mise run test-race` for
  concurrency work.
- `mise run fmt` formats with gofumpt.
- `mise run vet`, `mise run lint`, and `mise run lint-actions` run static
  checks.
- `mise run privacy-check` runs secret and PII gates.
- `mise run check` runs the full local gate; CI runs `mise run ci`.
- Releases publish only from a pushed `vMAJOR.MINOR.PATCH` tag through
  `.github/workflows/release.yml`; see ADR 0017.

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
- `internal/cli/web` wires the `domjudge-tool web` Cobra command and options.
- `internal/webui/server` composes the HTTP server, stores, security middleware,
  and production operation functions.
- `internal/webui/{assets,db,handler,oplog,profile,settings,templates}` implement
  the web UI.
- Library packages return data/errors or use structured `slog`; they do not
  print directly.

## Go Rules

- Go 1.27.3 is the module target. Do not upgrade the module directive casually.
- `goquery` is on v1.13.0 per ADR 0015; do not downgrade or change without an ADR.
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
  generated outputs, run `mise run privacy-check`.

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
cmd/e2e-server/      isolated web UI test server (in-memory data)
internal/            business logic, clients, CLI wiring, web UI
docs/                architecture, conventions, roadmap, ADRs
scripts/             installers, verification, utility scripts
testdata/            sanitized fixtures
.claude/             Claude Code shared settings and hooks
.codex/              Codex project settings
```
