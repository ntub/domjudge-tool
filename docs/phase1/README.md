# Phase 1 per-PR specs

This directory holds the per-PR specifications for the Phase 1 port. The
durable phase plan is in [`../PHASE1.md`](../PHASE1.md).

## Convention

Each PR has a single Markdown spec at `PR-N-<slug>.md` (e.g.
`PR-1-foundation.md`). Specs are drafted just-in-time, **immediately
before** the PR is implemented, so they don't drift before being executed.

A spec contains:

- **Goal** — what user-visible or internal capability the PR delivers.
- **Files created / modified** — exact paths.
- **Tests written first (RED)** — table of test names, what they assert,
  the cases they cover.
- **Edge cases to cover** — explicit list. Reviewer checks every box.
- **Definition of done** — acceptance criteria; ends with the Conventional
  Commit message used to merge.
- **Cross-references** — Python source paths, related ADRs, layered
  architecture notes.

## Workflow

1. **Codex drafts** the spec for the next PR before writing code.
2. **Claude reviews** the spec against the phase plan and approves it
   (or requests changes) before Codex begins TDD.
3. **Codex implements** in red → green → refactor commits on
   `feat/p1-<slug>`.
4. **Claude reviews** the diff against the spec's tests + edge cases +
   docs checklist and either merges or files findings at
   `PR-N-review.md`.

## Review log (one entry per PR)

| PR | Spec | Status | Review notes |
|----|------|--------|--------------|
| 0  | (CI + docs scaffolding only) | merged | — |
| 1  | `PR-1-foundation.md`     | merged | coverage 100 % model/pathx, 91 % config, 85 % csvio, 93 % password |
| 2  | `PR-2-api-v4.md`         | merged | coverage httpx 84 %, api/v4 92 % |
| 3  | `PR-3-web-v7.md`         | merged | coverage httpx 80 %, web 91 %, web/v7 89 %; 11 ops in red→green pairs; QUIRK locked; HTML fixtures sanitized from production NTUB instance (8 PII gates clean) |
| 4  | `PR-4-importer-exporter.md` | merged | coverage importer 85 %, exporter 86 %; 3 use-case functions (TemplateCSV / ImportUsersTeams / DeleteTeamsAndUsers) + 2 exporter functions; 30 tests; all TDD anchors green (`TestCSVImport_HeadersByteIdentical`, `TestTemplateCSV_MatchesGolden`) |
| 5  | `PR-5-email.md`          | merged | coverage 84.6 %; template/send 100/92 %; smtp 65 %; TDD anchors green; 4 findings (H1 Validate, H2 plainAuth, H3 ctx, M1 ErrNotExist) fixed; see `PR-5-review.md` |
| 6  | `PR-6-cli-users.md`      | merged | coverage cli 82.0 %, cli/general 86.8 %, cli/users 79.1 %; cobra v1.8.1 pinned; 7 leaf commands (root + general gen-config/version + users 5 cmds); 7 RED→GREEN commit pairs + 1 refactor; DoD #4 anchor `TestGenConfig_GoldenDomserverJSON` and DoD #5 anchor `TestImportUsersTeamsExample_GoldenCSV` byte-identical and verified end-to-end against the built binary |
| 7  | `PR-7-cli-rest.md`       | merged | coverage cli/problems 70.7 %, cli/submissions 75.9 %, cli/emails 85.0 %, cli/scoreboard 70.6 %, scoreboard 95.1 %, submissions 78.2 %; widened web.Gateway with Download (Layer-2 contract); added importer.ParseUsersCSV public API; 4 leaf commands + 2 Layer-3 packages; per-method RED→GREEN cadence (12 commits); anchor `TestScoreboard_GoldenCSV` byte-identical CSV |
| 8  | `PR-8-release.md`        | merged | release prep complete; Phase 1 marked done as `v1.0.0-rc1` |

## CLI UX/DX guardrails (PR-6, PR-7)

Locks the Cobra command tree's user-facing shape and the implementer's
patterns **before** PR-6/PR-7 specs are drafted. Source: <https://cobra.dev>;
the API surface below was re-verified against the upstream docs via
`ctx7 docs /spf13/cobra` on 2026-05-04. Pinned version: **cobra v1.8.x**
(Go 1.23 compatible — see `PHASE1.md` Deviations §1).

> Constraints already locked elsewhere: subcommand + flag names match
> Python bit-for-bit (`PHASE1.md` Cross-cutting rules); `cmd/` is a thin
> shell, all printing happens in `cmd/` or `internal/cli/`, library code
> uses `slog` only (`CLAUDE.md` "Things to avoid"); ctx is the first arg
> on every I/O call.

### 1. Wiring & file layout

- `cmd/domjudge-tool/main.go` builds a signal-aware ctx via
  `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`,
  calls `internal/cli.Execute(ctx)`, and `os.Exit(1)` on error. **No
  business logic, no `log.Fatal`, no global state.**
- `internal/cli.Execute(ctx)` builds the root command, calls
  `root.ExecuteContext(ctx)`, and returns the error. Subcommand `RunE`
  bodies read ctx via `cmd.Context()` and pass it as the **first arg**
  to every Layer-3 / Layer-2 call.
- One file per command at `internal/cli/{group}/{cmd}.go`, exporting a
  constructor `func newXxxCmd(deps Deps) *cobra.Command`. Tests call the
  constructor with stub deps; **no `var rootCmd` package globals**, and
  `init()` stays reserved for registry wiring per `CLAUDE.md`.

### 2. Command tree shape

Six top-level groups via `rootCmd.AddGroup(&cobra.Group{...})` +
`cmd.GroupID`, mirroring the Python tree:

| GroupID       | Title (in `--help`)        | Commands                                                                                       |
|---------------|----------------------------|------------------------------------------------------------------------------------------------|
| `general`     | "General Commands:"        | `gen-config`, `version`                                                                        |
| `users`       | "User & Team Commands:"    | `import-users-teams`, `import-users-teams-example`, `export-users`, `rm-teams-and-users`, `set-password` |
| `problems`    | "Problem Commands:"        | `download-problems`                                                                            |
| `submissions` | "Submission Commands:"     | `download-submissions`                                                                         |
| `emails`      | "Email Commands:"          | `send-user-accounts`                                                                           |
| `scoreboard`  | "Scoreboard Commands:"     | `download-scoreboard`                                                                          |

Pin the auto-generated commands into the same groups so `--help` doesn't
strand them under "Additional Commands":
`rootCmd.SetHelpCommandGroupID("general")`,
`rootCmd.SetCompletionCommandGroupID("general")`.

### 3. Naming & aliases — Python parity

- Subcommand and flag names match Python `domjudge_tool_cli` 0.3.0
  **bit-for-bit**. Cross-check against `docs/ROADMAP.md` Phase 1
  §"Backward compatibility" before drafting.
- Add Go-idiomatic shorthands via `Cmd.Aliases` and `Flag.Shorthand`,
  never via rename. Example: `Aliases: []string{"rm-teams"}` for
  `rm-teams-and-users`; `--config` / `-c`.

### 4. Errors, output, and ctx

- `RunE` only — never `Run`. Wrap returned errors with `%w`; the
  use-case sentinels from PR-2..PR-5 propagate to `Execute()`.
- Set `SilenceUsage: true` AND `SilenceErrors: true` on every command.
  Without these, Cobra prints `--help` on every runtime error (401 from
  DOMjudge, SMTP refused, IO error), which buries the cause. Print the
  error once in `cli.Execute` to `cmd.ErrOrStderr()` instead.
- All user-facing output goes through `cmd.OutOrStdout()` /
  `cmd.ErrOrStderr()`. **No `fmt.Println` in `internal/cli`** — tests
  hijack via `cmd.SetOut(&buf)` / `cmd.SetErr(&buf)`.

### 5. Args & flag validation

- Use the built-in validators; don't hand-roll length checks inside
  `RunE`:
  - `cobra.NoArgs`, `cobra.ExactArgs(n)`, `cobra.RangeArgs(min, max)`
  - `cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs)` paired
    with `ValidArgs: []string{...}` for closed enums.
- For required flags: `cmd.MarkFlagRequired("foo")`.
- For flag relationships (cobra v1.8 has all three):
  - `MarkFlagsRequiredTogether("user", "password")` — all-or-none.
  - `MarkFlagsMutuallyExclusive("json", "csv")` — at most one.
  - `MarkFlagsOneRequired("only", "exclude")` — at least one (combine
    with mutually-exclusive for "exactly one").
- For commands that don't accept filenames, return
  `cobra.ShellCompDirectiveNoFileComp` from `ValidArgsFunction` so tab
  completion stops walking the cwd.

### 6. Persistent flags & lifecycle hooks

Root persistent flags (`rootCmd.PersistentFlags()`):

| Flag               | Type   | Purpose                                                                              |
|--------------------|--------|--------------------------------------------------------------------------------------|
| `--config` / `-c`  | string | Path to `domserver.json`. Default `./domserver.json`.                                |
| `--verbose` / `-v` | count  | `slog` level: 0=warn, 1=info, ≥2=debug.                                              |
| `--disable-ssl`    | bool   | Override `cfg.DisableSSL` (`json:"disable_ssl"`); flips `httpx.Options.DisableSSL`.  |

Per-command flag names continue to mirror Python bit-for-bit (§3); only
this short list of cross-cutting flags is allowed at the root.

`PersistentPreRunE` on the root loads config from `--config` exactly
once, builds the `slog.Handler` from `--verbose`, applies the
`--disable-ssl` override, and stashes the resolved `*config.DomServerClient`
plus logger on the ctx via `context.WithValue`. Subcommands fetch via
typed `cli.ConfigFromContext(ctx)` / `cli.LoggerFromContext(ctx)`
helpers — **never re-load config inside a subcommand's `RunE`**.

Skip `PersistentPreRunE` for commands that don't need config
(`gen-config`, `version`, `completion`, `help`) by checking
`cmd.Name()` and returning early — or by attaching the loader as a
local `PreRunE` on the commands that do need it.

### 7. Help, examples, and completion

- Every leaf command sets `Example` with a copy-pasteable invocation.
  `cobra` shows it under "Examples:" in `--help`. Treat `Example` as
  durable docs — it's what shows up in shell completions and READMEs.
- Keep `Long` to ≤ 5 lines: what it does, what it reads, what it
  writes. `Short` is one line, sentence-case, no trailing period
  (Cobra convention; matches `kubectl`, `helm`).
- Phase 1 ships the canonical `completion [bash|zsh|fish|powershell]`
  subcommand. Use the snippet from <https://cobra.dev/#completions> as
  given — **don't customise**; users follow the upstream docs to install.
- For commands that take a username (`set-password`,
  `rm-teams-and-users`), wire `ValidArgsFunction` to fetch the API user
  list via `client/api/v4`. Completion **must not** be fatal: on API
  error return `nil, cobra.ShellCompDirectiveError` so the shell falls
  back gracefully.

### 8. Suggestions & matching — keep defaults

Leave `cobra.EnablePrefixMatching` and `cobra.EnableCaseInsensitive`
**off**. Phase 1 users have muscle memory from the Python CLI; prefix
matching could silently bind `import` to `import-users-teams` and break
existing scripts. Default typo suggestions (edit distance ≤ 2) stay on —
they only fire when no exact match exists.

### 9. Testability

- Each command has a sibling `_test.go` using the constructor +
  `cmd.SetArgs([]string{...})` + `cmd.SetOut(&buf)` / `SetErr(&buf)`
  pattern. `cmd.SetContext(ctx)` for ctx-cancellation cases.
- HTTP fakes via `httptest.Server` (PR-2 baseline); SMTP fakes via the
  `email.Sender` stub (PR-5). **Never hit a real DOMjudge or real SMTP
  in a test** (per `CLAUDE.md` Testing section).
- Coverage: ≥ 70 % per `internal/cli/{group}` package. Anchor tests:
  golden-file comparison on `gen-config`'s emitted `domserver.json`
  (locks Phase 1 DoD #4) and on `import-users-teams-example`'s emitted
  CSV (locks DoD #5).
