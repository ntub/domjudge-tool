# domjudge-tool

> A Go-based CLI (and, eventually, desktop app) for managing
> [DOMjudge](https://www.domjudge.org/) contest servers.

![status](https://img.shields.io/badge/status-phase%201%20complete-green) <!-- TODO: replace with real CI badge once a public remote is wired up -->

## Roadmap

| Phase | Scope | Status |
| ----- | ----- | ------ |
| **0** | Project bootstrap | Done |
| **1** | Port Python CLI → Go CLI, **DOMjudge 7.3.2 only** | **Done** — `v1.0.0-rc1` |
| **1.5** | API-first refactor with scraper fallback (DOMjudge 7.3.x) | **Done** — `v1.3.0` |
| **2** | Add **DOMjudge 8.3.1** support to the CLI | Planned |
| **3** | Cross-platform desktop app (PocketBase + Wails + React) | Planned |

See [`docs/ROADMAP.md`](docs/ROADMAP.md) for details.

## Install

**Pre-built binaries** (linux/darwin × amd64/arm64) are available on the
[Releases](https://github.com/ntub/domjudge-tool/releases) page. Download
the archive for your platform, extract it, and place `domjudge-tool` on
your `$PATH`.

**Install from source** (requires Go ≥ 1.23):

```bash
go install github.com/ntub/domjudge-tool/cmd/domjudge-tool@latest
```

## Quick start

```bash
# 1. Write a config file for your DOMjudge server.
domjudge-tool gen-config --host https://judge.example.org \
    --username admin --password secret

# 2. Generate a CSV template for bulk user import.
domjudge-tool import-users-teams-example

# 3. Import users and teams from the filled-in CSV.
domjudge-tool import-users-teams import-users-teams.csv

# 4. Explore the full command tree.
domjudge-tool --help
```

All commands support `--config` (`-c`) to point at a non-default
`domserver.json`, `--verbose` (`-v`) for debug output, and
`--no-progress` *(v1.3.0)* to suppress the per-row progress bar on
batch commands. The bar is also auto-disabled when stderr is not a
TTY, when `NO_COLOR` is set, or when `TERM=dumb` — so piped
invocations like `domjudge-tool import-users-teams ... 2>/dev/null`
stay byte-identical to pre-v1.3.0 output.

## Commands

Run `domjudge-tool --help` to list every command grouped by domain. The
command tree mirrors the Python CLI's; commands added in v1.2.0 are
flagged below. Long-running batch commands (`import-users-teams`,
`send-user-accounts`, `render-user-accounts`, `download-problems`,
`download-submissions`, `download-scoreboard`) print a per-row
progress bar to stderr on a TTY since v1.3.0.

`download-scoreboard` writes columns `Rank, TeamAffiliation, TeamName,
SolvedCount, Score, Category, <problem labels…>`. Since v1.4.0 it spans
every team category (one ranked section per sortorder, so `Rank` restarts
at 1 per section) and records each row's category; pass `--exclude-hidden`
to drop rows whose category group is hidden.

### Email

The email subtree handles credential delivery for newly imported users.
Two of the four commands are offline (no network); `send-user-accounts`
is the SMTP path.

- `email-template-example [-o <dir>] [--force]` *(v1.2.0)* — scaffold a
  template directory containing `subject.txt`, `body.txt`, and
  `body.html` with the six supported placeholders (`$username`,
  `$name`, `$email`, `$affiliation`, `$password`, `$server_host`).
  Round-trips through `email-template-validate` with no edits.
- `email-template-validate <dir>` *(v1.2.0)* — verify a template
  directory loads and references only the fixed placeholder set.
- `render-user-accounts <csv> <template-dir> [-d <output-dir>]
  [--from-email <addr>] [--force]` *(v1.2.0)* — render per-recipient
  RFC 5322 `.eml` files to disk, parallel to `send-user-accounts` but
  never touches SMTP. Useful for review, batch, and mailmerge workflows.
- `send-user-accounts <csv> <template-dir> [SMTP flags]` — mail the
  rendered messages directly via SMTP. Reuses the same template loader
  and message builder as `render-user-accounts`.

For the full flag list of any command, run
`domjudge-tool <command> --help`.

## Phase 1.5 architecture (since v1.1.0)

`domjudge-tool` talks to DOMjudge through **two transports** and picks
the right one per operation:

- **DOMjudge REST API v4** (`internal/client/api/v4`) — used for every
  read flow (`version`, `export-users`, `download-submissions`,
  `download-scoreboard`) and for the bulk-import write flows
  (`import-users-teams` create-flow, `set-password` without
  `--user-roles`).
- **HTML scraper over the jury web UI** (`internal/client/web/v7`) — used
  for the operations the v7 API does not support: deleting users and
  teams (`rm-teams-and-users`, `--delete-existing`), exporting problem
  ZIP files (`download-problems`), and per-row email or role-aware
  password updates (the CCS `accounts.tsv` schema has no email or role
  columns).

The choice is made by `internal/importer.Router` and surfaced in
structured `slog` lines (`router decision op=… path=… reason=…`) for
incident triage. To force every hybrid command back onto the scraper —
e.g. when an API path misbehaves in production — set
`"prefer_scraper": true` in `domserver.json`. The default is `false`
and existing config files load unchanged.

See [`docs/adr/0006-api-first-with-scraper-fallback.md`](docs/adr/0006-api-first-with-scraper-fallback.md)
for the rationale and the full operation-to-transport table.

## Development setup

```bash
# Prereqs: go ≥ 1.23, just, git, optionally lefthook + gofumpt + golangci-lint + trufflehog.
just setup
just build
./bin/domjudge-tool
```

`just setup` runs `go mod download` and installs the lefthook git hooks.
It also prints install hints for any missing optional tools (`lefthook`,
`gofumpt`, `golangci-lint`, `trufflehog`). See [`CONTRIBUTING.md`](CONTRIBUTING.md) for
the full workflow.

Run `just` (no args) to list every available task.

## Documentation

- [`CLAUDE.md`](CLAUDE.md) — shared instructions for Claude Code and Codex.
  `AGENTS.md` is a symlink for Codex compatibility.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — branch / commit / PR workflow.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — layered design and
  package responsibilities.
- [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) — Go style and project
  additions.
- [`docs/TESTING.md`](docs/TESTING.md) — test framework, fixtures,
  naming.
- [`docs/COMMITS.md`](docs/COMMITS.md) — Conventional Commits cheat
  sheet.
- [`docs/ROADMAP.md`](docs/ROADMAP.md) — phase plan and forward-looking
  notes.
- [`docs/DOCUMENTATION.md`](docs/DOCUMENTATION.md) — meta-rules for
  writing docs in this repo.
- [`docs/adr/`](docs/adr/) — Architecture Decision Records.

## License

[MIT](LICENSE) © Jason Xie / NTUB.
