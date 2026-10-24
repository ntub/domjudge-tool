# API v4 Fixture Capture Protocol

These fixtures lock the API v4 response shapes consumed by current tests.
Raw responses stay local under `tmp/raw_api_v4/`; only sanitized derivatives
belong in Git. Current transport rules are maintained in
[the compatibility contract](../../../docs/phase1.5/CONTRACT.md).

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

Capture only from an isolated throwaway server. Do not capture production
contest data. If a safe source is unavailable, do not run the capture.

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

## Fixture layout

Preserve the filenames read by current tests under `testdata/api/v4/`.
The capture script creates a nested raw tree under `tmp/raw_api_v4/`;
sanitizing that tree does not automatically make it a drop-in replacement
for the committed fixtures. Review response shape, nullable fields, and
test consumers before replacing files.

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
2. Student-id patterns containing eight or more digits.
3. CJK text.
4. Non-reserved IPv4 addresses.
5. Emails outside `example.org`.
6. Bearer tokens.
7. Password-like values except `REDACTED` or `TEST_*`.
8. CSRF token values except `TEST_CSRF_TOKEN`.
9. Basic Authorization credentials.

## Routing and capture evidence

Response fixtures do not establish whether repeating a live write is safe.
The default importer creates and updates users through the versioned scraper.
Standalone v7 password resets can use accounts TSV only when the router's
roles and scraper-preference rules permit it; v8 resets use the scraper.
`accountsTSVUpsertSafe(cfg)` is version-aware, not a probe-pending constant.
See [ADR 0006](../../../docs/adr/0006-api-first-with-scraper-fallback.md).

Do not infer completed live probes from fixture availability, and do not
change routing based on an old pending-capture checklist. Any additional
live validation requires an explicitly authorized isolated server; keep its
credentials and execution records outside tracked documentation.

## Review Checklist

- `tmp/raw_api_v4/` exists locally only and is ignored by git.
- Sanitized fixtures are committed under `testdata/api/v4/`.
- `python3 -m unittest discover -s scripts -p '*_test.py'` passes.
- `python3 scripts/check_pii_api.py testdata/api/v4` passes.
- Fixture replacements preserve the current decoder and routing contracts.
