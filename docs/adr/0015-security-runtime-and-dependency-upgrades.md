# 0015. Security Runtime and Dependency Upgrades

- **Status:** Proposed
- **Date:** 2026-10-23
- **Supersedes:** 0003, 0004, and the toolchain selection in 0014

## Context

The module declares Go 1.26.0, while the development toolchain is Go 1.27.2.
The baseline `govulncheck ./...` reports seven reachable vulnerabilities:
GO-2026-6809 in `crypto/x509`, GO-2026-5970 in `golang.org/x/text`, and
GO-2026-5025, GO-2026-5027, GO-2026-5028, GO-2026-5029, and GO-2026-5030 in
`golang.org/x/net/html`. Existing CLI tests and the macOS desktop build pass
before upgrading.

The frontend build uses Node.js without a repository version requirement.
Its npm lockfile reports five high-severity findings. Tailwind CLI 4.3.3
still pins `@parcel/watcher` 2.5.1, whose `micromatch` dependency brings in
`braces` with no patched release for GHSA-vfj7-8cjw-p6xm. Watcher 2.6.0
replaces that chain with `picomatch`. `source-map-js` has a patched 1.2.2
release.

The old Cobra and goquery pins protected a Go 1.23 baseline that ADR 0014
already raised for Phase 3. Keeping those pins no longer protects the
current build baseline.

## Decision

We selected Go 1.27.3 as the minimum language/toolchain version, including
the October `crypto/x509` security fix. CI reads the exact version from
`go.mod`. We selected Node.js 24.22.1, the current patched Active LTS
release, for frontend asset builds; Node.js is not an application runtime.

We lifted the Cobra and goquery pins and selected current non-retracted
stable releases for Go dependencies. The desktop shell already depends on
pre-release Wails v3; its upgrade follows the current v3 beta rather than
changing framework or reverting to Wails v2. Wails remains pre-release and
requires a tagged desktop build in addition to the normal CLI checks.

We selected Tailwind CSS and CLI 4.3.3 and a narrowly scoped npm override
of `@tailwindcss/cli`'s watcher to 2.6.0. This keeps the CLI on its current
release while removing the unpatched dependency chain. Reassess the
override when upstream ships a fixed watcher dependency; do not suppress
audit findings or downgrade Tailwind to satisfy the scanner.

We upgraded CI actions to current releases using Node.js 24 and the
linter to a version supporting Go 1.27. GitHub-hosted runners meet the
minimum Actions Runner 2.327.1 requirement. Go setup keeps its cache keyed
on `go.sum` explicitly because newer setup-go versions default to
`go.mod`.

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
- Source compatibility is verified against unchanged fixtures and tests,
  including the desktop build tag. Dependencies and generated assets must
  agree with their generators.
- Vulnerability checks cover the default build, desktop-tagged code, and
  npm development dependencies. Native desktop support on other operating
  systems still requires their respective platform SDKs.
- No package boundaries, CLI contracts, database schema, or DOMjudge
  transport policy change as part of this upgrade.

## References

- [Go release history](https://go.dev/doc/devel/release)
- [Go 1.27 compatibility changes](https://go.dev/doc/go1.27)
- [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809)
- [Node.js supported releases](https://nodejs.org/en/about/previous-releases)
- [braces advisory](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm)
- [goquery changelog](https://github.com/PuerkitoBio/goquery#changelog)
- [setup-go migration](https://github.com/actions/setup-go#breaking-changes-in-v6)
- [ADR 0014](0014-go-toolchain-bump-for-phase3.md)
