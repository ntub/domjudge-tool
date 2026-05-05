# PR-7 — CLI problems + submissions + emails + scoreboard

> Layered position: **Layer 4** (`internal/cli`) plus two new
> Layer-3 use-case packages (`internal/submissions`, `internal/scoreboard`)
> and one widened Layer-2 interface (`web.Gateway` gains `Download`).
> Closes the Phase-1 CLI surface; no use cases beyond what Python
> 0.3.0 ships are added.

## Goal

After PR-7 the binary covers the four remaining DOMjudge admin
workflows from Python 0.3.0:

1. Bulk-download all problems' export ZIPs (`download-problems`).
2. Bulk-download all submission source files for a contest, with
   optional unzip and per-team / per-problem layout
   (`download-submissions`).
3. Mail templated credential emails to a CSV-driven user list
   (`send-user-accounts`).
4. Export the public scoreboard as CSV
   (`download-scoreboard`).

PR-7 wires the four leaf commands defined in
`docs/phase1/README.md` §"Command tree shape" — exactly one leaf per
group, mirroring the locked tree shape. Group registration for
`problems`, `submissions`, `emails`, `scoreboard` lands here.

## Naming policy — bit-for-bit vs new commands

PHASE1 cross-cutting rule §"Public API stability" applies. None of
the four leaf names exist in Python 0.3.0 verbatim — Python uses
`download_problems`, `contest_files` (under `submissions`),
`send_user_accounts`, `export` (under `scoreboard`) — so PR-7 follows
the PR-6 policy for "new in Go" commands: pick a Go-idiomatic name,
add an alias when there is a near-miss Python command.

| Go leaf                | Aliases          | Python near-miss                          |
|------------------------|------------------|-------------------------------------------|
| `download-problems`    | —                | `download_problems` (1:1)                 |
| `download-submissions` | `contest-files`  | `contest_files` (1:1; the listing-only `submission_list` and per-id `submission_file` are **out of scope** — see below) |
| `send-user-accounts`   | —                | `send_user_accounts` (1:1)                |
| `download-scoreboard`  | `scoreboard-export` | `scoreboard export` (Python subgroup → Go leaf) |

### `download-submissions` consolidation

Python ships three `submissions` commands:

- `submission_list <cid>` — prints a table.
- `submission_file <cid> <id>` — downloads one source file.
- `contest_files <cid>` — downloads every source file in the contest.

The locked command tree (`docs/phase1/README.md` §"Command tree shape")
allocates one leaf to the submissions group: `download-submissions`.
PR-7 implements the Python `contest_files` semantics under that name
(the dominant admin use case — bulk-downloading all source for grading
review). The remaining two are deferred:

- `submission_list` — out of scope; ad-hoc inspection is better served
  by a future read-only `inspect` group, not retrofitted under
  `submissions`.
- `submission_file` — out of scope; rarely used vs. `contest_files`.

Both are tracked in the eventual Phase 1.5 backlog if real users ask.

## Files created

### Layer 2 — interface widening

| Path | Change |
|------|--------|
| `internal/client/web/gateway.go` | **Modified.** Add `Download(ctx context.Context, path string) (io.ReadCloser, error)` to the `Gateway` interface. Doc comment: "Performs an authenticated GET against the jury session and returns the raw response body. Caller MUST close." |
| `internal/client/web/v7/client.go` (or new `download.go`) | **Modified.** Implement `Download` on `*v7.Client` via `c.Session().GET(ctx, path)`; surface non-2xx as `*HTTPError`. |
| `internal/client/web/v7/download_test.go` | New. RED: `httptest.Server` returns a binary body; client streams it back; non-200 wraps `*HTTPError`. |
| `internal/importer/mock_gateway_test.go` | **Modified.** Add `downloadFn` and `Download` method so `mockGateway` still satisfies `web.Gateway`. |
| `internal/cli/users/mock_gateway_test.go` | **Modified.** Same as importer's mock. |

### Layer 3 — new use-case packages

| Path | Purpose |
|------|---------|
| `internal/submissions/submissions.go` | New. Package doc + `Options` (Mode, OutputPath, Strict, Extract). |
| `internal/submissions/download.go` | New. `Download(ctx, api *apiv4.Client, cid string, opts Options) (count int, err error)` — orchestrates Teams/Problems/Judgements indexes → loop submissions → call `api.Submissions.Files(ctx, cid, id, dst)` → rename to `{judgement}_{id}.zip` → optional unzip + rename to `{judgement}_{id}{ext}`. Mirrors Python `_submissions.py:download_contest_files` bit-for-bit (path mode mapping, skip-if-team-or-problem-missing). |
| `internal/submissions/download_test.go` | New. Table-driven; happy path with 2 submissions, mode-1/2/other layout, missing team / missing problem skip, `Strict=false` continues past one HTTP error, `Strict=true` aborts, ctx-cancel honoured. |
| `internal/submissions/extract.go` | New. `extractSingleFileZip(zipPath string) (extractedPath string, err error)` — opens via `archive/zip`, asserts ≥1 entry, extracts the first regular file beside the zip with the same stem, removes the zip on success. Pure function, no network. |
| `internal/submissions/extract_test.go` | New. Single-file zip happy path, multi-file zip → first file (Python parity), empty zip → wraps `ErrEmptyArchive`, traversal entry name → wraps `pathx`-style error. |
| `internal/scoreboard/scoreboard.go` | New. Package doc + types. |
| `internal/scoreboard/parse.go` | New. `Parse(html io.Reader) (header []string, rows [][]string, err error)` — goquery parser for `tr.scoreheader` (titles), `tbody tr.scoresummary` (totals), `tbody tr` (team rows). Mirrors Python `commands/scoreboard/__init__.py` cell-by-cell. |
| `internal/scoreboard/download.go` | New. `Download(ctx, base *url.URL, cid string, disableSSL bool) (header []string, rows [][]string, err error)` — builds `${base}/public?static=1`, sets cookie `domjudge_cid={cid}`, GETs via `httpx`, calls `Parse`. Public scoreboard is **unauthenticated** — no session, no login. |
| `internal/scoreboard/parse_test.go` | New. Two fixtures: minimal (1 team), realistic (sanitized snapshot from existing `testdata/web/v7/` snippets). Includes `TestScoreboard_GoldenCSV` anchor — parse + write → byte-identical to `testdata/scoreboard/golden.csv`. |
| `internal/scoreboard/download_test.go` | New. `httptest.Server` records cookie + query string; non-200 wraps an error; ctx cancel honoured. |
| `testdata/scoreboard/sample.html` | New. Sanitized scoreboard HTML (no PII; 8-rule grep gate per `testdata/web/v7/CAPTURE.md`). |
| `testdata/scoreboard/golden.csv` | New. Expected CSV emitted by the parser for `sample.html`. |

### Layer 3 — importer extension

| Path | Change |
|------|--------|
| `internal/importer/parse.go` | New. `ParseUsersCSV(r io.Reader) ([]model.CreateUser, error)` — public wrapper over the existing private `csvio.ReadAll` + row→`CreateUser` logic. Reused by both `import-users-teams` (refactor optional, keep behaviour) and `send-user-accounts`. |
| `internal/importer/parse_test.go` | New. Golden CSV → expected `[]model.CreateUser`; BOM header tolerated; missing column rejected; nil-optional fields preserved. |

### Layer 4 — CLI subtrees

| Path | Purpose |
|------|---------|
| `internal/cli/root.go` | **Modified.** Add four group constants (`groupProblems`, `groupSubmissions`, `groupEmails`, `groupScoreboard`) + `AddGroup` calls. Group titles match the README table. |
| `internal/cli/root_test.go` | **Modified.** Extend `TestRoot_HelpHasGroupHeaders` to assert all six group headers. |
| `internal/cli/problems/problems.go` | New. `Deps` struct (NewGateway factory). `groupID = "problems"`. |
| `internal/cli/problems/register.go` | New. `init()` calls `cli.RegisterSubtree(register)`. |
| `internal/cli/problems/download.go` | New. `NewDownloadProblemsCmd(deps Deps) *cobra.Command` — flags `--exclude` / `--only` (`StringSliceVar`), `--folder` (default `"export_problems"`); calls `gw.GetProblems` → loops → `gw.Download(ctx, p.ExportFilePath)` → writes `${folder}/${id}_${name}.zip` (spaces → `-`). Skips entries with empty `ExportFilePath`. |
| `internal/cli/problems/download_test.go` | New. Table-driven w/ `httptest.Server` + `mockGateway`. Happy path, default folder, custom folder, exclude / only honoured, empty-export-path skipped, ctx cancel. |
| `internal/cli/submissions/submissions.go` | New. `Deps` struct (NewAPI). `groupID = "submissions"`. |
| `internal/cli/submissions/register.go` | New. Same registry pattern. |
| `internal/cli/submissions/download.go` | New. `NewDownloadSubmissionsCmd(deps Deps) *cobra.Command` — `cobra.ExactArgs(1)` for cid; flags `--mode int` (default 2), `--path string`, `--strict bool`, `--extract bool` (default true, mirrors Python). Delegates to `submissions.Download`. |
| `internal/cli/submissions/download_test.go` | New. Stub `submissionsDownloadFn` (package seam, matches PR-6 `importUsersTeamsFn` pattern). Argument propagation, missing cid → cobra error, ctx cancel. |
| `internal/cli/emails/emails.go` | New. `Deps` struct (NewSender factory — wraps `email.Dial`; tests inject `email.Sender` stub). `groupID = "emails"`. |
| `internal/cli/emails/register.go` | New. |
| `internal/cli/emails/send_user_accounts.go` | New. `NewSendUserAccountsCmd(deps Deps) *cobra.Command` — `cobra.ExactArgs(2)` for `<csv-file>` `<template-dir>`; flags `--host` (default `"localhost"`), `--port` (default 25), `--from-email` (default `"noreply@localhost"`), `--use-ssl bool`, `--smtp-username string`, `--smtp-password string`, `--timeout duration`. Loads template, parses CSV via `importer.ParseUsersCSV`, opens sender, calls `email.SendUserAccounts`. |
| `internal/cli/emails/send_user_accounts_test.go` | New. Stub sender records calls; CSV → users → SendUserAccounts plumbing; flag propagation; missing template-dir errors; ctx cancel. |
| `internal/cli/scoreboard/scoreboard.go` | New. `Deps` struct (NewDownloader — returns `(header, rows, err)` for tests to stub). `groupID = "scoreboard"`. |
| `internal/cli/scoreboard/register.go` | New. |
| `internal/cli/scoreboard/download.go` | New. `NewDownloadScoreboardCmd(deps Deps) *cobra.Command` — flags `--cid int` (required), `--filename string` (default `"export"`), `--path string`, `--url string` (override host). Writes CSV via `csv.NewWriter`. |
| `internal/cli/scoreboard/download_test.go` | New. Stub downloader; flag propagation; default filename `export.csv`; `--path /tmp` prefixes path; alias `scoreboard-export` runs the same code path. |
| `cmd/domjudge-tool/main.go` | **Modified.** Add four blank imports for the new subtree packages. |

## Tests written first (RED)

All tests stdlib `testing` + `httptest.Server`; no testify; no real
DOMjudge or real SMTP. New packages follow the PR-3/PR-4/PR-5/PR-6
patterns: table-driven, fixtures under `testdata/<package>/`,
`t.Run(tc.name, ...)`. The lists below are not exhaustive — see the
"Edge cases to cover" section for additional checkboxes.

### Layer-2 widening (`web.Gateway.Download`)

| Test | What it asserts | Cases |
|------|------------------|-------|
| `TestClient_Download_StreamsBody` | `*v7.Client.Download` returns the response body verbatim. | 1KiB binary fixture |
| `TestClient_Download_NonOKWrapsHTTPError` | 404 / 500 wraps `*HTTPError` with the path and status. | 404, 500 |
| `TestClient_Download_CtxCancelled` | Ctx cancel before transport returns wraps `context.Canceled`. | one case |

### `internal/submissions`

| Test | What it asserts | Cases |
|------|------------------|-------|
| `TestDownload_HappyPath_Mode2` | Two submissions → two zips written under `problem_${short}/team_${name}/`; count = 2. | one case |
| `TestDownload_PathPrefix` | `OutputPath="/tmp/foo"` prefixes the per-submission folder. | one case |
| `TestDownload_ModeMapping` | mode=1 / 2 / other produce the documented layout. | three sub-cases |
| `TestDownload_SkipsMissingTeamOrProblem` | A submission whose team_id is not in the index is skipped silently (Python parity). | one case |
| `TestDownload_StrictAbortsOnFirstError` | Strict=true: first 500 from `Files` returns immediately. | one case |
| `TestDownload_NotStrictContinues` | Strict=false: 500 on submission 1 logged, submission 2 still downloaded. | one case |
| `TestDownload_FilenameRename` | Output zip lands as `${judgement}_${id}.zip`. | one case |
| `TestDownload_ExtractRenamesAndRemovesZip` | Extract=true: zip extracted, single inner file renamed to `${judgement}_${id}${ext}`, zip deleted. | one case w/ in-memory zip |
| `TestExtract_EmptyArchiveError` | `extractSingleFileZip` on empty zip wraps `ErrEmptyArchive`. | one case |
| `TestExtract_TraversalRejected` | Inner entry `../../etc/passwd` rejected. | one case |

### `internal/scoreboard`

| Test | What it asserts | Cases |
|------|------------------|-------|
| `TestParse_FixtureGolden` | Parse `testdata/scoreboard/sample.html` → header + rows match the in-test slice literal. | one canonical case |
| **`TestScoreboard_GoldenCSV`** *(anchor)* | Parse → CSV-write → byte-identical to `testdata/scoreboard/golden.csv`. | one case |
| `TestParse_RejectsMissingScoreboardTable` | HTML with no `table.scoreboard` returns a wrapped error. | one case |
| `TestDownload_PassesCidCookie` | `httptest.Server` records `Cookie: domjudge_cid=42`. | one case |
| `TestDownload_NonOKErrors` | 500 wraps with the URL. | one case |
| `TestDownload_CtxCancelled` | Ctx cancel honoured. | one case |

### `internal/importer/parse.go`

| Test | What it asserts | Cases |
|------|------------------|-------|
| `TestParseUsersCSV_GoldenRoundTrip` | Reading the Python golden template → expected `[]CreateUser`. | one case |
| `TestParseUsersCSV_StripsBOM` | Leading UTF-8 BOM tolerated. | one case |
| `TestParseUsersCSV_RejectsHeaderMismatch` | Wrong column name → wraps `csvio.ErrHeaderMismatch`. | one case |
| `TestParseUsersCSV_PreservesNilOptionals` | Empty `email` / `affiliation` cells stay `nil`, not `*""`. | one case |

### `internal/cli/problems`

| Test | What it asserts | Cases |
|------|------------------|-------|
| `TestDownloadProblems_HappyPath` | Two problems, each ExportFilePath fetched; files written to default `./export_problems/${id}_${name}.zip`. | one case in tempdir |
| `TestDownloadProblems_FolderFlag` | `--folder /tmp/p` overrides default. | one case |
| `TestDownloadProblems_ExcludeOnlyForwarded` | `--exclude a,b --only c` propagates as `web.ProblemFilter{Only:[c], Exclude:[a,b]}`. | one case |
| `TestDownloadProblems_SkipsEmptyExportPath` | Problem with `ExportFilePath=nil` is skipped silently. | one case |
| `TestDownloadProblems_FilenameSpacesToDash` | Problem name `"foo bar"` → `${id}_foo-bar.zip`. | one case (Python parity) |
| `TestDownloadProblems_CtxCancelled` | Ctx cancel propagates. | one case |

### `internal/cli/submissions`

| Test | What it asserts | Cases |
|------|------------------|-------|
| `TestDownloadSubmissions_FlagPropagation` | `--mode 1 --path /tmp/x --strict --extract=false` lands in `submissions.Options{Mode:1, OutputPath:"/tmp/x", Strict:true, Extract:false}`. | one case via stubbed `submissionsDownloadFn` |
| `TestDownloadSubmissions_DefaultsMatchPython` | Without flags: Mode=2, Extract=true, Strict=false, OutputPath="". | one case |
| `TestDownloadSubmissions_RequiresCid` | No positional → cobra `ExactArgs(1)` error. | one case |
| `TestDownloadSubmissions_CtxCancelled` | Ctx cancel honoured. | one case |
| `TestDownloadSubmissions_AliasContestFiles` | `contest-files <cid>` runs the same RunE. | one case |

### `internal/cli/emails`

| Test | What it asserts | Cases |
|------|------------------|-------|
| `TestSendUserAccounts_HappyPath` | Stub sender receives one Send per CSV row; from-email + recipient resolved correctly. | two-row CSV |
| `TestSendUserAccounts_FlagPropagation` | `--host smtp.example.com --port 587 --use-ssl --smtp-username u --smtp-password p --timeout 10s` lands in the constructed `email.DialOptions`. | one case |
| `TestSendUserAccounts_RequiresPositionals` | Missing `<csv>` or `<template-dir>` → cobra `ExactArgs(2)` error. | three sub-cases (0/1 args; 3 args) |
| `TestSendUserAccounts_TemplateDirNotFound` | Non-existent template-dir → wraps `os.ErrNotExist`. | one case |
| `TestSendUserAccounts_CSVNotFound` | Non-existent CSV → wraps `os.ErrNotExist`. | one case |
| `TestSendUserAccounts_CtxCancelled` | Ctx cancel honoured before first Send. | one case |

### `internal/cli/scoreboard`

| Test | What it asserts | Cases |
|------|------------------|-------|
| `TestDownloadScoreboard_DefaultFilename` | `--cid 1` writes `./export.csv` with the rows from the stub downloader. | one case in tempdir |
| `TestDownloadScoreboard_PathFlag` | `--path /tmp/s` writes `/tmp/s/export.csv`. | one case |
| `TestDownloadScoreboard_FilenameFlag` | `--filename foo` writes `./foo.csv`. | one case |
| `TestDownloadScoreboard_UrlOverride` | `--url https://other/public` overrides the cfg-derived base. | one case |
| `TestDownloadScoreboard_AliasScoreboardExport` | `scoreboard-export` runs the same RunE. | one case |
| `TestDownloadScoreboard_RequiresCid` | `MarkFlagRequired("cid")` rejects missing flag. | one case |

### Root scaffolding (`internal/cli`)

Already covered by PR-6's `TestRoot_HelpHasGroupHeaders`; only the
expected substring list grows. Added cases:

- `"Problem Commands:"`, `"Submission Commands:"`,
  `"Email Commands:"`, `"Scoreboard Commands:"` all appear in the
  rendered `--help`.

## Edge cases to cover

Reviewer ticks each box.

- [ ] `download-problems --exclude a,b,c` parses to `[]string{"a","b","c"}` via cobra `StringSliceVar`. **Don't** replicate Python's `len(==1) and split(",")` quirk — cobra handles it.
- [ ] `download-problems` to a folder that does not exist: wrapped `os.MkdirAll` error, no partial files.
- [ ] `download-problems --folder ""` falls back to `"export_problems"` (Python default).
- [ ] `download-submissions <cid>` with `--extract=false`: zip stays beside the renamed `${judgement}_${id}.zip`; no temp dir created.
- [ ] `download-submissions <cid>` with `--extract` on a multi-file zip: first inner regular file wins (Python `glob(...)` ordering: deterministic alphabetical).
- [ ] `download-submissions <cid>` ignores submissions whose `team_id` or `problem_id` does not appear in the index (Python parity).
- [ ] `download-submissions <cid>` aborts on ctx cancel before the next call to `api.Submissions.Files`.
- [ ] `send-user-accounts in.csv tmpl/` with `--use-ssl --port 465 --smtp-username u --smtp-password p` propagates the SMTP creds — the SMTP creds are **distinct** from `domserver.json`'s `username`/`password` (DOMjudge admin auth). Spec the disambiguation in the command's `Long`.
- [ ] `send-user-accounts` aborts on the first `sender.Send` error and does not silently continue (Python's `tablib` loop continued; we lock the Go behaviour from `email.SendUserAccounts`).
- [ ] `send-user-accounts` honours ctx cancel between rows (assertion already in `email.SendUserAccounts`).
- [ ] `download-scoreboard --cid 0` is rejected by `MarkFlagRequired("cid")` (cobra treats `0` as the zero value of `int` so we lock with a non-zero `MarkFlagRequired` only at the cobra layer; **document** in the command's `Long` that `--cid` is required and zero-valued cid is allowed when the user explicitly passes `--cid=0`).

      *Edge note:* if the user wants the "no contest cookie" behaviour
      Python had with `cid=None`, they pass no `--cid` at all and we
      reject — Phase 1 does not support unfiltered scoreboard. Track
      in spec; do not implement.

- [ ] `download-scoreboard --url https://other/public` skips loading `domserver.json` (no PreRunE attached to this branch).
- [ ] `download-scoreboard` emits `\r\n` line endings (`csv.NewWriter` default) — locked by the golden CSV.
- [ ] `download-scoreboard` emits the locked header row `Rank,TeamAffiliation,TeamName,SolvedCount,Score,<problemTitle>...`.
- [ ] `download-scoreboard` summary row puts the totals under `SolvedCount` (Python parity: column-3 of the summary is the only non-empty leading cell).
- [ ] All four new groups appear under the locked GroupID + Title in `--help` (PR-6 tests already cover the wiring; new tests assert the strings).
- [ ] `gateway.Download` widening: both existing `mockGateway` test doubles get a `Download` method that returns a no-op `io.NopCloser(bytes.NewReader(nil))` by default; importer tests don't break.

## Definition of done

1. `just check` clean — fmt, vet, lint, test all pass.
2. `go test ./internal/... -cover` reports each new package ≥ 70 %
   (`internal/submissions`, `internal/scoreboard`, `internal/cli/problems`,
   `internal/cli/submissions`, `internal/cli/emails`,
   `internal/cli/scoreboard`).
3. Anchor tests pass:
   - `TestScoreboard_GoldenCSV` (locks scoreboard CSV format).
   - `TestParseUsersCSV_GoldenRoundTrip` (locks new public CSV API).
4. `./bin/domjudge-tool --help` shows **six** group headers — the
   four new ones beside PR-6's General + User & Team. Each lists its
   single leaf command.
5. `./bin/domjudge-tool download-problems --help` shows the three
   flags `--exclude`, `--only`, `--folder` with cobra-rendered
   descriptions.
6. `./bin/domjudge-tool download-submissions --help` shows the four
   flags `--mode`, `--path`, `--strict`, `--extract` and the alias
   `contest-files`.
7. `./bin/domjudge-tool send-user-accounts --help` shows the seven
   SMTP flags (`--host`, `--port`, `--from-email`, `--use-ssl`,
   `--smtp-username`, `--smtp-password`, `--timeout`).
8. `./bin/domjudge-tool download-scoreboard --help` shows the four
   flags (`--cid`, `--filename`, `--path`, `--url`) and the alias
   `scoreboard-export`.
9. `cmd/domjudge-tool/main.go` is still ≤ 30 LOC; only the blank
   imports grow (signal ctx → `cli.Execute(ctx)` → `os.Exit(1)`).
10. No `var rootCmd` package globals, no `init()` outside registry
    wiring, no `fmt.Print*` in `internal/cli`.
11. Conventional Commit on the merge:
    **`feat(cli): problems + submissions + emails + scoreboard`**.

## Cross-references

- **Python source**:
  - `domjudge_tool_cli/commands/problems/{__init__,_problems}.py`
  - `domjudge_tool_cli/commands/submissions/{__init__,_submissions}.py`
  - `domjudge_tool_cli/commands/emails/__init__.py`
  - `domjudge_tool_cli/commands/scoreboard/__init__.py`
  - `domjudge_tool_cli/services/api/v4/submissions.py`
    (zip extraction reference)
- **Phase 1 docs**: `docs/PHASE1.md` (architecture, DoD, cross-cutting),
  `docs/phase1/README.md` §"CLI UX/DX guardrails" (Cobra patterns).
- **PR-3 client**: `internal/client/web/v7` — adds `Download`.
- **PR-4 use-cases**: `internal/importer/{importer,template}.go` —
  adds `ParseUsersCSV` public API; `internal/exporter/users.go`
  unchanged.
- **PR-5 use-case**: `internal/email/{template,send,smtp}.go` —
  reused verbatim; CLI builds the `email.Sender` and orchestrates.
- **ADRs**: none new. Layer-3 add (`internal/submissions`,
  `internal/scoreboard`) follows the existing pattern from
  `internal/importer` + `internal/exporter`.

## Out of scope (deferred)

- Python `submission_list` (table print) and `submission_file`
  (single-id download) — see "Consolidation" above. Tracked for
  Phase 1.5 if real users ask.
- Unauthenticated scoreboard with no `--cid` — Phase 1 requires
  `--cid`. Python's `cid=None` path is undocumented behaviour.
- Customising the scoreboard CSV header order or the per-problem
  column titles — Phase 1 locks the Python output bit-for-bit.
- HTTP retry / backoff — Phase 1.5 if real users hit transient
  failures (existing PHASE1 §"Out of scope" rule).
- Concurrent batch downloads — Python is sequential, Go matches.
- Parallel SMTP sending — abort-on-first-error semantics from PR-5
  preserved.

## Commit cadence (TDD discipline)

Following PR-3's per-method red→green pairing rather than PR-6's
batch-of-N pairing, because the four leaf commands here live in
**independent packages** with no shared types:

1. `docs(phase1): draft PR-7 cli-rest spec` — this file.
2. `test(web/v7): red — Gateway.Download`
3. `feat(web/v7): green — Gateway.Download` (interface + impl + mock updates)
4. `test(importer): red — ParseUsersCSV public API`
5. `feat(importer): green — ParseUsersCSV public API`
6. `test(submissions): red — Layer-3 download + extract`
7. `feat(submissions): green — Layer-3 download + extract`
8. `test(scoreboard): red — parse + download`
9. `feat(scoreboard): green — parse + download`
10. `test(cli/problems): red — download-problems`
11. `feat(cli/problems): green — download-problems`
12. `test(cli/submissions): red — download-submissions`
13. `feat(cli/submissions): green — download-submissions`
14. `test(cli/emails): red — send-user-accounts`
15. `feat(cli/emails): green — send-user-accounts`
16. `test(cli/scoreboard): red — download-scoreboard`
17. `feat(cli/scoreboard): green — download-scoreboard`
18. (optional) `refactor(cli): tidy shared Deps patterns` — only if
    duplication across the four new subtree packages becomes obvious;
    skip if it doesn't.

Squash on merge to: **`feat(cli): problems + submissions + emails + scoreboard`**.
