# PR-8 — Goreleaser + ADRs + README polish

> Layered position: **Phase 1 close-out**. No new packages. One tiny
> Layer-4 touch (`internal/cli/general` version var), plus release
> infrastructure, architecture decision records, and README polish.
> Locks the final three items in `PHASE1.md §"Definition of Done"`.

## Goal

After PR-8 the Phase 1 Definition of Done is fully satisfied:

1. **DoD #8**: ADRs 0002–0005 exist, capturing the four non-obvious
   architectural decisions made across Phase 1.
2. **DoD #9**: `goreleaser release --snapshot --clean` produces dist
   artifacts for darwin/linux × amd64/arm64.
3. **DoD #10**: Local tag `v1.0.0-rc1` exists (applied post-merge).

A secondary goal is that `domjudge-tool version` shows the git tag
(e.g. `v1.0.0-rc1`) on release and snapshot binaries, not `(devel)`.

## Files created

| Path | Change |
|------|--------|
| `docs/phase1/PR-8-release.md` | This document. |
| `.goreleaser.yaml` | New. goreleaser v2 config: 4 targets, archives, checksums, ldflag injection. |
| `docs/adr/0002-tdd-collaboration-model.md` | New. TDD + Codex-implements / Claude-reviews model. |
| `docs/adr/0003-cobra-version-pin.md` | New. Cobra pinned at v1.8.x for Go 1.23 compatibility. |
| `docs/adr/0004-goquery-version-pin.md` | New. goquery pinned at v1.10.3; independent unpin trigger. |
| `docs/adr/0005-python-quirk-preservation.md` | New. Inverted filter in `web/v7` locked bit-for-bit. |

## Files modified

| Path | Change |
|------|--------|
| `internal/cli/general/version.go` | Add `var Version string`; `versionLine()` prefers it over `debug.ReadBuildInfo` fallback. |
| `internal/cli/general/version_internal_test.go` | New (same-package). Two tests: ldflag wins, empty ldflag falls back. |
| `README.md` | Badge → "phase 1 complete"; Install → `go install`; Quick start → 4-line example. |
| `justfile` | Add `goreleaser-check` and `snapshot` tasks. |
| `.github/workflows/ci.yml` | Add `goreleaser-snapshot` job (push to `main` only). |

## Tests written first (RED)

### `internal/cli/general` — version ldflag

| Test | What it asserts | Cases |
|------|-----------------|-------|
| `TestVersionLine_LdflagInjectionWins` | When `general.Version = "v1.0.0-rc1"`, `versionLine()` contains `"v1.0.0-rc1"`. | one case |
| `TestVersionLine_FallsBackWhenLdflagEmpty` | When `general.Version = ""`, output still has prefix `"domjudge-tool "`. | one case |

These live in `version_internal_test.go` (`package general`, same-package
so they can read/write the unexported `Version` var). They **compile-fail**
before the `var Version string` is added — that's the valid RED state.

### Goreleaser config — validation step (RED)

| Step | What it asserts | Cases |
|------|-----------------|-------|
| `just goreleaser-check` exits non-zero | `.goreleaser.yaml` does not exist yet. | one case |
| CI `goreleaser-snapshot` job | Triggers on push to `main`; fails if config missing. | one case |

The `goreleaser-check` and `snapshot` tasks are committed to `justfile` and
`.github/workflows/ci.yml` **before** `.goreleaser.yaml` is created, making
the CI goreleaser-snapshot job the authoritative RED.

## Edge cases to cover

- [ ] `Version = ""` (plain `go build`, `go test`) → `versionLine` still
  returns a well-formed line beginning with `"domjudge-tool "`.
- [ ] `Version = "v1.0.0-rc1"` (goreleaser-injected) → `versionLine`
  includes the tag string.
- [ ] Existing `version_test.go` (external `_test` package) must remain
  green — its assertions (prefix `"domjudge-tool "` + trailing `\n`) are
  still satisfied after the change.
- [ ] `goreleaser check --config .goreleaser.yaml` exits 0 after the config
  lands.
- [ ] `goreleaser release --snapshot --clean` produces exactly 4 archives
  (`linux_amd64`, `linux_arm64`, `darwin_amd64`, `darwin_arm64`) plus
  `checksums.txt`.
- [ ] Each archive contains `LICENSE` and `README.md` alongside the binary.
- [ ] Snapshot binary `./domjudge-tool version` does NOT print `(devel)` —
  the goreleaser snapshot version template is injected via ldflag.

## Definition of done

1. `just check` clean — fmt, vet, lint, test all pass.
2. `go test ./internal/cli/general/...` — all 5 tests green, including
   the 2 new same-package tests.
3. `goreleaser check --config .goreleaser.yaml` exits 0.
4. `goreleaser release --snapshot --clean` exits 0; `dist/` contains the
   4 archives + `checksums.txt`.
5. Snapshot binary `version` output includes the snapshot version string,
   not `(devel)`.
6. `docs/adr/` contains 5 files: `0001-go-module-path.md` (existing) +
   the 4 new ones.
7. `README.md` badge shows "phase 1 complete"; Install and Quick start
   sections are populated.
8. `cmd/domjudge-tool/main.go` is unchanged — PR-8 does not touch it.
9. Conventional Commit on merge:
   **`chore(release): goreleaser + adrs + readme polish`**.

## Cross-references

- **PHASE1.md §"Definition of Done"** — items 8, 9, 10.
- **PHASE1.md §"PR plan"** — PR-8 row: `chore(release): goreleaser + adrs + readme polish`.
- **`docs/adr/template.md`** — ADR format reference.
- **`internal/cli/general/version.go`** — `versionLine()` function being extended.
- **`docs/phase1/README.md §CLI UX/DX guardrails §6`** — version must not
  attach `LoadConfigPreRunE`; `TestVersion_HasNoPreRunE` already locks this.
- **goreleaser docs**: v2 config schema. `goreleaser/goreleaser-action@v6`
  GitHub Action for CI.

## Commit cadence (TDD discipline)

1. `docs(phase1): draft PR-8 release spec` — this file.
2. `test(cli/general): red — ldflag-injectable Version`
3. `feat(cli/general): green — ldflag-injectable Version`
4. `test(goreleaser): add check + snapshot tasks to justfile and CI`
5. `chore(goreleaser): add .goreleaser.yaml`
6. `docs(adr): 0002 tdd-collaboration-model`
7. `docs(adr): 0003 cobra-version-pin`
8. `docs(adr): 0004 goquery-version-pin`
9. `docs(adr): 0005 python-quirk-preservation`
10. `docs(readme): update install, quick-start, and roadmap status`

Squash on merge: **`chore(release): goreleaser + adrs + readme polish`**
