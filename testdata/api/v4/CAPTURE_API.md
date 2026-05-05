# `testdata/api/v4/` API Fixture Capture Log

These fixtures lock the DOMjudge API v4 response shapes used by Phase 1.5
contract tests. Raw responses must stay local under `tmp/raw_api_v4/`; only
sanitized derivatives belong in git.

## Source

| Field | Value |
|-------|-------|
| Capture target | Throwaway DOMjudge 7.3.x demo instance |
| Required credentials | `DJ_BASE`, `DJ_USER`, `DJ_PASS` environment variables |
| Raw output | `tmp/raw_api_v4/` |
| Sanitized output | `testdata/api/v4/` |
| Capture script | `scripts/capture_api_fixtures.sh` |
| Sanitizer | `scripts/sanitize_api.py` |
| PII gate | `scripts/check_pii_api.py` |

Do not capture from a production contest server. If production data is the
only available source, hold the raw files outside git, sanitize immediately,
and run the PII gate before review.

## Procedure

```bash
export DJ_BASE="http://localhost:12345"
export DJ_USER="admin"
export DJ_PASS="<demo password>"

scripts/capture_api_fixtures.sh
python3 scripts/sanitize_api.py tmp/raw_api_v4 testdata/api/v4
python3 scripts/check_pii_api.py testdata/api/v4
python3 -m unittest discover -s scripts -p '*_test.py'
```

Optional capture knobs:

| Variable | Purpose |
|----------|---------|
| `RAW_DIR` | Override `tmp/raw_api_v4`. |
| `DJ_CONTEST_ID` | Contest used for contest-scoped API fixtures. Defaults to `2`. |
| `DJ_TEAM_ID` | Team id used for `/api/v4/users?team_id=...`. Defaults to `2`. |
| `DJ_ADMIN_USER_ID` | User id for `users/single_admin.json`. Defaults to `1`. |
| `DJ_USER_WITH_EMAIL_ID` | Optional user id for `users/single_with_email.json`. |
| `DJ_SUBMISSION_ID` | Submission id for the files ZIP endpoint. Defaults to `1`. |
| `DJ_FREEZE_CONTEST_ID` | Optional contest id for `scoreboard_with_freeze.json`. |
| `DJ_EMPTY_SUBMISSIONS_CONTEST_ID` | Optional contest id for `submissions_empty.json`. |

## Expected Fixture Layout

```text
testdata/api/v4/
  CAPTURE_API.md
  judgement.json
  judgement_type.json
  judgement_types_list.json
  judgements_list.json
  problem.json
  problems_list.json
  source_code.json
  submission.json
  submission_files.zip
  submissions_list.json
  team.json
  teams_list.json
  user.json
  users_list.json
  version.json
```

This is the committed Phase 1.5 fixture layout used by current tests. New
captures may start as a nested raw tree under `tmp/raw_api_v4/`, but sanitized
fixtures must be reviewed against the committed layout before replacing files.

## Sanitization Rules

The sanitizer applies these rules recursively to JSON string values and copies
binary fixtures unchanged:

1. Replace DOMjudge or school hostnames with `example.org`.
2. Replace IPv4 addresses with `192.0.2.1`.
3. Replace IPv6 addresses with `2001:db8::1`.
4. Replace emails with stable `userN@example.org` placeholders.
5. Replace CJK names with stable `SampleNNN` placeholders.
6. Replace school course codes with stable `courseNNN` placeholders.
7. Replace 8-digit student ids with stable `studentNNN` placeholders.
8. Replace CSRF/session token values with `TEST_CSRF_TOKEN`.
9. Replace phone numbers with `+1-555-0100`.
10. Replace real affiliation names with `Sample University`.
11. Replace `last_login_time` and `first_login_time` with `2026-01-01T00:00:00+00:00`.
12. Replace password hashes, API keys, bearer tokens, and similar secrets with `REDACTED`.
13. Replace real contest titles and shortnames with `ContestX`.
14. Replace `last_ip` and `ip` string values with `192.0.2.1`.
15. Preserve URL paths while replacing URL hosts with `example.org`.

## Verification Gates

`scripts/check_pii_api.py` must return zero findings for sanitized fixtures:

1. Raw hostname, maintainer, or school-domain tokens.
2. 8-digit student ids.
3. CJK text.
4. Private IPv4 addresses.
5. Emails outside `example.org`.
6. Bearer tokens.
7. Password-like values except `REDACTED` or `TEST_*`.
8. CSRF token values except `TEST_CSRF_TOKEN`.

## Findings - POST Idempotency

This section must be completed after running the probes against a throwaway
DOMjudge 7.3.x demo instance. Do not run these against production data.

> **Optimistic default in code**: `internal/importer/router.go` currently
> hardcodes `accountsTSVUpsertSafeOnPR0 = true`. This is the **expected**
> answer per DOMjudge 7's documented bulk-import contract (`misc-tools/
> import-contest.sh`'s upsert assumption). If the live probes below
> falsify this, flip the constant to `false`; the router then routes
> set-password / import-users-teams update flow back to the scraper. As
> an additional ops escape hatch, `domserver.json.prefer_scraper: true`
> forces the same fallback without code change.

### Test 1: `POST /api/v4/users/accounts`

Re-upload `accounts.tsv` with the same username and a changed password.

| Field | Result |
|-------|--------|
| Result | Pending demo capture |
| Evidence | `imports/accounts_post_first.json`, `imports/accounts_first_verify.json`, `imports/accounts_post_second.json`, `imports/accounts_second_verify.json` |
| Verdict for PR-3 / PR-4 | Pending demo capture |

### Test 2: `POST /api/v4/users/teams`

Re-upload a team payload with the same id and changed display name.

| Field | Result |
|-------|--------|
| Result | Pending demo capture |
| Evidence | `imports/teams_post_first.json`, `imports/teams_post_second.json` |
| Verdict for PR-3 / PR-4 | Pending demo capture |

### Test 3: `POST /api/v4/users/organizations`

Re-upload an organization payload with the same shortname.

| Field | Result |
|-------|--------|
| Result | Pending demo capture |
| Evidence | `imports/organizations_post_first.json`, `imports/organizations_post_second.json` |
| Verdict for PR-3 / PR-4 | Pending demo capture |

## Review Checklist

- `tmp/raw_api_v4/` exists locally only and is ignored by git.
- Sanitized fixtures are committed under `testdata/api/v4/`.
- `python3 -m unittest discover -s scripts -p '*_test.py'` passes.
- `python3 scripts/check_pii_api.py testdata/api/v4` passes.
- POST idempotency findings above contain concrete results from the demo
  instance before downstream PRs depend on them.
