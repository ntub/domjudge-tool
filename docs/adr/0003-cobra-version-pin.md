# 0003. Cobra pinned at v1.8.x for Go 1.23 compatibility

- **Status:** Superseded by 0015
- **Date:** 2026-05-04
- **Deciders:** @project-maintainer

## Context

Phase 1 requires a CLI framework. Cobra is the de-facto standard for
Go CLIs (used by `kubectl`, `helm`, `gh`, and the majority of serious
Go CLI projects). The decision to use Cobra was straightforward; the
non-obvious part is the version selection.

The module declares `go 1.23.0` in `go.mod`. This constraint exists
because:

- The project targets the same Go version range as the operators who
  deploy DOMjudge on university infrastructure — typically conservative
  Go environments.
- Bumping the directive to 1.24 or 1.25 risks silently requiring a
  newer toolchain than operators have installed.

At implementation time (PR-6, 2026-05):

- **cobra v1.8.1** is the latest 1.23-compatible release.
- **cobra v1.9.x and v1.10.x** were released but bumped the Go
  directive to `go 1.25`, which `go mod tidy` would propagate upward
  into our `go.mod`, breaking the `go 1.23` constraint.

## Decision

Cobra is pinned at **v1.8.1** in `go.mod`. The pin is explicit and will
remain until the `go 1.23` directive is intentionally lifted.

```
require (
    github.com/spf13/cobra v1.8.1
)
```

### Alternatives considered

- **cobra v1.10.x (latest at decision time).** Rejected: requires
  `go 1.25`, which violates the `go 1.23` directive and is too
  aggressive an upgrade for operators.
- **cobra v1.9.x.** Rejected: same reason as v1.10.x.
- **A different CLI framework** (e.g. `urfave/cli`, `alecthomas/kong`).
  Rejected: Cobra has the largest ecosystem, best shell-completion
  support, and `kubectl`-compatible UX patterns that the user base
  recognises. No functional gap that would favour an alternative.

## Consequences

- **Positive**: `go 1.23.0` directive stays stable through Phase 1.
- **Positive**: Operators on Go 1.23 toolchains can build from source
  without upgrading their toolchain.
- **Negative**: Cobra v1.9/v1.10 features (if any are relevant) are
  unavailable until the pin is lifted.
- **Unpin trigger**: When the `go` directive is intentionally raised to
  ≥ 1.25 (likely at the start of Phase 2 or Phase 3), update
  `go.mod` to the then-current Cobra stable release and run
  `go mod tidy`.

## References

- `go.mod` — current `require` stanza.
- `CLAUDE.md` §"Tech stack" — "Pinned at v1.8.1".
- `docs/PHASE1.md` §"Deviations" §2 — Go directive stays at 1.23.
- ADR-0004 — goquery is in the same situation; tracked separately
  because the unpin trigger may differ.
