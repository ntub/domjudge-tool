# DOMjudge v7 HTML Fixtures

These sanitized HTML snapshots protect the parsing contract of
`internal/client/web/v7`. The original capture metadata identifies DOMjudge
**7.3.4**, captured on **2026-05-03**. That provenance does not authorize a
new production capture or establish coverage of every v7 patch.

Refreshes require an explicitly authorized, isolated server with synthetic
data. Keep actual hosts, credentials, session cookies, raw HTML, and local
execution records in ignored locations. Do not recapture production data.

## Captured surfaces

| Filename | Source path | Consumer |
| -------- | ----------- | -------- |
| `login_form_initial.html` | `GET /login` | Login form fields and CSRF extraction |
| `post_login_landing.html` | Login redirect target | Successful jury landing |
| `jury_users_list.html` | `GET /jury/users` | User rows and delete links |
| `jury_user_edit.html` | `GET /jury/users/<id>/edit` | Password/email form snapshots |
| `jury_teams_list.html` | `GET /jury/teams` | Team rows and delete links |
| `jury_teams_add.html` | `GET /jury/teams/add` | Create-team form fields |
| `jury_team_edit.html` | `GET /jury/teams/<id>/edit` | Existing-team form snapshots |
| `jury_team_view.html` | `GET /jury/teams/<id>` | Linked user identity |
| `jury_affiliations_list.html` | `GET /jury/affiliations` | Affiliation rows and country fields |
| `jury_affiliations_add.html` | `GET /jury/affiliations/add` | Affiliation form fields |
| `jury_problems_list.html` | `GET /jury/problems` | Problem rows and full-package export links |

File names are stable test inputs. Capture only the required surface;
authentication may need a login POST, but reading a form does not authorize
submitting an operational write or deletion.

## Redaction contract

- Replace hosts with `example.org`, email with `user1@example.org`, and
  addresses with reserved examples such as `192.0.2.1`.
- Replace CSRF values with `TEST_CSRF_TOKEN`; remove live cookies and session
  values. Review HTML attributes and response headers, not only visible text.
- Replace names, usernames, institutions, contest titles, course/cohort
  identifiers, and student identifiers with generic placeholders.
- Review encoded email attributes such as `data-cfemail` after decoding.
  An encoded value can still disclose personal data.
- Preserve form field names, selected options, repeated values, checkbox
  roles, textarea contents, and cross-file ID relationships needed by the
  parser. Redact sensitive values without changing the behavior under test.
- Use stable clock placeholders instead of live timestamps. Reduce large
  tables or selects only when the retained rows preserve the tested behavior;
  a small capture is not proof that every parser branch is covered.
- Keep boundary-case HTML explicitly synthetic and separate from captured
  observations. Never fabricate capture provenance or claim an unexecuted
  workflow passed.

The API sanitizer handles JSON; it does not sanitize HTML or headers.
Review these files separately under the
[shared privacy protocol](../../../docs/phase1.5/CONTRACT.md#3-capture-and-privacy-protocol).

## Verification

Use the maintained scanner rather than the retired ad hoc grep checklist:

```bash
python3 scripts/check_pii_api.py testdata/web/v7
mise run api-fixture-check
mise run privacy-check
```

The scanner checks supported fixture files, including HTML, headers, and
ZIP entries, for host and identifier leaks, non-example email, addresses,
bearer/Basic credentials, password-like values, and non-placeholder CSRF
values. Resolve every finding and review the content manually before commit.
Tests use committed fixtures and `httptest`; they never fetch replacements
or contact a live server.
