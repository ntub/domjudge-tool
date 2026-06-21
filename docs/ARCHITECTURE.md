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
  abstraction lets us plug in versioned scrapers without touching callers;
  v1.5.0 ships v8 alongside v7.
- **Reuse from the desktop app.** Phase 3 (Wails + React) consumes the same
  `internal/` packages directly through Wails bindings — no business logic
  is duplicated in TypeScript.

## Layers

```
cmd/domjudge-tool/         CLI entry point — thin shell; flag parsing only
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
internal/client/web        HTML scraper Gateway (v7 and v8) — both
                            versioned clients implement the full Gateway
                            surface; v8 scraper writes cover create/update/
                            password/email/affiliation, and delete
                            remains scraper-only
internal/client/web/gatewayfactory
                            config.Version-driven factory for web.Gateway
        │
        ▼
internal/client/httpx      shared HTTP transports — two parallel clients:
                            • Client (Basic Auth, no redirects) for api/v4
                            • SessionClient (cookie jar, follows redirects,
                              path-relative resolution) for web scrapers
        │
        ▼
internal/model/            domain types (Team, Affiliation, Problem, …)
internal/config/           domserver.json read/write
internal/util/             shared helpers (passwords, BOM-safe CSV, …)
```

Phase 3 desktop app adds a separate entry point and package tree (gated
by the `desktop` build tag so the CLI build path is unaffected):

```
cmd/desktop/               Wails v3 shell — CGO_ENABLED=1, -tags desktop
        │
        ▼
internal/desktop/handler/  in-process net/http ServeMux; PRG + SSE handlers
        │
        ▼
internal/desktop/profile/  server-profile CRUD use-case (GORM model + Store)
        │
        ▼
internal/desktop/db/       GORM open helper (glebarez/sqlite, pure-Go driver)

internal/desktop/assets/   pre-built CSS files embedded via //go:embed
internal/desktop/templates/ templ components (layout, screens)
frontend/                  Tailwind v4 build source (input.css + daisyui.css);
                            output goes to internal/desktop/assets/
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
| `internal/client/httpx`    | Shared HTTP transports. `Client` (Basic Auth, no redirects) used by `client/api/v4`; `SessionClient` (cookie jar, follows redirects, BaseURL-relative path resolution) used by versioned web scrapers through `client/web/gatewayfactory`. Both honour optional TLS-skip and configurable timeout. |
| `internal/client/api/v4`   | Typed wrapper around the DOMjudge v4 REST API. Sub-resources: general, users, teams, problems, submissions, judgements, judgement-types (Phase 1) plus `ImportsResource` (bulk-import multipart POSTs to `/users/{groups,organizations,teams,accounts}`, `/contests`, `/contests/{cid}/problems`), `OrganizationsResource`, `GroupsResource`, `ContestsResource`, `ScoreboardResource` (Phase 1.5). Multipart bodies are built via the package-private `postMultipart` helper. |
| `internal/client/web`      | Version-agnostic `Gateway` interface (11 ops total). Phase 1.5 marks 6 as `// Deprecated:` (`GetAffiliations`, `GetAffiliation`, `CreateAffiliation`, `CreateTeamAndUser`, `UpdateTeam`, `SetUserPassword`) because v7's default path is API-first, with those implementations kept in `web/v7` for `cfg.PreferScraper` rollback. `web/v8` implements the full Gateway surface; on v8 the importer routes create, update, password, email, and affiliation writes through the scraper, and delete is scraper-only because DOMjudge 8.3.1 has no REST API DELETE. Shared helpers include `ParseFormFields` and gateway-level sentinel errors. |
| `internal/client/web/gatewayfactory` | Shared factory that reads `config.DomServerClient.Version`, builds the `httpx.SessionClient`, and returns `web/v7` for supported `7.3.x` configs or `web/v8` for supported `8.3.1` configs. It lives outside `internal/client/web` because the factory imports both version packages, while those packages import the base `web` interface. |
| `internal/importer`        | Use-case functions plus the Phase 1.5 router. `TemplateCSV` writes the `import-users-teams.csv` template (byte-identical to the Python golden). `ImportUsersTeams` reconciles a CSV against the live API user list and drives `Router` (added Phase 1.5) which dispatches each row to either `internal/client/api/v4.ImportsResource` or `web.Gateway`: v7 defaults to the API unless `cfg.PreferScraper=true` or the row needs a feature the API can't express, while v8 routes create, update, password, email, and affiliation writes through the scraper because those operations are not available as working v8 REST API paths. Returns `[]model.CreateUser` with passwords populated. `DeleteTeamsAndUsers` lists API users, applies the default-protect list (`admin`, `judgehost`, config username) plus caller-supplied include/exclude filters, then calls `gw.DeleteUsers` followed by `gw.DeleteTeams` (always scraper — v8 REST has no DELETE). The router emits `slog` decisions (`router decision op=… path=… reason=…`) for incident triage. |
| `internal/scoreboard`      | `Download` (Phase 1.5: drives `api/v4` `Scoreboard.Get` + `Contests.Get` + `Teams.List` + `Organizations.List`) and `Build` (Phase 1.5: joins those four JSON responses into a CSV row stream byte-identical to v1.0.0-rc1's HTML-parser output). `WriteCSV(header, rows, w)` — encodes with `UseCRLF=true` (Python default dialect lock; both CLI and desktop must use this function, never re-encode). The legacy HTML parser (`parse.go`) stays compiled and tested as a fallback path but is no longer wired into `Download`. |
| `internal/exporter`        | `Users([]model.User, Format, io.Writer) error` and `CreateUsers([]model.CreateUser, Format, io.Writer) error` — serialise user data to CSV or JSON. `Format` is a string constant (`"csv"` / `"json"`); both functions emit `[]` for nil slices and treat unknown formats as errors. Used by the CLI layer (PR-6) to write `export_users.*` and `import-users-teams-out.csv`. |
| `internal/problems`        | `Download(ctx, gw, filter, outDir, rep) (Result, error)` — lists contest problems via `web.Gateway.GetProblems`, creates outDir, then streams each export ZIP to `${outDir}/${id}_${name}.zip` (spaces → "-", locked filename format from Python `_problems.py:32`). `Result` carries `Downloaded`, `Skipped`, and the absolute `OutDir`. Caller must call `gw.Login` before Download. Both `internal/cli/problems` and `cmd/desktop` call this; the CLI keeps its own `resolveDownloadOnly` (API-based contest-scoping) and `cmd/desktop` has an equivalent `resolveDownloadFilter`. `Import(ctx, ProblemImporter, ProblemLister, version, cid, inputs, upsert, rep) (ImportResult, error)` — uploads each `ImportInput` (path+name+optional problemID) to DOMjudge via the v4 API. When `upsert=true`, queries the contest problem list once and resolves each input against existing problems by identity (external ID from `domjudge-problem.ini`, then filename stem); matched inputs are sent as updates, unmatched ones as creates. Routes the multipart field name by DOMjudge family (`zip[]` on 7.x, `zip` on 8.x). Extracted from `internal/cli/problems` so desktop and CLI share the identical branching logic. `internal/cli/problems` keeps only `collectProblemZIPs` (filesystem scanner) and `isZipPath`. |
| `internal/submissions`     | `Download(ctx, api *apiv4.Client, cid string, opts Options) (int, error)` — fetches every submission's source files for a contest and writes one ZIP per submission under `<OutputPath>/<layout>/<judgement>_<id>.zip`. `Options.Mode` selects folder layout (1=team-first, 2=problem-first default, other=contest_cid). `Options.ProblemIDs`, when non-empty, restricts the download to matching problem IDs (desktop optional filter). `Options.Extract` unzips in place (Python parity default: true). `Options.Reporter` is an optional progress sink. Returns bare count of ZIPs written; aborts on first API error. `internal/cli/submissions` keeps CLI flag wiring and defaults. |
| `internal/email`           | `LoadTemplate(dir)` reads `subject.txt` + `body.html` (HTML preferred over `body.txt`); `(*Template).Render(vars)` applies `os.Expand` variable substitution and unconditionally Q-encodes the subject. `Dial(ctx, DialOptions)` opens a plain or TLS SMTP connection, sends EHLO, and performs `PlainAuth` when credentials are non-empty (STARTTLS not implemented — Python parity). `SendUserAccounts(ctx, Sender, *Template, []model.CreateUser, *config.DomServerClient, SendOptions)` iterates the user list: abort-on-first-error; recipient resolved from `user.Email` or falls back to `{username}@{domain}` when `Email` is nil; builds complete RFC 5322 messages with explicit `Date:` and `Message-ID:` headers, base64-encoded body. `BuildEML(*Template, model.CreateUser, *config.DomServerClient, SendOptions) ([]byte, string, error)` extracts the message-construction half of `SendUserAccounts` so the offline `render-user-accounts` CLI can emit per-recipient `.eml` files without touching SMTP. `WriteTemplateScaffold(dir string, force bool) error` writes a runnable scaffold containing all six supported placeholders; collisions raise the sentinel `ErrTemplateDirExists` unless `force=true`. |
| `internal/util`            | Cross-cutting utilities (random passwords, BOM-safe readers, etc.).  |
| `internal/util/progress`   | Optional `Reporter` interface (`Start` / `Tick` / `Finish`) plus `New` and `Nop` constructors. The visual implementation wraps `github.com/schollz/progressbar/v3`; the package adds a uniform TTY / `NO_COLOR` / `TERM=dumb` / `--no-progress` gate so call sites don't re-implement it. Library packages accept a `Reporter` field on their option structs but never construct one — only `internal/cli` root PreRunE does, via `progress.New(cmd.ErrOrStderr(), …)`. Added in Phase 1.5 / v1.3.0. |
| `cmd/desktop`              | Phase 3 desktop entry point (`//go:build desktop`). Opens the SQLite DB, builds the in-process HTTP server, and starts the Wails v3 window. Only file that imports Wails; gated by the `desktop` build tag so `go build ./...` and `go test ./...` never pull in CGO-requiring Wails code on paths that must stay CGO-free. |
| `internal/desktop/db`      | `Open(path) (*gorm.DB, error)` — opens a SQLite file via `glebarez/sqlite` (pure-Go GORM driver). Configures WAL mode, foreign keys, and single-writer connection. |
| `internal/desktop/profile` | `ServerProfile` GORM model and `Store` — full CRUD for the `server_profiles` table. `NewStore` calls `AutoMigrate`. Operations: `List`, `Get`, `Create`, `Update`, `Delete` (hard), `SetDefault`, `SeedFromLegacy`. |
| `internal/desktop/handler` | `New(*profile.Store) *http.ServeMux` — routes for Screen 1 (server profiles). PRG pattern for sync screens (export-users, download-scoreboard). SSE + oplog + PRG pattern for batch screens (import-users-teams, remove-users-teams, download-problems, import-problems, download-submissions). Each batch screen registers four routes: `GET /…` form, `POST /…` submit (creates oplog, 303 redirect), `GET /…/{id}/run` run page, `GET /…/{id}/stream` SSE stream. Injectable `*Func` types for each operation enable test mocking without a real DOMjudge. `RegisterHistory` adds `GET /history` (filterable list) and `GET /history/{id}` (detail) backed by `oplog.Store.ListFiltered`. |
| `internal/desktop/templates` | templ components: `Layout`, `Flash`, `ProfileList`, `ProfileForm`. Generated `*_templ.go` files are committed. |
| `internal/desktop/assets`  | `var FS embed.FS` — embeds `tailwind.css` and `daisyui.css`. Rebuilt by `just build-desktop-css` when templates change; committed so the binary is self-contained. |

## The `web.Gateway` abstraction (Phase 1+)

`internal/client/web/gateway.go` declares the 11-method `Gateway`
interface that captures every shaped operation the CLI needs from the
HTML UI. `internal/client/web/v7.Client` and
`internal/client/web/v8.Client` both satisfy the same interface, so
callers stay behind `web.Gateway` while `gatewayfactory` selects the
versioned implementation from `cfg.Version`.

**Version routing:** the `// Deprecated:` markers on the interface and
on `web/v7`'s implementations describe v7's API-first default routing
since Phase 1.5. Those methods remain compiled and tested so
`cfg.PreferScraper=true` can route v7 writes through the scraper. On v8,
the same methods are active: create, update, password, email, and
affiliation changes route through `web/v8`, with update avoiding the
DOMjudge 8.3.1 REST API's missing working PUT-update path. Delete operations
(`DeleteUsers`, `DeleteTeams`) are scraper-only on both versions, and on
v8 specifically remain scraper-only because the REST API has no DELETE.

**Lifecycle**: callers MUST call `Login(ctx)` before any other method.
Phase 1 implementations do NOT auto-relogin on session expiry — a 302
to `/login` surfaces as `web.ErrLoginFailed` and the caller decides
whether to retry.

**Contest scoping**: `v7.Options.ContestID` is a string (empty = no
contest). The v7 implementation translates it into a `domjudge_cid`
cookie on the session jar at construction time. v8 may translate the
same field into path scoping (`/contests/{cid}/jury/...`); callers see
`web.Gateway` and don't depend on the mechanism.

**Problem package workflows**: contest scoping for problem package export is
API-assisted. `download-problems` uses `api/v4.Problems.List` to turn
`domserver.json.contest_id` into concrete problem ids, then uses the
versioned scraper only for the full ZIP export that the API does not expose.
`import-problems` remains API-backed but selects the multipart file field by
DOMjudge family (`zip[]` on 7.x, `zip` on 8.x). See
[`adr/0010`](adr/0010-problem-package-contest-scoping-and-import.md).
`import-problems --upsert` is a tool-side reconciliation mode: it reads
`api/v4.Problems.List` for the configured contest, matches each ZIP by
package identity, and then chooses either the existing create import path or
an update import with `problem=<existing id>`. It does not rely on a native
DOMjudge bulk upsert API. See
[`adr/0011`](adr/0011-import-problems-upsert.md).

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
