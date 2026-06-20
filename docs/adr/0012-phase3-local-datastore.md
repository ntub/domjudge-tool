# 0012. Phase 3 Desktop Local Datastore

- **Status:** Accepted
- **Date:** 2026-06-20
- **Deciders:** @project-maintainer

## Context

Phase 3 ships a cross-platform desktop app (Wails v3) that wraps the
existing Go CLI. Unlike the CLI, the desktop app needs durable local
state on the operator's machine: saved DOMjudge server profiles,
operation history (for example the structured import reports introduced
in Phase 1.5), cached domain data for display (affiliations, team and
user snapshots, scoreboard captures), and UI preferences.

The forces in play:

- **Single user, local, low concurrency.** One operator drives one app
  process. There is no multi-tenant access, no network fan-out, and no
  realtime collaboration requirement.
- **Query shape, not just storage.** The desktop value-add is surfacing
  history and cached entities in tables the operator can filter, sort,
  and report over. The DOMjudge domain is relational
  (contest -> team -> user, affiliations).
- **Go module lock.** The Phase 3 app reuses `internal/` packages and
  shares the module. ADR 0014 bumped the module directive from Go 1.23 to
  Go 1.26, lifting the original Go 1.23 pin constraint on this ADR.
- **Pure-Go / cgo packaging.** Wails cross-platform builds are simpler
  without an added cgo toolchain per target. The repo already prefers
  pure-Go pins (see ADR 0004 for the goquery precedent).
- **No casual dependency growth.** `CLAUDE.md` requires new dependencies
  be necessary and compatible with the phase plan.

The open question is whether the local store should be a relational,
queryable database or a simple embedded key/value store, and which
concrete library to pin.

## Decision

We decided to use **SQLite via `modernc.org/sqlite`** (the pure-Go,
cgo-free port) as the Phase 3 local datastore.

**Updated pin (2026-06-20):** ADR 0014 dissolved the Go 1.23 constraint by
bumping the module directive to Go 1.26.0. The pin is therefore updated from
the tentative v1.38.0 to **`modernc.org/sqlite` v1.52.0** (`go 1.25.0`
directive — compatible with Go 1.26). This version bundles SQLite 3.53.0,
confirmed by ctx7 on 2026-06-20, and supports all Wails desktop targets
(`darwin/{amd64,arm64}`, `windows/{386,amd64,arm64}`, and the `linux`
family). S-6 spike confirmed v1.52.0 builds and queries correctly alongside
Wails v3 under `CGO_ENABLED=1` (see `docs/phase3/SPIKES.md`).

Several versions remain upstream-retracted (`v1.33.0`, `v1.34.3`, `v1.42.0`,
and others); the pin at v1.52.0 avoids all retracted tags.

This decision is conditional on Phase 3 actually persisting queryable
state. If the shipped app only stores a handful of server profiles and
preferences with no in-app querying, a plain JSON file (or bbolt) is
sufficient and SQLite should not be added (YAGNI). The relational store
is justified by history and cached-entity views, not by config alone.

### Alternatives considered

- **bbolt (`go.etcd.io/bbolt`).** Pure-Go, cgo-free, ACID, MVCC
  single-writer/multi-reader, battle-tested as the etcd/Kubernetes
  backend, and fully Go 1.23 compatible. Rejected as the primary store
  because it is a low-level key/value engine: filtering, sorting, joins,
  and aggregation over history must be hand-built as secondary indexes in
  Go, and the on-disk file is opaque to standard tooling. For a UI that
  lists and filters operation history, that pushes database work into the
  application layer. bbolt remains a reasonable choice if Phase 3 state
  collapses to pure key->blob with no querying.
- **`mattn/go-sqlite3`.** The most common SQLite driver, but cgo-based.
  It requires a C toolchain for every cross-compile target and conflicts
  with the repo's pure-Go preference. Rejected in favor of the cgo-free
  `modernc.org/sqlite`.
- **Latest `modernc.org/sqlite` (v1.38.2+/`master`).** Previously
  rejected because the `go 1.25.0` directive would have forced a module
  bump. That constraint dissolved with ADR 0014 (Go 1.26 bump). The
  latest non-retracted tag (v1.52.0) is now the chosen pin.
- **PocketBase as the datastore.** Brings SQLite plus auth, realtime,
  an admin UI, and collection rules. Rejected as over-scoped for a
  single-user local tool: those features are unused, and embedding it
  next to a Wails HTTP server creates a "who owns the HTTP server"
  overlap. Evaluated separately from the storage-engine decision.
- **Plain JSON / TOML files.** Zero dependency, trivially inspectable.
  Adequate for config and preferences, but it does not scale to
  queryable history or cached relational data without reinventing an
  index. Retained as the right tool for config-only state.

## Consequences

- Positive: the app gets SQL (filter, sort, join, aggregate) for history
  and cached-entity views without hand-rolling indexes.
- Positive: the datastore stays pure-Go and cgo-free, so Linux and
  Windows builds add no C toolchain, and no second cgo dependency is
  introduced alongside Wails.
- Positive: the SQLite file is inspectable with standard tooling, which
  helps debugging and operator support.
- Positive: SQLite is the same engine PocketBase uses, so a later pivot
  to PocketBase would not change the storage substrate.
- Negative: the cgo-free benefit is partial. Wails v3 macOS builds still
  require cgo for WebView integration and are typically built on macOS,
  so the app as a whole is not cgo-free; the win is "no *added* cgo," not
  "no cgo."
- Neutral: the Go 1.23 version-pin constraint is dissolved by ADR 0014.
  The pin is updated to v1.52.0 (Go 1.25 directive, compatible with Go 1.26).
- Negative: SQLite introduces a schema and therefore a migration story
  (tooling choice deferred below). bbolt would have avoided schema
  migrations at the cost of manual struct-version handling.
- Done: `modernc.org/sqlite` v1.52.0 confirmed build and run under
  Wails v3 on darwin/arm64 — see S-6 in `docs/phase3/SPIKES.md`.
- Done: Go toolchain bumped to 1.26 (ADR 0014); pin updated to v1.52.0.
- Follow-up: choose a migration approach (hand-rolled, `goose`, or
  `golang-migrate`) in a Phase 3 implementation PR; keep it dependency-
  light per `CLAUDE.md`.
- Follow-up: settle whether Phase 3 ships any queryable state at all. If
  it does not, supersede this ADR with a JSON/bbolt decision.

## References

- [`docs/ROADMAP.md`](../ROADMAP.md) — Phase 3 desktop app scope and the
  Phase 3 pre-research checklist.
- [`docs/phase3/SPEC.md`](../phase3/SPEC.md) — Phase 3 specification.
- [0004. goquery version pin](0004-goquery-version-pin.md) — precedent for
  pinning a dependency to protect the Go-version tradeoff.
- [0014. Go Toolchain Bump for Phase 3](0014-go-toolchain-bump-for-phase3.md)
  — the decision that dissolves the Go 1.23 pin constraint and revisits the
  `modernc.org/sqlite` version selection.
- `modernc.org/sqlite` go.mod directives:
  [v1.38.0 (`go 1.23.0`)](https://raw.githubusercontent.com/modernc-org/sqlite/v1.38.0/go.mod),
  [`master` (`go 1.25.0`)](https://raw.githubusercontent.com/modernc-org/sqlite/master/go.mod).
- [`modernc.org/sqlite` on pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite).
- [`docs/phase3/SPIKES.md`](../phase3/SPIKES.md) — S-6 spike evidence for this ADR.
- [`go.etcd.io/bbolt`](https://github.com/etcd-io/bbolt).
- [Wails v3 Cross-Platform Building](https://v3.wails.io/guides/build/cross-platform/).
