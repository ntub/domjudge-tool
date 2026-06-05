# 0010. Problem Package Contest Scoping and Import

- **Status:** Proposed
- **Date:** 2026-06-05
- **Deciders:** @project-maintainer

## Context

`download-problems` is necessarily hybrid. DOMjudge v4 exposes contest
problem metadata through `/api/v4/contests/{cid}/problems`, but it does
not expose a full problem-package ZIP export endpoint on DOMjudge 7.3.x
or 8.3.1. ZIP export still comes from the authenticated jury UI path.

Live validation against throwaway DOMjudge 7.3.4 and 8.3.1 servers showed
three important details that are easy to get wrong:

1. On DOMjudge 7.3.4, `/jury/problems` is effectively the global jury
   problem catalog for export purposes. Setting `domjudge_cid` is not
   sufficient to make the scraped list equal the configured contest's
   problem set.
2. DOMjudge 7.3.x and 8.3.1 accept different multipart file field names
   when importing a new problem package into a contest. DOMjudge 7.3.x
   expects `zip[]`; DOMjudge 8.3.1 expects `zip`.
3. The optional multipart `problem` form field means "update this existing
   problem id", not "use this package's external id". Sending it on a
   create import causes DOMjudge to reject packages whose target problem
   does not already exist.

The same validation also found that DOMjudge 8.3.1 has
`DELETE /api/v4/contests/{cid}/problems/{id}`, but its documented
semantics are unlink-only: it removes the problem from that contest, not
from the global problem catalog. DOMjudge 7.3.x does not expose the same
contest problem unlink API.

## Decision

We decided that problem package workflows stay API-scoped before using
the scraper:

1. `download-problems` reads `/api/v4/contests/{cid}/problems` when
   `domserver.json.contest_id` is set and the operator did not pass
   `--only`. It then passes the returned API `id` values to the scraper
   as the effective `Only` filter. The scraper still downloads the ZIPs
   from jury export links.
2. Operator-provided `--only` remains an explicit override and is passed
   through unchanged for compatibility.
3. `import-problems` omits the optional `problem` form field for normal
   creates. It chooses the multipart file field from the configured
   DOMjudge version: `zip[]` for 7.x and `zip` for 8.x.
4. `--problem-id` remains an update-only escape hatch for a single ZIP.
   It intentionally sends `problem=<id>` and should not be used for
   creating new packages from metadata.

### Alternatives considered

- **Rely on the jury contest cookie for `download-problems`.** Rejected.
  Live 7.3.4 validation showed that the jury problem list can still expose
  the global catalog, yielding many more ZIPs than the contest contains.
- **Move problem ZIP export fully to the API.** Rejected. The API exposes
  metadata and statements, but not full problem-package ZIP export on the
  supported versions.
- **Always use `zip[]` for imports.** Rejected. DOMjudge 8.3.1 accepts the
  OpenAPI-documented `zip` field for create imports, while the legacy
  `zip[]` field is required on 7.3.x.
- **Treat v8 problem DELETE as a full delete.** Rejected. The documented
  endpoint is contest unlink only. Global problem deletion remains a jury
  UI operation and is too destructive to hide behind a normal import/export
  command.

## Consequences

- Positive: `download-problems` with `contest_id` now exports the actual
  contest problem set instead of the global jury catalog.
- Positive: `import-problems` works against both DOMjudge 7.3.x and 8.3.1
  without one version's multipart field requirement breaking the other.
- Positive: duplicate external-id failures are easier to reason about: they
  mean a global problem already exists and must be linked or removed; they
  are not a signal to send `problem=<externalid>` for create.
- Negative: problem package workflows remain hybrid. The API scopes and
  imports, while the scraper still exports full package ZIPs.
- Negative: recovery from an accidental v8 import may require two separate
  operations: unlink from the contest via API, then optionally delete
  orphan global problems through the jury UI. The CLI should not silently
  perform the second operation.
- Follow-up: if DOMjudge later adds a full problem-package export API or a
  non-destructive global problem delete/link workflow that is stable across
  versions, write a superseding ADR before changing these routes.

## References

- [0006. API-first with scraper fallback](0006-api-first-with-scraper-fallback.md)
- [0008. DOMjudge 8.3.1 support and version selection](0008-domjudge-8.3.1-support-and-version-selection.md)
- [0009. API v4 cross-version response compatibility](0009-api-v4-cross-version-response-compatibility.md)
- [`docs/CLI.md`](../CLI.md)
