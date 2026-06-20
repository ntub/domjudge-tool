# 0011. Import Problems Upsert

- **Status:** Accepted
- **Date:** 2026-06-20
- **Deciders:** @project-maintainer

## Context

`import-problems` currently has two explicit modes:

1. Normal imports omit the optional `problem` multipart form field and ask
   DOMjudge to create a new contest problem from package metadata.
2. `--problem-id` imports exactly one ZIP and sends `problem=<id>` as an
   update-only escape hatch.

That split keeps creates and updates understandable, but it makes repeated
folder imports awkward. A second run against the same contest can fail with
`problem.externalid: This value is already used` even when the operator's
intent is to update the contest's existing problem package.

DOMjudge does not expose a native "bulk upsert problem packages" operation.
The v4 API can list the problems linked to one contest through
`GET /api/v4/contests/{cid}/problems`, and the import endpoint can either
create a problem or update a specific existing problem depending on whether
the multipart request includes the `problem` field.

Problem package identity also differs by supported version. DOMjudge 8.x
honours `domjudge-problem.ini` `externalid` during create imports. The
current DOMjudge 7.3.x import path effectively falls back to filename-driven
identity because 7.3 imports ignore `externalid` from the ini file in the
supported path.

## Decision

We decided to add `import-problems --upsert` as tool-side, contest-scoped
reconciliation. It first indexes the configured contest's existing problems
with `GET /api/v4/contests/{cid}/problems`, then imports each ZIP as either
an update of a matched contest problem or a create of a missing contest
problem.

`--upsert` is mutually exclusive with `--problem-id`. `--problem-id` remains
the explicit single-ZIP update path; `--upsert` owns folder-friendly
create-or-update behavior.

The first implementation uses this matching contract:

1. Derive a package identity for each ZIP.
   - On DOMjudge 8.x, prefer `domjudge-problem.ini` `externalid` when it is
     present and non-empty.
   - Otherwise use the sanitized filename stem.
   - On DOMjudge 7.x, use the sanitized filename stem. This matches the
     current supported 7.3 import behavior, where the ini `externalid` is not
     a reliable create identity.
2. Build an index of contest problems by `id` and `externalid`.
   - If the same identity maps to different contest problems, fail before
     uploading. Silent overwrite could update the wrong package.
3. If the package identity matches an existing contest problem, send an
   update import with `problem=<existing id>`.
   - On DOMjudge 8.x, send the multipart ZIP filename as the matched
     problem's `externalid` plus `.zip`, falling back to the problem id when
     `externalid` is empty. DOMjudge 8.3.1 derives package identity from the
     uploaded filename in this path, so preserving the matched external id
     avoids accidental duplicate identities from local filenames.
4. If the package identity does not match the contest index, send a create
   import using the existing create path.
5. If the create fails because a problem with the same external id exists
   globally but is not linked to the contest, fail with the DOMjudge error
   body. The first version does not auto-link, unlink, or delete global
   problems.

The multipart file field remains version-specific:

- DOMjudge 7.x create uses `zip[]`.
- DOMjudge 8.x create uses `zip`.
- DOMjudge 7.x update uses the existing `Imports.Problem` path:
  `problem=<existing id>` plus `zip[]`.
- DOMjudge 8.x update uses `problem=<existing id>` plus `zip`, matching the
  DOMjudge 8.3.1 controller's request shape.

DOMjudge 7.3.x update imports can respond with numeric `problem_ids` instead
of the string arrays and `problem_id` object fields used by other import
paths. The shared import response decoder accepts both forms so update
success is not mistaken for a client-side decode failure.

The sanitized filename stem is the base filename with its final extension
removed and only ASCII letters, digits, `-`, and `_` retained. An empty result
is invalid. As a compatibility fallback for ZIPs produced by
`download-problems`, matching also tries the prefix before the first
underscore in names such as `{id}_{name}.zip`. Matching also tries the stem
after trimming leading and trailing `-` / `_`, which handles local filenames
whose unsupported prefix was removed during sanitization.

### Alternatives considered

- **Retry duplicate-create errors with `--problem-id` semantics.** Rejected.
  The duplicate external id might refer to a global problem that is not linked
  to the configured contest. Retrying as an update would hide that distinction
  and could mutate the wrong problem package.
- **Search and auto-link global problems on duplicate external id.** Rejected
  for the first version. Linking and deleting global problem catalog entries
  have version-specific and destructive edge cases, especially after ADR 0010
  confirmed that DOMjudge 8.3.1 contest DELETE is unlink-only.
- **Match ZIPs only by filename.** Rejected for DOMjudge 8.x because
  `domjudge-problem.ini` `externalid` is the authoritative package identity
  on the supported create path. Filename fallback remains necessary for 7.x
  compatibility and for packages that omit the ini field.
- **Make normal `import-problems` upsert by default.** Rejected. Existing
  create-only behavior is useful when operators want duplicate external-id
  failures to stop the run instead of mutating existing contest problems.

## Consequences

- Positive: repeated folder imports can update a contest's existing problems
  without requiring one `--problem-id` invocation per ZIP.
- Positive: upsert remains contest-scoped and explainable: only problems
  returned by `/api/v4/contests/{cid}/problems` are eligible update targets.
- Positive: global duplicate external ids remain visible operator problems;
  the CLI does not guess whether it should link, unlink, or delete anything.
- Negative: `--upsert` requires one extra API list request before imports.
- Negative: matching depends on package identity discipline. If an operator
  renames ZIP files on DOMjudge 7.x, the CLI treats them as different
  problems because 7.3 does not reliably honour ini `externalid`.
- Follow-up: if DOMjudge later exposes stable global problem lookup/link
  APIs across supported versions, write a superseding ADR before adding an
  auto-link mode.

## References

- [0006. API-first with scraper fallback](0006-api-first-with-scraper-fallback.md)
- [0010](0010-problem-package-contest-scoping-and-import.md)
- [`docs/CLI.md`](../CLI.md)
