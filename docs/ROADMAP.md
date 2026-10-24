# Roadmap

This page records the supported scope and the compatibility contracts that
bind every change. It is not a backlog of speculative features. Command
usage is in [CLI](CLI.md), implementation boundaries are in
[Architecture](ARCHITECTURE.md), and user workflows are in the
[User Guide](USER_GUIDE.md).

## Supported scope

- **CLI:** one executable manages DOMjudge 7.3.x and 8.3.1, API-first with
  scraper fallback, including contest-scoped `import-problems --upsert`.
- **Desktop:** a native Wails v3 preview with server profiles, settings, and
  operation history. Bundles for macOS, Linux, and Windows (amd64 and arm64)
  contain the independent CLI; they are unsigned and not notarized.
- **Releases:** published by GitHub Actions from annotated
  `vMAJOR.MINOR.PATCH` tags on `main`; see
  [CONTRIBUTING](../CONTRIBUTING.md#releases).

Runtime and tool versions are pinned in `mise.toml`; `go.mod` is the Go
dependency source of truth. No additional server versions, frontend
replacement, or new feature milestones are committed by this roadmap.

## Compatibility contracts

### Backward compatibility — must preserve bit-for-bit

- **`domserver.json`:** keep field names, types, defaults, and the
  distinction between nullable fields and omitted Go-only options.
  Canonical output uses snake_case keys in struct declaration order,
  four-space indentation, and no trailing newline. Missing fields retain
  defaults including `version="7.3.2"`, `api_version="v4"`, and
  `affiliation_country="TWN"`. Existing files without `prefer_scraper`
  or `contest_id` remain valid.
- **CLI interface:** preserve current command names, aliases, flags,
  defaults, and output file names. Desktop packaging must not route
  terminal commands through the GUI. The complete surface is in
  [CLI](CLI.md).
- **CSV:** keep the leading template columns
  `username,name,email,affiliation,password` in that order. The current
  template appends optional `category_id`; old CSVs remain accepted.
  Input parsing is field-name driven and strips a leading UTF-8 BOM.
  Import-result output stays
  `username,name,email,affiliation,password,is_exist` in
  `import-users-teams-out.csv`.
- **Deletion protection:** `rm-teams-and-users` protects `admin`,
  `judgehost`, and the active config username before applying filters.
  Deletion operates on global users and teams; a contest selection is
  not a server-side deletion boundary.
- **Passwords:** the default generated length is 10, using ASCII letters
  and digits with `crypto/rand`. Preserve explicit CSV passwords unless
  regeneration is requested.
- **Wire formats:** CCS import TSV is UTF-8 without BOM and uses LF.
  Scoreboard CSV uses CRLF through `scoreboard.WriteCSV`; its current
  leading columns are
  `Rank,TeamAffiliation,TeamName,SolvedCount,Score,Category`.
  Category inclusion and omission of the old summary row are deliberate
  current behavior, not a requirement to restore Phase 1 output.
- **Regression inputs:** preserve sanitized v7 and v8 fixtures and the
  canonical config/import goldens. Tests use standard-library `testing`
  and local fakes/`httptest`, never live DOMjudge or SMTP services.

### Quirks to preserve bit-for-bit

The Python `services/web/v7.py:161` and `:188` inverted filters are locked
in the v7 scraper: an empty `include` set skips every row, producing no
delete POSTs. Keep both
`TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk` and
`TestDeleteTeams_PythonInvertedFilter_LocksKnownQuirk`. Do not silently
change this behavior; a change requires an explicit ADR and regression
coverage. The v8 scraper also preserves the empty-include behavior.

Scoreboard team names retain the last whitespace-separated token of the
rendered display name, as in the legacy HTML parser. Keep that quirk
separate from the deliberate Category/cell-format changes.

## Maintained protocol references

- [API, routing, formats, and fixture contract](phase1.5/CONTRACT.md)
- [DOMjudge 8.3.1 compatibility and safe capture contract](phase2/CONTRACT.md)
- [Testing](TESTING.md) and [documentation maintenance](DOCUMENTATION.md)

Completed PR plans, superseded specifications, spike logs, and one-off
live-validation notes are removed rather than retained as active guidance.
