# 0006. API-first with scraper fallback (Phase 1.5)

- **Status:** Accepted
- **Date:** 2026-05-05
- **Deciders:** @project-maintainer
- Accepted on 2026-05-05 with the v1.1.0 release (Phase 1.5 PR-5).

## Context

Phase 1（v1.0.0-rc1）shipped a working Go port of `domjudge-tool-cli`
that drives DOMjudge 7.3.x via two transports:

- `internal/client/api/v4/` — typed read-only client over the REST API
  (`/api/v4/...`). Used for listing users / submissions / problems /
  judgements.
- `internal/client/web/v7/` — `goquery`-backed scraper over the jury
  HTML UI. Used for everything that mutates state (create/update team,
  set password, delete users/teams, create affiliation) and for the few
  read flows that don't have an API equivalent (problem ZIP download,
  public scoreboard).

The user has asked Phase 1.5 to **switch internal operations from
scraping to the native DOMjudge API "where possible"**. The reasons
are stated in the user's request: scraping is "unreliable" — it breaks
on HTML drift between minor versions, fails silently when forms add new
fields, and forces every state-changing op to navigate a CSRF dance.

Before deciding, we ran a live probe against
`https://test-station.example.org` (DOMjudge 7.3.4, the user's test
station). The findings — recorded in `docs/phase1.5/SPEC.md` §2 — are:

1. **The OpenAPI 2.0 spec at `/api/doc.json` lists 60 operations.**
   Only `POST` is supported for write operations, and only as bulk
   imports (TSV / JSON / YAML / ZIP). There is **no DELETE, PUT, or
   PATCH** on user / team / organization / problem resources — every
   variant tested returned `HTTP 405 Method Not Allowed`.
2. **There is no API for problem ZIP export.** `GET
   /api/v4/contests/{cid}/problems/{id}` returns metadata JSON only;
   the `.zip`, `/zip`, `/statement` variants return 404. ZIP download
   stays on the scraper's `/jury/problems/{id}/export` route.
3. **The scoreboard endpoint does not content-negotiate CSV.**
   `Accept: text/csv` returns JSON regardless. CSV must be reconstructed
   on the client side.

This rules out the naive "replace everything" strategy. The decision is
not "API or scraper", it is "**which operations move, and how does the
remainder coexist**".

## Decision

We adopt an **API-first with scraper fallback** architecture for Phase
1.5:

1. **Move every operation to the API where the API is sufficient.** The
   first-pass list (`SPEC.md` §3): `version`, `export-users`,
   `download-submissions` (already on API); `download-scoreboard`
   (move now); `import-users-teams` create-flow + per-user password
   update (move now, conditional on PR-0 idempotency findings).
2. **Keep `internal/client/web/v7/` and the `web.Gateway` interface for
   ops the API cannot do.** Specifically: `DeleteUsers`, `DeleteTeams`,
   `Download` (problem ZIP), and (conditionally) the per-row
   `UpdateUserEmail` / `SetUserPassword(roles!=nil)` paths whose CCS
   `accounts.tsv` format does not carry email / role columns.
3. **Introduce a router in `internal/importer`** (`importer.Router`)
   that decides per-call which transport to use, surfacing the choice
   via `slog` and via a config flag (`cfg.PreferScraper`) for ops to
   force the legacy path during incidents.
4. **Document the constraint with this ADR** so a future contributor
   doesn't try (again) to make `rm-teams-and-users` go through the API
   and then waste a day on it. The 405 result is locked-in for DOMjudge
   7.3.x and will not change.
5. **Treat the same constraint as a Phase 2 design input.** DOMjudge 8
   added some endpoints; Phase 2 will re-evaluate. Until then, the
   scraper code in `web/v7/` and the corresponding Python QUIRK locks
   stay in place exactly as today — Phase 1.5 does not delete them.

### Alternatives considered

- **Option A: Replace scraper entirely with API.** Rejected. The 405
  responses on user/team DELETE are a hard server-side limit; we
  cannot delete via the API on 7.3.x at all. There is also no API
  endpoint that exports a problem ZIP. A "full API" path does not
  exist on this server version.

- **Option B: Keep scraper as primary, expose API only for read.**
  Rejected. This is roughly Phase 1's current state and is the situation
  the user explicitly asked us to change. It also leaves the system
  exposed to the same HTML-drift risks Phase 1 has been hit by.

- **Option C: One unified gateway with "API or scraper" baked into each
  method.** Rejected. This collapses the visibility of which transport
  is in use into a single layer, which makes incident response harder
  ("which one of the two paths failed?") and prevents Phase 2 from
  re-using the same router for v8. The chosen architecture keeps
  `web.Gateway` and `api/v4` as orthogonal Layer-2 modules and puts the
  decision in a Layer-3 router (`internal/importer.Router`) — the same
  layering Phase 1 already uses for everything else.

- **Option D: Wait for DOMjudge 8 in Phase 2 and skip Phase 1.5.**
  Rejected. (a) Phase 2 is far enough out that the `domjudge-tool` user
  is exposed to scraping fragility for months. (b) The router /
  `ImportsResource` are required for Phase 2 anyway; building them
  against 7.3.x first means Phase 2 reduces from "build api + web/v8 +
  router" to "add web/v8 to an already-tested router".

## Consequences

### Positive

- The most fragile operations (HTML form scraping for create / update
  / set-password) move to a documented contract. HTML drift no longer
  silently breaks state-changing flows.
- Phase 2 (DOMjudge 8) becomes ~60% smaller: only the ops still on the
  scraper need a `web/v8` implementation.
- Adds clear, log-visible attribution per op for incident triage
  (`router decision: op=set-password path=api`).
- The `cfg.PreferScraper` escape hatch lets ops roll back to legacy
  behaviour without a redeploy if the new path misbehaves.

### Negative

- The codebase keeps two transports for the foreseeable future. We are
  not retiring `web/v7/` in this phase. Reviewers must understand both
  for any importer-related change.
- The `web.Gateway` interface keeps 11 methods (Phase 1.5 marks 6 of
  them `// Deprecated:` but does not delete them). Future contributors
  can mistake them for "still in use"; doc comments must be explicit.
- Per-user `UpdateUserEmail` and `SetUserPassword(roles!=nil)` cannot
  use the bulk import endpoint because CCS `accounts.tsv` has no
  columns for them. These remain on the scraper indefinitely on v7. We
  accept this and reflect it in the router's decision table.
- Tests and fixtures double in count (HTML + JSON / multipart). PR-0's
  `testdata/api/v4/` capture protocol mirrors the existing
  `testdata/web/v7/CAPTURE.md` pipeline.

### v1.1.1 amendment (2026-05-05) — accounts.tsv linkage on v7

Live validation against the production DOMjudge 7.3.4 station revealed that
the `OpCreateTeamAndUser` API path silently corrupts data even when
every individual TSV upload succeeds. The CCS `accounts.tsv` schema
has no `team_id` column. DOMjudge 7.3.x's `importAccountsTsv` resolves
each user's team by running the username through
`/^[^0-9]*0*([0-9]+)$/` and calling `Team::find($n)` — a Doctrine
primary-key lookup, NOT a CCS `teamID` lookup — meaning every new
user is linked to whichever low-id team auto-increment happened to
land at (admin / Example team in our capture).

Because we cannot control the server's auto-increment id and the
spec has no team-id column, there is no in-CLI fix that uses
`accounts.tsv`. The v1.1.1 hot-fix therefore reverts the default
routing of `OpCreateTeamAndUser` back to the scraper while leaving
`apiCreateFlow` and the resource clients in place for Phase 2 (which
will re-evaluate against DOMjudge 8 — the upstream may have grown
explicit team-id support, or accounts.tsv may have been replaced
with a JSON shape).

This amendment does not change the architectural intent of this ADR
(API-first where the API is sufficient). It records that on v7 the
API is **not** sufficient for the create-team-and-user flow, and the
router decision table reflects that.

### Locked decisions and follow-ups

- Python `services/web/v7.py:161` QUIRK lock (the inverted-filter on
  delete) **stays**. It lives in `web/v7/users.go` /
  `web/v7/teams.go`; Phase 1.5 does not move delete to API and
  therefore does not retire the QUIRK. Any future ADR that argues for
  deleting via API must replace this ADR (status: superseded by NNNN).
- `domserver.json` schema additions are limited to `prefer_scraper bool`
  (PR-4) plus an `affiliation_country` default (already exists). No
  other config is allowed in Phase 1.5; new fields require a new ADR.
- Phase 2 re-opens this ADR. If DOMjudge 8 ships DELETE / problem
  export endpoints, the router can flip those routes to API and we
  delete the corresponding `web/v8` methods (i.e. never write them).

## References

- `docs/phase1.5/SPEC.md` — full mapping table and live-server probe
  evidence
- `docs/phase1.5/PLAN.md` — PR sequence and branching strategy
- `docs/phase1.5/CONTRACT.md` — fixture protocol
- `docs/ROADMAP.md` — Phase 1 / 2 / 3 roadmap; this ADR amends Phase 2
  expectations (see PLAN §7)
- `docs/adr/0005-python-quirk-preservation.md` — the prior locked
  QUIRK; this ADR explicitly preserves it
- DOMjudge 7.3.4 admin manual, "Adding contest data programmatically":
  `<install>/doc/manual/html/import.html`
