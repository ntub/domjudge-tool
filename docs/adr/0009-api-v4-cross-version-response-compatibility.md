# 0009. API v4 cross-version response compatibility

- **Status:** Accepted
- **Date:** 2026-06-04
- **Deciders:** @project-maintainer

## Context

DOMjudge 7.3.x and 8.3.1 use API v4, but their JSON representations are not
identical. Sharing a protocol version does not justify assuming identical
field types.

Two differences require explicit decoding:

1. A problem's `time_limit` can be an integer or an integral floating JSON
   number such as `5.0`. The domain model deliberately keeps an `int`.
2. `/api/v4/version` can return string or numeric `api_version`. The client
   deliberately keeps its string return type.

Other changes are additive or affect fields that current operations do not
consume: contest/submission external-id spelling, missing organization
URL/hashtag, missing team members, and missing judgement judgehost. The
dual-version tests record the audited boundary without adding unused aliases
or making every model field dynamically typed. Source-code responses remain
arrays of `model.SubmissionFile`.

## Decision

Keep one typed `internal/client/api/v4` package for both server families,
with narrow, field-specific compatibility decoders:

- `Problem.TimeLimit` and `ProblemItem.TimeLimit` accept integer JSON
  numbers and integral floating numbers. Fractional and out-of-range
  values are errors; they are not silently truncated or rounded.
- Missing `time_limit` leaves the zero value; JSON `null` decodes to zero.
- `General.Version` accepts string or integer `api_version` and returns a
  string. JSON `null` produces an empty string; unrelated JSON types are
  not broadly coerced.
- Ordinary decoding continues to ignore unknown fields. Do not enable
  `Decoder.DisallowUnknownFields` for upstream response objects.
- Sanitized v7 and v8 fixtures and shared resource tests remain side by
  side. Supporting v8 does not replace or weaken the v7 fixture contract.
- Keep typed models rather than introducing `map[string]any` or a generic
  loose decoder.

### Alternatives considered

- **Fork the API client into `api/v4/v7` and `api/v4/v8`.** Rejected. The API
  surface is mostly shared, and the confirmed differences are small enough to
  handle with targeted decoders. Forking would duplicate resources, tests, and
  caller wiring for little gain.

- **Change model fields to `float64` or `any`.** Rejected. That would leak
  server drift into domain models and downstream code. The CLI does not need
  fractional time limits as a public value; it needs decoding to succeed and
  existing behaviour to remain stable.

- **Use `Decoder.DisallowUnknownFields` to detect all drift.** Rejected. The
  v8 OpenAPI is under-described compared with live responses, and DOMjudge may
  add fields that the CLI does not care about. Strict unknown-field rejection
  would turn harmless additive changes into command failures.

- **Use protocol-version probing to select a scraper.** Rejected:
  `api_version` identifies the API protocol, not the DOMjudge server
  family. Config-driven scraper selection remains a separate contract
  under [ADR 0008](0008-domjudge-8.3.1-support-and-version-selection.md).

## Consequences

- Both supported server families share resource clients and caller-facing
  types; server representation differences stay at the decoding boundary.
- Harmless additive upstream fields do not turn into command failures.
- Fractional time limits fail explicitly instead of changing downstream
  integer semantics.
- Tolerating missing fields does not promise aliasing or value recovery for
  every upstream rename. If an operation begins consuming an audited,
  currently unused field, its semantics need explicit dual-version tests.
- Decoder tolerance does not make broken write endpoints safe. The current
  v8 create/update/email/password/affiliation workflow remains scraper-backed
  under [ADR 0006](0006-api-first-with-scraper-fallback.md).

## References

- [Current v8 compatibility and fixture contract](../phase2/CONTRACT.md).
- [API-first routing](0006-api-first-with-scraper-fallback.md).
- [Version decoder](../../internal/client/api/v4/general.go#L30)
  (`internal/client/api/v4/general.go:30`).
- [Problem decoders and integer policy](../../internal/model/problem.go#L26).
- [Dual-version resource tests and silent-drift audit](../../internal/client/api/v4/cross_version_test.go#L13).
