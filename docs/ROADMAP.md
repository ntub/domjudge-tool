# Roadmap

`domjudge-tool` is being rewritten from a Python CLI
(`ntub-inc/domjudge-tool-cli`, version 0.3.0) into a Go-based stack across
four phases. Each phase is a stand-alone milestone.

| Phase | Scope | Tech | Status |
| ----- | ----- | ---- | ------ |
| **0** | Project bootstrap (this repo's foundation) | Git, Just, lefthook, git-cliff, golangci-lint | Done |
| **1** | Port Python CLI → Go CLI, **DOMjudge 7.3.2 only** | Go 1.23+, Cobra, goquery, net/http | **Done** — `v1.0.0-rc1`. See [`phase1/README.md`](phase1/README.md). |
| **1.5** | API-first refactor (DOMjudge 7.3.x) + email template tooling + operator UX | Go 1.23+, REST API v4, `net/smtp`, `crypto/rand`, `schollz/progressbar/v3` | **Done** — `v1.4.0` (`v1.1.0` API-first; `v1.1.1` hot-fix; `v1.2.0` email tooling; `v1.2.1` import auto-recovery; `v1.3.0` progress bars; `v1.4.0` scoreboard categories + import report) |
| **2** | Add DOMjudge **8.3.1** support to CLI | Strategy pattern over web scraper | **Done** — `v1.6.0` (`v1.5.0` 8.3.1 support; `v1.6.0` `import-problems --upsert`) |
| **3** | Cross-platform desktop app | PocketBase + Wails + Vite + React 19 + TS strict + Tailwind v4 + shadcn/ui + TanStack | Planned |

## Phase 0 — Bootstrap (done)

Foundation only — no business logic. The deliverables are tracked in this
file's commit history. See `README.md` for what's in the repo today.

### Out of scope for Phase 0 (deferred — don't lose these)

- CI workflow (`.github/workflows/ci.yml`) — added at the start of Phase 1
  once there's actual code to test.
- Goreleaser config — added at the end of Phase 1 when we cut the first
  release binary.
- Cobra dependency — added in Phase 1 with the first real command.
- HTML fixtures — captured during Phase 1 from a real 7.3.2 instance.
- `.tool-versions` (mise/asdf) — only if/when the team grows beyond one
  developer.
- Dependabot / Renovate — Phase 1+.
- `SECURITY.md`, `CODE_OF_CONDUCT.md` — only if/when the project goes public.

## Phase 1 — DOMjudge 7.3.2 CLI port (done)

Goal: feature parity with the Python CLI, single supported server version.

The phase plan and per-PR status live in [`PHASE1.md`](PHASE1.md) and
[`phase1/README.md`](phase1/README.md). PR-0 through PR-8 are merged; the
Phase 1 deliverable is `v1.0.0-rc1`.

### Backward compatibility — must preserve bit-for-bit

Carried from `MIGRATION_TO_GOLANG.md` §12 of the Python source repo.

- **`domserver.json`** — field names, types, and defaults stay exactly the
  same. Use snake_case JSON tags on Go structs.
- **`import-users-teams.csv`** — existing columns keep their order and
  names; optional columns such as `category_id` may be appended only when
  old CSVs remain accepted.
- **CLI subcommand and flag names** — keep current names; add Go-idiomatic
  aliases via `cobra.Command.Aliases`.
- **`rm-teams-and-users` default-protect list** — `admin`, `judgehost`, and
  the username from the active config.
- **`gen_password` defaults** — length 10, charset `ascii_letters + digits`.
- **BOM handling on CSV input** — strip a leading UTF-8 BOM (`﻿`) from the
  first column header.

### Bugs in the Python version — fix on the way over

Don't carry these forward. Reference paths are in the Python source repo.

- `commands/problems/__init__.py:21` — `len(None)` when neither `--exclude`
  nor `--only` is provided.
- `commands/problems/_problems.py:32` — `replace(" ", "-")` only sanitizes
  spaces; tighten to a real path-safe filter (also fixes a potential
  directory traversal).
- `services/api/v4/submissions.py:58` — `isdir(None)` crash when `file_path`
  is `nil`.

### Quirks to preserve bit-for-bit

These look like bugs but the existing user base depends on them.

- `services/web/v7.py:161` — the `if lower_name not in include or lower_name
  in exclude` filter looks inverted but is what users currently rely on.
  **Keep behaviour bit-for-bit.** Document the quirk in the Go scraper
  comment. Do not silently fix unless we add a regression test that locks
  the old behaviour for a deprecation window.

### `web.Gateway` interface (Phase 1, used in Phase 2)

Introduce a small interface in `internal/client/web` that captures the
shaped operations the CLI needs (create team, delete team, list
affiliations, …). Only ship a `v7` implementation in Phase 1; the interface
is in place so Phase 2 can plug in `v8` next to it without callers
changing.

> **Phase 2 P2-4 update:** the `web.Gateway` interface remains the
> version-agnostic contract. DOMjudge 8.3.1 still needs scraper-backed
> create/update/email/password/affiliation flows because the bulk REST import
> path drops email and update is not usable through `PUT /api/v4/users/{id}`.
> `web/v8` therefore implements those methods as real form flows, not
> unsupported stubs. See ADR 0006
> (`docs/adr/0006-api-first-with-scraper-fallback.md`) and ADR 0008.

## Phase 2 — DOMjudge 8.3.1 support

Goal: support both 7.3.2 and 8.3.1 servers from one binary, selected by the
existing config version field.

### Web-form differences from v7

(Captured from the Python source so they aren't lost.)

- `team[addUserForTeam]` enum changed: `"1"` → `"create-new-user"`.
- New-user field renamed:
  `team[users][0][username]` → `team[newUsername]`.
- Fresh team default field: `team[penalty] = "0"`; updates preserve the edit
  form snapshot value.
- Team delete flow: GET-and-click row link → POST to
  `/jury/teams/{id}/delete?_=<timestamp>`.
- Affiliation comments field renamed: `comments` → `internalcomments`.
- Affiliation list column offsets shifted by one.

> **Phase 2 P2-4 update:** live DOMjudge 8.3.1 write/readback showed that the
> bulk `accounts.json` REST import path drops email, so v8
> `import-users-teams` uses the `web/v8` scraper for
> create/update/email/password/affiliation. Delete remains scraper-only
> because 8.3.1 has no REST API DELETE for users/teams.

Future work: single-entity `POST /api/v4/users` on DOMjudge 8.3.1 was
verified to honor email, team linkage, password, and roles; a later Phase 2.x
PR may use it for v8 user create only, while update/delete stay scraper.

### Implementation note

Capture HTML fixtures from a real 8.3.1 instance into `testdata/web/v8/`
**before any Go code is written for v8.** That way the parser is built
against verifiable inputs, not screenshots of a hypothetical page.

## Phase 2 — completion log

| Tag | Date | Scope |
| --- | ---- | ----- |
| `v1.5.0` | 2026-06-04 | DOMjudge 8.3.1 support — one binary selects the v7 or v8 backend from `domserver.json`, adds cross-version API decoding, and completes the v8 jury scraper fallback for importer, delete, and problem-download workflows. |
| `v1.6.0` | 2026-06-20 | `import-problems --upsert` — tool-side, contest-scoped create-or-update reconciliation that indexes `/api/v4/contests/{cid}/problems` and imports each ZIP as a create or an update by package identity (ADR 0011). |

## Phase 3 — Cross-platform desktop app

Goal: a single signed binary per platform that wraps the CLI with a UI and
local state (PocketBase).

### Pre-research checklist

Run these via `ctx7` when Phase 3 starts — Phase 3 leans on libraries with
fast-moving APIs that training data lags hardest on.

- **Wails** v3 (or current LTS) project layout — separate `frontend/` dir
  vs monorepo root, build artifact targets (macOS notarization, Windows
  code signing).
- **PocketBase**: embedded (linked into the Go binary) vs sidecar process —
  confirm Wails embedding pattern works with PB's HTTP server.
- **Tailwind v4** — verify GA status at Phase 3 start; v4 introduced
  CSS-first config + `@theme` (training data may lag).
- **shadcn/ui** — verify Tailwind v4 compatibility track at that time.
- **TanStack** which packages: Query, Router, Table, Form, Store — likely
  Query + Router + Table for this domain.
- **Reuse strategy**: import `internal/` Go packages from Phases 1–2
  directly via Wails bindings; avoid duplicating business logic in TS.

### Local datastore

The local-state engine choice is captured in
[ADR 0012](adr/0012-phase3-local-datastore.md): SQLite via
`modernc.org/sqlite` (pure-Go, cgo-free), pinned to a Go 1.23-compatible
version — `v1.38.0` at proposal time, because later tags raise the `go.mod`
directive to `1.25.0` and would force a module-directive bump. bbolt,
`mattn/go-sqlite3`, PocketBase, and plain JSON were considered. Re-check the
pin and the Go toolchain when Phase 3 starts; the ADR stays `Proposed` until
a Wails v3 + `modernc` build is verified via `ctx7`.

### Frontend stack

The frontend stack choice is captured in
[ADR 0013](adr/0013-phase3-frontend-stack.md): a server-driven Datastar
stack (Wails v3 + an in-process Go HTTP server + templ + Datastar with its
Go SDK + Tailwind v4 standalone CLI + DaisyUI 5), replacing the original
React 19 + TanStack plan. Datastar's signals drop the separate Alpine
dependency, and its SSE model targets the Phase 1.5 batch-progress
commands. htmx (+ Alpine) is the named fallback. The ADR stays `Proposed`
until spikes confirm SSE through the Wails webview, Tailwind v4 rendering
on all target webviews, and acceptable Datastar v1.x stability.

## Phase 1.5 — completion log

| Tag | Date | Scope |
| --- | ---- | ----- |
| `v1.1.0` | 2026-05-05 | API-first refactor with scraper fallback (PRs 1–5). |
| `v1.1.1` | 2026-05-05 | Hot-fix after live validation against the production v7 station. |
| `v1.2.0` | 2026-05-06 | Email template tooling — offline scaffold, validate, and render-to-disk (PR-6); release PR-7. |
| `v1.2.1` | 2026-05-07 | Patch — `import-users-teams` UPDATE auto-recovers when the v7 API list snapshot returns `team_id == nil` (PR-8). |
| `v1.3.0` | 2026-05-07 | Operator UX — per-row stderr progress bar on the seven batch commands; persistent `--no-progress` flag; bundled ctx-cancel parity for `render-user-accounts` / `download-scoreboard` (PR-9). |
| `v1.4.0` | 2026-06-03 | Live-validation hardening — `download-scoreboard` matches the Python cell format, spans all team categories with a new `Category` column and `--exclude-hidden` opt-out, and drops the summary row (ADR 0007); `import-users-teams` caches affiliation lookups and returns a structured import report. |
