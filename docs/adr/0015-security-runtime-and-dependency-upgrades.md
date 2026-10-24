# 0015. Security Runtime and Dependency Upgrades

- **Status:** Accepted
- **Date:** 2026-10-23
- **Supersedes:** 0003, 0004, and the toolchain selection in 0014

## Context

The toolchain and dependencies need a security-maintenance policy that
follows supported, patched releases without changing the tool's behavior.
The older individual Cobra/goquery pins and desktop toolchain selection no
longer describe the build and have been retired.

The security refresh addresses findings recorded before the upgrade:
GO-2026-6809 in `crypto/x509`, GO-2026-5970 in `golang.org/x/text`, and
GO-2026-5025, GO-2026-5027, GO-2026-5028, GO-2026-5029, and GO-2026-5030 in
`golang.org/x/net/html`. These are the reason for the refresh, not a claim
about current scanner output.

Frontend asset builds also need a patched Node.js baseline and reproducible
npm dependencies. The Tailwind CLI's original watcher dependency chain
included `micromatch` and `braces`, affected by GHSA-vfj7-8cjw-p6xm. The
selected watcher uses `picomatch`; the lockfile also carries the patched
`source-map-js`. Removing the vulnerable chain is preferable to suppressing
audit findings or downgrading the CSS toolchain.

## Decision

Use **Go 1.27.3 or newer** for source builds, including the October
`crypto/x509` fix. The `go.mod` `go` directive is the module baseline;
`mise.toml` pins the same exact toolchain for contributors and CI
(see [ADR 0017](0017-mise-toolchain-and-tag-release.md)).

Prefer released, non-retracted stable dependency versions, with security
patches taking priority. The current manifest selects Cobra 1.10.2 and
goquery 1.13.0; their obsolete standalone pin decisions do not constrain
upgrades. Review breaking changes and adapt affected source logic while
preserving the contracts below.

The Desktop already uses pre-release Wails v3. That explicit architecture
exception tracks the selected v3 beta rather than changing frameworks or
reverting to Wails v2. Tagged Desktop builds need their own native
dependencies and checks in addition to the default CLI path.

Use **Node.js 24.22.1 or a later patched 24.x release** for frontend asset
builds. `mise.toml` records the selected build runtime; Node.js is not
an application runtime. Install with the committed npm lockfile.

Tailwind CSS and CLI 4.3.3 remain selected. `frontend/package.json` overrides
`@parcel/watcher` to 2.6.0 to remove the affected chain. Reassess that override
when upstream removes the need; do not hide findings or downgrade Tailwind
to satisfy an audit.

The configured CI actions and linter selections must support the Go/Node
baselines. CI installs them from `mise.lock`; updating workflow files does
not itself establish that CI has run.

Compatibility checks preserve the existing `encoding/json` API, CLI
commands and flags, `domserver.json`, CSV column order, DOMjudge v7/v8
routing, and the locked inverted deletion filters. Go 1.27 removes old
timer/TLS GODEBUG overrides and honors SSL_CERT_FILE/SSL_CERT_DIR on macOS
and Windows; no insecure compatibility overrides are introduced.

## Consequences

- Source builds require Go 1.27.3 or newer. Prebuilt CLI users do not need
  Go or Node.js installed.
- Frontend contributors use Node.js 24.22.1 or a later patched 24.x release
  and install from the committed npm lockfile.
- Upgrade acceptance requires unchanged compatibility fixtures and tests.
  Generated assets and their generators must agree; presence of an upgrade in
  the working tree is not evidence that a particular check passed.
- Vulnerability checks must cover the CLI and web UI builds, and npm
  development dependencies.
- HTML parser upgrades preserve behavior against the existing sanitized
  scraper fixtures. Any necessary recapture follows the fixture privacy
  protocol and secret/PII gates before commit; raw production captures are
  not replacement fixtures.
- No package boundaries, CLI contracts, database schema, or DOMjudge
  transport policy change as part of this upgrade.

## References

- [Go release history](https://go.dev/doc/devel/release)
- [Go 1.27 compatibility changes](https://go.dev/doc/go1.27)
- [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809)
- [Node.js supported releases](https://nodejs.org/en/about/previous-releases)
- [braces advisory](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm)
- [goquery changelog](https://github.com/PuerkitoBio/goquery#changelog)
- [Current Go manifest](../../go.mod) and [module checksums](../../go.sum).
- [Toolchain pins](../../mise.toml),
  [dependency policy](../../frontend/package.json), and
  [lockfile](../../frontend/package-lock.json).
- [CI toolchain configuration](../../.github/workflows/ci.yml).
- [Web UI architecture](0013-phase3-frontend-stack.md).
- [Fixture maintenance and privacy gates](../TESTING.md).
