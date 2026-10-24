# API, Routing, and Fixture Contract

This is the maintained contract for the CLI's API/scraper integration and
fixtures. The directory name identifies its origin, not unfinished Phase
1.5 work. DOMjudge 7.3.x and 8.3.1 are implemented; the
[version-specific contract](../phase2/CONTRACT.md) adds v8 decoder, form,
and live-write constraints. See [Architecture](../ARCHITECTURE.md) for
package responsibilities and [CLI](../CLI.md) for the public interface.

## 1. Scope and sources of truth

A contract fixture freezes a sanitized server response so both its client
decoder and the consuming use-case can exercise the same shape. Preserve
nullability, field types, identifier relationships, and relevant ordering
when redacting data. A convenient hand-written payload does not establish
what a real server returns; label synthetic edge cases as such.

Current behavior is defined by the implementation and its regression
tests, especially `internal/importer/router.go`, `importer.go`,
`internal/cli/users/set_password.go`, `internal/client/api/v4/imports.go`,
and `internal/scoreboard/`. Do not revive the old assumption that every
user write uses the API or that the gateway has only five active methods.

## 2. Fixture layout

| Location | Contract |
| -------- | -------- |
| `testdata/api/v4/` | Existing v7 JSON fixtures and synthetic submission ZIP; keep the flat files used by current tests. |
| `testdata/api/v4/v8/` | Separate DOMjudge 8.3.1 JSON fixtures, including `contest_1/`; never overwrite v7 fixtures to add v8 support. |
| `testdata/web/v7/` and `testdata/web/v8/` | Version-specific sanitized jury HTML and response headers. |
| `testdata/importer/` | CSV/TSV inputs and byte-comparison goldens. |
| `testdata/config/` and `testdata/cli/general/` | Synthetic canonical configuration goldens. |
| `testdata/scoreboard/` | Legacy HTML parser inputs and expected CSV. |

Capture provenance and endpoint-specific observations belong in the
corresponding `CAPTURE_API.md`, `CAPTURE_API_V8.md`, or `CAPTURE.md` next
to the fixtures. Use generic identifiers in file names. Raw responses,
credentials, cookie jars, local configs, and operator evidence stay under
ignored `tmp/` or outside the repository.

## 3. Capture and privacy protocol

### 3.1 Authorized environment

Capture from a throwaway, reset demo instance containing only synthetic
data. Do not recapture from a production contest server. Tests themselves
never perform capture or contact a live DOMjudge/SMTP service.

Keep `DJ_BASE`, `DJ_USER`, and `DJ_PASS` in the local shell or an ignored
secret file. Never commit `.env` files or put credentials in tracked
commands, fixture metadata, or logs. Examples use reserved hosts such as
`test-station.example.org`; they are not actual station addresses.

### 3.2 Existing capture tooling

Run from the repository root against the authorized demo instance:

```bash
export DJ_BASE="http://localhost:12345"
export DJ_USER="<demo-user>"
export DJ_PASS="<demo-password>"

scripts/capture_api_fixtures.sh
python3 scripts/sanitize_api.py tmp/raw_api_v4 tmp/sanitized_api_v4
python3 scripts/check_pii_api.py tmp/sanitized_api_v4
```

The capture script uses a temporary mode-0600 `--netrc-file` and removes
it on exit; it does not replace the operator's `~/.netrc`. `RAW_DIR`
overrides the raw directory. Optional selectors are `DJ_CONTEST_ID`,
`DJ_TEAM_ID`, `DJ_ADMIN_USER_ID`, `DJ_USER_WITH_EMAIL_ID`,
`DJ_SUBMISSION_ID`, `DJ_FREEZE_CONTEST_ID`, and
`DJ_EMPTY_SUBMISSIONS_CONTEST_ID`. See the script for their defaults.

The capture script writes a nested raw tree. Review and map the sanitized
files to the fixture paths actually consumed by tests; do not blindly
replace the existing flat v7 tree or the v8 directory.

### 3.3 Write and idempotency evidence

The capture script is read-only. Any separate write probe requires explicit
operator approval and synthetic data. For accounts, teams, and
organizations, record both the first response and a repeated write with
changed values, followed by GET readback. Check identity, team linkage,
password behavior, duplicate rows, and field persistence; HTTP success
alone is insufficient.

These observations explain routing decisions; an unexecuted checklist is
not evidence that a server supports safe upsert. The current v7 standalone
password API path does not make the bulk create path safe. Apply the
[global-resource write protocol](../phase2/CONTRACT.md#6-live-write-safety)
whenever live writes or deletes are authorized.

### 3.4 Sanitization rules

| Data | Required replacement |
| ---- | -------------------- |
| Hostnames and URL hosts | `example.org` or a reserved `*.example.org` host; preserve relevant URL paths. |
| IPv4 / IPv6 addresses | `192.0.2.1` / `2001:db8::1`; `203.0.113.10` is also a reserved example. |
| Email addresses | Stable `userN@example.org` values. |
| Personal/CJK names | Stable generic names such as `Sample001`. |
| School course/cohort tokens | Generic `course001` values. |
| Student identifiers | Generic `student001` values. |
| CSRF tokens | `TEST_CSRF_TOKEN`. |
| Session cookies | Remove or replace with a non-live placeholder such as `TEST_COOKIE`. |
| Phone numbers | `+1-555-0100` or remove. |
| Institution/affiliation names | `Sample University` or another generic name. |
| Person-related login timestamps | `2026-01-01T00:00:00+00:00`. |
| Passwords, hashes, keys, and bearer tokens | `REDACTED`. |
| Contest titles and shortnames | `ContestX` or a generic synthetic title. |
| `last_ip` and `ip` | Reserved example addresses. |
| Submission source | Synthetic sample code only; never real participant submissions. |

`scripts/sanitize_api.py` recursively sanitizes JSON string values but
copies non-JSON files unchanged. Inspect ZIP entry names and contents,
HTML, headers, and encoded attributes separately. A copied binary is not
a sanitized binary. Preserve cross-file IDs and references while replacing
identifying text.

### 3.5 Required gates

`scripts/check_pii_api.py` checks API and web fixtures, including `.json`,
`.txt`, `.tsv`, `.csv`, `.zip`, `.html`, and `.headers`; ZIP entry names and
contents are inspected. Its gates cover sensitive host tokens, student-id
patterns, CJK text, IP addresses, non-example email, bearer tokens,
password-like values, and non-placeholder CSRF values. HTML review also
includes decoded `data-cfemail` addresses.

Before committing fixture changes:

```bash
mise run api-fixture-check
mise run privacy-check
```

The first task runs the fixture-tooling unit tests and scans both
`testdata/api/v4` and `testdata/web`. The second adds the repository secret
scan, including reachable git history. Every finding must be resolved;
these gates do not replace manual privacy review.

### 3.6 Capture notes

Record the DOMjudge/API version, endpoint and method, synthetic-data scope,
sanitization applied, observed response types, and the checks actually
executed with their results. Write probes also record authorization,
readback, and cleanup. Do not record actual station hosts, operator
credentials, participant identifiers, cookies, or tokens. Never turn a
planned probe or an assumed result into a completed validation claim.

## 4. Contract testing

Use standard-library `testing` and `httptest`; no testify or ginkgo.
Shared helpers use `t.Helper()` and `t.Cleanup()`. `t.TempDir()` already
registers its own cleanup. Tests do not generate or fetch replacement
capture fixtures as a side effect.

### 4.1 GET responses

Serve the committed JSON bytes from an `httptest.Server`, assert the
method/path/query, then decode through the real API resource. Where a
use-case transforms that response, consume the same fixture there too.
Cover null/missing optionals, empty lists, and server errors separately
without changing the captured baseline to hide failures.

### 4.2 Multipart requests

Follow `internal/client/api/v4/multipart_test.go` and `imports_test.go`:
parse the inbound multipart body and assert method, path, media type,
form-field names, filenames, and exact file bytes. Do not compare a
random boundary literal. Cover rejection/status errors and context
cancellation as well as success.

The maintained upload surface is:

| Endpoint under `/api/v4` | Multipart fields / filename |
| ----------------------- | --------------------------- |
| `/users/groups` | `tsv` / `groups.tsv`, or `json` / `groups.json` |
| `/users/organizations` | `json` / `organizations.json` |
| `/users/teams` | `tsv` / `teams2.tsv`, or `json` / `teams.json` |
| `/users/accounts` | `tsv` / `accounts.tsv` |
| `/contests` | `yaml` / `contest.yaml` |
| `/contests/{cid}/problems` | Version-specific ZIP field; optional `problem` identifies an existing update target. |

The presence of a client method does not mean the default importer calls
it. Problem creates omit `problem`; updates include the existing internal
problem ID. See the [v8 contract](../phase2/CONTRACT.md#4-problem-packages)
for `zip[]` versus `zip` and upsert identity rules.

### 4.3 Router and use-case tests

Assert transport decisions with table-driven tests and injected API/gateway
fakes. Also exercise the actual orchestration: a router's unused
`OpUpdateTeam` or affiliation decision is not proof that the update cohort
or scraper affiliation resolver uses the API. Keep stable reason strings
and the corresponding `op`, `path`, and `reason` log fields covered.

### 4.4 Byte-identical goldens

`TestImportUsersTeams_OutputCSV_Golden_API_vs_Scraper` in
`internal/importer/golden_test.go` runs the same input through explicitly
pinned API and scraper test paths, then compares exporter output to
`testdata/importer/golden_out.csv`. This protects mapping/serialization
parity; it does not enable API create routing in production.

Retain the canonical config, template, and import-result goldens, including
`TestDomServerClient_RoundTrip_Golden`, `TestGenConfig_GoldenDomserverJSON`,
and `TestImportUsersTeamsExample_GoldenCSV`. Canonical config goldens are
synthetic serializer output, not a copy of an operator's hand-edited file.
Keep the v7 delete-quirk regression tests linked from the
[roadmap](../ROADMAP.md#quirks-to-preserve-bit-for-bit).

## 5. Current transport and importer behavior

| Operation | Implemented path |
| --------- | ---------------- |
| User export, submissions, scoreboard reads | API v4; scoreboard CSV is reconstructed locally. |
| User/team import create, both server families | Default router selects the versioned scraper because bulk `accounts.tsv` does not safely express v7 user/team linkage. |
| User/team import update, both server families | Update cohort always uses `processRowScraper`; it does not consult `OpUpdateTeam`. |
| Import affiliation, password, and email writes | The default scraper row flow resolves/caches affiliations, creates or updates the team, writes the password, and backfills non-empty email through the user edit form. |
| Standalone `set-password` on v7 | API accounts TSV unless explicit roles or `prefer_scraper=true` select the scraper. |
| Standalone `set-password` on v8 | Scraper; the router rejects the unsafe bulk/PUT password path before other choices. |
| User/team delete | API user snapshot plus scraper deletion, with protected-account filtering and the locked empty-include semantics. |
| Problem download | API-assisted contest problem selection plus jury scraper full-package ZIP export. |
| Problem import | Version-aware API multipart upload; optional tool-side upsert. |

`prefer_scraper` is an optional config escape hatch for hybrid routing,
not version detection. It does not convert API-only reads into scraper
operations. Router reason identifiers include:

```text
cfg.prefer_scraper=true
api lacks DELETE on v7
api lacks problem zip export on v7
accounts.tsv has no email column
accounts.tsv has no role column; explicit roles requested
PR-0 finding: accounts.tsv re-upload is not idempotent
accounts.tsv user→team linkage broken on v7
default api-first
```

Those strings are identifiers, including their legacy version/PR wording.
Do not infer broader server support from their text or rename them as a
documentation cleanup.

Importer lifecycle remains login first, API user snapshot, parse input,
optional pre-delete, then create/update. It aborts on the first operation
error and returns the completed partial result with the wrapped error.
Existing rows update unless skipped or explicitly deleted/recreated.
Missing update `team_id` is recovered via the singular user endpoint,
then, when available, contest team-name/display-name lookup; unresolved
rows return `web.ErrTeamIDMissing`. Transport errors are not converted to
that missing-data sentinel.

The API create helper and its contest-resolution tests remain maintained
even though production's default router selects scraper create. That helper
uses the configured contest hint or exactly one active contest; zero or
multiple active contests require an explicit selection. Do not assume
contest list responses contain nested `state`; the implemented fallback
uses start/end timestamps.

## 6. Current format and default invariants

- Keep configuration defaults, CLI names, output names, delete protection,
  and the first five CSV columns as defined by the
  [roadmap compatibility contracts](../ROADMAP.md#compatibility-contracts).
- Current template header:
  `username,name,email,affiliation,password,category_id`. The optional
  per-row category overrides the configured/flag category; blank or absent
  values use that fallback. Import-result header:
  `username,name,email,affiliation,password,is_exist`.
  The template keeps its no-trailing-newline behavior.
- Input headers are name-driven and BOM-tolerant. `username` and `name`
  are required; optional email/affiliation/password cells become nil when
  blank. Legacy four-column CSVs and import-result CSVs remain accepted.
- CCS payloads are UTF-8 without BOM and use LF: groups start with
  `File_Version\t1\n`, teams with `File_Version\t2\n`, accounts with
  `accounts\t1\n`. Account rows are
  `team\t{fullName}\t{username}\t{password}\n`.
  Group rows contain `id` and `name`; teams2 rows contain, in order,
  `teamID`, `externalID`, `categoryID`, `teamName`, `instName`,
  `instShortName`, `country`, and `externalInstID`. The current builders
  use the username as `teamID`, leave the two external-ID cells blank,
  and preserve empty institution cells when no organization is resolved.
- `exporter.Users` preserves the following CSV column order:

  ```csv
  last_login_time,first_login_time,team,roles,id,username,name,email,last_ip,ip,enabled,team_id,affiliation,password
  ```

  Roles are JSON-array cells (`[]` for a nil role slice); nil pointer
  cells are empty strings. User/import-result CSV uses LF. JSON export of
  an empty user list is `[]`, never `null`.
- Scoreboard leading columns are
  `Rank,TeamAffiliation,TeamName,SolvedCount,Score,Category`, followed by
  API problem-label order. Use `scoreboard.WriteCSV` for CRLF output.
  Solved cells use `91/1 try` or `143/4 tries`; attempted unsolved cells
  contain the tries phrase; unattempted cells are empty. Tries include
  judged and pending submissions. Team names retain their legacy final
  whitespace-token behavior.
- Scoreboard download merges populated category sortorders in ascending
  order with `allteams=true`; hidden categories are included unless
  `--exclude-hidden` is set. A multi-group team is labeled/filtered using
  the group matching its current section. There is no summary row.
  These deliberate changes supersede the old Phase 1 byte-parity claim.
- Progress reporting is caller-injected and writes CLI bars to stderr;
  stdout data must stay usable in pipes. Non-TTY output, `--no-progress`,
  `NO_COLOR`, and `TERM=dumb` disable visual bars. Libraries do not
  construct CLI progress bars.

## 7. Maintenance

For an upstream response/form change, capture only authorized synthetic
data, review the sanitized diff, and update both decoder and consuming
use-case coverage before changing behavior. Retain both server families'
fixtures. A new API route or a successful status code alone is not grounds
to remove a scraper flow or its compatibility locks.
