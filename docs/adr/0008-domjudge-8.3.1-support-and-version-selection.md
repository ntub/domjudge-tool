# 0008. DOMjudge 8.3.1 support and version selection

- **Status:** Accepted
- **Date:** 2026-06-04
- **Deciders:** @project-maintainer

## Context

The CLI supports DOMjudge 7.3.x and 8.3.1 through one API client and two
versioned jury scrapers. The existing `config.DomServerClient.Version`
field selects the scraper; its default remains `"7.3.2"` for config
compatibility.

API v4 alone cannot identify the DOMjudge server family. For example,
8.3.1 returns numeric `api_version: 4`, while older responses use a string.
The tolerant decoder handles that response, but an extra version probe
would still introduce a network dependency before the requested operation.
Operators already configure the target server explicitly.

The v8 scraper is required for user/team deletion, full problem-package ZIP
export, and the current user/team import write workflow. Bulk account
imports drop email on 8.3.1, and the tested user PUT path does not perform a
working update. An explicit version choice keeps these constraints visible.

## Decision

Use **config-driven version selection** through the existing `version`
field and `internal/client/web/gatewayfactory`.

- The factory maps `7.3` and values prefixed by `7.3.` to `web/v7`.
- It maps `8.3` and values prefixed by `8.3.` to `web/v8`; `8.3.1` is the
  documented v8 compatibility target. Accepting the family spelling does
  not establish validation of every upstream patch release.
- Unsupported families return `gatewayfactory.ErrUnsupportedVersion`;
  there is no silent fallback to v7.
- No runtime auto-detection request selects the scraper.
- Both implementations satisfy the same `web.Gateway` interface. Callers
  use the factory rather than selecting HTML paths themselves.
- Current create/update/email/password/affiliation import writes use the
  scraper on v8. The supported standalone password path is also
  scraper-backed on v8. See [ADR 0006](0006-api-first-with-scraper-fallback.md)
  for the complete routing contract.

The existing `domserver.json` field names, types, and defaults remain
unchanged by version selection. Choosing another supported server changes
the value of `version`, not the config schema or CLI command names.

### Alternatives considered

- **Auto-detection via `/api/v4/version` or `/api/v4/info`.** Rejected:
  selecting a transport should not require an unrelated request, and API
  protocol version is not a reliable scraper-family selector.
- **Separate v7 and v8 binaries.** Rejected: most CLI, domain, API,
  import/export, and scoreboard code is shared. Only the jury HTML
  implementation needs a family-specific adapter.
- **Use bulk REST imports for all v8 writes.** Rejected: successful
  transport status does not prove preservation of email, team linkage,
  password, roles, or updates.
- **Add `server_version` or `auto_detect` config fields.** Rejected:
  `version` already provides the required explicit choice without changing
  Python-compatible config.

## Consequences

- A single binary serves both supported DOMjudge families.
- Version routing is deterministic and inspectable in operator-managed
  config, with no auto-detection startup dependency.
- The v8 gateway includes the write methods needed by active workflows;
  they are not unfinished placeholders or unused future support.
- Operators update `version` when their server changes family. A typo in
  an unsupported family fails instead of invoking the wrong scraper.
- Additional server families require a factory entry and representative,
  sanitized fixtures. API routing changes also need operation-level
  readback evidence under ADR 0006.

## References

- [Current v8 compatibility contract](../phase2/CONTRACT.md).
- [API-first routing](0006-api-first-with-scraper-fallback.md).
- [Cross-version JSON decoding](0009-api-v4-cross-version-response-compatibility.md).
- [Config model](../../internal/config/config.go).
- [Gateway interface](../../internal/client/web/gateway.go).
- [Factory and family predicate](../../internal/client/web/gatewayfactory/gatewayfactory.go#L28)
  (`internal/client/web/gatewayfactory/gatewayfactory.go:28`).
