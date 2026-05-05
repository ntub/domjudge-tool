# PR-6 — CLI root + general + users subtree

> Layered position: **Layer 4** (`internal/cli`). Wires the Cobra
> command tree on top of the use-cases shipped in PR-1..PR-5. No new
> business logic.

## Goal

Land the CLI surface for `cmd/domjudge-tool` such that the binary is
usable end-to-end for the user/team workflows. After PR-6, a Phase 1
DOMjudge admin can:

1. Bootstrap a `domserver.json` from flags (`gen-config`).
2. Print the build version (`version`).
3. Generate the example CSV template (`import-users-teams-example`).
4. Import users + teams from a CSV (`import-users-teams`).
5. Export the live user list to CSV/JSON (`export-users`).
6. Delete users + teams (`rm-teams-and-users`).
7. Reset a single user's password (`set-password`).

PR-7 adds the remaining four groups (problems, submissions, emails,
scoreboard); the **groups for those subtrees are NOT registered yet**
(adding empty groups confuses `--help`).

## Naming policy — bit-for-bit vs new commands

PHASE1 cross-cutting rule §"Public API stability" says subcommand and
flag names "match Python bit-for-bit". This PR has **two classes** of
commands; the spec resolves the apparent contradiction explicitly:

| Class | Commands | Naming rule |
|-------|----------|-------------|
| **Existing in Python 0.3.0** | `import-users-teams`, `import-users-teams-example`, `rm-teams-and-users` | Subcommand name matches Python verbatim. Flag names match Python verbatim. Add idiomatic shorthand via `Flag.Shorthand`; no renames. |
| **New in Go (no Python equivalent)** | `gen-config`, `version`, `export-users`, `set-password` | Free to choose a Go-idiomatic name. Document the choice + provide an alias when there's a near-miss Python command (e.g. `export-users` aliases `user-list`). |

Rationale for the four new commands:

- `gen-config` — Python had `general config` (interactive); Go is
  flag-driven (DoD #4 demands "given identical flags"), so the user
  experience differs enough to warrant a new name. Alias `config` is
  intentionally **not** added — it would shadow the `--config` global
  flag in muscle memory.
- `version` — Python has no top-level version command (`__version__`
  string only). Go ships one because users expect `--version`.
- `export-users` — Python `users user-list --format csv --file out.csv`
  did the same thing. Go renames for clarity (the Python name implied
  "list to stdout"). Alias: `user-list`.
- `set-password` — new admin convenience (was inline-only inside
  `import-users-teams`'s reconcile loop in Python).

## Files created

| Path | Purpose |
|------|---------|
| `cmd/domjudge-tool/main.go` | **Modified.** Build signal-aware ctx and call `internal/cli.Execute(ctx)`; exit 1 on error. Drops the standalone `versionLine()` body — moved to `internal/cli/general/version.go`. |
| `internal/cli/root.go` | New. `Execute(ctx context.Context) error`, `NewRootCmd(deps Deps) *cobra.Command`, `Deps` struct, persistent flags, `AddGroup` for `general` + `users`. Loads config via attached `PreRunE` on the subcommands that need it (NOT a blanket `PersistentPreRunE`). |
| `internal/cli/context.go` | New. `WithConfig`/`ConfigFromContext`, `WithLogger`/`LoggerFromContext`. Typed keys, never re-load config inside a `RunE`. |
| `internal/cli/root_test.go` | New. Persistent-flag parsing, group headers in `--help`, error-printed-once, signal-aware ctx cancels children. |
| `internal/cli/context_test.go` | New. Round-trip + missing-key behaviour. |
| `internal/cli/general/gen_config.go` | New. `newGenConfigCmd(deps Deps) *cobra.Command` writes `domserver.json` from flags via `config.Save`. |
| `internal/cli/general/version.go` | New. `newVersionCmd() *cobra.Command` prints the build line (logic moved from `cmd/domjudge-tool/main.go`). |
| `internal/cli/general/gen_config_test.go` | New. Including the **DoD #4 anchor** `TestGenConfig_GoldenDomserverJSON`. |
| `internal/cli/general/version_test.go` | New. |
| `internal/cli/users/import_users_teams.go` | New. Wires `importer.ImportUsersTeams`. |
| `internal/cli/users/import_users_teams_example.go` | New. Wires `importer.TemplateCSV`. |
| `internal/cli/users/export_users.go` | New. Wires `apiv4.UsersResource.List` + `exporter.Users`. |
| `internal/cli/users/rm_teams_and_users.go` | New. Wires `importer.DeleteTeamsAndUsers`. |
| `internal/cli/users/set_password.go` | New. Wires `web.Gateway.SetUserPassword`. |
| `internal/cli/users/*_test.go` | New, one per command. Includes the **DoD #5 anchor** `TestImportUsersTeamsExample_GoldenCSV`. |
| `testdata/cli/general/gen_config_golden.json` | New. Canonical Python-style serializer output for the locked flag set (asserts DoD #4 byte-for-byte). |
| `go.mod` / `go.sum` | Pin **`github.com/spf13/cobra v1.8.1`** + transitive `pflag v1.0.5`, `inconshreveable/mousetrap v1.1.0` (Go 1.23-compatible). |

## Tests written first (RED)

All tests use stdlib `testing` + `httptest.Server`; no testify, no
ginkgo, no real DOMjudge. HTTP fakes mirror PR-2/PR-3 patterns.
SMTP not needed (emails land in PR-7).

### Root scaffolding (`internal/cli`)

| Test | What it asserts | Cases |
|------|------------------|-------|
| `TestExecute_Returns_FromSubcommand` | `cli.Execute` propagates the subcommand's error wrapped with `%w`. | success, RunE returns `errors.New("boom")`, ctx-cancelled before run |
| `TestExecute_PrintsErrorOnce` | Even with `SilenceErrors: true` everywhere, `cli.Execute` writes the final error to `cmd.ErrOrStderr()` exactly once and Cobra does not print `--help`. | mock subcommand returning error; assert stderr contains the error string but no `Usage:` substring |
| `TestRoot_PersistentFlags_BindAtRoot` | `--config`/`-c`, `--verbose`/`-v` (count), `--disable-ssl` parse on the root. | each flag parses to expected type; `-vv` sets count to 2 |
| `TestRoot_VerboseSetsSlogLevel` | After parsing, `LoggerFromContext(ctx)` returns a `*slog.Logger` whose handler level matches `--verbose` (0=warn, 1=info, ≥2=debug). | three sub-cases by verbose count |
| `TestRoot_HelpHasGroupHeaders` | `domjudge-tool --help` output contains `"General Commands:"` and `"User & Team Commands:"` strings. | sanity that `AddGroup` + `GroupID` wiring works |
| `TestRoot_HelpAndCompletionInGeneralGroup` | Auto-generated `help` and `completion` commands sit under `"General Commands:"`, not under "Additional Commands:". | grep stdout |
| `TestContextHelpers_RoundTrip` | `WithConfig` / `ConfigFromContext` and `WithLogger` / `LoggerFromContext` round-trip. | nil ctx → panic? no: returns nil + ok=false; missing key → ok=false |

### General subtree (`internal/cli/general`)

| Test | What it asserts | Cases |
|------|------------------|-------|
| **`TestGenConfig_GoldenDomserverJSON`** *(anchor)* | Running `gen-config` with the canonical flag set writes a `domserver.json` byte-identical to `testdata/cli/general/gen_config_golden.json`. **Locks Phase 1 DoD #4.** | one canonical case |
| `TestGenConfig_RequiredFlagsMissing` | Cobra's `MarkFlagRequired` rejects calls missing `--host`/`--username`/`--password`. | each flag missing in turn → non-nil error, no file written |
| `TestGenConfig_DefaultsForOptionals` | When optional flags are unset, the saved file matches `config.Default()` for those fields (e.g. `affiliation_country = "TWN"`, `version = "7.3.2"`). | one case asserting key-by-key |
| `TestGenConfig_OutputFlagWritesElsewhere` | `--output /tmp/foo.json` writes there; default is `./domserver.json` (cwd). | two sub-cases |
| `TestGenConfig_DoesNotLoadConfig` | gen-config has no `PreRunE` that calls `config.Load`. | run with non-existent `--config` path → success |
| `TestVersion_PrintsVersionLine` | `version` writes the build version line (same content as the previous `cmd/main.go`). | one case |
| `TestVersion_NoArgsAccepted` | `version` rejects positional args via `cobra.NoArgs`. | one positional → error |

### Users subtree (`internal/cli/users`)

| Test | What it asserts | Cases |
|------|------------------|-------|
| **`TestImportUsersTeamsExample_GoldenCSV`** *(anchor)* | `import-users-teams-example` writes `./import-users-teams.csv` byte-identical to `testdata/importer/template_python_golden.csv`. **Locks Phase 1 DoD #5.** | one canonical case in tempdir |
| `TestImportUsersTeams_HappyPath` | Given a CSV + httptest API + httptest web, the use-case is invoked once and an output CSV is written. | uses minimal stub gateway + apiv4 client |
| `TestImportUsersTeams_FlagPropagation` | Each Python flag (`--category-id`, `--affiliation-id`, `--user-roles`, `--enabled`, `--ignore-existing`, `--delete-existing`, `--password-length`, `--password-pattern`, `--new-password`) lands in `importer.ImportOptions` with the right value. | table-driven, one case per flag |
| `TestImportUsersTeams_FilePositional_Required` | Missing positional CSV path is rejected by `cobra.ExactArgs(1)`. | no args → error |
| `TestExportUsers_FormatCSV` | `--format csv` emits a CSV starting with the locked header row. | httptest API returning two users |
| `TestExportUsers_FormatJSON` | `--format json` emits a JSON array. | same fixture |
| `TestExportUsers_StdoutByDefault` | No `--output`: bytes go to `cmd.OutOrStdout()`. | check `&buf` |
| `TestExportUsers_OutputFlag` | `--output /tmp/x.csv`: bytes go to the file, stdout untouched. | check tempfile |
| `TestExportUsers_AliasUserList` | Invoking `user-list` (alias) runs the same code path. | `cmd.SetArgs([]string{"user-list", ...})` |
| `TestRmTeamsAndUsers_DefaultProtects` | Without flags, the use-case runs with `Include=nil, Exclude=nil` — `protectedSet` (admin/judgehost/cfg.Username) handles the default. | stub gateway records args |
| `TestRmTeamsAndUsers_IncludeExcludeFlags` | `--include foo --include bar --exclude baz` propagates to `DeleteOptions`. | one case |
| `TestRmTeamsAndUsers_AliasRmTeams` | `rm-teams` alias works. | one case |
| `TestSetPassword_HappyPath` | `set-password <username>` with `--password p` looks up the user-id from the API and calls `web.Gateway.SetUserPassword(ctx, userID, "p", roles, enabled)`. | stub gateway records args |
| `TestSetPassword_GeneratesIfMissing` | Without `--password`, generates one via `password.Generate` and prints it once to stdout. | regex match on output |
| `TestSetPassword_UnknownUser` | API returns no user with that name → error wraps a sentinel like `errors.New("user not found")` (or similar; locked by the test). | non-nil error |
| `TestSetPassword_RequiresUsername` | `cobra.ExactArgs(1)` rejects no-args call. | one case |

## Edge cases to cover

Reviewer ticks each box.

- [ ] `gen-config --output ./tmp.json` with a non-existent parent directory returns a wrapped `os.PathError`.
- [ ] `gen-config --user-roles 3,4,5` parses to `[]int{3,4,5}` (cobra's `IntSliceVar` semantics).
- [ ] `gen-config --timeout 30` parses to `*float64(30)` — pointer non-nil only when `cmd.Flags().Changed("timeout")` is true.
- [ ] `version` ignores `--config`, `--verbose`, `--disable-ssl` (no PreRunE attached).
- [ ] `import-users-teams nonexistent.csv` returns a wrapped `os.ErrNotExist`.
- [ ] `import-users-teams ./input.csv` writes the output CSV via `exporter.CreateUsers` to `./import-users-teams-out.csv` (matches Python `_users.py:153` filename).
- [ ] `--ignore-existing` and `--delete-existing` set both fields on `ImportOptions`; behaviour difference is decided by `importer` (PR-4), not the CLI.
- [ ] `export-users --format json` with an empty user list emits `[]`, not `null` (matches `exporter.writeJSONArray` contract).
- [ ] `rm-teams-and-users` runs the use-case with `Include=nil` (Python parity: `None`, not `[]`) when `--include` is unspecified.
- [ ] `set-password sampleuser` (no `--password`) prints exactly one line of generated password (no double-print, no leak).
- [ ] `set-password admin` succeeds (admin is protected for *delete*, not for *set-password*).
- [ ] Ctx cancellation: `cmd.SetContext(cancelled)` — RunE returns the wrapped `context.Canceled`.
- [ ] `MarkFlagsMutuallyExclusive("ignore-existing", "delete-existing")` rejects setting both at once. (Python silently let `delete_existing` win; we **fix the bug** here. Document in the commit message.)
- [ ] Persistent-flag `--config /missing.json` errors via the loader's `PreRunE` (only on commands that attached the loader, not on `gen-config`/`version`/`completion`/`help`).

## Definition of done

1. `just check` clean — fmt, vet, lint, test all pass.
2. `go test ./internal/cli/... -cover` reports each package ≥ 70 %.
3. Anchor tests pass:
   - `TestGenConfig_GoldenDomserverJSON` (DoD #4)
   - `TestImportUsersTeamsExample_GoldenCSV` (DoD #5)
4. `./bin/domjudge-tool --help` shows two group headers ("General Commands:", "User & Team Commands:") and the expected commands under each.
5. `./bin/domjudge-tool gen-config --host https://example.com --username user --password pass` writes a `domserver.json` byte-identical to `testdata/cli/general/gen_config_golden.json`.
6. `./bin/domjudge-tool import-users-teams-example` writes `./import-users-teams.csv` byte-identical to `testdata/importer/template_python_golden.csv`.
7. `./bin/domjudge-tool version` prints the build line.
8. `cmd/domjudge-tool/main.go` is ≤ 30 LOC after the rewire (signal ctx → `cli.Execute(ctx)` → `os.Exit(1)`-on-error). No business logic.
9. No `var rootCmd` package globals, no `init()` outside registry wiring, no `fmt.Print*` in `internal/cli`.
10. Conventional Commit on the merge: **`feat(cli): root + general + users subtree`**.

## Cross-references

- **Python source**:
  - `domjudge_tool_cli/__init__.py` (entry + typer wiring)
  - `domjudge_tool_cli/commands/general/{__init__,_check}.py` (config CRUD)
  - `domjudge_tool_cli/commands/users/{__init__,_users}.py` (5 commands)
- **Phase 1 docs**: `docs/PHASE1.md` (architecture, DoD, cross-cutting),
  `docs/phase1/README.md` §"CLI UX/DX guardrails" (Cobra patterns).
- **PR-4 use-cases**: `internal/importer/{importer,delete,template}.go`,
  `internal/exporter/users.go` — re-used verbatim, no signature change.
- **PR-2 / PR-3 clients**: `internal/client/api/v4`, `internal/client/web`,
  `internal/client/web/v7` — re-used verbatim.
- **ADRs**: none new. (No non-obvious architecture decisions; Cobra
  patterns are a vendored convention not a design choice.)

## Out of scope (deferred to PR-7 or later)

- `download-problems`, `download-submissions`, `send-user-accounts`, `download-scoreboard` → PR-7.
- `gen-config` interactive prompts → not adding; flags-only (DoD #4
  demands flag-driven for byte-identical output).
- `--config` env-var fallback (`DOMSERVER_CONFIG=...`) → matches Python
  via `cobra` `BindEnv`-equivalent? **Yes**, wire `viper`-free env via
  `cmd.Flags().Lookup("config").Value.Set(os.Getenv("DOMSERVER_CONFIG"))`
  in `PersistentPreRunE` only when the flag isn't `Changed`. Test:
  `TestRoot_ConfigEnvVarFallback`.
- Shell completion install instructions → README polish in PR-8.
