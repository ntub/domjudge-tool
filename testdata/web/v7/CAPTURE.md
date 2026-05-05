# `testdata/web/v7/` capture log

These fixtures lock the DOMjudge HTML surface that
`internal/client/web/v7` parses. Each file is the **sanitized** snapshot
of a real page captured from a production DOMjudge server. **No raw page
ever entered git.**

## Source

| Field | Value |
|-------|-------|
| Capture date | 2026-05-03 |
| DOMjudge build (per `<!-- DOMjudge version ... -->` comment) | 7.3.4 |
| Capture method | `curl` with form-login session cookie. Single GET per page; no POST except `/login` for authentication. |
| Cookie used at capture time | `domjudge_cid=19` (contest selector) |

The capture target is a live production server. The HTML below is a
**sanitized derivative**, not the verbatim response. The raw responses
were held only on the capturing developer's local disk, in a
`.gitignore`'d `tmp/` directory, and were deleted immediately after the
files in this directory were verified clean.

## Files

| Filename | Source path | Used by |
|----------|-------------|---------|
| `login_form_initial.html` | `GET /login` (unauthenticated) | `Login` — extract `_csrf_token` hidden input |
| `post_login_landing.html` | `POST /login` redirect target | `Login` — confirm landing path is `/jury` |
| `jury_users_list.html` | `GET /jury/users` | `DeleteUsers` — iterate `table tbody tr`, read `a[0].text` (username) and `a[-1]['href']` (delete link) |
| `jury_user_edit.html` | `GET /jury/users/<id>/edit` | `SetUserPassword` — extract hidden inputs + form field names |
| `jury_teams_list.html` | `GET /jury/teams` | `DeleteTeams` — iterate rows, read `a[0].text` (team id) and `a[-2]['href']` (delete link) |
| `jury_teams_add.html` | `GET /jury/teams/add` | `CreateTeamAndUser` — extract form field names |
| `jury_team_edit.html` | `GET /jury/teams/<id>/edit` | `UpdateTeam` — extract form field names |
| `jury_team_view.html` | `GET /jury/teams/<id>` | `CreateTeamAndUser` / `UpdateTeam` — read user link in `.container-fluid a` |
| `jury_affiliations_list.html` | `GET /jury/affiliations` | `GetAffiliations` — iterate rows, read 4 td anchors |
| `jury_affiliations_add.html` | `GET /jury/affiliations/add` | `CreateAffiliation` — extract form field names |
| `jury_problems_list.html` | `GET /jury/problems` | `GetProblems` — iterate rows, read 5 td anchor texts + export href |

## Sanitization rules applied

These rules are enforced by `tmp/sanitize.py` (kept locally, not in
git). They are listed here for review.

1. **CSRF tokens** — every `<input type="hidden" name="_csrf_token"
   value="...">` value of length ≥ 20 is replaced with the literal
   string `TEST_CSRF_TOKEN`.
2. **Server FQDN** — any occurrence of the production hostname is
   replaced with `example.org`.
3. **Email addresses** — any `RFC 5322`-shaped address is replaced with
   `user@example.org`.
4. **IPv4 addresses** — any dotted-quad is replaced with `192.0.2.1`
   (RFC 5737 TEST-NET-1).
5. **Clock-block constants** — the `var initial`/`activatetime`/
   `starttime`/`endtime` script block is replaced with stable
   placeholder values (`0`, `0.0`).
6. **Navbar contests dropdown** — truncated to "No contest" plus the
   first two real entries.
7. **Navbar username** — `developer` greeting in nav replaced with
   `tester`.
8. **Form fields with personal data** — `value="..."` attributes are
   stripped from `user[name]`, `team[name]`, `team[displayName]`,
   `team_affiliation[name]`, `team_affiliation[shortname]`, `team[room]`,
   `team[icpcid]`, `user[ipAddress]`.
9. **Select dropdown options** — large dropdowns
   (`team_category`, `team_affiliation`, `team_contests`, `user_team`,
   `team_affiliation_country`) are truncated to the empty option (if
   present) plus the selected option (if any) plus the first 2–3
   non-empty entries. The parser only reads `option[selected]`, so
   dropping the rest is safe.
10. **Table `<tbody>` rows** — every `tbody` is truncated to its first
    3 rows. The parser iterates rows uniformly, so 3 rows fully exercise
    the per-row extraction logic.
11. **NTUB course/year codes** — tokens matching
    `\d{3,4}[_-][A-Za-z][\w_-]{2,}` (e.g. `112_Case2024`,
    `1131_Java_Final`, `114_0620_IMD`) are replaced with sequential
    `course###` placeholders.
12. **Internal contest titles** — `ICPC\d{4}` and `\d+OpenCV\d+`
    contest titles are replaced with the literal `ContestX`.
13. **Student ID patterns** — tokens matching `N\d{7}` or
    `(?<!\d)\d{7,10}(?!\d)` (e.g. `N1116430`, `11146008`,
    `1140100018`) are replaced with sequential `student###`
    placeholders.
14. **CJK characters** — every contiguous run of Han Unified Ideographs
    is replaced with a sequential `Sample###` placeholder.
15. **Real entity IDs in `/jury/...` URLs** — numeric IDs in
    `/jury/users/N`, `/jury/teams/N`, `/jury/teamcategories/N`,
    `/jury/affiliations/N`, `/jury/problems/N`, `/jury/judgehosts/N`
    are remapped to the small constant `1`. This collapses URL
    distinctness (a parser test that wants to compare URLs across rows
    should use a hand-written fixture instead — see "Known coverage
    gaps" below).

## Verification gates

Before any file in this directory was committed, every regex below was
required to return zero hits. The check is reproducible:

```bash
grep -rlE '[一-鿿]' testdata/web/v7/                     # CJK
grep -rl  '<raw-hostname>' testdata/web/v7/                       # FQDN
grep -rE  'N[0-9]{7}|\b[0-9]{7,10}\b' testdata/web/v7/           # student id
grep -rEo '\b[0-9]{3,4}_[A-Za-z][A-Za-z0-9_]+' testdata/web/v7/  # course code
grep -rE  'ICPC[0-9]{4}|OpenCV[0-9]{4}' testdata/web/v7/         # contest title
grep -rE  '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' testdata/web/v7/  # email
grep -rE  'value="[A-Za-z0-9_-]{30,}"' testdata/web/v7/          # high-entropy hidden
```

## Known coverage gaps (handled by hand-authored fixtures, not here)

- **Country flag `<img alt>` branch in `GetAffiliations`.** The NTUB
  capture has no affiliations with country flags configured, so
  `jury_affiliations_list.html` exercises only the text-fallback branch
  of `v7.py:236-238`. PR-3 adds a small synthetic fixture
  (`affiliations_list_with_flag.html`) to exercise the `img[alt]` branch.
- **Nil-href / malformed row tolerance.** Same approach: tiny
  hand-authored fixtures for `tbody_empty.html` and `tbody_malformed.html`.
