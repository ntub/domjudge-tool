# 0005. Preserve the Python inverted-filter quirk bit-for-bit

- **Status:** Accepted
- **Date:** 2026-05-04
- **Deciders:** @project-maintainer

## Context

`services/web/v7.py:161` in the Python CLI contains an inverted
`in` / `not in` condition inside the team-deletion filter:

```python
# Python source — services/web/v7.py:161
if lower_name not in include or lower_name in exclude:
    continue  # skip this team
```

The intended semantics are "delete teams that are in `include` and not
in `exclude`". The actual semantics (due to the inverted condition) are
the opposite: teams that are **not** in `include` or **are** in
`exclude` get skipped (i.e. kept), while teams that *are* in `include`
and *not* in `exclude` get deleted.

This is a bug — but the production DOMjudge operators have been running the
Python CLI with this behaviour for multiple contest cycles. Their admin
workflows are built around the inverted semantics. Silently "fixing" the
filter in the Go port would delete the wrong set of teams in a live
contest environment, potentially corrupting contest state.

## Decision

The Go implementation of the team-deletion filter in
`internal/client/web/v7` **preserves the inverted behaviour
bit-for-bit**. The Go code is annotated with a `// QUIRK:` comment
pointing at this ADR and `docs/ROADMAP.md §Phase 1 quirks`.

The behaviour is locked by a named regression test:

```go
// TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk
// internal/client/web/v7/delete_test.go
```

A future contributor who reads the code, "notices the bug", and is
tempted to "fix" it will:
1. See the `// QUIRK:` comment and be redirected here.
2. Find the named test that will fail if they change the semantics.
3. Understand that the "fix" requires a migration path for existing
   operators, not a silent change.

### When to revisit

The inverted filter may be corrected in Phase 2 if:
- A deprecation window is announced to operators.
- A flag (e.g. `--correct-filter`) is provided to opt in to the
  fixed behaviour during transition.
- The regression test is updated to lock the *new* (correct) behaviour
  and a separate compatibility test covers the old one.

### Alternatives considered

- **Fix the bug silently.** Rejected: would corrupt contest state for
  existing operators without warning. The Go port's goal is feature
  parity, not silent bug fixes to user-observable behaviour.
- **Fix it with a feature flag only.** Rejected for Phase 1: adds
  complexity (new flag, new code path, new tests) without a pressing
  user request. Track in Phase 2 backlog instead.
- **Document without locking.** Rejected: documentation drifts; only
  a failing test reliably prevents a "fix" from slipping through review.

## Consequences

- **Positive**: Operators who upgrade from Python 0.3.0 to Go 1.0.0
  get identical team-deletion behaviour with no surprises.
- **Negative**: The bug persists in Phase 1. New operators reading
  the `--include` / `--exclude` flag documentation will find the
  behaviour confusing.
- **Follow-up**: File a Phase 2 backlog item for a `--correct-filter`
  migration flag with a deprecation period.

## References

- Python source: `services/web/v7.py:161` in
  `ntub-inc/domjudge-tool-cli`.
- `internal/client/web/v7` — Go implementation with `// QUIRK:` comment.
- `docs/ROADMAP.md §Phase 1 §"Quirks to preserve bit-for-bit"`.
- `docs/PHASE1.md §"TDD anchors — preserve Python quirk bit-for-bit"`.
