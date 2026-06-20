# Phase 3 — Specification

> This document is the Phase 3 contract. It defines what the desktop app
> does, which screens exist, how local state is managed, and which constraints
> must hold before a single line of implementation is written. Consult here
> first; supplement, don't guess.

## 1. Goal and Non-Goals

### Goal

Ship a **cross-platform desktop app** (macOS arm64/amd64, Windows
amd64/arm64, Linux amd64) that wraps the existing Go CLI commands with a
native GUI. The app reuses `internal/` packages directly — no business logic
is duplicated in templates or frontend code. The seven batch commands (Phase
1.5) get live SSE-driven progress screens.

### Non-Goals

- No multi-server concurrent operations.
- No multi-user / collaboration features.
- No real-time contest monitoring dashboard.
- No new CLI flags or output file format changes. The CLI and the app ship
  from the same binary; CLI behaviour is preserved bit-for-bit.
- No new DOMjudge API version support (that is Phase 4+).

---

## 2. Screens

| # | Screen | Key Interactions |
|---|--------|-----------------|
| 1 | **Server profiles** | List, add, edit, delete DOMjudge server configs (replaces `domserver.json` file). Default profile indicator. |
| 2 | **Import users & teams** | File picker for CSV, server selector, live per-row SSE progress, structured report on finish. |
| 3 | **Remove teams & users** | Include/exclude filter inputs, dry-run toggle, live SSE progress. |
| 4 | **Export users** | Server selector, format (CSV/JSON), save-to-disk dialog. |
| 5 | **Download problems** | Contest selector, output directory picker, live SSE progress. |
| 6 | **Import problems** | ZIP file pickers (one per problem), contest selector, `--upsert` toggle, live SSE progress. |
| 7 | **Download scoreboard** | Contest selector, `--exclude-hidden` toggle, save-to-disk dialog. |
| 8 | **Download submissions** | Contest selector, optional problem filter, output directory picker, live SSE progress. |
| 9 | **Set password** | Username list, target password field, server selector. |
| 10 | **Email tools** | Sub-screens for scaffold / validate / send / render; SMTP config drawn from settings. |
| 11 | **Operation history** | Table of past operations with timestamp, type, server, row count, status. Filter by type and server; sort by timestamp. Click row for detail (structured report). |
| 12 | **Settings** | SMTP config, theme (Light / Dark / System), default server profile. |

Screens 2, 3, 5, 6, 8 are the **batch progress screens** — the primary
motivation for the Datastar SSE stack (ADR 0013).

---

## 3. Local State

Phase 3 ships queryable local state, which confirms the SQLite decision in
ADR 0012.

| Entity | Storage | Notes |
|--------|---------|-------|
| Server profiles | SQLite | Name, URL, username, password (encrypted at rest?), version, contest\_id, prefer\_scraper, import options. Replaces `domserver.json`. |
| Operation log | SQLite | One row per batch operation: type, server\_id, started\_at, finished\_at, status, summary JSON (row count, errors). |
| Operation detail | SQLite (JSONB col) | Structured import report per row (Phase 1.5 `[]model.CreateUser` or error). |
| Cached snapshots | SQLite | Affiliations, teams, users — populated lazily; TTL-invalidated. |
| UI preferences | SQLite | theme, default\_server\_id. |

The `domserver.json` file is still read on first launch to seed the server
profile table (migration-friendly); it is not written after that.

---

## 4. Module Strategy

The desktop app lives **in the same Go module** as the CLI
(`github.com/ntub/domjudge-tool`). Wails v3 requires Go 1.25, which forces a
module-directive bump from `go 1.23.0` to `go 1.25.0`. This is tracked in
ADR 0014.

The CLI binary remains at `cmd/domjudge-tool/`. The desktop app entry point
lives at `cmd/desktop/` (thin Wails shell). Both share all `internal/`
packages without duplication.

Because the Go module directive moves to 1.25, the `modernc.org/sqlite` pin
in ADR 0012 is revisited: the v1.38.0 constraint (chosen to stay at Go 1.23)
no longer applies. ADR 0012 is updated when ADR 0014 is accepted.

---

## 5. Technology Stack

The full stack decision is captured in ADR 0013. Summary:

| Component | Choice | Status |
|-----------|--------|--------|
| Native shell | Wails v3 | Spike required |
| HTTP server | `net/http` (or `chi`) in-process | Spike required |
| HTML templating | templ | Spike required |
| Reactivity + SSE | Datastar + Go SDK | Spike required; fallback htmx+Alpine |
| CSS | Tailwind v4 standalone CLI + DaisyUI 5 | Spike required |
| Local DB | SQLite via `modernc.org/sqlite` | Spike required (build under Wails) |

Both ADR 0012 and ADR 0013 remain **Proposed** until the spikes in §6
pass.

---

## 6. Spike Sequence (gates implementation)

Spikes run in `tmp/phase3-spike/` (git-ignored), not in the main module.
They are throwaway code, not subject to TDD.

| # | Spike | Gate | Fail action |
|---|-------|------|-------------|
| S-1 | Wails v3 bare window builds and opens on macOS arm64 | Confirms toolchain; resolves ADR 0014 | Block Phase 3 |
| S-2 | In-process `net/http` localhost listener reachable from Wails webview | Confirms HTTP transport | Try `AssetsHandler` path |
| S-3 | SSE (`text/event-stream`) through the localhost listener received by `EventSource` in the webview | **Make-or-break for Datastar** | Fall back to htmx+Alpine; supersede ADR 0013 |
| S-4 | templ renders a page served by the in-process server | Low risk | Inline template strings |
| S-5 | Tailwind v4 + DaisyUI renders correctly in the macOS WebKit webview | CSS compat | Fall back to Tailwind v3.4 |
| S-6 | `modernc.org/sqlite` (latest non-retracted ≥ v1.38.0 at Go 1.25) builds inside a Wails project | DB compat | Try bbolt |

All six spikes pass → flip ADR 0012 and ADR 0013 to **Accepted** →
begin TDD feature implementation.

---

## 7. Directory Layout (target)

```text
cmd/desktop/               Wails app entry point (thin shell)
cmd/domjudge-tool/         CLI entry point (unchanged)
internal/                  Shared business logic (unchanged packages)
internal/desktop/          Desktop-specific use cases (DB, server profiles, op log)
internal/desktop/db/       SQLite schema and migrations
internal/desktop/handler/  HTTP handlers (templ + Datastar)
internal/desktop/templates/ templ templates
frontend/                  Tailwind + DaisyUI static assets (no Node)
build/                     Wails platform packaging
```

---

## 8. Compatibility Locks

All Phase 1–2 compatibility locks (§ CLAUDE.md and ROADMAP.md) remain
in force. The CLI subcommand names, flag names, `domserver.json` field
names, and output file names do not change.

The desktop app ships under a different binary name (`domjudge-tool-app` or
the Wails-packaged `.app`/`.exe`); it does not replace the CLI binary.

---

## 9. Open Questions

| # | Question | Resolution needed by |
|---|----------|---------------------|
| Q1 | Credential encryption for stored server passwords? OS keychain vs AES-GCM in SQLite? | Before S-1 implementation; for now assume plaintext + file permissions. |
| Q2 | Single-window vs multi-window? (tab per batch run) | After S-3 spike passes. |
| Q3 | Code-signing and notarization for macOS distribution? | Before first public release; out of scope for MVP spikes. |
| Q4 | `chi` vs `net/http` for the in-process router? | During S-2 spike; `net/http` is preferred to avoid an extra dependency. |
