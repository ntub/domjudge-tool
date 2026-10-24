# Operator's Guide: Desktop and CLI

Use `domjudge-tool` to manage DOMjudge users, teams, problem packages,
scoreboards, submission sources, and credential emails. This guide follows
operator tasks; the [CLI reference](CLI.md) owns the complete flag tables,
and [CLI snippets](SNIPPETS.md) provides additional command examples.

[繁體中文版](USER_GUIDE.zh-TW.md) · [Quick start](../README.md)

| Your goal | Read |
| --- | --- |
| First installation and connection | Sections 1–3 |
| Prepare or retire contest accounts | Sections 4–7 |
| Move problems and collect contest data | Sections 8–11 |
| Deliver credentials safely | Section 12, then Section 14 |
| Diagnose a result or find local data | Sections 13–15 |

## 1. Before you start

### 1.1. Requirements and supported servers

- Use a DOMjudge **7.3.x** or **8.3.1** server. Enter the server's actual
  version, such as `7.3.2` or `8.3.1`; the version selects the compatible
  jury-page implementation. Do not infer support for other server versions.
- Obtain authorized DOMjudge credentials. Reading users requires API access;
  account writes, deletion, and problem ZIP downloads also require the
  appropriate jury-page permissions. A successful API read does not prove
  that the account can perform every write.
- Confirm the contest ID with the contest administrator. Use its numeric ID
  where possible; the scoreboard CLI's `--cid` takes a number.
- Have a private, writable local folder for configs, credential CSVs, exports,
  problem packages, and emails. These files can contain confidential data.
- A released CLI binary does not require Go. Building from source requires
  [mise](https://mise.jdx.dev/), which installs the pinned Go **1.27.3** and
  other tools; repository tasks use `mise run`. The desktop preview
  additionally requires a native CGO toolchain and the platform's native GUI
  prerequisites. The macOS application-bundle task is macOS-only.

The CLI is the scripting interface. The desktop application is a native
preview using the same operation packages, with forms and local History.
Neither interface installs or administers the DOMjudge server itself.

### 1.2. Understand which actions change data

| Action | Effect |
| --- | --- |
| Generate config, CSV template, or email scaffold | Writes local files; does not modify DOMjudge |
| Export users, download problems, scoreboard, or submissions | Reads DOMjudge and writes local data |
| Validate or render email | Reads local inputs; rendering writes `.eml` files without SMTP |
| Import users, set a password, or import problems | Changes DOMjudge immediately when executed |
| Remove Users with Dry-run checked | Reads a snapshot and previews planned removals |
| Remove Users without Dry-run, or CLI removal | Permanently deletes remote users and teams |
| Send Emails or the SMTP CLI command | Sends actual messages containing account information |

> Import, password reset, deletion, problem update, and SMTP sending are real
> operations. Use an authorized test station before production. There is no
> general transaction, automatic rollback, or undo button. A failed batch may
> already have changed earlier rows or delivered earlier messages.

All command examples use reserved hosts and example identities. Replace
values only in private local files. `REPLACE_LOCALLY` is a placeholder, not a
usable password. Never commit real config, credentials, account CSVs,
submissions, rendered emails, or live screenshots.

### 1.3. What the illustrations show

Every illustration below is a **desktop HTML preview**, captured at
**1280 × 1000** from the isolated preview server at `127.0.0.1:9901` with
reserved example data. It uses the desktop templates with simulated
operations. It does not demonstrate a successful remote import, deletion,
password change, download, or SMTP delivery, and it does not show native
file dialogs. The real desktop app listens on `127.0.0.1:9900`.

The app's interface labels remain English in both editions of this guide.
Screenshots explain where to act; the text explains the real operation and
its limitations. Screenshot production rules are in
[Documentation](DOCUMENTATION.md).

## 2. Install and launch

### 2.1. Start the CLI

Download the archive matching your operating system and architecture from
[Releases](https://github.com/ntub/domjudge-tool/releases), extract it, and
put `domjudge-tool` on `PATH`. Alternatively, install from source:

```bash
go install github.com/ntub/domjudge-tool/cmd/domjudge-tool@latest
domjudge-tool --help
domjudge-tool version
```

From a repository checkout, build and expose that checkout's CLI in the
current terminal:

```bash
mise install
mise run setup
mise run build
./bin/domjudge-tool --help
./bin/domjudge-tool version
export PATH="$PWD/bin:$PATH"
```

`mise install` installs the pinned toolchain. `mise run setup` downloads Go
modules and frontend packages and installs Git hooks. Subsequent examples
assume the checkout is the working directory and `domjudge-tool` is on
`PATH`. Installed-binary users can substitute their own private working
directory for `tmp/operator`.

`version` reports the tool's build version; it is not a server connection
test. Use `domjudge-tool <command> --help` for the installed binary's flags.
Progress and diagnostic logs go to stderr; exports without an output file go
to stdout. `--no-progress` is useful in scripts.

### 2.2. Start the desktop app or macOS bundle

From the checkout:

```bash
mise run build-desktop
./bin/domjudge-tool-app
```

On macOS, package and open the app bundle instead of the raw executable:

```bash
mise run package-desktop
open ./bin/domjudge-tool.app
```

The bundle includes the application icon. Packaging creates a local bundle;
it does not imply signing, notarization, or installation in Applications.
The native window starts at 1280 × 800 and has a minimum size of
**800 × 560**. Resize the window or scroll the content/sidebar to reach
controls. Keyboard users can Tab to **Skip to content**.

Run one desktop instance at a time because the application uses a fixed
local port. Keep its local service on loopback; do not publish port 9900
through a proxy or tunnel. The local UI is not a multi-user management
service.

Sources: [build tasks](../mise.toml),
[`cmd/desktop/main.go:27`](../cmd/desktop/main.go#L27),
[`cmd/desktop/main.go:94`](../cmd/desktop/main.go#L94).

### 2.3. Install the bundled CLI command

Desktop artifacts contain the independent CLI. Installing the command does
not require Desktop to remain running, and does not create a TUI or share
the Desktop profile database with the CLI.

Build a native package with `mise run package-desktop`. On macOS the app stays
at `bin/domjudge-tool.app`; on Linux and Windows the portable directory is
`bin/desktop/domjudge-tool`. Archives and SHA-256 checksums are written to
`dist/desktop`. Each tagged release attaches unsigned macOS, Linux and
Windows packages for amd64 and arm64; downloading one is not an installation.
Linux requires GTK4, WebKitGTK 6.0 and the applicable system libraries; the
build baseline is Ubuntu 24.04. Windows needs WebView2.

1. Extract the matching package to a permanent, user-writable location.
   Keep the complete directory together. On macOS, move the `.app` before
   opening it; installation from App Translocation is refused.
2. Open **Settings** in the packaged app and find the **Command-Line Tool**
   card. Review the bundled CLI version, **User Command Path** and
   **This App's CLI**, tick the **I confirm** checkbox, then choose
   **Install CLI**.
3. Follow **Manual PATH Setup and Command Lookup**, or invoke the displayed
   full command path directly. The app never changes PATH, shell startup
   files or the Windows registry. It cannot check another terminal's PATH,
   aliases or command cache.
4. Run `domjudge-tool --help` and `domjudge-tool version` in the configured
   terminal. Create a separate `domserver.json` for CLI operations.

| Platform | Current-user command | Implementation |
| --- | --- | --- |
| macOS / Linux | `~/.local/bin/domjudge-tool` | Symlink through private state to the bundled CLI |
| Windows | `%LOCALAPPDATA%\domjudge-tool\cli-install\bin\domjudge-tool.exe` | Native console launcher pointing to the bundled CLI |

Each action requires its own confirmation checkbox. If the integration
changed after Settings was opened, the action is refused; review the
current paths and confirm again. Replacing the complete app at the same
location updates its CLI target. Close running Windows binaries before
replacing their files. After moving the app, open it from the new location
and use **Relink CLI to This App**. Another copy of the app never
automatically takes over the command. Deleting the app leaves a broken
command; a missing target fails visibly, and the Windows launcher never
falls back to another command found on PATH.

**Uninstall CLI Integration** removes only verified, app-managed command
integration. It does not delete Desktop, server profiles, CLI configuration
or other CLI installations. Foreign files, modified launchers, stale forms
and permission errors are reported without forced overwrite or elevation.
If another installation takes precedence in PATH, inspect it before changing
anything. The HTML preview and a raw development executable without bundle
metadata do not offer installation.

## 3. Connect to the correct server

### 3.1. Keep CLI configuration and desktop profiles separate

| Interface | Connection storage | How you choose the target |
| --- | --- | --- |
| CLI | Plain-text JSON, normally `domserver.json` in the working directory | Use `--config` / `-c` for an explicit file |
| Desktop | Profiles in the app's local SQLite database | Select **Server Profile** on each operation form |

Creating one does not create or update the other. The desktop startup does
not automatically import the CLI's `domserver.json`. Desktop SMTP settings
also remain separate from CLI SMTP flags.

Create a private CLI config, then make a read-only server request:

```bash
umask 077
mkdir -p tmp/operator
domjudge-tool gen-config \
  --host https://test-station.example.org \
  --username admin --password 'REPLACE_LOCALLY' \
  --version 8.3.1 --contest-id 1 \
  -o tmp/operator/domserver.json

domjudge-tool -c tmp/operator/domserver.json export-users \
  -o tmp/operator/users-before.csv
```

`gen-config` writes a local file without contacting DOMjudge and can
replace an existing file at that path. The export reads the server and
writes the specified CSV. Config stores the password in plain text; a
password supplied as a flag may also appear in shell history or process
arguments. Use your organization's approved local secret-handling procedure.

Leave TLS certificate verification enabled. Fix the certificate or trust
configuration rather than enabling `--disable-ssl` on a production station.
The full config field semantics are in the [CLI reference](CLI.md).

### 3.2. Add a desktop server profile

![Server Profiles preview showing an example server row, Set default, Edit, and Delete actions](images/user-guide/profiles.png)

*Desktop HTML preview: Servers lists saved connection profiles and local
profile actions; the example host is reserved.*

1. Open **Servers** and choose **+ Add Server**.
2. Enter a unique **Profile name**, **Host URL**, **Username**, and
   **Password**. Use a base URL such as `https://test-station.example.org`.
3. Set **DOMjudge version** to the actual version. Set **Contest ID** for
   contest-scoped work. Leave **Disable SSL verification** unchecked.
4. Select **Add**. The list returns with **Server profile created.**

![Add Server preview showing name, host, username, password, version, and contest fields](images/user-guide/profile-form.png)

*Desktop HTML preview: the connection form stores credentials locally.
Saving a form is not a connection or permission test.*

After saving, use **Export Users** as the first read-only check. A profile
being listed only confirms local persistence.

### 3.3. Set default, edit, or delete a profile

- **Set default** moves the list's `default` badge to that profile and shows
  **Default server updated.** Still select the profile explicitly on each
  operation form; the badge does not guarantee preselection.
- **Edit** opens the saved connection fields. Review the host, version,
  contest, and password before **Save**. The success message is
  **Server profile saved.**
- **Delete** opens the app's **Delete Server Profile** confirmation.
  **Cancel** or Escape closes it; **Delete Profile** removes the local
  connection and shows **Server profile deleted.** It does not delete
  accounts or contest data on DOMjudge. Deleted credentials have no
  in-app restore action.

The **Default Server Profile** preference in Settings is stored separately
from the Servers list's default badge. Do not use either as a substitute for
checking the selected target before a write.

Sources: [`internal/desktop/handler/profiles.go:14`](../internal/desktop/handler/profiles.go#L14),
[`internal/desktop/templates/profiles.templ:89`](../internal/desktop/templates/profiles.templ#L89),
[`cmd/desktop/run_import.go:64`](../cmd/desktop/run_import.go#L64).

## 4. Import users and teams

### 4.1. Prepare the CSV and credential-delivery plan

Generate the input template locally:

```bash
domjudge-tool import-users-teams-example \
  -o tmp/operator/import-users-teams.csv
```

**Replace the generated demonstration row completely before importing.**
Keep the canonical header and save as UTF-8 CSV. A UTF-8 BOM is accepted.
For example, these reserved rows request generated passwords:

```csv
username,name,email,affiliation,password,category_id
user1,Example Team A,user1@example.org,Example Organization,,1
user2,Example Team B,user2@example.org,Example Organization,,1
```

- `username` and `name` are required and non-empty; usernames must be unique
  within the file. Keep usernames as text when editing in a spreadsheet.
- The first five columns remain
  `username,name,email,affiliation,password`. `category_id` is an optional
  input-only per-row override; blank uses the CLI/config fallback.
- A non-empty `affiliation` resolves or creates an affiliation. An empty
  value uses the configured affiliation ID when available.
- A blank `password` causes the **importer** to generate a password. Importing
  an existing username normally updates its team/account and password;
  repeating an import is not a harmless preview.
- Each row describes the user/team mapping used by this importer. There is
  no free-form `team` column; do not add one based on the form's abbreviated
  format hint. Unknown or duplicate column names are rejected.

> Decide how to retain generated passwords **before** importing. The CLI
> writes a credential CSV after success. The desktop import currently
> displays only a summary and does **not** provide that CSV or expose
> generated passwords. For credential delivery, prefer the CLI workflow.
> If importing through the desktop, prepare and privately retain explicit
> passwords, or reset individual passwords afterward.

### 4.2. Import from the desktop

![Import Users preview with server selection, CSV picker, and Start Import button](images/user-guide/import-users.png)

*Desktop HTML preview: select a profile and CSV before starting a real
import; simulated preview results do not confirm server writes.*

1. Open **Import Users** and explicitly select the target **Server Profile**.
2. Choose the prepared **CSV File**.
3. After authorizing the remote write, select **Start Import** once.
4. Keep the progress page open until it reports completion or failure.
   The success result reports rows processed, created, and updated.
5. Open **History** for the recorded summary. Independently check the
   intended users and teams on DOMjudge before distributing credentials.

The desktop uses its saved profile and default import options. Advanced CLI
controls such as ignore-existing, delete-existing, and password generation
settings are not exposed on this form. A progress count alone does not
prove that every intended account is usable.

### 4.3. Import with the CLI and retain the output

The following **changes DOMjudge**. The subshell keeps the fixed output file
inside the private operator directory:

```bash
(
  cd tmp/operator
  domjudge-tool -c domserver.json import-users-teams import-users-teams.csv
)
```

After success, the CLI prints **Import summary:** with created, updated,
skipped, and affiliation counts. It writes
`tmp/operator/import-users-teams-out.csv`; the output header is:

```csv
username,name,email,affiliation,password,is_exist
```

This file contains effective/generated plain-text passwords. It is the
credential-delivery input for email, not the same format as **Export Users**.
`category_id` is not an output column. A subsequent successful import can
overwrite the same file; preserve an authorized private copy before reruns.

For existing accounts, `--ignore-existing` skips rather than updates them.
`--new-password` regenerates passwords, including supplied values.
`--delete-existing` deletes before recreation and is destructive; it cannot
be combined with `--ignore-existing`. Consult [CLI details](CLI.md) before
using these options. Import failures can leave earlier server changes in
place, and the CLI does not write a complete success CSV for a failed run.

Sources: [`internal/importer/parse.go:14`](../internal/importer/parse.go#L14),
[`internal/cli/users/import_users_teams.go:103`](../internal/cli/users/import_users_teams.go#L103),
[`cmd/desktop/run_import.go:42`](../cmd/desktop/run_import.go#L42).

## 5. Remove users and teams safely

### 5.1. Preview the desktop removal plan

![Remove Users preview showing include and exclude controls and the checked Dry-run option](images/user-guide/remove-users.png)

*Desktop HTML preview: Dry-run is checked initially. Clearing it enables
an irreversible operation after the app's confirmation.*

1. Open **Remove Users** and choose the exact target profile.
2. In **Include**, select only the usernames to delete. Loaded selections
   appear as tags; remove a tag to remove it from the selection. If the API
   lookup fails, enter comma-separated usernames in the fallback text field.
3. Use **Exclude** for usernames to keep. **Blank Include means all
   non-protected users**, subject to Exclude; it does not mean “none.”
4. Leave **Dry-run (preview only, no changes)** checked and select
   **Remove Users & Teams**.
5. Read **Dry-run: would delete …**, its username list, and History details.
   Review the associated teams and take an appropriate server backup.

`admin`, `judgehost`, and the connection profile's username are protected
from this removal workflow. Exclude takes priority over Include. Protection
is not a substitute for checking team relationships and the full scope.

### 5.2. Execute an authorized removal

Return to the form, reselect the same profile and filters, then clear
**Dry-run** only after reviewing the plan. Submitting opens the custom
**Permanently Delete Users & Teams?** dialog. **Cancel** or Escape leaves the
form unsubmitted. **Permanently Delete** starts the remote deletion.

Keep the progress page open. After completion, independently export the
remaining users and review the jury's users/teams pages. The displayed
counts are derived from the selected snapshot, not a read-back audit of
every deleted object. A plan can become stale if the server changes between
preview and execution.

### 5.3. CLI equivalent: deletion has no dry-run

First make a fresh read-only export and inspect it privately:

```bash
domjudge-tool -c tmp/operator/domserver.json export-users \
  --format json -o tmp/operator/users-before-removal.json
```

> The next command **permanently deletes users and their teams**. The CLI
> has no dry-run flag or interactive confirmation. Run it only after
> authorizing the server and reviewing every CSV username.

```bash
domjudge-tool -c tmp/operator/domserver.json rm-teams-and-users \
  --include-csv tmp/operator/import-users-teams-out.csv
```

`--include` accepts an explicit comma-separated username list instead.
`--include` and `--include-csv` are mutually exclusive. With neither, all
non-protected users are candidates. Protected accounts are always skipped.

The underlying versioned scrapers retain a historical empty-include filter
quirk. The command constructs its candidate list before calling them; do
not rely on that quirk, an empty filter, or an empty CSV as a safety mode.
Use desktop Dry-run for a preview and read back the server after deletion.

Sources: [`internal/desktop/templates/remove_users.templ:42`](../internal/desktop/templates/remove_users.templ#L42),
[`cmd/desktop/run_delete.go:42`](../cmd/desktop/run_delete.go#L42),
[`internal/cli/users/rm_teams_and_users.go:34`](../internal/cli/users/rm_teams_and_users.go#L34),
[`internal/client/web/v7/users.go:71`](../internal/client/web/v7/users.go#L71).

## 6. Export users as CSV or JSON

![Export Users preview showing format, user-ID filtering, team filtering, and Download](images/user-guide/export-users.png)

*Desktop HTML preview: API-backed user and team selectors filter the
export. Native Save dialogs are not shown in this illustration.*

1. Open **Export Users** and select a profile.
2. Choose **CSV** or **JSON**. Optionally select **User IDs** and a **Team**;
   these filters use IDs, not usernames. Blank filters export all users.
3. Select **Download**. In the native desktop app, choose a destination in
   the Save dialog. The proposed name is `users-YYYYMMDD-HHMMSS.csv` or
   `users-YYYYMMDD-HHMMSS.json`.
4. Confirm that the file exists. Cancelling Save creates no file and leaves
   the form available. In a browser, the browser's download behavior applies.

The form remains available after an attachment response; another click can
produce another export. The operation reads DOMjudge without changing it.

CLI equivalents write explicit files; omitting `-o` writes data to stdout:

```bash
domjudge-tool -c tmp/operator/domserver.json export-users \
  -o tmp/operator/users.csv
domjudge-tool -c tmp/operator/domserver.json export-users \
  --format json --ids 1,2 --team-id 1 -o tmp/operator/users.json
```

User and team filters are applied again locally, including for 7.3.x servers
that ignore a team filter. The export contains live user fields, including
IDs, roles, enabled state, and any available login/network metadata. Its
`password` field is only whatever DOMjudge returns; it does not recover
previously generated plain-text passwords. Do not use this export directly
as an import/email credential CSV; it has additional, incompatible columns.
Both formats require private storage.

Sources: [`internal/desktop/handler/export_users.go:50`](../internal/desktop/handler/export_users.go#L50),
[`cmd/desktop/download_response.go:21`](../cmd/desktop/download_response.go#L21),
[`internal/exporter/users.go:22`](../internal/exporter/users.go#L22).

## 7. Reset a password

![Set Password preview showing server selection, username control, and optional new password](images/user-guide/set-password.png)

*Desktop HTML preview: choosing a profile loads usernames from the API;
leaving New Password blank requests generation during the actual reset.*

1. Open **Set Password** and select a profile. The username input becomes an
   API-backed dropdown showing display names and usernames. If lookup fails,
   the text fallback accepts the exact username; it does not bypass the
   operation's need to read users from the server.
2. Select the intended account. Enter **New Password**, or leave it blank
   to generate one. The desktop fields display password text visibly.
3. Authorize the change, then select **Set Password**. After a successful
   write, copy the value shown as **Generated Password** or **Password Set**
   into approved private storage before leaving the result page.
4. Use **Set Another** for another reset. There is no password-recovery
   action in History; a lost generated value requires another reset.

The default generator uses 10 letters/digits. Use the CLI when you need an
explicit length or character set. A password pattern is a character set,
not a regular expression; see [password examples](SNIPPETS.md).

The following **changes the selected account's password** and prints the
generated value to stdout:

```bash
domjudge-tool -c tmp/operator/domserver.json set-password user1 \
  --password-length 16
```

Keep terminal logs and screenshots private. The CLI can print a generated
value before attempting the write: require a successful exit before
considering the reset complete. A manually supplied `--password` can be
exposed in shell history/process arguments.

Username lookup uses REST API v4. Password writes use the supported
version-aware route: ordinarily API on 7.3.x and jury-page scraping on
8.3.1. Explicit CLI roles can select the scraper route. Do not change the
server version to work around a permission error.

Sources: [`internal/desktop/templates/set_password.templ:44`](../internal/desktop/templates/set_password.templ#L44),
[`cmd/desktop/run_set_password.go:34`](../cmd/desktop/run_set_password.go#L34),
[`internal/cli/users/set_password.go:67`](../internal/cli/users/set_password.go#L67).

## 8. Download problem packages

![Download Problems preview showing contest, output directory, Browse, and problem filters](images/user-guide/download-problems.png)

*Desktop HTML preview: contest selection and problem filters scope a
local ZIP export. Browse opens a native directory picker in the real app.*

1. Open **Download Problems** and select a profile.
2. Choose a **Contest**, or enter its ID if the API selector falls back to
   text. A submitted contest overrides the profile's contest for this run;
   blank uses the profile's contest when present.
3. Set an absolute **Output Directory**. **Browse…** chooses a local folder;
   cancelling keeps the current value. The desktop default is
   `~/Downloads/export_problems`.
4. Optionally select **Only Problem IDs** or **Exclude Problem IDs**. Loaded
   values appear as tags; fallback text fields take comma-separated IDs.
5. Select **Download** and wait for the result. It reports downloaded/skipped
   counts and the destination; inspect the directory before reusing files.

When Only is blank and a contest is configured, the tool reads that
contest's problem IDs through the API, then downloads matching packages
through the jury UI. Without a contest or Only filter, the scraper can see
the global problem catalog. An API scoping error stops the run; a manual
selector fallback does not promise an API-free download. Check the actual
IDs, not just displayed problem labels.

CLI equivalent, using the contest in the config:

```bash
domjudge-tool -c tmp/operator/domserver.json download-problems \
  --folder tmp/operator/problems
```

The CLI flag is `--folder`. Its default is `export_problems` relative to the
working directory. ZIP names follow a sanitized `{id}_{name}.zip` pattern;
existing same-named files can be replaced. Problems without an export link
are skipped. The CLI's **Processed … problems.** count includes skipped
items, so inspect actual ZIPs rather than treating it as a file count.

Sources: [`internal/desktop/handler/download_problems.go:154`](../internal/desktop/handler/download_problems.go#L154),
[`cmd/desktop/run_download_problems.go:53`](../cmd/desktop/run_download_problems.go#L53),
[`internal/cli/problems/download.go:54`](../internal/cli/problems/download.go#L54).

## 9. Import or update problem ZIPs

![Import Problems preview showing contest selection, multiple ZIP upload, and Upsert option](images/user-guide/import-problems.png)

*Desktop HTML preview: Import uploads real problem packages when used in
the application. Upsert can overwrite matching server problems.*

1. Open **Import Problems** and select the target profile and contest.
   The form's contest overrides the profile; one of them must provide it.
2. Choose one or more **Problem ZIP Files** containing valid DOMjudge
   problem packages. Selecting ZIPs does not import them yet.
3. Leave **Upsert** off to create from package metadata. Enable it only
   when you intend to update matching contest problems and create missing
   ones. Keep a backup of packages that may be replaced.
4. After authorizing the remote write, select **Import**, wait for the
   terminal result, and confirm the contest's problems in the jury UI.

CLI examples below are **alternative remote writes**, not consecutive
preview commands. Choose the operation you intend:

```bash
domjudge-tool -c tmp/operator/domserver.json import-problems \
  tmp/operator/problems
```

```bash
domjudge-tool -c tmp/operator/domserver.json import-problems \
  tmp/operator/problems --upsert
```

The CLI requires `contest_id` in its config and accepts a single ZIP or all
top-level `.zip` files in a folder; it does not recursively import folders.
A successful run prints **Imported … problems.** For a deliberate update to
one existing problem, the CLI also supports `--problem-id` with a single
ZIP; it is mutually exclusive with `--upsert` and unavailable in the desktop
form.

Upsert matches against the configured contest's problem list. On 8.3.1 it
prefers the package's `externalid`; filename-based fallbacks support
export/import round trips. On 7.3.x filename identity is used. Ambiguous
identities fail before upload. A global external ID already used by a
problem outside the selected contest is not automatically linked or deleted.
Do not repeatedly retry a create or guess an existing problem ID; resolve
that conflict in the jury UI. See the identity rules in [CLI reference](CLI.md).

Sources: [`internal/desktop/handler/import_problems.go:90`](../internal/desktop/handler/import_problems.go#L90),
[`cmd/desktop/run_import_problems.go:39`](../cmd/desktop/run_import_problems.go#L39),
[`internal/cli/problems/import.go:45`](../internal/cli/problems/import.go#L45).

## 10. Export a scoreboard

![Scoreboard preview showing server, contest, hidden-category filter, and Download CSV](images/user-guide/scoreboard.png)

*Desktop HTML preview: Download CSV creates a file attachment; the real
native app then asks where to save it.*

1. Open **Scoreboard** and choose the profile and contest. Blank contest
   uses the profile's contest; if both are blank, the request is rejected.
2. Check **Exclude hidden team categories** only if those categories should
   be omitted from the export. The default includes them.
3. Select **Download CSV**, choose a destination in the native Save dialog,
   and verify the saved file. The suggested name is
   `scoreboard-YYYYMMDD-HHMMSS.csv`.

The CSV starts with `Rank,TeamAffiliation,TeamName,SolvedCount,Score,Category`,
followed by problem labels. Categories are represented in ranked sections;
rank restarts at 1 per section. This is a reconstruction of the server's
available scoreboard data, not a way to reveal information hidden by the
server's public/frozen scoreboard policy.

CLI equivalent, writing `tmp/operator/scoreboard.csv`:

```bash
domjudge-tool -c tmp/operator/domserver.json download-scoreboard \
  --cid 1 --path tmp/operator --filename scoreboard
```

`--cid` is required even if the config has `contest_id`. The destination
directory must already exist; `.csv` is appended to `--filename`.
`--exclude-hidden` applies the same category filter. A successful CLI run
prints **Wrote … (… rows).** Existing output files can be replaced.

The desktop records CSV generation in History before the Save dialog.
Cancelling Save can therefore leave a completed History entry without a
saved file. History's row count of zero for this operation is not the number
of scoreboard teams; check the CSV itself.

Sources: [`internal/desktop/handler/download_scoreboard.go:68`](../internal/desktop/handler/download_scoreboard.go#L68),
[`internal/cli/scoreboard/download.go:59`](../internal/cli/scoreboard/download.go#L59).

## 11. Download submission sources

![Submissions preview showing contest, output directory, problem filters, folder layout, and extraction](images/user-guide/submissions.png)

*Desktop HTML preview: use a dedicated destination and review Extract
before downloading confidential source archives.*

1. Open **Submissions**, select a profile, then select or enter the contest
   explicitly. The profile contest can preselect a matching API option, but
   a submitted contest value is required.
2. Choose an absolute **Output Directory** with the text field or
   **Browse…**. The current desktop default is the same
   `~/Downloads/export_problems` used by problem downloads; change it to a
   dedicated submissions folder to avoid mixing artifacts.
3. Optionally select **Problem IDs** to limit the desktop download.
4. Choose **Folder Layout** and decide whether to keep the full ZIPs.
5. Select **Download**, wait for completion, and inspect the result files
   and History. Downloading does not run or judge the source code.

| Layout | Directory below the chosen output prefix |
| --- | --- |
| Problem-first, default / mode `2` | `problem_{problem-shortname}/team_{team-name}` |
| Team-first / mode `1` | `team_{team-name}/problem_{problem-shortname}` |
| Contest folder / mode `0` | `contest_{contest-id}` |

Archives use `{judgement}_{submission-id}.zip`; a submission with no matching
judgement uses `none`. Entries whose team or problem cannot be matched to
the contest index are skipped.

> **Extract source files** is checked by default. Its compatibility behavior
> keeps only the alphabetically first regular file from each archive,
> renames it with the judgement/submission ID, and deletes that ZIP after
> successful extraction. To retain a complete multi-file submission, clear
> Extract in the desktop or use `--extract=false` in the CLI.

CLI equivalent retaining complete archives:

```bash
domjudge-tool -c tmp/operator/domserver.json download-submissions 1 \
  --path tmp/operator/submissions --mode 2 --extract=false
```

The contest ID is positional. `--path` prefixes the layout and defaults to
the current directory when omitted. The CLI prints **Downloaded …
submissions.** Problem-ID filtering is currently a desktop control; do not
invent a CLI problem-filter flag. Use a fresh destination for each snapshot
to avoid replacing prior files. Treat source code and team names as private.

Sources: [`internal/desktop/handler/download_submissions.go:84`](../internal/desktop/handler/download_submissions.go#L84),
[`internal/submissions/download.go:68`](../internal/submissions/download.go#L68),
[`internal/submissions/extract.go:24`](../internal/submissions/extract.go#L24).

## 12. Prepare and send credential emails

### 12.1. Choose the safe delivery workflow

![Email Tools preview showing Scaffold Template, Validate Template, Render Emails, and Send Emails](images/user-guide/email-tools.png)

*Desktop HTML preview: the first three tools prepare local artifacts;
Send Emails performs real SMTP delivery in the application.*

Use **scaffold → edit → validate → render → inspect → authorize → send**.
For credential delivery, prefer CLI import followed by CLI rendering with
an explicit config and From address. This retains actual passwords and
supplies the intended server URL to the template.

Use the credential CSV from a successful CLI import, or a privately retained
CSV with known effective passwords. Email commands do not fetch passwords,
generate missing passwords, or reset DOMjudge accounts. A blank CSV password
renders blank. **Export Users** is not a substitute for that credential CSV.

Template placeholders are `$username`, `$name`, `$email`, `$affiliation`,
`$password`, and `$server_host`. `subject.txt` is required, with `body.html`
or `body.txt`; HTML wins when both exist. Validation checks the selected
subject/body and supported placeholders. It does not certify recipient
addresses, password correctness, authorization, or mail delivery.

### 12.2. Scaffold, validate, and render locally

In **Email Tools**:

1. **Scaffold Template**: enter a private absolute **Template Directory**,
   then **Write Scaffold**. It creates `subject.txt`, `body.html`, and
   `body.txt`. Enable **Overwrite existing files** only to intentionally
   replace those files. Edit the templates in a text editor.
2. **Validate Template**: enter that directory and select **Validate**.
   Read the success message or correct the reported template error.
3. **Render Emails**: enter absolute paths for **CSV File**, **Template
   Directory**, and **Output Directory**, then select **Render**. The CSV
   field is a local file path, not an upload picker. Existing `.eml` files
   are refused unless overwrite is enabled.

Desktop rendering currently uses `noreply@localhost` as From and an empty
server config, so `$server_host` is blank. Desktop sending also supplies an
empty `$server_host`, but takes From from Settings. The email tools do not
have a server-profile selector. Use the CLI below when the template needs
the configured DOMjudge host; do not assume a default desktop profile fills
that variable.

CLI local-only preparation:

```bash
domjudge-tool email-template-example -o tmp/operator/email-template
domjudge-tool email-template-validate tmp/operator/email-template
domjudge-tool -c tmp/operator/domserver.json render-user-accounts \
  tmp/operator/import-users-teams-out.csv tmp/operator/email-template \
  --output-dir tmp/operator/rendered-emails \
  --from-email contest@example.org
```

Edit between scaffold and validation. `email-template-example` prints its
directory; validation prints **template OK:** on success. Rendering writes
`{safe-username}.eml` files and prints **Rendered … emails to …** without
contacting SMTP. It still needs the CLI config for `$server_host`.

For the scheme-qualified config above, edit both scaffold bodies before
use: replace `https://$server_host` with `$server_host`. The scaffold adds
its own `https://`; leaving it produces an invalid `https://https://…`
link. Template validation checks placeholders, not URL correctness.

Open the `.eml` files for review without sending them. Inspect From, To,
server URL, username, and the actual password. A missing CSV email falls
back to `{username}@{From-domain}` rather than being skipped. Explicitly fill
all approved recipient addresses to avoid unintended fallback delivery.
Keep the output directory private; rendered messages contain credentials.

### 12.3. Configure SMTP and authorize real sending

In **Settings**, save SMTP **Host**, **Port**, **From Email**, and optional
**Username** / **Password**. These credentials belong to the mail server,
not DOMjudge. AUTH is attempted only when both username and password are
non-empty.

**Use TLS** in Settings and `--use-ssl` in the CLI both mean **TLS on
connect**, commonly on port 465. **STARTTLS is not implemented**; a server
requiring STARTTLS on port 587 is not supported by this transport. Do not
turn TLS off to bypass that mismatch. Plain SMTP can expose both AUTH
credentials and message contents.

> **Send** has no dry-run or separate final confirmation dialog. After
> reviewing the exact recipients, template, credentials, host, port, and TLS
> mode, obtain authorization before clicking it. It reads the CSV/template
> again and sends newly rendered messages; it does not send the `.eml` files
> you previously reviewed. Keep the reviewed inputs unchanged.

For the desktop, open **Send Emails**, enter the CSV and template directory
paths, then select **Send** only when authorized. Keep the progress page
open. A failure stops the batch at its first error; earlier messages may
already be accepted by SMTP. Check the failure and sent recipients before
rerunning, or recipients can receive duplicates.

CLI alternative below **sends real email**. Set `SMTP_PASSWORD` privately
using your approved procedure; the shell guard refuses an empty value.
Expanding a variable still places the value in process arguments, so do not
record or share the invocation with secrets:

```bash
domjudge-tool -c tmp/operator/domserver.json send-user-accounts \
  tmp/operator/import-users-teams-out.csv tmp/operator/email-template \
  --host smtp.example.org --port 465 --use-ssl \
  --smtp-username mailer \
  --smtp-password "${SMTP_PASSWORD:?Set SMTP_PASSWORD locally}" \
  --from-email contest@example.org --timeout 30s
```

Success prints **Sent … emails.** SMTP acceptance is not proof of inbox
delivery. Verify through the mail system's approved operational process.
Never use a production recipient list as a harmless demonstration.

Sources: [`cmd/desktop/run_email_tools.go:32`](../cmd/desktop/run_email_tools.go#L32),
[`internal/email/template.go:20`](../internal/email/template.go#L20),
[`internal/email/send.go:44`](../internal/email/send.go#L44),
[`internal/email/smtp.go:65`](../internal/email/smtp.go#L65),
[`internal/cli/emails/send_user_accounts.go:95`](../internal/cli/emails/send_user_accounts.go#L95).

## 13. Read operation History

![Empty Operation History preview showing the operation-type filter and No operations recorded yet](images/user-guide/history.png)

*Desktop HTML preview: History is empty because capture performs no
operations. The type filter appears above the local operation log.*

Open **History** to see recorded desktop imports, removals, problem
transfers, scoreboard generation, submission downloads, and email sending.
Use **Type → Filter** to narrow the displayed operations, then **Clear** to
remove the filter. **Detail →** opens timestamps, status, row count, error,
summary, and saved parameters when present.

- `RUNNING` means the log has not recorded a terminal outcome. It is not a
  guarantee that a background worker is still running.
- `DONE` records successful completion of that operation's code path.
  Check saved files and remote state separately where relevant.
- `ERROR` contains the failure text. Earlier changes or sends may remain.

The Type menu lists the six account/problem/contest operation types. Email
send entries can appear in the unfiltered list even though that menu has no
email-send option. Email operations do not have a server profile and can
show an empty Server cell.

History is local to the desktop database, not a complete DOMjudge audit log.
It does not collect CLI runs, user exports, password values/resets, profile
edits, or local email scaffold/validate/render actions. Row counts are
operation-specific and do not always equal users, files, or scoreboard rows.
There are no History controls for undo, resume, or automatic retry.

Keep a progress page open until it reaches a terminal result. The work is
tied to its live request, not a durable job queue. If a page closes or the
app stops, inspect History, outputs, and remote state before submitting
again. Do not refresh old run pages to recover a result; that can reconnect
to an operation endpoint rather than merely displaying an audit entry.

Sources: [`internal/desktop/templates/history.templ:17`](../internal/desktop/templates/history.templ#L17),
[`internal/desktop/handler/history.go:20`](../internal/desktop/handler/history.go#L20),
[`internal/desktop/handler/email_tools.go:241`](../internal/desktop/handler/email_tools.go#L241).

## 14. Settings, local data, and credential protection

### 14.1. Save preferences and SMTP settings

![Settings preview showing Theme, Default Server Profile, SMTP configuration, and Save Settings](images/user-guide/settings.png)

*Desktop HTML preview: settings are local. The screenshot contains no real
SMTP credentials or private server configuration.*

**Settings** offers **Dark**, **Light**, and **System** theme choices, a
**Default Server Profile** preference, and SMTP configuration. Theme changes
apply immediately through local browser storage; select **Save Settings**
to persist the settings form in SQLite. If System does not give the desired
appearance on your platform, choose Dark or Light explicitly.

Default profile preference, the Servers default badge, and an operation's
selected profile are not interchangeable. Explicitly choose the operation's
profile each time. Review the SMTP host, port, From address, and TLS mode
before saving; sending uses these saved settings. No test-email button or
SMTP connection verification is provided by this form.

### 14.2. Locate and protect local data

| Data | Location or behavior |
| --- | --- |
| CLI connection | The path passed to `--config`, otherwise working-directory `domserver.json` |
| CLI import credentials | Working-directory `import-users-teams-out.csv` |
| Desktop profiles, SMTP settings, History | `domjudge-tool/domjudge-tool.db` below the OS user configuration directory |
| macOS desktop database | `~/Library/Application Support/domjudge-tool/domjudge-tool.db` |
| Linux desktop database | Normally `~/.config/domjudge-tool/domjudge-tool.db`, or below `XDG_CONFIG_HOME` |
| Windows desktop database | Normally `%AppData%\domjudge-tool\domjudge-tool.db` |
| Desktop theme | Embedded browser's local storage key `dj-theme`, plus the saved settings preference |
| Import uploads | Temporary CSV/ZIP files created for the operation; normal stream cleanup removes them |
| Exports and emails | Your selected file or output directory; not automatically removed afterward |

If the OS config directory cannot be resolved, the desktop can fall back to
a path in its working directory. Check launch errors and the actual storage
location before assuming that data was lost. Source-built and bundled apps
use the same OS-user database path.

> Desktop DOMjudge and SMTP passwords are stored as plain strings in
> SQLite. There is no application-level encryption or Keychain/credential
> vault integration. A masked password field is only a display control.
> CLI JSON, credential CSVs, `.eml` files, database backups, SQLite WAL
> sidecars, and logs can also disclose secrets or personal data.

Use a private OS account, restrictive folder permissions, and encrypted
storage/backups. The examples set `umask 077`; do not assume every output
writer independently enforces private permissions. Quit the app before
making a database backup and handle it, plus any remaining `-wal` / `-shm`
files, with SQLite-aware procedures. Copying a database shares its stored
credentials and operation history. Deleting a profile is not a secure erase
of backups or old database pages.

Protect the local loopback service and do not share the workstation with
untrusted local processes while the app is running. Keep sensitive output
outside tracked source or under ignored `tmp/`; ignored does not mean
encrypted. Never attach unsanitized screenshots or terminal logs to issues.

Sources: [`cmd/desktop/main.go:113`](../cmd/desktop/main.go#L113),
[`internal/desktop/profile/store.go:18`](../internal/desktop/profile/store.go#L18),
[`internal/desktop/settings/store.go:12`](../internal/desktop/settings/store.go#L12),
[`internal/desktop/templates/settings.templ:27`](../internal/desktop/templates/settings.templ#L27),
[`internal/desktop/db/db.go:26`](../internal/desktop/db/db.go#L26).

## 15. Troubleshooting

| Symptom | Action and limitation |
| --- | --- |
| CLI cannot find config | Check the working directory or use an explicit `-c` path. Desktop profiles do not create CLI JSON. |
| `version` succeeds but an operation cannot connect | `version` is local build information. Confirm the selected host, network access, API permissions, and jury login permissions without sharing credentials. |
| Profile saved, but selectors or writes fail | Saving a profile does not test it. Check host/version/credentials with Export Users first; jury writes can require additional rights. |
| Contest/user/problem/team dropdown cannot load | Read the warning and verify the profile. Use the fallback text field only with a confirmed exact ID/username. The operation may still require the same API. |
| `contest_id is required` | Set a contest on the form/profile for problem import or scoreboard. Submissions requires a submitted contest; the scoreboard CLI always requires `--cid`. |
| CSV header/row/duplicate error | Regenerate the canonical template, preserve its header, replace sample data, and correct the named row. Do not add a `team` column. |
| Desktop import succeeded, but no password CSV appears | Desktop import does not produce one. Use retained explicit passwords or reset accounts. Use CLI import for future credential-delivery runs; avoid a blind re-import. |
| Export lacks passwords or cannot be re-imported | Export Users returns live user fields, not the credential CSV. Keep successful CLI import output for delivery; reset unknown passwords. |
| Save cancelled, or History says done without a file | Cancel creates no file. Scoreboard History records generation before saving. Choose a valid destination and verify the file yourself. |
| Output cannot be created | Use a writable private path. Create the scoreboard CLI output directory first. Avoid relying on the macOS bundle's working directory. |
| Problem download count differs from ZIP count | Processed includes items skipped for missing export links. Verify contest scope and inspect downloaded ZIPs. |
| Problem import reports duplicate external ID | Check the selected contest and global problem catalog. Upsert does not auto-link a global duplicate; do not guess an update target. |
| Submission download loses extra source files | Default extraction keeps one file and removes the ZIP. Download again into a fresh folder with Extract cleared or `--extract=false`. |
| `$server_host` is blank in desktop emails | Desktop email tools pass no server profile. Prefer CLI render/send with the explicit config, or intentionally edit the template's server text. |
| SMTP fails on port 587 | Confirm whether the server requires STARTTLS. This tool supports TLS-on-connect or plain SMTP only; use an approved compatible endpoint. |
| Some recipients received mail before failure | Sending stops at the first error and does not roll back earlier SMTP sends. Reconcile recipients before retrying to avoid duplicates. |
| Theme differs between desktop and browser | Theme local storage belongs to that browser/webview context. Choose a theme there and save Settings; use an explicit Dark/Light choice if needed. |
| Desktop startup reports port 9900 in use | Close the already running instance or resolve the port conflict. Do not start multiple instances or expose the service publicly. |
| History stays RUNNING after closing a page/app | Inspect the server and output files before another write. History does not resume interrupted operations automatically. |

For CLI diagnostics, add `-v` for routing information or `-vv` for debug
logging and keep the output private. Report the command name, tool build
version, configured DOMjudge version, expected result, actual error, and a
sanitized reproduction. Remove passwords, recipients, student/team data,
real URLs, IPs, cookies, and file contents before sharing evidence.

## 16. References and manual maintenance

- [CLI reference](CLI.md): canonical flags, configuration fields, routing,
  CSV layouts, and detailed problem identity rules.
- [CLI snippets](SNIPPETS.md): additional copy-and-edit examples.
- [Quick start](../README.md): shortest installation and first-use path.
- [Documentation production contract](DOCUMENTATION.md): sanitized screenshot
  capture, paired-language structure, and isolated AI documentation review.

These manuals describe the checked-in command wiring and desktop forms.
They intentionally distinguish read-only previews, local persistence,
remote mutations, and confirmed file/SMTP outcomes. When behavior changes,
update both language editions together and regenerate only the reserved-data
preview screenshots through the documented production workflow.
