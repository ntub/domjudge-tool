# 0001. Go module path is `github.com/ntub/domjudge-tool`

- **Status:** Accepted
- **Date:** 2026-04-26
- **Deciders:** @project-maintainer

## Context

The Go rewrite needs a stable module path before the first `go.mod` is
committed. Renaming a module path later is mechanically simple but
disruptive — every importer rewrites, every release tag pivots, and any
external tooling that pins the old path breaks.

Constraints in scope at decision time:

- The Python predecessor lives at `github.com/ntub/domjudge-tool-cli`.
- The new repository lives at `github.com/ntub/domjudge-tool` (the `-cli`
  suffix is dropped because the project itself is the CLI in Phase 1–2;
  Phase 3 will add a desktop app on top of the same packages, so a
  CLI-specific suffix would be misleading).
- We want a path that matches the canonical GitHub URL so `go install`
  works without redirects.
- The module is `internal/`-heavy by design (see `docs/ARCHITECTURE.md`),
  so external imports are not a near-term concern, but the path still
  needs to be stable for tagging and release tooling.

## Decision

The module path is **`github.com/ntub/domjudge-tool`**, declared on the
first line of `go.mod`.

### Alternatives considered

- **`github.com/ntub/domjudge-tool-cli`** — matches the Python repo name
  exactly. Rejected because Phase 3 will add a desktop app on top, making
  the `-cli` suffix misleading.
- **A vanity path under an `source.example.edu` domain.** Rejected: adds a
  redirector to maintain, no current benefit, and the GitHub path already
  works as a canonical identity.
- **`github.com/sampleuser/domjudge-tool`** (personal account). Rejected:
  ownership of the project belongs to NTUB; the org path reflects that.

## Consequences

- All in-repo imports use `github.com/ntub/domjudge-tool/internal/...`.
- `goreleaser` (Phase 1+) keys releases on this path.
- If/when external packages need to consume our types, they import
  `github.com/ntub/domjudge-tool/pkg/...` — which we'll add as a public
  surface only when there's a real consumer (currently none planned).
- Renaming the GitHub repo without filing a follow-up ADR breaks this
  decision; treat the module path as a public API.

## References

- `go.mod`
- `MIGRATION_TO_GOLANG.md` §11 in the Python source repo
