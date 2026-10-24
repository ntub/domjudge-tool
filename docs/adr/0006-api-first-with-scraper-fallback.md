# 0006. API-first with scraper fallback

- **Status:** Accepted
- **Date:** 2026-05-05
- **Deciders:** @project-maintainer

## Context

DOMjudge 7.3.x and 8.3.1 share API v4, but the API cannot safely perform
every operation exposed by this tool. HTML scraping is sensitive to form
and CSRF changes; using the documented API where it is sufficient reduces
that coupling. Replacing the scraper entirely would remove working
operations or silently change their semantics.

The supported servers impose these constraints:

- User/team DELETE is unavailable through the REST API. The v8 contest
  problem DELETE endpoint is a different, unlink-only operation.
- Full problem-package ZIP export is a jury UI operation. The API supplies
  contest metadata, not an equivalent ZIP export.
- Scoreboard endpoints return JSON, so the tool reconstructs the CSV.
- On v7, CCS `accounts.tsv` has no explicit team-id, email, or role
  columns. During account creation the server derives a team primary key
  from the username; a successful import can link the user to the wrong
  team. This is why create-team-and-user remains on the scraper despite
  the original API migration intent.
- On 8.3.1, bulk `accounts.json` import drops email and the tested
  `PUT /api/v4/users/{id}` path reports "already exists" without applying
  the update. The current v8 write workflow uses verified jury forms.

## Decision

Keep `internal/client/api/v4` and the versioned `web.Gateway`
implementations as separate transports. Use API-first routing only where
the API preserves the operation's complete contract.

### Current operation routing

| Operation | DOMjudge 7.3.x | DOMjudge 8.3.1 |
| --- | --- | --- |
| User, contest, problem metadata, submission, and scoreboard reads | API | API |
| `import-users-teams` create and update, including affiliation, password, and email writes | Scraper | Scraper |
| Standalone `set-password` | API accounts import unless explicit roles or `prefer_scraper` requires the scraper | Scraper |
| User/team deletion | Scraper | Scraper |
| Full problem-package download | API contest scoping, then scraper ZIP export | API contest scoping, then scraper ZIP export |
| Problem-package import/update | API with version-specific multipart fields | API with version-specific multipart fields |

`importer.DefaultRouter` exposes a transport decision and logs its reason.
Its default create-team-and-user decision is scraper-backed on both
versions. The importer also keeps its existing-row update cohort on the
scraper; it does not route those rows through bulk API updates. The API
resource clients and injectable API create path do not imply that
production account creation uses them.

`domserver.json.prefer_scraper=true` forces eligible hybrid operations onto
the scraper. API-only reads remain on the API path. This is a per-operation
selection rule, not an automatic retry after an API error: a failed write
must not be replayed through a second transport with uncertain side effects.

The factory selects the v7 or v8 gateway from the existing config `version`
field, as specified in [ADR 0008](0008-domjudge-8.3.1-support-and-version-selection.md).
Callers log in before using scraper operations. The API client uses Basic
Auth, while the scraper uses its own cookie-backed session and jury forms.
The layered transport boundaries are documented in [architecture](../ARCHITECTURE.md).

### Alternatives considered

- **Replace all scraper operations with REST calls.** Rejected: the
  supported APIs lack required operations and have unsafe write semantics
  for the import workflow.
- **Use the scraper for everything.** Rejected: API-supported reads,
  scoreboard reconstruction, and safe password imports should not depend
  on unrelated jury HTML changes.
- **Hide API/scraper selection inside one protocol client.** Rejected:
  callers and operators need to distinguish transport failures and reason
  about mutation semantics. Routing belongs above the two clients.
- **Retry a failed API write through the scraper automatically.** Rejected:
  an error does not establish that the first request made no changes.

## Consequences

- Both transports remain necessary, with versioned HTML fixtures and API
  fixtures covering their distinct contracts.
- Deprecated annotations on legacy gateway write methods describe the
  API migration history, not permission to delete active scraper callers.
- The v7 `accounts.tsv` linkage limitation remains a routing constraint;
  explicit roles and email cannot be assumed to survive bulk account TSV
  imports.
- The inverted deletion filters from
  [ADR 0005](0005-python-quirk-preservation.md), existing CLI names,
  `domserver.json` compatibility, and CSV contracts remain unchanged.
- A transport cutover requires evidence that the replacement supports the
  full operation on each supported server, including linkage, readback,
  and destructive scope. A successful HTTP status alone is insufficient.

## References

- [Current API, TSV, and fixture contract](../phase1.5/CONTRACT.md).
- [Current v8 compatibility contract](../phase2/CONTRACT.md).
- [Router decision table](../../internal/importer/router.go#L105)
  (`internal/importer/router.go:105`).
- [Importer create/update composition](../../internal/importer/importer.go#L193)
  and [scraper row workflow](../../internal/importer/importer.go#L311).
- [Standalone password routing](../../internal/cli/users/set_password.go#L80).
- [Scoreboard CSV contract](0007-scoreboard-cell-format-and-categories.md).
- [Problem-package routing](0010-problem-package-contest-scoping-and-import.md).
