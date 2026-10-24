# CLI Snippets

Copy-paste oriented examples for common `domjudge-tool` workflows. For
the complete flag reference, see [`CLI.md`](CLI.md).

Use reserved example values in shared notes and commits. Do not commit
real DOMjudge URLs, credentials, production captures, or student data.

## Generate Config

Create a `domserver.json` for a DOMjudge 8.3.1 server:

```bash
domjudge-tool gen-config \
  --host https://judge.example.org \
  --username admin \
  --password secret \
  --version 8.3.1 \
  --contest-id 2
```

Write the config to an explicit path:

```bash
domjudge-tool gen-config \
  --host https://judge.example.org \
  --username admin \
  --password secret \
  --output /path/to/domjudge-tool/tmp/live-test/domserver.json
```

Use a config file with any command:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  version
```

## Import Users And Teams

Generate the import CSV template:

```bash
domjudge-tool import-users-teams-example
```

Import users and teams from a filled CSV:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-users-teams import-users-teams.csv
```

Write generated account credentials to `import-users-teams-out.csv` in
the current directory. Keep that output in ignored storage because it
contains account data.

Regenerate passwords while importing existing rows:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-users-teams import-users-teams.csv \
  --new-password
```

## Password Pattern

`--password-pattern` is a character set, not a regular expression. The
generator draws each password character from the supplied string.

Use numbers plus uppercase and lowercase letters, while excluding
characters that are easy to confuse in print or during verbal readout.
This example excludes `0`, `1`, `I`, `L`, `O`, `i`, `l`, and `o`:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-users-teams import-users-teams.csv \
  --new-password \
  --password-length 12 \
  --password-pattern "23456789ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz"
```

Use the same readable pattern when resetting one account:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  set-password contestant1 \
  --password-length 12 \
  --password-pattern "23456789ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz"
```

For stricter readout rules, remove more lookalikes from the character
set, such as `S` / `s` / `5`, `Z` / `z` / `2`, `B` / `8`, `G` / `6`,
and `q` / `9`. Keep the remaining set large enough for the selected
password length.

## Export Users

Export users as CSV:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  export-users \
  --output /path/to/domjudge-tool/tmp/live-test/users.csv
```

Export users as JSON:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  export-users \
  --format json \
  --output /path/to/domjudge-tool/tmp/live-test/users.json
```

## Delete Users And Teams

These commands are destructive. Check the target list before running
them, and keep live validation outputs in ignored storage.

Delete specific users and their teams:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  rm-teams-and-users \
  --include contestant1,contestant2
```

Delete all non-protected users except the listed accounts:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  rm-teams-and-users \
  --exclude contestant1,contestant2
```

Delete the usernames listed in an `import-users-teams` format CSV:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  rm-teams-and-users \
  --include-csv /path/to/domjudge-tool/tmp/live-test/import-users-teams-out.csv
```

`--include-csv` reads the CSV `username` column. Protected accounts
(`admin`, `judgehost`, and the configured username) are always skipped.

## Download Contest Data

Download the scoreboard for contest id `2`:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  download-scoreboard \
  --cid 2 \
  --path /path/to/domjudge-tool/tmp/live-test \
  --filename scoreboard-cid-2
```

Download and extract submissions for contest id `2`:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  download-submissions 2 \
  --path /path/to/domjudge-tool/tmp/live-test/submissions-cid-2
```

Download problem packages:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  download-problems \
  --folder /path/to/domjudge-tool/tmp/live-test/problems
```

## Upload Problem Packages

`import-problems` uploads problem package ZIPs into the contest selected
by `domserver.json.contest_id`. Set `--contest-id` when generating the
config, or edit `contest_id` before running these commands.

Upload one ZIP as a new problem. DOMjudge reads the problem id and
metadata from the package:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-problems /path/to/domjudge-tool/tmp/live-test/problem-zips/A_easy.zip
```

Upload one ZIP to update an existing problem id:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-problems /path/to/domjudge-tool/tmp/live-test/problem-zips/A_easy.zip \
  --problem-id A
```

Batch upload every top-level `.zip` file in a folder:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-problems /path/to/domjudge-tool/tmp/live-test/problem-zips
```

Use `--problem-id` only with a single ZIP file. Folder uploads create
or import each package from its metadata and do not accept
`--problem-id`.

## Email Account Credentials

Create an email template directory:

```bash
domjudge-tool email-template-example \
  --output /path/to/domjudge-tool/tmp/live-test/email-template
```

Validate the template:

```bash
domjudge-tool email-template-validate \
  /path/to/domjudge-tool/tmp/live-test/email-template
```

Render account emails without sending SMTP:

```bash
domjudge-tool render-user-accounts \
  /path/to/domjudge-tool/tmp/live-test/import-users-teams-out.csv \
  /path/to/domjudge-tool/tmp/live-test/email-template \
  --output-dir /path/to/domjudge-tool/tmp/live-test/eml \
  --from-email contest@example.org
```

Send account emails through SMTP:

```bash
domjudge-tool send-user-accounts \
  /path/to/domjudge-tool/tmp/live-test/import-users-teams-out.csv \
  /path/to/domjudge-tool/tmp/live-test/email-template \
  --from-email contest@example.org \
  --host smtp.example.org \
  --port 465 --use-ssl \
  --smtp-username contest@example.org
```

Review the rendered messages before sending. This example uses TLS on
connect; STARTTLS on port 587 is not supported. Supply SMTP credentials
locally, and never commit credential CSVs or rendered messages.

## Live Test Pattern

Keep live validation artifacts in an ignored directory:

```bash
mkdir -p /path/to/domjudge-tool/tmp/live-test
```

Run commands with explicit config and output paths:

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  --no-progress \
  export-users \
  --format json \
  --output /path/to/domjudge-tool/tmp/live-test/users-before.json
```

Before committing docs, scripts, fixtures, or generated outputs, run the
privacy gate:

```bash
mise run privacy-check
```
