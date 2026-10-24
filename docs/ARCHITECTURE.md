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
- **DOMjudge version compatibility.** `web.Gateway` separates versioned
  scrapers from callers; the CLI supports DOMjudge 7.3.x and 8.3.1.
- **Reuse from the local web UI.** The `domjudge-tool web` command serves an
  in-process HTTP server with templ/Datastar screens that call the same
  `internal/` use cases. Business logic stays in Go, without a React client or
  generated bindings.

## Layers

```text
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

The local Web UI is served directly from the CLI via `domjudge-tool web`:

```text
internal/cli/web/              Cobra command wiring, flag validation, and opening browser
        │
        ▼
internal/webui/server/         Composes HTTP server, stores, security middleware, and run funcs
        │
        ▼
internal/webui/handler/        In-process net/http ServeMux; PRG + SSE handlers
        │
        ▼
internal/webui/profile/        Server-profile CRUD use-case (GORM model + Store)
        │
        ▼
internal/webui/db/             GORM open helper (glebarez/sqlite, pure-Go driver)

internal/webui/assets/         Embedded web UI assets (webui.css, tailwind.css, daisyui.css, datastar.min.js)
internal/webui/templates/      templ components (layout, screens)
frontend/                      Tailwind v4 build source (input.css + daisyui.css);
                               output goes to internal/webui/assets/tailwind.css
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
| `internal/client/web`      | Version-agnostic 11-method `Gateway` implemented by the v7/v8 scrapers. Default CSV creation, existing-user updates, affiliation, password, and email writes use the versioned scraper. Legacy `Deprecated` comments describe an earlier migration, not methods that can be removed. REST has no user/team DELETE; deletion stays scraper-only. |
| `internal/client/web/gatewayfactory` | Shared factory that reads `config.DomServerClient.Version`, builds the `httpx.SessionClient`, and returns `web/v7` for supported `7.3.x` configs or `web/v8` for supported `8.3.1` configs. It lives outside `internal/client/web` because the factory imports both version packages, while those packages import the base `web` interface. |
| `internal/importer`        | Import reconciliation and operation routing. `TemplateCSV` preserves the CSV column order, with optional `category_id` last. `ImportUsersTeams` partitions an API user snapshot into create/update cohorts; default creates and all existing-user updates use the versioned scraper, including password/email writes. Standalone v7 password resets can use accounts TSV under the router's safety rules; v8 resets use the scraper. `DeleteTeamsAndUsers` protects `admin`, `judgehost`, and the configured user, applies include/exclude filters, then calls scraper deletion. Router choices are logged through `slog`. |
| `internal/scoreboard`      | `Download` (Phase 1.5: drives `api/v4` `Scoreboard.Get` + `Contests.Get` + `Teams.List` + `Organizations.List`) and `Build` (Phase 1.5: joins those four JSON responses into a CSV row stream byte-identical to v1.0.0-rc1's HTML-parser output). `WriteCSV(header, rows, w)` — encodes with `UseCRLF=true` (Python default dialect lock; both CLI and web UI must use this function, never re-encode). The legacy HTML parser (`parse.go`) stays compiled and tested as a fallback path but is no longer wired into `Download`. |
| `internal/exporter`        | `Users([]model.User, Format, io.Writer) error` and `CreateUsers([]model.CreateUser, Format, io.Writer) error` — serialise user data to CSV or JSON. `Format` is a string constant (`"csv"` / `"json"`); both functions emit `[]` for nil slices and treat unknown formats as errors. Used by the CLI layer (PR-6) to write `export_users.*` and `import-users-teams-out.csv`. |
| `internal/problems`        | `Download(ctx, gw, filter, outDir, rep) (Result, error)` — lists contest problems via `web.Gateway.GetProblems`, creates outDir, then streams each export ZIP to `${outDir}/${id}_${name}.zip` (spaces → "-", locked filename format from Python `_problems.py:32`). `Result` carries `Downloaded`, `Skipped`, and the absolute `OutDir`. Caller must call `gw.Login` before Download. Both `internal/cli/problems` and `internal/webui/server` call this; the CLI keeps its own `resolveDownloadOnly` (API-based contest-scoping) and `internal/webui/server` has an equivalent `resolveDownloadFilter`. `Import(ctx, ProblemImporter, ProblemLister, version, cid, inputs, upsert, rep) (ImportResult, error)` — uploads each `ImportInput` (path+name+optional problemID) to DOMjudge via the v4 API. When `upsert=true`, queries the contest problem list once and resolves each input against existing problems by identity (external ID from `domjudge-problem.ini`, then filename stem); matched inputs are sent as updates, unmatched ones as creates. Routes the multipart field name by DOMjudge family (`zip[]` on 7.x, `zip` on 8.x). Extracted from `internal/cli/problems` so web UI and CLI share the identical branching logic. `internal/cli/problems` keeps only `collectProblemZIPs` (filesystem scanner) and `isZipPath`. |
| `internal/submissions`     | `Download(ctx, api *apiv4.Client, cid string, opts Options) (int, error)` — fetches every submission's source files for a contest and writes one ZIP per submission under `<OutputPath>/<layout>/<judgement>_<id>.zip`. `Options.Mode` selects folder layout (1=team-first, 2=problem-first default, other=contest_cid). `Options.ProblemIDs`, when non-empty, restricts the download to matching problem IDs (web UI optional filter). `Options.Extract` unzips in place (Python parity default: true). `Options.Reporter` is an optional progress sink. Returns bare count of ZIPs written; aborts on first API error. `internal/cli/submissions` keeps CLI flag wiring and defaults. |
| `internal/email`           | `LoadTemplate(dir)` reads `subject.txt` + `body.html` (HTML preferred over `body.txt`); `(*Template).Render(vars)` applies `os.Expand` variable substitution and unconditionally Q-encodes the subject. `Dial(ctx, DialOptions)` opens a plain or TLS SMTP connection, sends EHLO, and performs `PlainAuth` when credentials are non-empty (STARTTLS not implemented — Python parity). `SendUserAccounts(ctx, Sender, *Template, []model.CreateUser, *config.DomServerClient, SendOptions)` iterates the user list: abort-on-first-error; recipient resolved from `user.Email` or falls back to `{username}@{domain}` when `Email` is nil; builds complete RFC 5322 messages with explicit `Date:` and `Message-ID:` headers, base64-encoded body. `BuildEML(*Template, model.CreateUser, *config.DomServerClient, SendOptions) ([]byte, string, error)` extracts the message-construction half of `SendUserAccounts` so the offline `render-user-accounts` CLI can emit per-recipient `.eml` files without touching SMTP. `WriteTemplateScaffold(dir string, force bool) error` writes a runnable scaffold containing all six supported placeholders; collisions raise the sentinel `ErrTemplateDirExists` unless `force=true`. |
| `internal/util`            | Cross-cutting utilities (random passwords, BOM-safe readers, etc.).  |
| `internal/util/progress`   | Optional `Reporter` interface (`Start` / `Tick` / `Finish`) plus `New` and `Nop` constructors. The visual implementation wraps `github.com/schollz/progressbar/v3`; the package adds a uniform TTY / `NO_COLOR` / `TERM=dumb` / `--no-progress` gate so call sites don't re-implement it. Library packages accept a `Reporter` field on their option structs but never construct one — only `internal/cli` root PreRunE does, via `progress.New(cmd.ErrOrStderr(), …)`. Added in Phase 1.5 / v1.3.0. |
| `internal/cli/web`         | Cobra command wiring for `domjudge-tool web`, flag validation, terminal startup output, and launching the user's default browser. |
| `internal/webui/server`    | Local web UI server composition. Mounts routes, applies security middleware (token/basic auth, DNS rebinding, CSRF, security headers), wires production operation functions, and manages server lifecycle. |
| `internal/webui/handler`   | `New(*profile.Store) *http.ServeMux` — routes for Screen 1 (server profiles). PRG pattern for sync screens (export-users, download-scoreboard). SSE + oplog + PRG pattern for batch screens (import-users-teams, remove-users-teams, download-problems, import-problems, download-submissions). Each batch screen registers four routes: `GET /…` form, `POST /…` submit (creates oplog, 303 redirect), `GET /…/{id}/run` run page, `GET /…/{id}/stream` SSE stream. Injectable `*Func` types for each operation enable test mocking without a real DOMjudge. `RegisterHistory` adds `GET /history` (filterable list) and `GET /history/{id}` (detail) backed by `oplog.Store.ListFiltered`. `RegisterSetPassword` adds `GET /POST /set-password` (sync form). |
| `internal/webui/profile`   | `ServerProfile` GORM model and `Store` — full CRUD for the `server_profiles` table. `NewStore` calls `AutoMigrate`. Operations: `List`, `Get`, `Create`, `Update`, `Delete` (hard), `SetDefault`, `SeedFromLegacy`. |
| `internal/webui/settings`  | `AppSettings` GORM model (singleton row at ID=1) and `Store` — persists theme, default profile ID, and SMTP config. `Get` returns defaults (theme="dark", port=25) when no row exists. `Save` upserts at ID=1. |
| `internal/webui/oplog`     | `OperationLog` GORM model and `Store` — operation lifecycle (StartedAt, FinishedAt, Status, RowCount, SummaryJSON, ErrorMessage) with newest-first filtered listing. |
| `internal/webui/db`        | `Open(path) (*gorm.DB, error)` — opens a SQLite file via `glebarez/sqlite` (pure-Go GORM driver). Configures WAL mode, foreign keys, and single-writer connection. |
| `internal/webui/templates` | templ components: `Layout`, `Flash`, `ProfileList`, `ProfileForm`, and screen views. Generated `*_templ.go` files are committed. |
| `internal/webui/assets`    | Embedded CSS and JavaScript assets via `FS` (`webui.css`, `tailwind.css`, `daisyui.css`, `datastar.min.js`). Styles are rebuilt by `mise run build-css`; committed assets keep the binary self-contained. |

Scoreboard attachment responses also record their running/done/error
lifecycle in `oplog.Store`; they remain synchronous and do not use the
navigating-form busy guard. Download Problems uses the existing API contest
selector and stores the per-operation contest override in its parameters,
without mutating the saved server profile.

### Web UI command and security architecture

The local web UI is served by `domjudge-tool web` (see
[ADR 0018](adr/0018-local-web-ui-command.md)). The command starts an
in-process HTTP server and opens the browser, replacing the earlier Wails
native window.

Security controls:
- **Authentication:** `token` (default) generates a 256-bit random secret;
  opening the URL with `?token=` sets an `HttpOnly`, `SameSite=Strict` cookie
  and redirects to the clean URL. `basic` uses HTTP Basic authentication.
  `--auth none` is restricted strictly to loopback interfaces.
- **DNS rebinding:** Loopback listeners require a loopback `Host` header.
- **CSRF:** Go's `http.CrossOriginProtection` rejects cross-origin state-changing requests.
- **Headers:** `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Content-Security-Policy: frame-ancestors 'none'`, `Referrer-Policy: no-referrer`.

### Web UI interaction contracts

Both themes (Dark and Light) use the same responsive layout, keyboard
navigation, focus indicators, and reduced-motion behavior. Wide tables scroll
locally within overflow wrappers rather than widening the document.
The implementation lives in the embedded styles (`webui.css`) and templ components.

Destructive removal defaults to dry-run and requires a confirmation dialog
before a real submission. Navigating POST forms guard against repeat
submissions and reset on `pageshow`; attachment forms remain repeatable.

Removal I/O uses the request context. Its terminal History write has an
independent five-second context so disconnection cannot leave a completed
operation permanently running. Persistence failures remain visible through
logging and, where connected, an audit warning.



## The `web.Gateway` abstraction (Phase 1+)

`internal/client/web/gateway.go` declares the 11-method `Gateway`
interface that captures every shaped operation the CLI needs from the
HTML UI. `internal/client/web/v7.Client` and
`internal/client/web/v8.Client` both satisfy the same interface, so
callers stay behind `web.Gateway` while `gatewayfactory` selects the
versioned implementation from `cfg.Version`.

**Version routing:** legacy `// Deprecated:` markers on `Gateway` and
`web/v7` refer to the earlier API-first migration. These methods remain
active: default CSV creation and existing-user updates use the scraper on
both families, including affiliation, password, and email writes. Standalone
v7 `set-password` can use accounts TSV when no roles override or scraper
preference requires the scraper; v8 password resets always use the scraper.
Delete operations stay scraper-only because REST has no user/team DELETE.
See [ADR 0006](adr/0006-api-first-with-scraper-fallback.md) for routing boundaries.

**Lifecycle**: callers MUST call `Login(ctx)` before any other method.
Clients do not auto-relogin on session expiry; a redirect to `/login`
surfaces as `web.ErrLoginFailed`, and the caller decides whether to retry.

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

- `domserver.json` — snake_case field names, types, and defaults remain stable;
  see the [configuration reference](CLI.md#configuration-domserverjson).
- `import-users-teams.csv` / `import-users-teams-out.csv` — fixed column order;
  see the [compatibility contracts](ROADMAP.md#compatibility-contracts).
- `export_problems/` — directory layout of exported problems is the same as
  the Python CLI.

## Cross-references

- Code style: `CONVENTIONS.md`
- Test layout: `TESTING.md`
- Phase plan: `ROADMAP.md`
- ADRs: `adr/`
