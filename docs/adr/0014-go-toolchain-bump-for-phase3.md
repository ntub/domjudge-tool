# 0014. Go Toolchain Bump for Phase 3

- **Status:** Accepted
- **Date:** 2026-06-20
- **Deciders:** @project-maintainer

## Context

The repo's `go.mod` directive is currently `go 1.23.0`. `CLAUDE.md` states:
"Do not upgrade the module directive casually." This ADR provides the
deliberate decision record for upgrading it.

Phase 3 adds a cross-platform desktop app entry point (`cmd/desktop/`) to the
same Go module (`github.com/ntub/domjudge-tool`). Sharing the module lets the
desktop app reuse all `internal/` packages directly via standard imports —
the architecture's explicit goal (see `docs/ARCHITECTURE.md` § Goals).

The chosen native shell is Wails v3 (ADR 0013). Wails v3's published
documentation states:

> "Wails v3 requires Go version 1.25 or newer."

This is a hard requirement: `wails3 build` and `wails3 dev` fail on older
toolchains. Verified via `ctx7` (`/websites/v3_wails_io`) on 2026-06-20.

The Go 1.23 → 1.25 bump is therefore **not optional** if Phase 3 uses Wails
v3 in the same module as the CLI.

## Decision

We decided to **bump the module directive from `go 1.23.0` to `go 1.26.0`**
when Phase 3 implementation begins (after ADR 0013 spikes pass).

**Applied.** The module directive was bumped from `go 1.23.0` to `go 1.26.0`
on 2026-06-20. The bump was applied ahead of spike completion because the
developer machine has Go 1.26.4 installed, Wails v3's Go 1.25 minimum is
met, and all CLI tests pass under Go 1.26.0 (`just test && just build` clean).
Phase 3 spike work in `tmp/phase3-spike/` will use the same toolchain.

### Cascading effects

#### `modernc.org/sqlite` pin (ADR 0012)

ADR 0012 pinned `modernc.org/sqlite` at `v1.38.0` specifically to stay at
`go 1.23.0`. Once the module directive moves to `go 1.26.0`, that constraint
is lifted. At implementation time, pick the **latest non-retracted
`modernc.org/sqlite` tag whose `go.mod` directive is ≤ 1.25**. Confirm via
`ctx7` or `go get` at that moment; do not assume the pin here. ADR 0012's
"Follow-up" section covers this; its status will be updated to Accepted
alongside this ADR.

#### CI and toolchain pinning

The CI workflow (`.github/workflows/ci.yml`) pins the Go version. Update the
`go-version` field to `'1.25'` in the same PR that bumps `go.mod`. Check
`go.sum` and `go mod tidy` after the bump.

#### CLI build

The CLI (`cmd/domjudge-tool/`) compiles under Go 1.25 without changes — Go
guarantees backward compatibility for code written against older language
versions. The module directive change is transparent to CLI users.

#### `goquery` pin (ADR 0004)

ADR 0004 pins `goquery` at `v1.10.3` to avoid transitively requiring a Go
version above 1.23. With the module directive at 1.25, this constraint
dissolves. However, do not upgrade `goquery` opportunistically — it is working
and there is no feature need. The pin may be relaxed in a dedicated PR if a
security or correctness issue arises.

## Alternatives Considered

- **Separate Go module for the desktop app** (e.g., `desktop/go.mod`). A
  separate module avoids the global directive bump but **cannot import
  `internal/` packages** (Go toolchain rejects cross-module imports of paths
  containing `/internal/`). The only workarounds — moving packages out of
  `internal/` to `pkg/` or using a `replace` directive — are significant
  invasive refactors that undermine the architecture. Rejected.
- **Wails v2 (stable, Go 1.21 compatible).** Wails v2 is stable and works on
  Go 1.21+, avoiding the directive bump. Rejected because Wails v3 is the
  current development branch; v2 will receive only security patches. Building
  on v2 means migrating again before v3 stabilises, or being stuck on an
  end-of-life path.
- **Staying at Go 1.23 and dropping Wails.** Would require an alternative
  native shell (Fyne, Gio, etc.) that works on Go 1.23. Those alternatives
  were not evaluated for Phase 3 and would require new ADRs. Rejected as the
  baseline plan.

## Consequences

- Positive: unlocks Wails v3 as the native shell, which is the Phase 3
  architecture decision.
- Positive: `modernc.org/sqlite` pin constraint dissolves — can track a more
  recent, non-retracted version.
- Positive: Go 1.25 language features (range over int, range-over-func,
  tool directives in `go.mod`) become available.
- Negative: developers building the desktop app need Go 1.25. The CLI still
  compiles from the same codebase, so CI must be updated in lock-step.
- Negative: the `goquery` version constraint from ADR 0004 dissolves. This
  is a positive outcome but should be noted so future maintainers know ADR
  0004's rationale no longer holds post-bump.
- Follow-up: confirm Go 1.25 availability in CI at Phase 3 PR time; update
  `.github/workflows/ci.yml` `go-version` field.
- Follow-up: after the bump, run `go mod tidy` and verify no surprise
  transitive dependency bumps.
- Follow-up: update ADR 0012 status to Accepted (or revise the pin) when this
  ADR is accepted.

## References

- [Wails v3 installation — Go version requirement](https://v3.wails.io/quick-start/installation)
  (verified via `ctx7` 2026-06-20)
- [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) — Phase 3 reuse rationale
- [`docs/ROADMAP.md`](../ROADMAP.md) — Phase 3 scope
- [`docs/phase3/SPEC.md`](../phase3/SPEC.md) — Phase 3 specification
- [0012. Phase 3 Desktop Local Datastore](0012-phase3-local-datastore.md)
- [0013. Phase 3 Desktop Frontend Stack](0013-phase3-frontend-stack.md)
- [0004. goquery version pin](0004-goquery-version-pin.md)
