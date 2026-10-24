# 0017. mise Toolchain, Tasks, and Tag-Only Release Pipeline

- **Status:** Proposed
- **Date:** 2026-10-24
- **Deciders:** project maintainers

## Context

Contributors and CI installed tools differently. `just` ran tasks, Go came
from `actions/setup-go`, Node.js from `.node-version`, formatters and linters
from `@latest` installs or Homebrew, and CI piped a TruffleHog install script
from its `main` branch. Versions drifted between machines, an upstream
release race broke the secret-scan install, and nothing verified most tool
downloads. Releases were cut by hand without automated verification.

## Decision

[mise](https://mise.jdx.dev/) is the single source for every development
runtime, tool, and task.

- `mise.toml` pins exact versions of Go, Node.js, Python, gofumpt,
  golangci-lint, templ, Lefthook, TruffleHog, actionlint, zizmor, git-cliff,
  GoReleaser, and the GitHub CLI. It sets `GOTOOLCHAIN=local` so builds never
  download another Go toolchain. The Go pin equals the `go.mod` directive;
  the templ pin equals the `github.com/a-h/templ` module version.
- `mise.lock` records URLs, checksums, and verified provenance for six
  platforms: Linux, macOS, and Windows on x64 and arm64. CI installs with
  `--locked` and `locked_verify_provenance`, so an unrecorded or altered
  artifact fails the job.
- Tasks replace the Justfile with the same names (`mise run check`,
  `mise run build`, …). Git hooks call `mise exec`/`mise run`, so they use the
  pinned tools without shell activation. npm packages stay in the committed
  `frontend/package-lock.json` and install with `npm ci --ignore-scripts`.
- Every workflow installs mise with `jdx/mise-action` at a pinned mise
  version and runs repository tasks. Jobs that need only some tools pass
  `install_args` and disable task auto-install.

Releases publish only from `.github/workflows/release.yml`, triggered by
pushing a tag matching `v[0-9]+.[0-9]+.[0-9]+`:

1. `verify` requires an annotated tag on a commit reachable from `main`, the
   strict `vMAJOR.MINOR.PATCH` form, and a non-empty `## [MAJOR.MINOR.PATCH]`
   section in `CHANGELOG.md`, which becomes the release notes.
2. The CI gates and the six GoReleaser CLI archives build in parallel with
   read-only tokens; the one-line installer scripts (`scripts/install.sh` and
   `scripts/install.ps1`) are staged as release assets.
3. `publish` runs in the `release` environment, verifies every checksum,
   records Sigstore build-provenance attestations for every archive, and
   creates the GitHub release. It is the only job with `contents: write`.

Workflow hardening: top-level `permissions: {}` with per-job grants, every
action pinned to a full commit SHA, `persist-credentials: false`, no caches in
release jobs, release jobs never cancelled mid-run, and `mise run lint-actions`
(actionlint and zizmor) as a CI gate.

Dependency security reporting is read-only. Dependabot alerts stay enabled;
Dependabot version and security-update pull requests are not configured.
`.github/workflows/dependabot-report.yml` runs monthly and opens a single
issue, `Dependabot report YYYY-MM`, listing open alerts. It uses the
`vulnerability-alerts: read` token permission and never changes code,
dependencies, or alerts.

`.github/workflows/release.yml` is implemented in the working tree; no
release has been produced by it yet.

### Alternatives considered

- **Keep just with per-tool installers.** Leaves versions and integrity
  unpinned and duplicates setup between machines and CI.
- **GoReleaser publishes the release.** GoReleaser builds the portable CLI
  archives; a dedicated publish job in the release environment verifies
  checksums, attaches attested archives, checksums, and installer scripts, and
  uses one write-scoped token and one curated notes source.
- **Generate notes with git-cliff at release time.** Curated
  `CHANGELOG.md` sections are reviewed before the tag exists; generated
  notes are not.
- **Draft releases.** Publishing is already gated by the tag, the `main`
  ancestry check, and the `release` environment; drafts add a manual step
  without a security benefit.

## Consequences

- Positive: one command (`mise install`) reproduces the toolchain on every
  supported platform, verified against `mise.lock`.
- Positive: published archives carry SHA-256 checksums and verifiable
  provenance (`gh attestation verify <file> --repo ntub/domjudge-tool`).
- Negative: contributors install mise. Upgrading any tool means editing
  `mise.toml`, running `mise lock`, and committing both files.
- Negative: a release tag without a matching `CHANGELOG.md` section, or on
  a commit outside `main`, fails before anything is built.
- Constraints: CLI binaries are not OS-code-signed; integrity and authenticity
  come from SHA-256 checksums and Sigstore attestations. Tag protection rules
  and `release` environment reviewers are repository settings, not files;
  they must be configured on GitHub to stop a writer from pushing a release tag
  alone.

## References

- [mise configuration](../../mise.toml) and [lockfile](../../mise.lock).
- [CI](../../.github/workflows/ci.yml) and
  [release](../../.github/workflows/release.yml) workflows.
- [Release-notes extractor](../../scripts/release_notes.py).
- [ADR 0015](0015-security-runtime-and-dependency-upgrades.md) for runtime
  security baselines and [ADR 0018](0018-local-web-ui-command.md) for the
  local web UI command.
- [mise lockfile](https://mise.jdx.dev/dev-tools/mise-lock.html) and
  [mise-action](https://github.com/jdx/mise-action).
- [GitHub artifact attestations](https://docs.github.com/actions/security-for-github-actions/using-artifact-attestations).
