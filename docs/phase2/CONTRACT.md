# DOMjudge 8.3.1 Compatibility and Fixture Contract

DOMjudge 8.3.1 support is implemented through the CLI `v1.6.0` milestone.
This document maintains its decoder, scraper, problem-package, fixture,
and live-write contracts. It supplements the shared
[API and fixture contract](../phase1.5/CONTRACT.md); it is not an execution
plan for adding v8 support.

## 1. Version selection and compatibility boundary

`internal/client/web/gatewayfactory` selects a complete `web.Gateway`
from the existing `domserver.json.version` field:

- `7.3` and `7.3.*` select `web/v7`.
- `8.3` and `8.3.*` select `web/v8`.
- Other values return `gatewayfactory.ErrUnsupportedVersion`.

These are routing predicates, not evidence that every upstream patch has
been validated. The supported v8 baseline is **8.3.1**. There is no runtime
version probe or auto-detection; the default configuration remains
`version="7.3.2"` and `api_version="v4"`.

Keep the shared [compatibility locks](../ROADMAP.md#compatibility-contracts):
CLI command/flag/output names, config schema/defaults/nullability, existing
CSV order, and the v7 delete quirks. Both clients implement the full
Gateway, including real import-related methods; no unsupported import
stubs remain in the intended implementation.

The current Go/dependency versions come from `go.mod`, not the obsolete
Phase 2 Go 1.23/Cobra/goquery pins.

## 2. API response contracts

| Response / field | Implemented compatibility |
| ---------------- | ------------------------- |
| Problem `time_limit` | `model.Problem` and `ProblemItem` retain an `int`; accept integer numbers and integral floats such as `5.0`. Reject fractional values such as `1.5` rather than truncating. Null maps to zero. |
| `/api/v4/version` `api_version` | Accept a JSON string or integer; preserve the string result (`"v4"` from the v7 fixture, `"4"` from v8). Null maps to an empty string. |
| Submission `source-code` | Decode an array of `{id, submission_id, filename, source}` objects, not a single object. |
| Problem metadata | Keep fixture coverage for `ordinal`, `test_data_count`, and `externalid`; an incomplete OpenAPI description is not proof that a field is absent. |
| Contest/submission external IDs | v8 uses `external_id`; current shipped consumers do not depend on those optional external IDs. Do not invent an unused model migration merely to match names. |
| Organization / team / judgement | Missing `url`/`twitter_hashtag`, `members`, or `judgehost` must not break currently used fields or downstream workflows. |

Keep narrow field decoders and the standard JSON decoder's tolerance of
unknown additive fields. Do not use a global loose-map parser or
`DisallowUnknownFields` to address isolated type drift. Preserve v7
fixtures alongside v8 fixtures.

`internal/client/api/v4/cross_version_test.go` covers string/integer API
versions, integer/integral-float time limits, rejected fractional limits,
and fixture decoding across resources. Client and consuming use-case tests
must protect the fields the application actually reads, not merely assert
that JSON decoding succeeds.

## 3. Write routing and jury form contracts

The default importer uses the scraper for user/team create and update on
both server families. On v8 it also performs affiliation, per-row password,
and email writes through the scraper. Standalone v8 `set-password` is
scraper-routed. See the
[current transport table](../phase1.5/CONTRACT.md#5-current-transport-and-importer-behavior)
for the distinct v7 standalone password path.

Bulk `accounts.json` import is unsuitable for the v8 baseline because email
is dropped. The documented single-entity user API routes do not establish
that updates preserve all required fields; they are not a replacement for
the implemented scraper path. Users/teams have no REST DELETE replacement
in the supported 8.3.1 contract.

Maintain these fixture-backed form differences:

| Flow | Required behavior |
| ---- | ----------------- |
| Fresh team/user | `team[addUserForTeam]=create-new-user`, `team[newUsername]`, and `team[penalty]=0`. |
| Existing team | Replay the edit form snapshot; preserve its existing `team[penalty]` instead of resetting it. |
| Team identity | `team[name]` contains the username and `team[displayName]` the display name. |
| Password | Override `user[plainPassword]` and the requested enabled value. Non-empty role overrides use repeated `user[user_roles][]` keys; nil/empty roles preserve checked snapshot roles. |
| Email | Write `user[email]` through the user edit form, not the team add form. |
| Affiliation | Preserve the v8 `team_affiliation[internalcomments]` snapshot field; list parsing uses the v8 fixture's shifted columns. |
| Delete | Fetch the row's confirmation page, require a POST form, replay its fields/CSRF placeholder, then POST to the captured delete URL. |

Login is required before gateway operations. Read/write redirects to
`/login` return `web.ErrLoginFailed`; they must not look like successful
empty results. Staying on an add/edit form after POST indicates a failed
create/update. Replay textareas and all selected values so unrelated form
state is not silently lost.

The scraper delete predicate lowercases membership values and preserves
empty-include-as-no-op behavior. A matching row excluded by the exclude
set is skipped. Keep v7's named inverted-filter regression tests unchanged
and protect the v8 behavior with its own fixture-driven request tests.

## 4. Problem packages

Full problem ZIP export still uses the authenticated jury path
`GET /jury/problems/{id}/export`. API problem metadata and samples ZIPs
are not interchangeable with the full package. Contest selection is
API-assisted before applying the scraper download filter.

Problem imports POST to `/api/v4/contests/{cid}/problems`:

- DOMjudge 7.x uses multipart file field `zip[]`; 8.x uses `zip`.
- New creates omit the optional `problem` value.
- Updates include `problem=<existing internal problem id>`.
- `--upsert` is tool-side reconciliation, not a native bulk-upsert API.
  It indexes only the selected contest's problem list by internal ID and
  external ID; ambiguous identities fail instead of selecting a target
  silently.
- On v8, package `domjudge-problem.ini` `externalid` is preferred for
  matching. Filename-derived identities are the fallback. Preserve the
  implementation's bounded identity read and its version-aware upload
  filename when updating an existing package.

`internal/problems/import.go` and API multipart tests define the current
create/update distinction. Existing command names, flags, and filenames do
not change to accommodate the server's field-name difference.

## 5. Fixture capture and contract tests

### 5.1 Maintained layout

| Directory | Current contents |
| --------- | ---------------- |
| `testdata/api/v4/v8/` | `version.json`, `info.json`, `users.json`, `contests.json`, and `CAPTURE_API_V8.md`. |
| `testdata/api/v4/v8/contest_1/` | Contest, problems, teams, organizations, groups, scoreboard, submissions, and source-code JSON fixtures. |
| `testdata/web/v8/` | Login/landing, problem list/export headers, user/team lists and delete confirmations, team add/edit/view, user edit, and affiliation add/list HTML plus `CAPTURE.md`. |

Existing v7 fixtures stay in their original directories. Capture only
synthetic/sample submission source. Generic file IDs preserve relationships
without exposing live contest titles, participant identifiers, or station
hosts.

### 5.2 Capture rules

Use an explicitly authorized, synthetic-data station. Default to GET-only
capture: version/info, contests, contest metadata, users, problems, teams,
organizations, groups, scoreboard, submissions, and synthetic source code.
Authenticated jury GETs may capture add/edit/delete-confirm forms without
submitting them. Reading a confirmation page is not evidence that deletion
was exercised.

Raw API responses belong in ignored `tmp/raw_api_v4_v8/`; raw jury HTML,
headers, cookies, and tokens belong in ignored `tmp/raw_web_v8/` or another
private location. Never record the actual station hostname or credentials
in tracked capture notes. Refer to it generically as the live 8.3.1
station; reserved `*.example.org` addresses are examples only.

Apply the shared
[sanitization rules](../phase1.5/CONTRACT.md#34-sanitization-rules), including
encoded email attributes, CSRF/session placeholders, source-code content,
and ZIP entry metadata. `sanitize_api.py` only sanitizes JSON; copied HTML,
headers, and binary files require separate review.

Record response types and drift findings in the adjacent capture notes.
Keep captured observations separate from assumptions and synthetic edge
cases. A refresh for another patch version must not silently redefine the
supported baseline.

### 5.3 Privacy and test boundaries

The current project-specific scanner already covers API and web fixtures,
including `.html`, `.headers`, and ZIP entries. Do not retain the old
instruction to add that support as future Phase 2 work.

```bash
python3 scripts/check_pii_api.py testdata/api/v4/v8 testdata/web/v8
mise run api-fixture-check
mise run privacy-check
```

Every finding blocks committing the fixture. The full privacy task also
runs the repository secret/history scan. All committed hosts, email, names,
IDs, tokens, timestamps, and addresses must be reserved examples or
non-identifying placeholders.

Tests use standard-library `testing` and `httptest.Server`. Serve the
sanitized HTML/JSON and assert method, path/query, multipart field names,
filenames, posted form values, and errors. Keep both server families'
regressions. Never replace deterministic delete correctness tests with a
live deletion experiment, and never contact a live server from a test.

## 6. Live write safety

DOMjudge **users, teams, groups, and organizations are global resources**.
Selecting a contest does not confine a write/delete sweep to that contest.
Default to read-only investigation. A live mutation requires all of:

1. Explicit operator authorization for the exact station, writable scope,
   operations, and values at the point of action.
2. A unique synthetic prefix, for example
   `phase2-synthetic-<timestamp>-<case>`, used for every created resource.
3. A GET/HTML-derived predicted match set reviewed before any delete POST.
   Every selected row must belong to that synthetic prefix; no existing
   real users, teams, groups, or organizations may be included.
4. A cleanup plan and readback verifying the intended field changes and
   the absence of unintended changes.

The predicted match set includes the operation, include/exclude filters,
selected IDs/names, and non-matching row count. Account for the preserved
empty-include semantics; do not infer a target set from contest selection
or command success alone.

Keep actual approval/evidence privately. A sanitized capture note records
only the server version, generic contest ID, synthetic prefix, allowed
operations, whether the predicted match set was approved, observed
readback, and cleanup result. Do not name the real operator or station.

After an authorized write test, remove only its synthetic resources under
the approved cleanup scope, then list/read them again. Record residual
synthetic groups/organizations when the supported UI/API cannot remove
them. Never claim cleanup without readback or use broad live deletion as
a diagnostic shortcut.

## 7. Maintenance

An upstream field, form, or API capability change needs a sanitized fixture
diff and regression coverage before routing changes. A proposed replacement
for scraper delete or full-package export also requires an explicit
architectural decision. Retain the working v7 path and all
[compatibility locks](../ROADMAP.md#compatibility-contracts); version
selection alone is not a migration or rollback procedure for server data.
