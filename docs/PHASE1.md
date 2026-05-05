# Phase 1 — DOMjudge 7.3.2 CLI Port (TDD)

> Durable team-facing copy of the Phase 1 plan. The active per-PR specs
> live under `docs/phase1/PR-N-<slug>.md` (drafted just-in-time so they
> don't drift before being executed).

## Context

Phase 0 (`c76f017 chore: bootstrap project (phase 0)`) shipped the engineering
baseline: Go 1.23 module skeleton, Cobra-ready stub, just/lefthook/golangci-lint
toolchain, docs tree, ADRs, Claude Code hooks. **No business logic exists yet.**

Phase 1 ports the working Python CLI
(`/path/to/domjudge-tool-cli` 0.3.0) to Go with
**feature parity for DOMjudge 7.3.2 only**. The Python CLI works in
production but has zero automated tests, three known bugs, and one quirky
behaviour the user base now relies on. The migration is the right time to
fix the bugs, lock the quirk with a regression test, and grow real coverage
from zero.

## Collaboration model

- **Codex implements** each PR following strict TDD (red → green → refactor).
- **Claude reviews** every PR against three dimensions:
  **功能正常 (functionality)**, **邊界例外 (edge cases)**, **文件完整
  (documentation completeness)**.
- **No GitHub PRs.** Local feature branches `feat/p1-<slug>` only;
  Claude reviews via `git diff main..feat/p1-<slug>`. Approved branches
  merge into `main` with a Conventional Commit.

## Locked decisions

| Decision | Value |
|----------|-------|
| Methodology | TDD — failing test first, every PR |
| Implementer | Codex |
| Reviewer | Claude |
| Branching | Local `feat/p1-<slug>` branches; **no GitHub PRs** |
| Scoreboard scope | Included in Phase 1 (full parity) |
| HTML fixtures source | **Updated 2026-05-03**: captured from `https://prod-station.example.org` (production DOMjudge, READ-ONLY access). Original plan named `test-station.example.org` test station; superseded after the test station was unavailable. Fixtures sanitized via the 15-rule pipeline in `testdata/web/v7/CAPTURE.md` and verified clean by 8 PII grep gates before commit. Creds out-of-band, never committed. |
| CLI framework | Cobra |
| HTML scraping | goquery |
| Logger | `log/slog` only — no `fmt.Print*` from library code |
| Test framework | stdlib `testing` + `httptest` only — no testify, no ginkgo |
| First-arg rule | `ctx context.Context` first arg on all I/O |

## Architecture — four layers

```
                    ┌─────────────────────┐
                    │  cmd/domjudge-tool  │   thin: version + slog + Execute
                    └──────────┬──────────┘
                               ▼
                    ┌─────────────────────┐
        Layer 4 ──▶ │   internal/cli      │   Cobra command tree
                    └──────────┬──────────┘
                               ▼
        Layer 3 ──▶ ┌──────────────────────────────────────────┐
                    │  importer │ exporter │ email │ scoreboard │
                    └──────────┬───────────────────────────────┘
                               ▼
        Layer 2 ──▶ ┌──────────────────────────────────────────┐
                    │  client/api/v4   │   client/web (Gateway) │
                    │                  │      └── v7/           │
                    └──────────┬───────────────────────────────┘
                               ▼
        Layer 1 ──▶ ┌──────────────────────────────────────────┐
                    │  config │ model │ util/{password,csvio,pathx} │
                    └──────────────────────────────────────────┘
```

Each layer depends only on layers below it. Tests at every layer use
`httptest.Server` or fixtures — never real network.

## TDD anchors — locked golden tests

These three tests exist solely to lock backward compatibility and bug fixes
that span the rewrite. They are written and committed **before** the
production code that satisfies them, so the red→green transition is
auditable in `git log`.

1. **`TestDomServerClient_RoundTrip_Golden`** (`internal/config`)
   Round-trips a synthetic Python-style `domserver.json` golden through the
   Go struct, byte-comparing the marshalled output. Locks: snake_case JSON
   tags, default values, indent style.

   > **Reinterpreted from the original plan**: the user's actual
   > `domserver.json` is hand-edited (mixed indent, missing fields added
   > later). We lock the *Python serializer's canonical output* (4-space
   > indent, Pydantic field order), not the user's specific edited file.

2. **`TestCSVImport_HeadersByteIdentical`** (`internal/importer`)
   Loads the Python template (`templates/csv/import-users-teams.csv`),
   confirms exact column order
   `username,name,email,affiliation,password` and that a UTF-8 BOM is
   stripped from the first column header.

3. **Three Python bug-regression tests**
   - `TestProblemFilter_NoFlags_DoesNotPanic` (locks Python `__init__.py:21`)
   - `TestPathSafe_RejectsTraversalAndOSReservedChars` (locks `_problems.py:32`)
   - `TestSubmissionsDownload_NilFilePath_DoesNotPanic` (locks `submissions.py:58`)

## TDD anchors — preserve Python quirk bit-for-bit

`services/web/v7.py:161` has an inverted filter condition that the user
base now depends on. Lock with:

- **`TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk`**
  (`internal/client/web/v7`). Includes a `// QUIRK:` comment in the test
  body pointing at `docs/ROADMAP.md` Phase 1 quirks section so a future
  contributor doesn't "fix" it.

## TDD discipline applied to every PR

For every public function or method, in this order:

1. **RED** — write a failing table-driven test with cases for happy path,
   each documented error path, nil/empty optionals, context cancellation
   where relevant.
2. **GREEN** — minimum code to pass. Don't add features that aren't tested.
3. **REFACTOR** — extract helpers, improve names, while tests stay green.
4. **DOC** — every exported symbol gets a doc comment; package gets a
   `// Package x ...` comment.
5. **`just check` clean** before commit.

## PR plan — eight PRs, ordered

| PR | Scope | Commit |
|----|-------|--------|
| 0 | CI workflow + docs/PHASE1.md (deps deferred to first-use) | `build(deps): wire CI for phase 1` (merged) |
| 1 | Foundation layer (`util` + `model` + `config`) | `feat(foundation): util + model + config` (merged) |
| 2 | HTTP transport + API v4 client | `feat(api/v4): http transport + typed json client` (merged) |
| 3 | Web Gateway interface + v7 implementation | `feat(web/v7): gateway interface + scraper` (merged) |
| 4 | Importer + Exporter use-cases | `feat(usecase): importer + exporter` |
| 5 | Email use-case | `feat(email): smtp sender + template loader` |
| 6 | CLI root + general + users | `feat(cli): root + general + users subtree` |
| 7 | CLI problems + submissions + emails + scoreboard | `feat(cli): problems + submissions + emails + scoreboard` |
| 8 | Goreleaser + ADRs + README polish | `chore(release): goreleaser + adrs + readme polish` |

Per-PR specs land at `docs/phase1/PR-N-<slug>.md` just before each PR is
implemented.

## Deviations from the original plan (recorded in PR-0)

1. **Cobra + goquery deferred to first-use PRs.** The original plan put
   `go get cobra` and `go get goquery` in PR-0, but `go mod tidy` prunes
   unused deps. Cobra lands when `internal/cli` first imports it (PR-1 for
   the foundation, PR-6 for the actual command tree); goquery lands in
   PR-3 with the v7 scraper. Versions will be pinned to be Go 1.23
   compatible.

2. **Go directive stays at 1.23.** Latest cobra (v1.10.2) and goquery
   (v1.12.0) both bumped to Go 1.25, so they will be pinned to
   1.23-compatible versions when first imported (e.g. cobra v1.8.x,
   goquery v1.10.3).

3. **`TestDomServerClient_RoundTrip_PythonGolden` reinterpreted as
   `_RoundTrip_Golden`.** The original plan said byte-compare against the
   user's `/path/to/domjudge-tool-cli/domserver.json`.
   That file is hand-edited (mixed indent, predates the
   `affiliation_country` field) so it isn't a valid golden for the Python
   serializer's canonical output. The Go test instead targets a **synthetic
   Python-style golden** committed to `testdata/config/`, sanitized of
   real credentials. The user's actual file is never copied into the
   repo (it contains real production credentials).

4. **CI uses `gofumpt -l` and individual steps, not `just check`.**
   `just fmt` is mutating (`gofumpt -w`); CI must not write back to the
   workspace. Steps run individually with the same gates.

## Phase 1 — Definition of Done

Run from project root:

1. `git log --oneline main` shows all 8 PR merge commits.
2. CI green on `main`.
3. `go test ./... -cover` reports parsers ≥ 80 %, clients ≥ 70 %.
4. `./bin/domjudge-tool gen-config ...` produces a `domserver.json`
   byte-identical to the Python tool given identical flags.
5. `./bin/domjudge-tool import-users-teams-example` writes a CSV
   byte-identical to Python `templates/csv/import-users-teams.csv`.
6. The three Python bug-regression tests pass.
7. The Python quirk lock test passes.
8. ADRs 0002, 0003, 0004, 0005 exist.
9. `goreleaser release --snapshot --clean` produces dist artifacts for
   darwin/linux × amd64/arm64.
10. Local tag `v1.0.0-rc1` exists.

## Cross-cutting rules (apply to every PR)

- **Errors:** wrap with `fmt.Errorf("doing X: %w", err)`. Sentinel errors
  per package (`var ErrXxx = errors.New(...)`).
- **Context:** every I/O method takes `ctx context.Context` as the **first**
  argument.
- **Logging:** `log/slog` only. **No `fmt.Print*` from library code.**
  CLI handlers (`internal/cli`) print user-facing output via
  `cmd.OutOrStdout()`.
- **`init()`:** only for registry wiring. Never for side effects.
- **Tests:** stdlib `testing` + `httptest` only. Table-driven with
  `t.Run(tc.name, ...)`. Fixtures in
  `testdata/<package>/<case>.{html,json,csv}`. **Never hit a real DOMjudge.**
- **Imports:** grouped by `gci` — stdlib → third-party → `github.com/ntub/...`.
- **Public API stability:** CLI subcommand names + flag names match Python
  bit-for-bit. Add Go-idiomatic aliases via `cobra.Command.Aliases`, never
  rename.

## Out of scope — Phase 1

- DOMjudge 8.x web form differences → Phase 2.
- Wails / PocketBase / React desktop app → Phase 3.
- HTTP retry / backoff middleware → Phase 1.5 has been re-allocated to
  the API-first migration; retry/backoff is deferred to Phase 1.6 or an
  on-demand patch when ops actually observe transient failures.
- Concurrent batch operations beyond what Python does sequentially.
- Excel (`.xlsx`) export — CSV/JSON cover the cases.

## HTML fixture capture protocol — completed in PR-3

Done 2026-05-03. Outcome:

1. Capture target: `https://prod-station.example.org` (production DOMjudge
   DOMjudge 7.3.4). READ-ONLY: only GET, plus a single POST to `/login`
   for session establishment. Admin credentials held out-of-band, never
   committed.
2. Eleven endpoints captured under `tmp/raw_v7/` (gitignored). The raw
   responses contained student names, IDs, emails, IPs, and live CSRF
   tokens.
3. Sanitization: 15 rules applied via an auditable Python script (CSRF
   tokens → `TEST_CSRF_TOKEN`, FQDN → `example.org`, IPs → RFC 5737
   TEST-NET-1, CJK → `Sample###`, student IDs → `student###`, school
   course codes → `course###`, contest titles → `ContestX`, and so on).
   Full rule list at `testdata/web/v7/CAPTURE.md`.
4. Verification: 8 grep gates re-ran against `testdata/web/v7/` before
   commit; all returned zero hits.
5. `tmp/` (raw HTML + sanitizer + cookie jar) deleted post-commit.

Future PRs that need additional fixtures repeat the same protocol.

## Open risks tracked

| Risk | Mitigation |
|------|------------|
| Test station HTML drifts from production | `CAPTURE.md` records build; PR-3 review verifies parser tolerates known v7.3.4 minor variant. |
| Codex hand-rolls a non-obvious decision without ADR | Review checklist gates: ADR required for non-obvious decisions. |
| TDD discipline slips into "tests-after" | Commit history audit: each PR must have a `test(...): add failing tests` commit BEFORE the corresponding `feat(...)`. |
| Backward-compat regression slips through | TDD anchors are byte-comparison golden tests against Python output. CI fails on any drift. |
| Quirk gets "fixed" by Codex | `// QUIRK:` comment + named locked test + ROADMAP cross-reference. |

## Status note — superseded by Phase 1.5

Phase 1.5 (API-first refactor with scraper fallback) shipped on
2026-05-05 as `v1.1.0`. This file is preserved as Phase 1 history and
is no longer the source of truth for the operations migrated in
Phase 1.5; see `docs/phase1.5/SPEC.md`, `docs/phase1.5/PLAN.md`, and
ADR 0006 for the current architecture. The TDD discipline, commit
conventions, and Python-parity locks documented above remain in force.
