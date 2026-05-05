# PR-4 — Importer + Exporter use-cases

## Goal

Deliver the Layer 3 use-case packages that orchestrate the Layer 2
clients from PR-1/2/3:

- **`internal/importer`** — read a CSV produced by
  `import-users-teams-example`, reconcile against the live API user list,
  and drive `web.Gateway` to create, update, or delete teams and users.
- **`internal/exporter`** — serialise `[]model.User` to CSV or JSON so
  PR-6 CLI commands can write `import-users-teams-out.csv` and
  `export_users.{csv,json}`.

These packages unblock **PR-6** (CLI `users` subtree: `user-list`,
`import-users-teams`, `import-users-teams-example`, `rm-teams-and-users`)
and the output step of `import_users_teams` that writes
`import-users-teams-out.csv`.

## Files created / modified

### Source

| Path | Role |
|------|------|
| `internal/importer/importer.go` | `ImportUsersTeams` — main orchestration loop (login → reconcile API vs CSV → create/update/delete via Gateway → return `[]model.CreateUser` with passwords). |
| `internal/importer/template.go` | `TemplateCSV` — writes the template CSV to `io.Writer`; output must be byte-identical to Python `templates/csv/import-users-teams.csv`. Carries `// PHASE1.md DoD #5` cross-reference. |
| `internal/importer/delete.go` | `DeleteTeamsAndUsers` — lists API users, derives team-ids, applies default-protect and include/exclude filters, then calls `gw.DeleteUsers` followed by `gw.DeleteTeams`. |
| `internal/importer/importer_test.go` | Tests for `ImportUsersTeams`. |
| `internal/importer/template_test.go` | Tests for `TemplateCSV`. |
| `internal/importer/delete_test.go` | Tests for `DeleteTeamsAndUsers`. |
| `internal/exporter/users.go` | `Users([]model.User, Format, io.Writer) error` and `CreateUsers([]model.CreateUser, Format, io.Writer) error` — marshal to CSV or JSON. |
| `internal/exporter/users_test.go` | Tests for `Users` and `CreateUsers`. |

### Fixtures (already committed)

| Path | Purpose |
|------|---------|
| `testdata/importer/template_python_golden.csv` | 2-row (header + sample) golden: `username,name,email,affiliation,password\nntubuser,test,ntubuser@mail.com,ntub,testpassword` |
| `testdata/importer/with_bom.csv` | Same content with leading UTF-8 BOM (0xEF 0xBB 0xBF). |

No new web-scraper fixtures are needed; importer tests use a mock
`web.Gateway` (interface test double) and an `httptest.Server` for
API v4 list calls.

## Architecture

### Package boundary

`internal/importer` and `internal/exporter` are Layer 3 packages. They
depend on Layer 2 (`internal/client/api/v4`, `internal/client/web`) and
Layer 1 (`internal/model`, `internal/config`, `internal/util`). They
**must not** print to stdout or write files — all output goes to the
`io.Writer` / `io.Reader` arguments so the CLI layer (Layer 4) controls
I/O.

### Function signatures

```go
// Package importer provides the import-users-teams use case.
package importer

// TemplateCSV writes the import template CSV to w.
//
// The output must be byte-identical to the Python template at
// domjudge_tool_cli/templates/csv/import-users-teams.csv.
//
// PHASE1.md DoD #5: ./bin/domjudge-tool import-users-teams-example
// writes a CSV byte-identical to Python templates/csv/import-users-teams.csv.
func TemplateCSV(w io.Writer) error

// ImportOptions configures a single ImportUsersTeams call.
type ImportOptions struct {
    CategoryID          *int
    AffiliationID       *int
    UserRoles           []int
    AffiliationCountry  string // "" → callers should populate from cfg.AffiliationCountry or "TWN"
    Enabled             bool
    IgnoreExisting      bool
    DeleteExisting      bool
    NewPassword         bool
    PasswordLength      int    // 0 → password.DefaultLength (10)
    PasswordCharset     string // "" → password.DefaultCharset (ascii+digits)
}

// ImportUsersTeams reconciles a CSV against the live API user list and
// drives gw to create, update, or delete teams and users.
//
// Lifecycle: gw must NOT be pre-logged-in; ImportUsersTeams calls
// gw.Login(ctx) as its first gateway operation (mirrors Python
// context-manager "async with … as web: await web.login()").
//
// Returns []model.CreateUser with Password fields populated (either
// from the CSV column or freshly generated). The CLI layer is
// responsible for writing the output CSV from this slice using
// exporter.Users — this function never writes files.
func ImportUsersTeams(
    ctx   context.Context,
    gw    web.Gateway,
    users *apiv4.UsersResource,
    r     io.Reader,
    opts  ImportOptions,
) ([]model.CreateUser, error)

// DeleteOptions configures a single DeleteTeamsAndUsers call.
type DeleteOptions struct {
    Include []string // nil → all non-protected users
    Exclude []string // nil → empty exclude set
}

// DeleteTeamsAndUsers deletes users and their teams from DOMjudge.
//
// The default-protect list (admin, judgehost, cfg.Username) is applied
// BEFORE Include/Exclude filtering; protected users are never passed to
// gw.DeleteUsers or gw.DeleteTeams regardless of other options.
//
// Internally this function:
//  1. Calls users.List to obtain the current user list.
//  2. Derives team-id lists from the API result.
//  3. Calls gw.Login(ctx).
//  4. Calls gw.DeleteUsers(ctx, include, exclude).
//  5. Calls gw.DeleteTeams(ctx, includeTeams, excludeTeams).
func DeleteTeamsAndUsers(
    ctx   context.Context,
    gw    web.Gateway,
    users *apiv4.UsersResource,
    cfg   *config.DomServerClient, // cfg.Username drives default-protect
    opts  DeleteOptions,
) error
```

```go
// Package exporter serialises user data to CSV or JSON.
package exporter

// Format identifies the serialisation target.
type Format string

const (
    FormatCSV  Format = "csv"
    FormatJSON Format = "json"
)

// Users marshals users to w in the given format.
//
// CSV: header row matches model.User field declaration order
// (last_login_time, first_login_time, team, roles, id, username, name,
// email, last_ip, ip, enabled, team_id, affiliation, password).
// Pointer fields render as the pointed-to value or empty string for nil.
// Slice fields (roles) render as a JSON array string (e.g. `["team"]`)
// so that the output is machine-parseable; this deviates from Python
// tablib repr (`['team']`) but is more portable.
// JSON: valid JSON array using json.Marshal field order; empty slice → "[]" not "null".
// Unknown format → non-nil error.
func Users(users []model.User, format Format, w io.Writer) error

// CreateUsers marshals createUsers to w in the given format.
//
// Mirrors Users but accepts the []model.CreateUser slice returned by
// ImportUsersTeams. This is the function PR-6 uses to write
// import-users-teams-out.csv.
// CSV: header row is username, name, email, affiliation, password, is_exist
// (model.CreateUser field declaration order).
// JSON: valid JSON array; empty slice → "[]".
func CreateUsers(createUsers []model.CreateUser, format Format, w io.Writer) error
```

### `ImportUsersTeams` lifecycle

1. `gw.Login(ctx)` — must be called first; gw must not be pre-logged-in.
2. `users.List(ctx, UsersListOpts{})` — snapshot the existing user list.
3. `csvio.ReadAll(r, csvHeaders)` — parse the CSV (BOM-tolerant via `csvio.NewReader`).
4. For each CSV row:
   a. Build a `model.CreateUser` from the row; set `Email = nil` if the column is empty.
   b. **Update-vs-create dispatch**: if `username` exists in the API snapshot, the row produces a `model.User` (existing); otherwise a `model.CreateUser` (new). This dispatch mirrors Python's `isinstance(user, User)` guard and happens **inside `ImportUsersTeams`**, not at the Gateway boundary.
   c. Apply `IgnoreExisting` (skip the row) and `DeleteExisting` (queue for pre-delete).
5. Pre-delete pass: call `gw.DeleteUsers` and `gw.DeleteTeams` for the queued entries.
6. Creation/update pass: for each user in the reconciled list:
   a. Resolve affiliation via `gw.GetAffiliation`; if not found, `gw.CreateAffiliation` with `opts.AffiliationCountry`.
   b. Generate password if CSV column is empty OR `opts.NewPassword == true`.
   c. If existing user: `gw.UpdateTeam(ctx, user, categoryID, affiliationID, opts.Enabled)`.
   d. If new user: `gw.CreateTeamAndUser(ctx, cu, categoryID, affiliationID, opts.Enabled)`.
   e. `gw.SetUserPassword(ctx, userID, password, opts.UserRoles, opts.Enabled)`.
7. Return `[]model.CreateUser` with `Password` fields populated.

**Per-row error policy**: `ImportUsersTeams` aborts on the first error
from any gateway call and returns the partial slice accumulated so far
plus the error. This matches Python's behaviour (no try/except around
individual user calls; an unhandled exception from `create_team_and_user`
propagates and terminates the loop).

### `AffiliationCountry` default chain

`ImportOptions.AffiliationCountry` is a plain `string` (not a pointer).
Callers are responsible for resolving the default before calling
`ImportUsersTeams`. The idiomatic pattern at the CLI layer (PR-6) is:

```go
country := "TWN"
if cfg.AffiliationCountry != nil {
    country = *cfg.AffiliationCountry
}
opts := importer.ImportOptions{AffiliationCountry: country, ...}
```

`config.Default()` sets `AffiliationCountry` to `&"TWN"` so a freshly
loaded config always has a non-nil pointer, but `ImportOptions` receives
a resolved string to keep the importer free of config-pointer logic.

### `both-false` case (`IgnoreExisting=false`, `DeleteExisting=false`)

When neither flag is set, an existing user follows the **update path**:
the CSV row is treated as overrides, the user struct is updated (Step 4b
above), and `gw.UpdateTeam` is called instead of `gw.CreateTeamAndUser`.
This matches Python's `existing_user.update(**item)` branch.

## Behavioural deviations from Python

| Deviation | Python | Go |
|-----------|--------|----|
| Empty `email` column | `item["email"] = None if not item.get("email")` | `CreateUser.Email = nil` (pointer) — empty string in CSV → nil |
| Password generation | `gen_password(length, pattern)` using `random.choices` (not crypto) | `password.Generate(length, charset)` using `crypto/rand` — same contract, secure PRNG |
| CSV BOM | `file.read().replace("﻿", "")` string-replace on entire file | `csvio.NewReader` peeks+discards the 3-byte BOM at read time |
| `AffiliationCountry` default | Python `create_affiliation(name, name, client.affiliation_country)` inlines config | Go caller resolves `*string` → `string` before constructing `ImportOptions` |
| Output CSV | `format.export(new_users, name="import-users-teams-out")` written inside the service function | Returned `[]model.CreateUser` — CLI layer writes `import-users-teams-out.csv` via `exporter.Users` |
| Progress bar | `typer.progressbar(users)` | No progress in library code; CLI layer may add one in PR-6 |
| `delete_teams_and_users` exclude default | Python `if not exclude: exclude = default_ignore_users` — if caller passes `[]`, exclude is replaced | Go: default-protect list is applied unconditionally before include/exclude; a nil `Exclude` is treated as empty (no additional exclusions beyond the protected list) |

## TDD anchors

| Test | Location | What it locks |
|------|----------|---------------|
| `TestCSVImport_HeadersByteIdentical` | `importer_test.go` | **PHASE1.md anchor**: column order `username,name,email,affiliation,password` exact; BOM in `with_bom.csv` stripped so first header matches. |
| `TestTemplateCSV_MatchesGolden` | `template_test.go` | **PHASE1.md DoD #5**: output byte-identical to `testdata/importer/template_python_golden.csv`. |

## Tests written first (RED) — per function

Per the confirmed TDD rule: each function lands as its own
red→green→refactor commit triple.

### `TemplateCSV`

| Test | Asserts |
|------|---------|
| `TestTemplateCSV_MatchesGolden` | Output byte-identical to `testdata/importer/template_python_golden.csv`. |

### `ImportUsersTeams`

| Test | Asserts |
|------|---------|
| **`TestCSVImport_HeadersByteIdentical`** | Column order `username,name,email,affiliation,password` exact (PHASE1.md anchor). |
| `TestCSVImport_BOMStripped` | `testdata/importer/with_bom.csv` parses without `csvio.ErrHeaderMismatch`. |
| `TestImportUsersTeams_CreateNewUser` | Username not in API list → `gw.CreateTeamAndUser` and `gw.SetUserPassword` called once each. |
| `TestImportUsersTeams_UpdateExistingUser` | Username in API list (both-false case) → `gw.UpdateTeam` called (not `CreateTeamAndUser`). |
| `TestImportUsersTeams_IgnoreExisting` | `IgnoreExisting=true` → existing username skipped; no `UpdateTeam` or `CreateTeamAndUser` call. |
| `TestImportUsersTeams_DeleteExisting` | `DeleteExisting=true` → `gw.DeleteUsers` + `gw.DeleteTeams` called before creation pass. |
| `TestImportUsersTeams_GeneratesPassword` | Empty CSV password column → `gw.SetUserPassword` receives a non-empty generated string. |
| `TestImportUsersTeams_NewPasswordFlag` | `NewPassword=true` → generated password even when CSV column is non-empty. |
| `TestImportUsersTeams_AffiliationLookupFound` | `gw.GetAffiliation` returns an affiliation → returned ID passed to create/update; `gw.CreateAffiliation` not called. |
| `TestImportUsersTeams_AffiliationLookupNotFound` | `gw.GetAffiliation` returns `(nil, nil)` → `gw.CreateAffiliation` called with `opts.AffiliationCountry`. |
| `TestImportUsersTeams_EmptyEmail_BecomesNil` | CSV row with empty `email` column → `CreateUser.Email == nil` in returned slice. |
| `TestImportUsersTeams_ContextCancelled` | Cancelled context → first gateway call surfaces `context.Canceled`; function returns an error. |

### `DeleteTeamsAndUsers`

| Test | Asserts |
|------|---------|
| `TestDeleteTeamsAndUsers_DefaultProtectsAdmin` | `admin`, `judgehost`, and `cfg.Username` are never included in the slice passed to `gw.DeleteUsers`, regardless of `Include`. |
| `TestDeleteTeamsAndUsers_IncludeExclude` | `Include=["alice","bob"], Exclude=["bob"]` → only `alice` reaches `gw.DeleteUsers`. |
| `TestDeleteTeamsAndUsers_EmptyInclude_DeletesAll` | `opts.Include=nil` → all non-protected users passed to `gw.DeleteUsers`. |
| `TestDeleteTeamsAndUsers_DerivesTeamIDs` | `gw.DeleteTeams` receives the team-ids derived from the API list, in both include and exclude lists. |
| `TestDeleteTeamsAndUsers_BothGatewayCalls` | `gw.DeleteUsers` is called before `gw.DeleteTeams` in all paths. |

### `exporter.Users`

| Test | Asserts |
|------|---------|
| `TestUsers_CSV_RoundTrips` | CSV header row matches `model.User` field declaration order; pointer fields render as value-or-empty; `[]string` roles render as JSON array string. |
| `TestUsers_JSON_RoundTrips` | Output is valid JSON array; fields match `model.User` JSON tags. |
| `TestUsers_CSV_EmptySlice_WritesHeaderOnly` | No panic; output is header line only. |
| `TestUsers_JSON_EmptySlice_WritesEmptyArray` | Output is `[]`, not `null`. |
| `TestUsers_UnknownFormat_ReturnsErr` | Format `"xlsx"` → non-nil error. |

### `exporter.CreateUsers`

| Test | Asserts |
|------|---------|
| `TestCreateUsers_CSV_RoundTrips` | CSV header row is `username,name,email,affiliation,password,is_exist`; data rows match values. |
| `TestCreateUsers_JSON_RoundTrips` | Output is valid JSON array matching `model.CreateUser` JSON tags. |
| `TestCreateUsers_CSV_EmptySlice_WritesHeaderOnly` | No panic; header only. |
| `TestCreateUsers_JSON_EmptySlice_WritesEmptyArray` | Output is `[]`, not `null`. |

## Edge cases covered

- **BOM stripping** — UTF-8 BOM in `with_bom.csv` stripped by `csvio.NewReader` before header comparison.
- **Empty email column → nil pointer** — not the empty string; matches Python `None` sentinel.
- **Password generation defaults** — length 10, charset `ascii_letters + digits` (from `password.DefaultLength`/`password.DefaultCharset`).
- **`NewPassword` flag overrides CSV value** — even a non-empty CSV password is replaced.
- **Affiliation country default chain** — `ImportOptions.AffiliationCountry=""` is a caller contract violation (PR-6 must resolve `*string` from config before calling). The spec documents this to prevent silent `"TWN"` drift.
- **Update-vs-create dispatch** — both branches covered by separate tests.
- **`IgnoreExisting` / `DeleteExisting` mutual behaviour** — when both are false, update path runs. When `DeleteExisting=true`, the delete pass fires before the creation pass.
- **Default-protect list** — `admin`/`judgehost`/`cfg.Username` are unconditionally excluded from deletion regardless of `Include` contents.
- **`gw.DeleteUsers` before `gw.DeleteTeams`** — call order locked by `TestDeleteTeamsAndUsers_BothGatewayCalls`.
- **Empty `Include`** — nil `Include` in `DeleteOptions` triggers "delete all non-protected" path (mirrors Python `if not include: include = existing_users`).
- **JSON empty slice** — must produce `[]` not `null`; enforced by `TestUsers_JSON_EmptySlice_WritesEmptyArray`.
- **Context cancellation** — propagated through the first gateway call; test confirms error bubbles up.

## Definition of done

- [ ] One red→green→refactor commit triple per function (auditable in `git log`).
- [ ] `go test -race ./internal/importer/... ./internal/exporter/...` green.
- [ ] Coverage ≥ 80 % on both packages.
- [ ] `TestCSVImport_HeadersByteIdentical` passes (PHASE1.md TDD anchor #2).
- [ ] `TestTemplateCSV_MatchesGolden` passes (PHASE1.md DoD #5).
- [ ] `gofmt -s` / `go vet` / `golangci-lint run` clean (0 issues).
- [ ] No real DOMjudge URLs, credentials, or PII in any committed file.
- [ ] No `fmt.Print*` in library code; no `init()` outside registry wiring.
- [ ] Every exported symbol has a doc comment; package-level `// Package importer …` and `// Package exporter …` comments present.
- [ ] `exporter.CreateUsers` exists beside `exporter.Users`; PR-6 uses `CreateUsers` (not `Users`) for the `[]model.CreateUser` output slice.
- [ ] `// PHASE1.md DoD #5` cross-reference appears in `TemplateCSV` doc comment.
- [ ] `gw.Login(ctx)` lifecycle requirement documented in `ImportUsersTeams` doc comment.

## Cross-references

- Python source:
  - `domjudge_tool_cli/commands/users/_users.py` — `create_teams_and_users`, `delete_teams_and_users`, `UserExportFormat`
  - `domjudge_tool_cli/commands/users/__init__.py` — `import_users_teams`, `import_users_teams_example`, `rm_teams_and_users`
  - `domjudge_tool_cli/templates/csv/import-users-teams.csv` — template golden
- Architecture: `docs/ARCHITECTURE.md` Layer 3 (importer, exporter).
- Roadmap: `docs/ROADMAP.md` Phase 1.
- Conventions: `docs/CONVENTIONS.md`, `docs/TESTING.md`, `docs/COMMITS.md`.
- Reusable components:

| What | Path |
|------|------|
| BOM-safe CSV reader | `internal/util/csvio/csvio.go` — `NewReader`, `ReadAll`, `Write` |
| Password generation | `internal/util/password/password.go` — `Generate(length, charset)` |
| User models | `internal/model/user.go` — `User`, `CreateUser`, `CreateUserFromUser` |
| Gateway interface | `internal/client/web/gateway.go` |
| API users list | `internal/client/api/v4/users.go` — `UsersResource.List` |
| Config struct | `internal/config/config.go` — `DomServerClient.Username`, `DomServerClient.AffiliationCountry` |

## Conventional Commit (merge)

```
feat(usecase): importer + exporter
```

## Notes for downstream PRs

- **PR-6** (CLI users subtree) wires `importer.ImportUsersTeams` into
  `import-users-teams` and `importer.TemplateCSV` into
  `import-users-teams-example`. The CLI command writes the output CSV as
  `import-users-teams-out.csv` by passing the returned `[]model.CreateUser`
  to `exporter.CreateUsers(users, exporter.FormatCSV, f)` (not `exporter.Users`,
  which takes `[]model.User`). PR-6 is also responsible for resolving
  `cfg.AffiliationCountry` (`*string`) into the plain `string` that
  `ImportOptions.AffiliationCountry` expects.
- **PR-6** also wires `importer.DeleteTeamsAndUsers` into
  `rm-teams-and-users` and `exporter.Users` into `user-list --format`.
- **PR-7** (CLI problems / scoreboard) reuses `exporter.Users` for any
  JSON/CSV problem export if needed, but does not depend on `importer`.
- **Phase 2** may add `exporter.Teams` beside `exporter.Users`; the
  `Format` type is declared at package level so it can be reused without
  a new type definition.
