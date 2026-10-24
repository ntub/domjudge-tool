# 0005. Preserve the Python inverted-filter quirk bit-for-bit

- **Status:** Accepted
- **Date:** 2026-05-04
- **Deciders:** @project-maintainer

## Context

The Python predecessor's user/team deletion filters require an explicit
include match. An empty include set skips every row, rather than meaning
"delete everything except excluded accounts." Existing operator workflows
depend on this accidental safety boundary.

The filter is equivalent to:

```python
if lower_name not in include or lower_name in exclude:
    continue
```

A row is eligible only when its case-insensitive name is in `include` and
not in `exclude`. Exclusion wins if a name appears in both sets. Changing
the empty-include behavior silently could turn a previously harmless
invocation into destructive deletion.

## Decision

Both `internal/client/web/v7` and `internal/client/web/v8` preserve this
filter behavior for `DeleteUsers` and `DeleteTeams`. An empty include set
issues no deletion POSTs.

The implementation carries `// QUIRK:` comments, and both versioned
gateways have named regression tests:

- `TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk`
- `TestDeleteTeams_PythonInvertedFilter_LocksKnownQuirk`

User/team deletion remains scraper-backed under [ADR 0006](0006-api-first-with-scraper-fallback.md).
API-first routing does not retire this compatibility lock.

### Alternatives considered

- **Silently make empty include mean all rows.** Rejected: it changes the
  destructive scope of existing commands without operator consent.
- **Document the behavior without regression tests.** Rejected: the unusual
  condition looks like a candidate for cleanup, so executable regression
  coverage is necessary.

## Consequences

- Existing deletion workflows retain their explicit-include safety net on
  both supported DOMjudge families.
- The behavior can surprise callers who expect an empty filter to select
  everything. Callers must supply the intended include set.
- Any change requires an explicit replacement decision and an operator
  migration contract. No filter-correction flag or phase deadline is part
  of the current design.

## References

- [Compatibility contracts](../ROADMAP.md#compatibility-contracts).
- [CLI deletion options](../CLI.md#rm-teams-and-users).
- [v7 user filter and rationale](../../internal/client/web/v7/users.go#L71)
  (`internal/client/web/v7/users.go:71`).
- [v7 user regression](../../internal/client/web/v7/users_delete_test.go#L58)
  and [team regression](../../internal/client/web/v7/teams_delete_test.go#L109).
- [v8 filter](../../internal/client/web/v8/helpers.go#L42) and
  [regressions](../../internal/client/web/v8/delete_test.go#L99).
