# 0012. Desktop Local Datastore: GORM and SQLite

- **Status:** Accepted
- **Date:** 2026-06-20
- **Deciders:** @project-maintainer

## Context

The native Desktop preview persists server profiles, operation history, and
application settings on the operator's machine. History is queryable by
operation type and profile, so the implemented state is more than a handful
of unstructured config values.

The storage boundary has these constraints:

- One local operator and low write concurrency; no hosted multi-tenant
  service or realtime collaboration layer.
- Relational models and filtered history, without application-maintained
  key/value indexes.
- Direct reuse of Go operation packages from the same module.
- No added C database driver requirement alongside the native shell.
- Preserve CLI `domserver.json`; Desktop state must not redefine the CLI
  configuration format.

## Decision

Use **SQLite through GORM and `github.com/glebarez/sqlite`**. The driver is
pure Go; its `modernc.org/sqlite` implementation is transitive rather than a
separately pinned application-facing SQL API. Exact selected dependencies
live in `go.mod` and `go.sum`, under the upgrade policy in
[ADR 0015](0015-security-runtime-and-dependency-upgrades.md).

### Current storage boundary

- `internal/desktop/db.Open` opens the database, limits the SQL pool to one
  open connection, enables foreign keys, and enables WAL for file-backed
  databases. `:memory:` supports isolated tests without WAL.
- Production composition resolves
  `os.UserConfigDir()/domjudge-tool/domjudge-tool.db`.
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
not part of this design. CLI configuration remains independent of the
Desktop database. CLI-install receipts and target records are separate
filesystem state under [ADR 0016](0016-desktop-bundled-cli.md).

### Alternatives considered

- **Plain JSON/TOML files.** Appropriate for the CLI config, but filtered
  operation history would require loading and indexing records in Go.
- **bbolt or another key/value store.** It avoids SQL schemas but makes
  filtered history and relational lookup the application's responsibility.
- **A cgo SQLite driver.** It adds a database-specific C toolchain
  requirement that the pure-Go GORM driver avoids.
- **PocketBase.** Its authentication, realtime, admin UI, and collection
  server exceed the local tool's needs and overlap with the in-process HTTP
  server already owned by Desktop.
- **Direct `database/sql` access as a parallel persistence convention.**
  The implemented stores use GORM models and migrations; introducing another
  access style would split schema ownership without a current requirement.

## Consequences

- SQL filtering and ordering support the actual history screens without
  hand-built secondary indexes.
- GORM models and store constructors own the current schema; schema changes
  must preserve existing local data. `AutoMigrate` is the present mechanism,
  not evidence that arbitrary destructive migrations are safe.
- The database layer adds no cgo requirement. Native Wails builds still
  require their platform SDKs and, where applicable, cgo.
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
- [Database initialization](../../internal/desktop/db/db.go#L14)
  (`internal/desktop/db/db.go:14`).
- [Profile model and migration](../../internal/desktop/profile/store.go#L16).
- [Operation history model and filtering](../../internal/desktop/oplog/store.go#L26).
- [Settings model and defaults](../../internal/desktop/settings/store.go#L12).
- [Production database path](../../cmd/desktop/main.go#L112).
- [Current dependency manifest](../../go.mod).
