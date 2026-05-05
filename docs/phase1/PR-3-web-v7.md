# PR-3 — Web Gateway interface + DOMjudge 7 scraper

## Goal

Provide a typed Go gateway over DOMjudge 7.3.x's authenticated `/jury`
HTML UI so PR-4 (importer/exporter), PR-6 (CLI users subtree) and PR-7
(CLI problems / scoreboard) can drive create / update / read / delete
operations against a DOMjudge whose REST API does not expose them.

This PR also introduces the cross-cutting `internal/client/web.Gateway`
interface so Phase 2 can plug in a `v8` implementation without touching
callers.

## Files created / modified

### Source

| Path | Role |
|------|------|
| `internal/client/httpx/session.go` | `SessionClient` — cookie-jar HTTP transport that follows redirects (parallel to existing `Client` for API). Resolves path-only `path` arguments against `BaseURL` so scraped delete hrefs (e.g. `/jury/users/3/delete`) don't need caller-side prefixing. |
| `internal/client/httpx/session_test.go` | Cookie persistence, redirect handling, context cancellation, timeout, relative-path resolution. |
| `internal/client/web/gateway.go` | `Gateway` interface (the 11 operations). One-screen contract; doc comment defines lifecycle (callers must call `Login` first; no auto-relogin in Phase 1). |
| `internal/client/web/errors.go` | **Gateway-level sentinel errors** (`ErrLoginFailed`, `ErrCreateFailed`, `ErrUpdateFailed`, `ErrMalformedRow`, `ErrUnauthorized`, `ErrServerError`). Both `v7.Client` and the future `v8.Client` return these so callers depend on the abstraction, not a version-specific package. |
| `internal/client/web/parse.go` | Shared `ParseFormFields(r) (url.Values, error)` — browser-form snapshot of every `<input>`, `<textarea>`, and `<select>` element with a `name` attribute. Multi-selects emit every selected option via `url.Values.Add`. Unselected selects contribute nothing (key absent). **Diverges from Python on purpose** to fix two data-loss bugs (textarea cleared, multi-select truncated to first). |
| `internal/client/web/parse_test.go` | Helper tests (input-field parser, nameless inputs dropped, single + multi-select, textarea preservation, no-selection-is-key-absent). |
| `internal/client/web/v7/paths.go` | Unexported `const` block mapping Python `HomePath`/`UserPath`/`TeamPath`/`AffiliationPath`/`ProblemPath` enums (`pathLogin`, `pathJury`, `pathUsersList`, `pathUsersEditFmt`, …). |
| `internal/client/web/v7/errors.go` | `HTTPError` typed error matching `api/v4.HTTPError` shape, with `Is()` matching the gateway-level `ErrUnauthorized`/`ErrServerError`. |
| `internal/client/web/v7/form.go` | Shared `(c *Client).postForm(ctx, path, overrides)` — GET → `ParseFormFields` → merge `overrides` (caller wins; `[]string` values expand to repeated form keys for `user[user_roles][]` etc.) → POST. Inspects `resp.Request.URL.Path` on both legs to surface `web.ErrLoginFailed` if the session expired and the request was redirected to `/login`. Eliminates six near-identical write-op bodies. |
| `internal/client/web/v7/client.go` | `v7.Client` — implements `web.Gateway`. |
| `internal/client/web/v7/login.go` | `Login(ctx)`. |
| `internal/client/web/v7/affiliations.go` | `GetAffiliations`, `GetAffiliation`, `CreateAffiliation`. |
| `internal/client/web/v7/problems.go` | `GetProblems`. |
| `internal/client/web/v7/users.go` | `SetUserPassword`, `DeleteUsers`. |
| `internal/client/web/v7/teams.go` | `CreateTeamAndUser`, `UpdateTeam`, `DeleteTeams`. |
| `internal/client/web/v7/login_test.go` etc. | One `_test.go` per source file. |

### Fixtures (already shipped in this PR)

`testdata/web/v7/` — 11 sanitized HTML pages captured from a production
DOMjudge 7.3.4 instance. See `testdata/web/v7/CAPTURE.md` for the full
sanitization rule set and the 8 verification grep gates that returned
zero hits before the files were committed. **No raw page ever entered
git.**

Plus three small hand-authored fixtures committed alongside this PR:

| Path | Why hand-authored |
|------|-------------------|
| `testdata/web/v7/affiliations_list_with_flag.html` | Production fixture has no `<img alt="...">` country flags configured, so the `img alt` branch of `v7.py:236-238` would otherwise have zero coverage. |
| `testdata/web/v7/tbody_empty.html` | Empty-table tolerance for `Get*` methods. |
| `testdata/web/v7/tbody_malformed.html` | Row missing expected `<a>` tags — parser must return a wrapped error, not panic. |

## Architecture

### `httpx.SessionClient` vs `httpx.Client`

The two clients live side-by-side in `internal/client/httpx`. They
share neither type nor wrap-relation; the only commonality is that each
provides ctx-first methods and respects an `Options.DisableSSL` toggle.

| Trait | `Client` (existing, API/v4) | `SessionClient` (new, web/v7) |
|-------|-----------------------------|-------------------------------|
| Auth | Basic Auth header on every request | Session cookie, set by `Login` |
| Redirects | `http.ErrUseLastResponse` (none) | Follow (default Go behaviour) |
| Cookie jar | None | `cookiejar.New(...)` |
| Methods | `GET`, `GETStream` | `GET(ctx, path)`, `PostForm(ctx, path, url.Values)` |
| Path resolution | Caller passes a full URL | Path-only argument (e.g. `/jury/users/3/delete`) is resolved against `BaseURL` inside the client. Scraped delete hrefs work without caller-side prefixing. |
| Default timeout | 30 s | 30 s (matched intentionally) |
| Jar accessor | n/a | `Jar() http.CookieJar` (used by `v7.Client.New` to set `domjudge_cid` if `Options.ContestID` is non-empty — see open question above) |

A single `Client` "with a `Mode` flag" was rejected: the redirect /
cookie / auth differences would force every method to branch on the
flag, defeating the goal of having one place where transport policy
lives.

### `web.Gateway` interface

```go
package web

// Gateway is the version-agnostic contract for driving a DOMjudge UI.
//
// Lifecycle: callers MUST call Login(ctx) before any other method.
// Phase 1 implementations do NOT auto-relogin on session expiry — a
// 302 to /login surfaces as ErrLoginFailed and the caller decides
// whether to retry.
type Gateway interface {
    Login(ctx context.Context) error
    GetAffiliations(ctx context.Context) ([]model.Affiliation, error)
    GetAffiliation(ctx context.Context, name string) (*model.Affiliation, error)
    CreateAffiliation(ctx context.Context, shortname, name, country string) (*model.Affiliation, error)
    GetProblems(ctx context.Context, opts ProblemFilter) ([]model.ProblemItem, error)
    SetUserPassword(ctx context.Context, userID, password string, roles []int, enabled bool) error
    DeleteUsers(ctx context.Context, include, exclude []string) error
    CreateTeamAndUser(ctx context.Context, user model.CreateUser, categoryID, affiliationID int, enabled bool) (teamID, userID string, err error)
    UpdateTeam(ctx context.Context, user model.User, categoryID, affiliationID int, enabled bool) (teamID, userID string, err error)
    DeleteTeams(ctx context.Context, include, exclude []string) error
}

// Errors live in `internal/client/web/errors.go` (gateway level), not
// in `internal/client/web/v7/`, so callers depend on the abstraction:
//
//   var (
//       ErrLoginFailed   = errors.New("web: login failed")
//       ErrCreateFailed  = errors.New("web: create failed")
//       ErrUpdateFailed  = errors.New("web: update failed")
//       ErrMalformedRow  = errors.New("web: malformed table row")
//       ErrUnauthorized  = errors.New("web: unauthorized")
//       ErrServerError   = errors.New("web: server error")
//   )
```

`v7.Client` is the only Phase-1 implementation and is constructed with
`v7.New(opts v7.Options) *Client`.

**Mixed-shape signatures are intentional.** `GetProblems(ctx, ProblemFilter{...})`
mirrors Python's `(exclude=, only=)` keyword args; `DeleteUsers(ctx, include, exclude)`
mirrors Python's positional list args. Phase 1 prioritises Python parity
over signature uniformity; if PR-6 wiring proves painful we revisit then.

**`GetAffiliation(name)` returns `(nil, nil)` for not-found** to match
Python's `Optional[Affiliation]`. CLI callers in Phase 1 don't
distinguish "not found" from "transport error" in any user-visible way.
A `web.ErrNotFound` retrofit is cheap if a Phase 2+ caller needs it.

### Domjudge contest scoping

Callers specify the active contest via `v7.Options.ContestID` (a
`string`; empty means "no contest"). `v7.Client.New` translates this
into the v7 mechanism — a `domjudge_cid` cookie set on the session jar
— before any operation runs. Phase 2's `v8.Client` will satisfy the
same `Options.ContestID` field via whatever v8 actually uses (path
scoping is plausible) without changing caller code.

```go
gw := v7.New(v7.Options{Session: sc, ContestID: "19"})
```

Callers depend only on `web.Gateway` and never type-assert through to
`httpx.SessionClient`. The cookie convention is a v7 implementation
detail.

## Behavioural deviations from Python

| Deviation | Python | Go |
|-----------|--------|----|
| `disable_ssl` honoured | `api_client.py:48-55` constructor bug drops it | Honoured via `httpx.SessionClient` (same fix as PR-2) |
| `login` failure | `assert res.url.path == "/jury"` panics | Returns `ErrLoginFailed` |
| `delete_users` filter | Inverted condition silently no-ops when `include` is empty | **Locked bit-for-bit** with `// QUIRK:` comment + regression test (see TDD anchors) |
| `create_team_and_user` user-link parse | `BeautifulSoup` `select_one(".container-fluid a")` returns first match | `goquery` `.Find(".container-fluid a").First()` |
| Empty `tbody` | Iterates zero rows, returns empty list | Same |
| Malformed row (missing `<a>`) | Python `IndexError` → 500 from caller | Wrapped error `ErrMalformedRow` for **every** table-parsing op (`GetAffiliations`, `GetProblems`, `DeleteUsers`, `DeleteTeams`). Locks the analogous Python panic at `v7.py:159` and `:186`. |
| `get_problems` `exclude` nil | Python crashes on `None in exclude` | Treated as empty slice |
| `create_affiliation` country default | Python signature has `country = "TWN"` default | Required arg; the default lives at the importer / CLI layer |
| `delete_users` / `delete_teams` concurrency | Python builds a list of coroutines and awaits them concurrently | Sequential ranges over collected links. Acceptable for Phase 1; revisit only if real users hit throughput problems (Phase 1.5). |
| `_get_input_fields` selector breadth | `soup.select("input")` — every input type (hidden, text, password, submit) | `ParseFormFields` mirrors this — selecting only `[type=hidden]` would silently drop `_username`, `_password`, `team[users][0][username]`, and every other text field. |
| `<textarea>` content | Python `_get_input_fields` reads only `<input>` and `<select>` — textarea content is invisible to it, so `update_team` silently clears any existing `team[members]` / `team[comments]` value. | `ParseFormFields` reads `<textarea name=…>` as another single-value source. **Deviates from Python on purpose** to fix the data-loss bug surfaced by the Codex adversarial review. Locked by `TestUpdateTeam_PreservesTextareaContent`. |
| Multi-select preservation | Python's `select_one("option[selected]")` returns only the FIRST selected option, so `update_team` on a team in N contests posts back only contest 1 — silently detaching N-1 associations. | `ParseFormFields` emits every selected option via `url.Values.Add`, so all assignments survive the form replay. **Deviates from Python on purpose**; locked by `TestUpdateTeam_PreservesMultiSelectContestAssignments`. |
| Redirect-to-login on read/write paths | Python's `WebClient.get/post` follow redirects but never check the post-redirect path; an expired session can silently make `get_problems` / `delete_users` / `create_affiliation` look like a happy no-op. | Both `fetchDoc` (read path) and `postForm` (write path) inspect `resp.Request.URL.Path` after every redirect-following request. A landing on `/login` returns wrapped `web.ErrLoginFailed`. Locked by `TestFetchDoc_RedirectsToLogin_*` and `TestPostForm_*RedirectsToLogin_*`. |

## TDD anchors

| Test | Location | What it locks |
|------|----------|---------------|
| `TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk` | `users_test.go` | Locks `v7.py:161` quirk: with empty `include`, the filter `lower_name not in include or lower_name in exclude` short-circuits to "skip every row", so `DeleteUsers` issues zero POSTs. The test posts on a fake server and asserts no requests beyond the initial GET. Includes a `// QUIRK:` comment pointing to ROADMAP. |
| `TestLogin_AssertsLandingOnJury` | `login_test.go` | Login failure yields wrapped `ErrLoginFailed`, not a panic. |
| `TestSessionClient_CookieJarPersistsAcrossRequests` | `session_test.go` | Cookies set by the server in response to one request are sent on the next. |
| `TestSessionClient_FollowsRedirects` | `session_test.go` | 302 → final GET; `resp.Request.URL.Path` reflects landing page. |

## Tests written first (RED) — per operation

Per the user's confirmed rule: **each operation lands as its own
red→green→refactor commit triple**. Eleven operations, eleven cycles.
Order is foundation → reads → writes → composite, so each subsequent
operation's tests can reuse fixtures and helpers added earlier.

The matrix below names every test that must exist; cells in **bold** are
the explicit edge cases for that operation.

### 0. `httpx.SessionClient` (foundation, no Python equivalent)

| Test | Asserts |
|------|---------|
| `TestSessionClient_GET_HappyPath` | 200 body returned. |
| `TestSessionClient_PostForm_SendsURLEncoded` | `Content-Type: application/x-www-form-urlencoded` and body matches. |
| **`TestSessionClient_CookieJarPersistsAcrossRequests`** | Server sets cookie on req 1; client sends it on req 2. |
| **`TestSessionClient_FollowsRedirects`** | 302 followed; final URL surfaced via `resp.Request.URL`. |
| `TestSessionClient_DisableSSLFlipsInsecureSkipVerify` | TLS toggle plumbed. |
| **`TestSessionClient_ContextCancelled_AbortsRequest`** | `context.Canceled` propagated. |
| `TestSessionClient_TimeoutHonoured` | Timeout enforced. |
| `TestSessionClient_JarAccessorReturnsConfigurableJar` | Caller-set cookie is sent on subsequent requests. |

### 1. `Login` (Python `v7.py:38-47`)

| Test | Asserts |
|------|---------|
| `TestLogin_HappyPath_ExtractsCSRFAndPostsCreds` | GET form → POST contains the form's CSRF token + creds. |
| **`TestLogin_AssertsLandingOnJury`** | Landing path `/jury` → nil error. |
| **`TestLogin_LandingNotJury_ReturnsErrLoginFailed`** | Landing path `/login` → `errors.Is(err, ErrLoginFailed)`. |
| `TestLogin_FormGETFails_WrapsErr` | 500 on `GET /login` propagated. |
| `TestLogin_ContextCancelled` | Context propagated. |

### 2. `GetAffiliations` (Python `v7.py:225-248`)

| Test | Asserts |
|------|---------|
| `TestGetAffiliations_HappyPath` | Fixture parses to `len(got) == 3` with expected ids/shortnames/names/countries. |
| **`TestGetAffiliations_CountryFromImgAlt`** | Hand-authored `affiliations_list_with_flag.html` exercises img-alt branch of `v7.py:236-238`. |
| **`TestGetAffiliations_CountryFromTextFallback`** | NTUB capture exercises text-fallback branch. |
| **`TestGetAffiliations_EmptyTable_ReturnsEmptySlice`** | `tbody_empty.html` → `[]model.Affiliation{}`, no error. |
| `TestGetAffiliations_500Error` | 500 → wrapped error. |
| `TestGetAffiliations_ContextCancelled` | Context propagated. |
| **`TestGetAffiliations_MalformedRow_ReturnsErrMalformedRow`** | `tbody_malformed.html` → `errors.Is(err, ErrMalformedRow)`, no panic. |

### 3. `GetAffiliation` (Python `v7.py:250-257`)

| Test | Asserts |
|------|---------|
| `TestGetAffiliation_FoundByName` | Match by `Name`. |
| `TestGetAffiliation_FoundByShortname` | Match by `ShortName`. |
| `TestGetAffiliation_NotFound_ReturnsNilNilNil` | Returns `nil, nil` (matches Python `Optional[Affiliation]` semantics). |
| `TestGetAffiliation_PropagatesListError` | Underlying `GetAffiliations` error surfaces. |

### 4. `GetProblems` (Python `v7.py:259-291`)

| Test | Asserts |
|------|---------|
| `TestGetProblems_HappyPath` | Parses fixture into `[]ProblemItem{...}` with id/name/time_limit/test_data_count/export_file_path. |
| **`TestGetProblems_OnlyFilter`** | `only={"1"}` → only id 1 returned. |
| **`TestGetProblems_ExcludeFilter`** | `exclude={"2"}` → id 2 dropped. |
| **`TestGetProblems_NilOnlyAndNilExclude_DoesNotPanic`** | Locks Python regression `__init__.py:21` (no flags → no panic). |
| `TestGetProblems_500Error` | wrapped. |
| `TestGetProblems_ContextCancelled` | propagated. |
| `TestGetProblems_EmptyTable_EmptySlice` | `tbody_empty.html`. |

### 5. `CreateAffiliation` (Python `v7.py:198-223`)

`country` is required (no default applied at this layer). The Python
`country = "TWN"` default migrates up to PR-4 / PR-6 callers, keeping
`v7.Client` purely about HTML form mechanics.

| Test | Asserts |
|------|---------|
| `TestCreateAffiliation_PostsAllRequiredFields` | POST body contains the four `team_affiliation[*]` fields plus the form's hidden inputs. |
| **`TestCreateAffiliation_ResponseRedirectExtractsID`** | Final URL `/jury/affiliations/42` → returned `Affiliation.ID == "42"`. |
| **`TestCreateAffiliation_StaysOnAdd_ReturnsErrCreateFailed`** | Final path still `/jury/affiliations/add` → `errors.Is(err, ErrCreateFailed)`. |
| `TestCreateAffiliation_500Error` | wrapped. |

### 6. `SetUserPassword` (Python `v7.py:119-142`)

| Test | Asserts |
|------|---------|
| `TestSetUserPassword_PostsHiddenAndOverrides` | POST body merges form's hidden inputs with `user[plainPassword]`, `user[enabled]`, `user[user_roles][]` (each role as separate value). |
| `TestSetUserPassword_RedirectsAwayFromEdit_ReturnsNil` | Happy. |
| **`TestSetUserPassword_StaysOnEdit_ReturnsErr`** | Final path equals edit path → wrapped error. |
| `TestSetUserPassword_500Error` | wrapped. |

### 7. `CreateTeamAndUser` (Python `v7.py:49-82`)

| Test | Asserts |
|------|---------|
| `TestCreateTeamAndUser_HappyPath` | Two-step flow: GET form, POST, GET view → returns `(teamID, userID)` parsed from the redirect path and `.container-fluid a` href. |
| **`TestCreateTeamAndUser_DropsTeamContestsWhenNil`** | If form yields `team[contests][]: None`, key is dropped before POST. |
| `TestCreateTeamAndUser_StayedOnAdd_ReturnsErrCreateFailed` | `/jury/teams/add` final → `ErrCreateFailed`. |
| **`TestCreateTeamAndUser_TeamViewMissingUserLink_ReturnsErr`** | View page without `.container-fluid a` → wrapped error. |
| `TestCreateTeamAndUser_ContextCancelled` | propagated. |

### 8. `UpdateTeam` (Python `v7.py:84-117`)

| Test | Asserts |
|------|---------|
| `TestUpdateTeam_HappyPath` | Same shape as Create. |
| `TestUpdateTeam_StayedOnEdit_ReturnsErr` | wrapped. |
| `TestUpdateTeam_TeamContestsNil_Dropped` | mirror of Create. |

### 9. `DeleteUsers` (Python `v7.py:144-169`)

| Test | Asserts |
|------|---------|
| `TestDeleteUsers_HappyPath_DeletesIncludedNotExcluded` | With `include=["alice","bob"], exclude=["bob"]`, only `alice` → POST. |
| **`TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk`** | `include=nil` (or `[]`) → zero POSTs. Comment block: `// QUIRK: see docs/ROADMAP.md Phase 1 quirks; do not "fix" without ADR.` |
| `TestDeleteUsers_NotFoundUsernamesIgnored` | Names not in table are silently skipped (matches Python). |
| `TestDeleteUsers_500FromList` | wrapped. |

### 10. `DeleteTeams` (Python `v7.py:171-196`)

| Test | Asserts |
|------|---------|
| `TestDeleteTeams_HappyPath` | Mirror of `DeleteUsers`. |
| **`TestDeleteTeams_PythonInvertedFilter_LocksKnownQuirk`** | Same quirk lives on `v7.py:188`. |
| `TestDeleteTeams_PostHrefIsSecondToLastAnchor` | Locks the `a[-2]` selector vs `a[-1]` for users. |

## Edge cases covered

The bold rows in the matrix above are the explicit edge cases. Summary:

- **Filter quirks** — empty `include`, `nil` slices, mixed include/exclude.
- **Form mechanics** — hidden inputs preserved, `team[contests][]: None` dropped, multi-value role roles encoded as repeated keys.
- **Redirect-as-success** — every "create" / "update" / "delete" / "set" op decides success/failure by inspecting the final URL after redirects, never by status alone.
- **Empty / malformed tables** — empty `tbody` is success; missing expected `<a>` is `ErrMalformedRow`.
- **Country flag fallback** — `img alt` branch covered by hand-authored fixture, text branch by NTUB capture.
- **Context cancellation** — every method that issues HTTP propagates `ctx.Err()`.
- **TLS** — `disable_ssl` plumbed (Python constructor bug fixed, same as PR-2).
- **Inversion quirk** — locked bit-for-bit; comment + ROADMAP cross-reference; review checklist gate.

## Definition of done

- [x] Eleven `test(...): add failing tests for <op>` commits each precede their `feat(...): implement <op>` commit (TDD audit visible in `git log`).
- [x] `go test -race ./internal/client/web/... ./internal/client/httpx/...` green.
- [x] Coverage ≥ 70 % on `internal/client/httpx` (80.3 %), ≥ 80 % on `internal/client/web/v7` (89.0 %), and on `internal/client/web` (91.3 %).
- [x] `gofmt -s` / `go vet` / `golangci-lint run` clean (0 issues).
- [x] No real DOMjudge URLs, credentials, PII, or production CSRF tokens in any committed file. Verified by re-running the eight grep gates from `CAPTURE.md`.
- [x] No `init()` outside registry wiring; no `fmt.Print*` from library code.
- [x] `// QUIRK:` comment exists on the locked filter (`users.go` `DeleteUsers` docstring + cross-reference to `docs/ROADMAP.md` §"Quirks to preserve bit-for-bit", which holds the v7.py:161 entry).
- [x] Every exported symbol has a doc comment; package-level `// Package v7 ...` and `// Package web ...` comments present.

## Cross-references

- Python source: `domjudge_tool_cli/services/web/{base.py, v7.py}`, `domjudge_tool_cli/services/api_client.py:76-115` (Python `WebClient`).
- Architecture: `docs/ARCHITECTURE.md` Layer 2 (`internal/client/web` + `internal/client/httpx`).
- Roadmap: `docs/ROADMAP.md` Phase 1 — locked v7 quirks.
- Conventions: `docs/CONVENTIONS.md`, `docs/TESTING.md`, `docs/COMMITS.md`.
- Capture protocol: `testdata/web/v7/CAPTURE.md`.

## Conventional Commit (merge)

```
feat(web/v7): gateway interface + scraper for domjudge 7.3.x
```

## Notes for downstream PRs

- PR-4 (importer / exporter) imports `internal/client/web` and uses `Gateway` to drive create / update / read / delete.
- PR-6 (CLI users) wires `Gateway` into `users add`, `users edit`, `users delete`.
- PR-7 (CLI problems) reuses `GetProblems` for the problem-listing subcommand.
- Phase 2 will add `internal/client/web/v8/` next to `v7/`; the `Gateway` interface stays unchanged so callers don't move.
