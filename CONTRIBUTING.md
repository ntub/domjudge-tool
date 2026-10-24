# Contributing to domjudge-tool

Thanks for working on this project. This guide covers the contributor
workflow: setup, branching, commits, tests, PRs, and releases.

## Prerequisites

| Tool                 | Version            | Required?                        |
| -------------------- | ------------------ | -------------------------------- |
| [mise](https://mise.jdx.dev/getting-started.html) | ≥ 2026.10.14 | Yes (tools and tasks) |
| git                  | ≥ 2.30             | Yes                              |
| Claude Code          | latest, authenticated | Required for feature commits with hooks |
| Chrome / Chromium    | installed          | Only for screenshot capture      |
`mise.toml` pins everything else: Go, Node.js, Python, gofumpt,
golangci-lint, templ, Lefthook, TruffleHog, actionlint, zizmor, git-cliff,
GoReleaser, and the GitHub CLI. `mise.lock` records their checksums and
provenance for Linux, macOS, and Windows on x64 and arm64. Do not install
these tools another way; CI uses exactly the same pins.

## First-time setup

```bash
git clone https://github.com/ntub/domjudge-tool.git
cd domjudge-tool
mise trust            # allow this repository's mise.toml
mise install          # install the pinned tools from mise.lock
mise run setup        # go mod download + npm ci + lefthook install
```

`mise run setup` is idempotent — re-run it whenever dependencies change.
`mise tasks` lists every task. Append arguments after the task name, for
example `mise run run -- --help` or `mise run build-css`.
Activating mise in your shell (`mise activate`) is optional; Git hooks call
the pinned tools through `mise exec` and `mise run`.

### Updating tools

1. Edit the version in `mise.toml`. Keep `go` equal to the `go.mod` `go`
   directive and `aqua:a-h/templ` equal to the `github.com/a-h/templ`
   module version.
2. Run `mise lock` to refresh `mise.lock` for all six platforms.
3. Run `mise install` and `mise run check`, then commit both files.

## Daily workflow

1. **Branch.** Branch off `main`. Naming: `feat/<slug>`, `fix/<slug>`,
   `docs/<slug>`, `chore/<slug>`.
2. **Code + test.** Keep changes scoped. New behaviour gets a test;
   bug fixes get a regression test that reproduces the bug first.
3. **Run the gate locally.** `mise run check` runs
   `fmt + vet + lint + lint-actions + privacy-check + test`.
   The Lefthook pre-commit hook runs formatting, lint, then staged-source AI
   documentation review sequentially; the pre-push hook runs the test suite.
4. **Commit.** Conventional Commits, one logical change per commit. See
   [`docs/COMMITS.md`](docs/COMMITS.md).
5. **Open a PR.** Target `main`. Fill in the description: what + why.
   Keep PRs reviewable (≤ ~400 lines of code change is the soft cap).

## Testing

- Standard library `testing` only — no testify or ginkgo.
- Table-driven tests with `t.Run(...)` subtests.
- Fixtures in `testdata/<package>/<case>.{html,json,csv}`.
- Never hit a real DOMjudge — use `httptest.NewServer`.
- Run `mise run privacy-check` before committing fixture or documentation
  changes. TruffleHog catches secrets; the project PII gate catches known
  DOMjudge host, school, student, and fixture sanitization leaks.
- Full guidance: [`docs/TESTING.md`](docs/TESTING.md).

## Code style

- `mise run fmt` before committing (the hook does it for you).
- Errors are wrapped with `%w`; no `log.Fatal` in libraries.
- I/O takes `ctx context.Context` first.
- Full guidance: [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md).

## Documentation changes

- User-facing change → also update `README.md` and (if it ships in a
  release) `CHANGELOG.md` will pick it up automatically from the
  Conventional Commit.
- Architectural change → update `docs/ARCHITECTURE.md`.
- Significant decision → add an ADR via `docs/adr/template.md`.

### Illustrated manuals and AI review

The paired [English](docs/USER_GUIDE.md) and
[Taiwan Traditional Chinese](docs/USER_GUIDE.zh-TW.md) manuals share their
section/command/image order. The complete policy is in
[`docs/DOCUMENTATION.md`](docs/DOCUMENTATION.md#illustrated-user-guides-and-automated-review).

Install Chrome or Chromium for screenshots and the current
[Claude Code CLI](https://code.claude.com/docs/en/setup). Authenticate with
your own account; no credentials belong in this repository:

```bash
claude auth login
mise run setup
```

`mise run setup` installs Lefthook through the existing `lefthook install`
workflow. It does not install/authenticate an AI agent or change its
permission scope. The documentation hook needs a recent Claude Code with
`--safe-mode`, `--restricted`, and structured JSON output. Outdated or
missing tools and authentication errors stop feature commits visibly.

```bash
# Reproduce the 13 screenshots without a live DOMjudge or profile database.
mise run user-guide-capture
mise run user-guide-capture --browser "/absolute/path/to/chrome"

# Local, deterministic checks; no agent, network, or usage charge.
mise run user-guide-check

# Intentional production from the current tracked files, including local edits.
mise run user-guide-update --timeout 180 --max-budget-usd 3

# Reproduce exactly the staged-source review used by pre-commit.
mise exec -- python3 scripts/update_user_guide.py hook --timeout 180 --max-budget-usd 3
```

Capture builds only `cmd/e2e-server` with `-mod=readonly` into a unique
ignored `tmp/` directory, refuses an occupied loopback port 9901, and owns
the server process and disposable browser profile it creates. It captures
only the 13 fixed loopback routes at 1280×1000 with a 3-second virtual-time
budget, then cleans up on completion, failure, or interruption. It never
uses a live VM, the native application's profile database, or a signed-in
browser profile. Screenshots show the web UI **HTML preview with seeded
example data and simulated operations**; they do not prove native-dialog
behavior or successful remote operations. Rendering can vary with the
installed browser version, operating system, and fonts.

The `check` command checks both manual/reference pairs, fenced code and
command consistency, all 13 PNG dimensions, and local Markdown links,
images, and heading/source-line fragments. It does not fetch external
URLs, assess translation quality, or prove the correctness of screenshots
or runtime behavior. Run `mise run privacy-check` before publishing text or
images. Tooling tests use Python's standard-library `unittest` and
disposable Git repositories; `mise run api-fixture-check` includes them.

The hook detects staged additions, modifications, renames, and deletions
under `cmd/`, `internal/`, `frontend/`, and `config/`, plus module/build
configuration that affects users. Go test-only and documentation-only
changes skip the AI call. Formatting completes before lint and review,
so the reviewer sees the post-format index.

The reviewer gets an isolated, allowlisted **staged index snapshot and
staged diff**, never unstaged source, ignored/untracked files, Git
configuration/hooks, agent settings, operator credentials, or `tmp/`.
Symlink source/targets are rejected. The intentional `update` command
instead reads the current contents of tracked files; it does not include
untracked source. Each snapshot is limited to 32 MiB and each file to
8 MiB. No source is truncated to fit these limits.
Tracked `.gitignore`, local `.git/info/exclude`, and configured global
ignore rules all exclude provider-visible files and diff paths. Local Git
configuration is consulted only to resolve ignores; it is never copied.

Before any provider call, TruffleHog scans that exact snapshot with
credential verification disabled and includes unverified findings; the
project's PII checker also scans included sanitized API/web fixtures.
These gates reduce disclosure risk but do not guarantee that source or
images contain no private information. Inspect staged content first.
No credential validation request is sent by this snapshot scan.

Claude Code runs with read-only `Read,Glob,Grep` tools confined to the
snapshot, `dontAsk` permission mode, no permission-prompt handler, and
customizations/MCP servers disabled. It has no shell, network-fetch, or
edit tools. The model's own API request still sends selected source and
documentation to the account's configured provider and can incur usage
charges. Each review has a default 180-second provider timeout and
US$3 budget cap, configurable with the options above. Privacy scans and
build/capture have separate bounded deadlines. There is no review cache;
retrying can incur another charge. Provider quotas and model availability
still apply, and administrator-managed policy can further restrict access.

The process validates the complete response before applying anything.
Only `README.md`, both user guides, and both CLI references are writable;
application source never is. Proposed documents are checked and scanned
again. A hook proposal that conflicts with pre-existing local
documentation edits is refused without overwriting them. Both workflows
also refuse concurrent document changes detected during review.

When the hook applies updates, it exits nonzero and prints instructions:
inspect the diff, stage the documents yourself, then retry the commit.
The documentation updater never stages files or commits, and the agent
never runs Git, so it cannot recursively trigger the hook. Existing Go
formatting retains its prior auto-staging behavior. Re-running the hook
is required after review; an empty update list passes. Provider failure,
timeout, invalid output, denied permissions, privacy findings, or a
missing agent/scanner never count as a successful feature review.

Fix the reported issue and rerun the same command. Preserve local edits
by resolving/staging them yourself before retrying conflicting proposals;
do not bypass hooks. AI review is a drafting aid and can still miss
features or make incorrect recommendations, so inspect every change.

## Releases

Releases publish only through GitHub Actions
([`release.yml`](.github/workflows/release.yml)) when an annotated tag
matching `vMAJOR.MINOR.PATCH` is pushed. Nobody uploads release files by
hand. The flow:

1. Add a curated `## [X.Y.Z] - YYYY-MM-DD` section to `CHANGELOG.md`.
   `mise run changelog-unreleased` previews the Conventional Commits since
   the last tag. The section becomes the release notes; preview them with
   `mise run release-notes vX.Y.Z`.
2. Commit it as `chore(release): prepare for vX.Y.Z` (filtered out of the
   next generated changelog by `cliff.toml`), push `main`, and wait for CI.
3. On an up-to-date, clean `main`: `mise run release-tag vX.Y.Z`. This
   validates the notes and creates the annotated tag locally.
4. `git push origin vX.Y.Z`. The workflow then:
   - rejects the tag unless it is annotated, points at a commit on `main`,
     and has a non-empty `CHANGELOG.md` section;
   - reruns the CI gates and builds six CLI archives with read-only tokens,
     packaging the one-line installer scripts;
   - in the `release` environment, verifies every SHA-256 checksum, records
     build-provenance attestations, and publishes the GitHub release.
5. Run [`release-smoke.yml`](.github/workflows/release-smoke.yml) (Actions,
   **Run workflow**, tag `vX.Y.Z`). On all six native platforms it downloads
   the published assets, verifies checksums and attestations, tests the
   one-line installer scripts (`install.sh` / `install.ps1`) with PATH
   verification, and verifies `domjudge-tool web` startup.
Tags with other shapes (`v1.7`, `v1.7.0-rc.1`) never start the workflow.
A failed run publishes nothing; fix the cause, delete the tag locally and
remotely, and tag again. CLI archives are verified with published checksums and Sigstore attestations.

## Dependency security reports

Dependabot alerts are enabled for the repository. On the 1st of each month,
[`dependabot-report.yml`](.github/workflows/dependabot-report.yml) opens one
issue, `Dependabot report YYYY-MM`, listing open alerts. It does not open
pull requests or change dependencies; maintainers decide the fixes.
`GH_REPO=ntub/domjudge-tool mise run dependabot-report --dry-run` previews the
issue body locally.

## Reporting issues

Open a GitHub issue. Include:

- What you tried (the exact command).
- What you expected.
- What happened (full output, no truncation).
- Versions: `go version`, `domjudge-tool` version (when there is one).

## Git hook bypass

Don't pass `--no-verify`. If a hook fails, fix the underlying issue. CI
runs the same checks and will reject the PR anyway.
