# 0012. Web UI Local Datastore: GORM and SQLite

- **Status:** Accepted
- **Date:** 2026-06-20
- **Deciders:** @project-maintainer

## Context

The local web UI (served by `domjudge-tool web`; see
[ADR 0018](0018-local-web-ui-command.md)) persists server profiles, operation
history, and application settings on the operator's machine. History is
queryable by operation type and profile, so the implemented state is more than
a handful of unstructured config values.
The storage boundary has these constraints:

- One local operator and low write concurrency; no hosted multi-tenant
  service or realtime collaboration layer.
- Relational models and filtered history, without application-maintained
  key/value indexes.
- Direct reuse of Go operation packages from the same module.
- No added C database driver requirement alongside the native shell.
- Preserve CLI `domserver.json`; web UI state must not redefine the CLI
  configuration format.

## Decision

Use **SQLite through GORM and `github.com/glebarez/sqlite`**. The driver is
pure Go; its `modernc.org/sqlite` implementation is transitive rather than a
separately pinned application-facing SQL API. Exact selected dependencies
live in `go.mod` and `go.sum`, under the upgrade policy in
[ADR 0015](0015-security-runtime-and-dependency-upgrades.md).

### Current storage boundary

- `internal/webui/db.Open` opens the database, limits the SQL pool to one
  open connection, enables foreign keys, and enables WAL for file-backed
  databases. `:memory:` supports isolated tests without WAL.
- Production composition resolves the SQLite path from the `--db` flag,
  which defaults to `os.UserConfigDir()/domjudge-tool/domjudge-tool.db` (the
  former Desktop path, so existing local data carries over).
- `profile.Store` owns server-profile CRUD and default selection.
- `oplog.Store` owns operation status, parameters, summaries, counts, and
  timestamps, with newest-first history filtered by type or profile.
- `settings.Store` owns a singleton row for theme, default profile, and SMTP
  settings. Missing settings use the implemented defaults, including dark
  theme and SMTP port 25.
- Each store applies GORM `AutoMigrate` when constructed. There is no
  separate migration framework or promised domain-cache schema.

These stores contain current local state, not a second DOMjudge server.
PocketBase, a network database, account management, and synchronization are
not part of this design. CLI configuration (`domserver.json`) remains
independent of the web UI database.
### Alternatives considered

- **Plain JSON/TOML files.** Appropriate for the CLI config, but filtered
  operation history would require loading and indexing records in Go.
- **bbolt or another key/value store.** It avoids SQL schemas but makes
  filtered history and relational lookup the application's responsibility.
- **A cgo SQLite driver.** It adds a database-specific C toolchain
  requirement that the pure-Go GORM driver avoids.
- **PocketBase.** Its authentication, realtime, admin UI, and collection
  server exceed the local tool's needs and overlap with the in-process HTTP
  server already owned by the web UI.
- **Direct `database/sql` access as a parallel persistence convention.**
  The implemented stores use GORM models and migrations; introducing another
  access style would split schema ownership without a current requirement.

## Consequences

- SQL filtering and ordering support the actual history screens without
  hand-built secondary indexes.
- GORM models and store constructors own the current schema; schema changes
  must preserve existing local data. `AutoMigrate` is the present mechanism,
  not evidence that arbitrary destructive migrations are safe.
- The database layer adds no cgo requirement. CLI and web UI builds remain
  pure Go without native GUI dependencies.
- The database is inspectable with ordinary SQLite tooling, but contains
  sensitive data: profile and SMTP passwords are stored as strings by the
  current models. This decision provides no database encryption or OS
  keychain integration.
- Keep real databases, credentials, operation exports, and diagnostic
  captures out of tracked files. Use isolated temporary stores and reserved
  example data for tests and documentation.

## References

- [Current architecture](../ARCHITECTURE.md).
- [CLI config contract](../CLI.md#configuration-domserverjson).
- [ADR 0018](0018-local-web-ui-command.md) for the `web` command contract.
- [Database initialization](../../internal/webui/db/db.go#L14)
  (`internal/webui/db/db.go:14`).
- [Profile model and migration](../../internal/webui/profile/store.go#L16).
- [Operation history model and filtering](../../internal/webui/oplog/store.go#L26).
- [Settings model and defaults](../../internal/webui/settings/store.go#L12).
- [Current dependency manifest](../../go.mod).
