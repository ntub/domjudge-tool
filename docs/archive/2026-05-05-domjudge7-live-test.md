# DOMjudge 7 Live Test Archive

Date: 2026-05-05

Status: archived live verification of `domjudge-tool` against a DOMjudge 7
test server.

## Scope

Live test of `domjudge-tool` against a DOMjudge 7 test server.

- DOMjudge web UI version observed: `7.3.4`
- Target team category: id `6`
- Source CSV: operator-provided live export
- Mailpit docs reference used during verification:
  `https://mailpit.axllent.org/docs/`

Secrets were used during the live test but are not recorded here.

## Current Remote State

The successful import changed the live test server:

- Category `6` had `0` teams before import.
- Category `6` has `51` teams after import.
- CLI `export-users` count changed from `3` users before import to `54`
  users after import.
- The imported users/teams are visible in the DOMjudge Web UI.
- Team category and team affiliation are visible correctly in the Web UI.
- User email fields are written back in the DOMjudge user edit view.

Mailpit state:

- A temporary Docker container named `domjudge-tool-mailpit` was started from
  `axllent/mailpit:latest`.
- SMTP: `localhost:1025`
- Web/API: `localhost:8025`
- `send-user-accounts` sent `51` messages successfully to Mailpit.
- Mailpit API reported `total=51`, `unread=51`.
- The Mailpit UI showed the captured messages.

## Commands And Results

Build:

```bash
just build
```

Original CSV inspection:

```text
username,email,affiliation,name
```

The source CSV had 51 data rows, a UTF-8 BOM, 4 columns, and the order
differs from the canonical import template.

Import with a non-root host:

```bash
domjudge-tool gen-config --host <domjudge-public-base> ...
domjudge-tool --verbose import-users-teams --category-id 6 <source-csv>
```

Observed error:

```text
api/v4: HTTP 404 from <domjudge-public-base>/api/v4/users
```

Import with the DOMjudge root host and the original source CSV:

```bash
domjudge-tool --verbose import-users-teams --category-id 6 <source-csv>
```

Observed error before the parser fix:

```text
import-users-teams: importer: parse csv: csvio: header row does not match expected columns: got 4 columns, want 5
```

Import with the converted CSV:

```bash
domjudge-tool --verbose import-users-teams --category-id 6 <converted-import-csv>
```

Result:

- Exit code `0`
- Wrote `import-users-teams-out.csv`
- The output CSV has 51 rows plus header

Email test against Mailpit:

```bash
domjudge-tool send-user-accounts <import-output-csv> \
  <email-template-dir> \
  --host localhost --port 1025 --from-email <noreply-address>
```

Result:

```text
Sent 51 emails.
```

Follow-up import after parser fix:

```bash
just build
domjudge-tool --verbose import-users-teams --category-id 6 <source-csv>
```

Result:

- Exit code `0`
- The original 4-column source CSV was accepted directly
- `import-users-teams-out.csv` was regenerated in the repo root

Follow-up export after email persistence fix:

```bash
domjudge-tool export-users --format csv -o <export-csv>
```

Result:

- The exported CSV includes non-empty `email` values for imported users
- The DOMjudge user edit form reflects the imported emails

## Issues Collected

### 1. Non-root host paths are not supported consistently

Using a non-root DOMjudge URL causes API requests to resolve under the
subpath and fail. Web UI paths are also absolute, so a subpath host is not
handled consistently.

Likely resolution options:

- Document that `host` must be the DOMjudge root.
- Normalize the subpath away in config handling with a warning.
- Implement true base-path support across API and web clients.

### 2. API user export does not expose team affiliation

The Web UI shows affiliations correctly, but `export-users` still shows blank
affiliation for imported users. This may be DOMjudge API behavior rather than
a tool bug.

### 3. Success paths are quiet

`import-users-teams` succeeds with no output. For 51 live rows this made
progress hard to observe.

Potential enhancement: print a concise success summary such as
created/updated/skipped counts and output file path.

## Code Areas To Inspect Next

- `internal/client/httpx/session.go`
  - `ResolveReference` with absolute paths drops configured URL path.
- `internal/client/api/v4/client.go`
  - API base construction appends `/api/v4` to `cfg.Host`.
- `internal/client/web/v7/paths.go`
  - Web paths are absolute `/login`, `/jury/...`.
- `internal/importer/parse.go`
  - `ParseUsersCSV` accepts the source 4-column layout and the 6-column
    import output layout.
- `internal/importer/importer.go`
  - import flow and returned `CreateUser` slice.
- `internal/client/web/v7/users.go`
  - `UpdateUserEmail` persists the imported email.
- `internal/cli/emails/send_user_accounts.go`
  - email command reuses the flexible `ParseUsersCSV`.

## Recommended Next Steps

1. Decide whether `host` should mean DOMjudge root only or support subpaths.
2. Consider whether to surface a concise success summary for the quiet CLI
   paths.
3. If you want to keep tightening coverage, add a focused live regression
   around `UpdateUserEmail` and the 4-column source CSV.

## Cleanup Notes

The live test server currently has the 51 imported test users/teams. If a
clean server is needed, remove those users/teams from category `6` or use the
CLI delete flow carefully with explicit include filters.

The Mailpit container is still expected to be running:

```bash
docker ps --filter name=domjudge-tool-mailpit
```

Stop it when no longer needed:

```bash
docker stop domjudge-tool-mailpit
```

## Follow-up Tests On 2026-05-05

Additional checks completed after the initial live pass:

- `go test ./internal/cli/emails ./internal/importer ./internal/client/web/v7 ./internal/client/httpx`
- `go test ./internal/cli/users -run 'TestImportUsersTeams|TestSendUserAccounts'`
- `go test ./internal/email ./internal/cli/emails -run 'TestSendUserAccounts|TestLoadTemplate'`
- `go test ./...`

All of those package-level tests passed.

The rebuilt CLI was re-verified live against the DOMjudge 7 server:

- `import-users-teams` accepted the original 4-column source CSV directly
- `export-users` showed non-empty `email` values for imported users
- `send-user-accounts` accepted `import-users-teams-out.csv` directly
- Mailpit received 51 credential emails from the 6-column output CSV

The team add form still does not expose a user email field, so the email
writeback is handled by the user edit form instead of the team create form.
