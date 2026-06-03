# 0008. DOMjudge 8.3.1 support and version selection

- **Status:** Proposed
- **Date:** 2026-06-04
- **Deciders:** @project-maintainer

## Context

Phase 1.5 shipped an API-first CLI for DOMjudge 7.3.x with scraper fallback.
The codebase now has two independent transport layers:

- `internal/client/api/v4/`, used by the default path for API-supported
  commands.
- `internal/client/web`, whose `Gateway` interface remains the
  version-agnostic compile-time contract, but has only 5 active default
  callers since ADR 0006: `Login`, `GetProblems`, `Download`, `DeleteUsers`,
  and `DeleteTeams`.

Phase 2 adds DOMjudge 8.3.1 support without changing the user-facing CLI or
the `domserver.json` schema. The existing config model already has
`config.DomServerClient.Version`; it defaults to `"7.3.2"` and is stable
because Python compatibility requires the config field names, types, and
defaults to remain unchanged.

The Phase 2 live investigation against the live 8.3.1 station also found that
runtime auto-detection is not free:

1. `/api/v4/version` returns `{"api_version":4}` on 8.3.1, while the current
   Go decoder expects a string.
2. DOMjudge operators already know the target server version before running
   the CLI.
3. Auto-detection would add a network request before selecting the web
   scraper, and that request can fail for reasons unrelated to the command the
   operator is trying to run.

The scraper still matters on 8.3.1. The live findings confirmed there is no
REST API DELETE for users or teams, and there is no full problem-package ZIP
export endpoint. `rm-teams-and-users` and `download-problems` therefore still
need a DOMjudge 8 jury HTML implementation.

## Decision

We will support DOMjudge 8.3.1 through **config-driven version selection**
using the existing `config.Version` field, plus a `web.Gateway` factory that
returns either the v7 or v8 implementation.

- `config.Version == "8.3.1"` selects `internal/client/web/v8`.
- Supported 7.3.x values select `internal/client/web/v7`.
- Unsupported values return an explicit error.
- The CLI will not auto-detect the DOMjudge version at runtime.
- `web/v8` satisfies the full `web.Gateway` interface without splitting the
  interface or changing callers.
- `web/v8` actively implements the 5 active `Gateway` methods:
  `Login`, `GetProblems`, `Download`, `DeleteUsers`, and `DeleteTeams`.
- The 6 deprecated fallback methods (`GetAffiliations`, `GetAffiliation`,
  `CreateAffiliation`, `CreateTeamAndUser`, `UpdateTeam`, and
  `SetUserPassword`) are implemented on v8 as unsupported stubs returning a
  new `web.ErrUnsupportedOnV8` sentinel error. If `prefer_scraper` forces one
  of those flows on v8, it fails fast with an explicit error rather than
  silently misbehaving.
- Any other non-active fallback method required by the current `web.Gateway`
  shape is also stubbed with the same sentinel so the concrete v8 client is
  assignable to `web.Gateway`.

The `domserver.json` schema remains unchanged. Phase 2 only changes the value
an operator may put in the existing `version` field.

### Alternatives considered

- **Auto-detection via `/api/v4/version` or `/api/v4/info`.** Rejected. The
  version endpoint itself has a confirmed v8 response-shape drift, so using it
  for routing creates a startup dependency on the same compatibility work
  Phase 2 is trying to isolate. It also makes every scraper-backed operation
  pay an extra network request and introduces failure modes unrelated to the
  command's actual work.

- **Separate binary for DOMjudge 8.3.1.** Rejected. It would duplicate the
  CLI, config, importer, exporter, scoreboard, and API client code while the
  actual version-specific surface is small. A single binary with an explicit
  config value keeps the operational model simple and preserves Phase 1
  compatibility.

- **Actively implement all legacy form flows in `web/v8`.** Rejected. ADR
  0006 already shrank the active scraper surface. Recreating deprecated form
  flows for v8 would increase maintenance cost and reintroduce HTML scraping
  where the API is the intended path. v8 still satisfies the compile-time
  interface, but deprecated flows return an explicit unsupported sentinel.

- **Add a new config field such as `server_version` or `auto_detect`.**
  Rejected. `version` already exists, and Phase 2 has a hard compatibility
  lock on `domserver.json` field names, types, and defaults.

## Consequences

- Positive: Version routing is explicit, deterministic, and reviewable from a
  checked-in or operator-managed `domserver.json`.
- Positive: The v8 scraper's active surface is small: 5 real methods, while
  deprecated methods are explicit unsupported stubs.
- Positive: `web.Gateway` and all callers stay unchanged; no interface split
  is needed for Phase 2.
- Positive: Forced deprecated scraper paths on v8 fail fast via
  `web.ErrUnsupportedOnV8`.
- Positive: The existing config schema and Python compatibility contract stay
  intact.
- Negative: Operators must set `version` correctly. If a server is upgraded
  from 7.3.x to 8.3.1, the config must be updated manually.
- Negative: The factory needs a clear unsupported-version error path so
  mistyped versions do not silently fall back to v7.
- Follow-up: Phase 2 PR P2-3 adds the factory and CLI wiring tests. Future
  DOMjudge versions can be added by extending the factory decision table and
  adding fixtures before code.

## References

- `docs/phase2/README.md`
- `docs/phase2/SPEC.md`
- `docs/phase2/PLAN.md`
- ADR 0006 — API-first with scraper fallback.
- `internal/config/config.go`
- `internal/client/web/gateway.go`
