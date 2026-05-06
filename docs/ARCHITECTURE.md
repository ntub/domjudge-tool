# Architecture

This document describes the layered structure of `domjudge-tool` and the role
of each top-level package. It is the map every contributor (human or AI)
should consult before adding a new file.

## Goals

- **Separation of concerns.** The CLI front-end, the configuration store, the
  domain model, the protocol clients, and the import/export pipelines each
  live in their own packages and depend in one direction only.
- **Testability without a real DOMjudge.** All I/O (HTTP, filesystem, email)
  is reachable through narrow interfaces so unit tests can substitute fakes.
- **Forward compatibility with DOMjudge versions.** A `web.Gateway`-style
  abstraction lets us plug in a new scraper for a future server without
  touching callers (Phase 2 ships v8 alongside v7).
- **Reuse from the desktop app.** Phase 3 (Wails + React) consumes the same
  `internal/` packages directly through Wails bindings — no business logic
  is duplicated in TypeScript.

## Layers

```
cmd/domjudge-tool/         entry point — thin shell; flag parsing only
        │
        ▼
internal/cli/              cobra command tree; turns args into use-cases
        │
        ▼
internal/{importer, exporter, email,         use-case orchestration
          scoreboard}                          (importer.Router picks
                                                API vs scraper per op)
        │
        ▼
internal/client/api/v4     REST client for DOMjudge JSON API
                            (12 sub-resources incl. Imports,
                             Organizations, Groups, Contests,
                             Scoreboard — added in Phase 1.5)
internal/client/web        HTML scraper Gateway (v7 in Phase 1, v8 in Phase 2)
                            — 5 active methods + 6 deprecated since v1.1.0
        │
        ▼
internal/client/httpx      shared HTTP transports — two parallel clients:
                            • Client (Basic Auth, no redirects) for api/v4
                            • SessionClient (cookie jar, follows redirects,
                              path-relative resolution) for web/v7
        │
        ▼
internal/model/            domain types (Team, Affiliation, Problem, …)
internal/config/           domserver.json read/write
internal/util/             shared helpers (passwords, BOM-safe CSV, …)
```

Dependencies always flow downward in this diagram. `model/`, `config/`, and
`util/` are leaf packages and must not import any of the layers above them.
`client/httpx` is the *only* package allowed to construct an `*http.Client`
directly; both the JSON and HTML clients funnel their requests through it
so the auth header, TLS toggle, and redirect policy are configured in one
place.

## Package responsibilities

| Package                    | Role                                                                 |
| -------------------------- | -------------------------------------------------------------------- |
| `cmd/domjudge-tool`        | `main()`. Wires version info, hands off to `internal/cli`.           |
| `internal/cli`             | Cobra commands; flag parsing; printing.                              |
| `internal/config`          | Loading, saving, and migrating `domserver.json`.                     |
| `internal/model`           | Plain data types shared across packages. No I/O.                     |
| `internal/client/httpx`    | Shared HTTP transports. `Client` (Basic Auth, no redirects) used by `client/api/v4`; `SessionClient` (cookie jar, follows redirects, BaseURL-relative path resolution) used by `client/web/v7`. Both honour optional TLS-skip and configurable timeout. |
| `internal/client/api/v4`   | Typed wrapper around the DOMjudge v4 REST API. Sub-resources: general, users, teams, problems, submissions, judgements, judgement-types (Phase 1) plus `ImportsResource` (bulk-import multipart POSTs to `/users/{groups,organizations,teams,accounts}`, `/contests`, `/contests/{cid}/problems`), `OrganizationsResource`, `GroupsResource`, `ContestsResource`, `ScoreboardResource` (Phase 1.5). Multipart bodies are built via the package-private `postMultipart` helper. |
| `internal/client/web`      | Version-agnostic `Gateway` interface (11 ops total). Phase 1.5 marks 6 as `// Deprecated:` (`GetAffiliations`, `GetAffiliation`, `CreateAffiliation`, `CreateTeamAndUser`, `UpdateTeam`, `SetUserPassword`) — implementations stay in `web/v7` for `cfg.PreferScraper` rollback but no caller drives them in the default code path. The remaining 5 are active: `Login`, `GetProblems`, `Download` (problem ZIP), `DeleteUsers`, `DeleteTeams`. Plus shared `ParseFormFields` helper and gateway-level sentinel errors. Version-specific scrapers live in subpackages — `web/v7` in Phase 1, `web/v8` in Phase 2 (5 active methods only). |
| `internal/importer`        | Use-case functions plus the Phase 1.5 router. `TemplateCSV` writes the `import-users-teams.csv` template (byte-identical to the Python golden). `ImportUsersTeams` reconciles a CSV against the live API user list and drives `Router` (added Phase 1.5) which dispatches each row to either `internal/client/api/v4.ImportsResource` (default) or `web.Gateway` (when `cfg.PreferScraper=true` or the row needs a feature the API can't express, e.g. role-aware password). Returns `[]model.CreateUser` with passwords populated. `DeleteTeamsAndUsers` lists API users, applies the default-protect list (`admin`, `judgehost`, config username) plus caller-supplied include/exclude filters, then calls `gw.DeleteUsers` followed by `gw.DeleteTeams` (always scraper — no API equivalent). The router emits `slog` decisions (`router decision op=… path=… reason=…`) for incident triage. |
| `internal/scoreboard`      | `Download` (Phase 1.5: drives `api/v4` `Scoreboard.Get` + `Contests.Get` + `Teams.List` + `Organizations.List`) and `Build` (Phase 1.5: joins those four JSON responses into a CSV row stream byte-identical to v1.0.0-rc1's HTML-parser output). The legacy HTML parser (`parse.go`) stays compiled and tested as a fallback path but is no longer wired into `Download`. |
| `internal/exporter`        | `Users([]model.User, Format, io.Writer) error` and `CreateUsers([]model.CreateUser, Format, io.Writer) error` — serialise user data to CSV or JSON. `Format` is a string constant (`"csv"` / `"json"`); both functions emit `[]` for nil slices and treat unknown formats as errors. Used by the CLI layer (PR-6) to write `export_users.*` and `import-users-teams-out.csv`. |
| `internal/email`           | `LoadTemplate(dir)` reads `subject.txt` + `body.html` (HTML preferred over `body.txt`); `(*Template).Render(vars)` applies `os.Expand` variable substitution and unconditionally Q-encodes the subject. `Dial(ctx, DialOptions)` opens a plain or TLS SMTP connection, sends EHLO, and performs `PlainAuth` when credentials are non-empty (STARTTLS not implemented — Python parity). `SendUserAccounts(ctx, Sender, *Template, []model.CreateUser, *config.DomServerClient, SendOptions)` iterates the user list: abort-on-first-error; recipient resolved from `user.Email` or falls back to `{username}@{domain}` when `Email` is nil; builds complete RFC 5322 messages with explicit `Date:` and `Message-ID:` headers, base64-encoded body. `BuildEML(*Template, model.CreateUser, *config.DomServerClient, SendOptions) ([]byte, string, error)` extracts the message-construction half of `SendUserAccounts` so the offline `render-user-accounts` CLI can emit per-recipient `.eml` files without touching SMTP. `WriteTemplateScaffold(dir string, force bool) error` writes a runnable scaffold containing all six supported placeholders; collisions raise the sentinel `ErrTemplateDirExists` unless `force=true`. |
| `internal/util`            | Cross-cutting utilities (random passwords, BOM-safe readers, etc.).  |
| `internal/util/progress`   | Optional `Reporter` interface (`Start` / `Tick` / `Finish`) plus `New` and `Nop` constructors. The visual implementation wraps `github.com/schollz/progressbar/v3`; the package adds a uniform TTY / `NO_COLOR` / `TERM=dumb` / `--no-progress` gate so call sites don't re-implement it. Library packages accept a `Reporter` field on their option structs but never construct one — only `internal/cli` root PreRunE does, via `progress.New(cmd.ErrOrStderr(), …)`. Added in Phase 1.5 / v1.3.0. |

## The `web.Gateway` abstraction (Phase 1+)

`internal/client/web/gateway.go` declares the 11-method `Gateway`
interface that captures every shaped operation the CLI needs from the
HTML UI. `internal/client/web/v7.Client` is the only Phase 1
implementation; Phase 2 will add `internal/client/web/v8.Client` beside
it without touching callers — both will satisfy the same interface.

**Phase 1.5 (since v1.1.0):** the interface still lists 11 methods,
but only 5 are wired into default callers — `Login`, `GetProblems`,
`Download` (problem ZIP), `DeleteUsers`, and `DeleteTeams`. The other
6 (`GetAffiliations`, `GetAffiliation`, `CreateAffiliation`,
`CreateTeamAndUser`, `UpdateTeam`, `SetUserPassword`) are marked
`// Deprecated:` on the interface and on `web/v7`'s implementations;
they remain compiled and tested so `cfg.PreferScraper=true` continues
to route everything through the scraper, but no caller drives them
when the router is in its default state. Phase 2's `web/v8` only needs
to implement the 5 active methods.

**Lifecycle**: callers MUST call `Login(ctx)` before any other method.
Phase 1 implementations do NOT auto-relogin on session expiry — a 302
to `/login` surfaces as `web.ErrLoginFailed` and the caller decides
whether to retry.

**Contest scoping**: `v7.Options.ContestID` is a string (empty = no
contest). The v7 implementation translates it into a `domjudge_cid`
cookie on the session jar at construction time. v8 may translate the
same field into path scoping (`/contests/{cid}/jury/...`); callers see
`web.Gateway` and don't depend on the mechanism.

**Locked Python QUIRK**: `v7.Client.DeleteUsers` and `DeleteTeams`
preserve the inverted-filter behaviour at Python `services/web/v7.py:161`
and `:188` — an empty `include` argument short-circuits to "skip every
row" so neither method posts anything. Two named regression tests
(`TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk` and the team
variant) and a `// QUIRK:` comment cross-reference `docs/ROADMAP.md`
§"Quirks to preserve bit-for-bit". **Do not "fix" without an ADR.**

> See `docs/ROADMAP.md` for the v7 → v8 form-field deltas captured ahead
> of time.

## What lives in `cmd/`

`cmd/domjudge-tool/main.go` is **deliberately tiny**: build version,
`os.Args`, then call into `internal/cli`. No business logic. Adding a new
flag means editing `internal/cli`, not `cmd/`.

## Generated artifacts and configuration files

These names are user-facing and **must remain stable** unless ROADMAP says
otherwise:

- `domserver.json` — config file; field names are snake_case (see
  ROADMAP Phase 1 notes).
- `import-users-teams.csv` / `import-users-teams-out.csv` — column order is
  fixed; see Phase 1 spec.
- `export_problems/` — directory layout of exported problems is the same as
  the Python CLI.

## Cross-references

- Code style: `CONVENTIONS.md`
- Test layout: `TESTING.md`
- Phase plan: `ROADMAP.md`
- ADRs: `adr/`
