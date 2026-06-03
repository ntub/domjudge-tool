# 0009. API v4 cross-version response compatibility

- **Status:** Proposed
- **Date:** 2026-06-04
- **Deciders:** @project-maintainer

## Context

Phase 1.5 made `internal/client/api/v4` the primary transport for most CLI
operations. Phase 2 keeps that package name because DOMjudge 8.3.1 still uses
API v4, but the live 8.3.1 station confirmed that "same API version" does not
mean "identical JSON response shapes."

The confirmed hard breaks are:

1. `problems[].time_limit` is a JSON number with a decimal on 8.3.1. The live
   value observed was `5.0`, while `model.Problem.TimeLimit` and
   `model.ProblemItem.TimeLimit` are Go `int` fields. This causes
   `encoding/json` to fail before downstream commands can run.
2. `/api/v4/version` returns `{"api_version":4}` on 8.3.1, while
   `versionBody.APIVersion` is a Go `string`.

The investigation also found live-confirmed non-breaks:

- `source-code` is still a JSON array compatible with
  `[]model.SubmissionFile`.
- Live problem responses still include `ordinal`, `test_data_count`, and
  `externalid`, even though the v8 OpenAPI description under-describes those
  fields.

There are also silent drifts to verify during P2-0 fixture capture: contest
and submission `externalid` may appear as `external_id`, organizations dropped
`url` and `twitter_hashtag`, teams dropped `members`, and judgements dropped
`judgehost`. These do not necessarily fail decoding because Go's JSON decoder
ignores unknown fields by default, but downstream code can still lose values if
it reads only the old field name.

## Decision

We will make `internal/client/api/v4` decoders tolerant across DOMjudge 7.3.x
and 8.3.1 while keeping the package API stable for callers.

- `time_limit` accepts both integer JSON numbers and integral floating JSON
  numbers such as `5.0`.
- `/api/v4/version` accepts both string and integer `api_version` values and
  returns the existing Go string result.
- The API client remains additive-safe by continuing to use the default
  `encoding/json` behaviour that ignores unknown fields. We will not enable
  `Decoder.DisallowUnknownFields`.
- P2-0 captures sanitized v8 fixtures under `testdata/api/v4/v8/`. P2-1 adds
  dual-version decoder tests so v7 fixture coverage remains in place while v8
  support is added.
- Compatibility logic is narrow and field-specific. We do not replace typed
  models with `map[string]any` or a generic loose decoder.

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

- **Ignore `/api/v4/version` because no command currently calls it.**
  Rejected. The endpoint is part of the public API client and is relevant to
  future diagnostics. Its drift is easy to fix narrowly, and leaving it broken
  would invite accidental use as a failing auto-detection path.

## Consequences

- Positive: Existing API-first commands can continue to share one
  `internal/client/api/v4` package across DOMjudge 7.3.x and 8.3.1.
- Positive: The fix is small and testable: field-specific decoders plus
  dual-version fixtures.
- Positive: Additive upstream fields do not break the CLI.
- Negative: Some silent drifts still require explicit downstream audits. A
  decoder can accept missing `members` or `judgehost`, but a use-case that
  depends on those fields must still be tested against v8 fixtures.
- Negative: Integral-float acceptance must define a policy for non-integral
  values. Phase 2 should reject or explicitly round only with tests; it should
  not silently truncate.
- Follow-up: P2-0 captures the v8 fixtures and records the silent drift table.
  P2-1 implements the decoder changes and dual-version tests. P2-4 uses the
  organization drift to send a v8-safe import body without `url` or
  `twitter_hashtag`.

## References

- `docs/phase2/SPEC.md`
- `docs/phase2/CONTRACT.md`
- ADR 0006 — API-first with scraper fallback.
- `internal/client/api/v4/general.go`
- `internal/client/api/v4/problems.go`
- `internal/client/api/v4/client.go`
- `internal/model/problem.go`
