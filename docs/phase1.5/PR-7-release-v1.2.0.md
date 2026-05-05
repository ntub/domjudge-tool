# PR-7 — Phase 1.5 release：docs / changelog / `v1.2.0`

## Goal

把 PR-6 的成果包成可釋出的 `v1.2.0`：

- `CHANGELOG.md` 加 release note。
- `README.md` 更新「Commands」段，列入三個新指令。
- `docs/ROADMAP.md` 更新 Phase 1.5 完成狀態描述。
- `docs/ARCHITECTURE.md` 視 layer 是否有變更補上。
- 確認 `goreleaser` snapshot 仍跨平台跑得起來。
- 本地 tag `v1.2.0`（不 push tag — 對齊 Phase 1 的本地 release 流程）。

## 為什麼獨立成 PR

跟 Phase 1 的 PR-8 一致，避免把 docs / release 跟功能 PR 混在一起。
`v1.2.0` 是本次唯一面向使用者的 deliverable，必須有清晰可追溯的單一
commit / merge 點。

## Files created / modified

### 修改

| Path | 改動 |
|------|------|
| `CHANGELOG.md` | 加 `## [1.2.0] - YYYY-MM-DD` section（見下） |
| `README.md` | 在「Commands」段加入 `email-template-example`、`email-template-validate`、`render-user-accounts` 三個新指令的說明 |
| `docs/ROADMAP.md` | Phase 1.5 段補上 v1.2.0 完成標記；email template tooling 功能列入已交付清單 |
| `docs/ARCHITECTURE.md` | 視 PR-6 是否新增 package 或 layer 調整補上對應說明；`internal/email` 補 `BuildEML` 與 `WriteTemplateScaffold` 條目 |
| `docs/CONVENTIONS.md` | 視需要：補 `nowFn`/`randRead` seam 的 repo 慣例（package-level var for test injection） |

## CHANGELOG 範本

```markdown
## [1.2.0] - YYYY-MM-DD

Phase 1.5 — email template tooling: offline scaffold, validate, and
per-recipient render-to-disk for the credential mailer flow.

### Added

- `email-template-example [-o <dir>] [--force]` — scaffold a template
  directory with subject + text/HTML body containing all six allowed
  placeholders. The scaffolded files pass `email-template-validate`
  immediately (round-trip guarantee).
- `email-template-validate <dir>` — verify a template directory loads
  correctly and uses only the fixed placeholder set (username, name,
  email, affiliation, password, server_host).
- `render-user-accounts <csv> <template-dir> [-d <output-dir>]
  [--from-email <addr>] [--force]` — offline render of per-recipient
  `.eml` files (RFC 5322), parallel to `send-user-accounts` but never
  touches SMTP. Requires a loaded `domserver.json` (for `$server_host`).

### Internal

- `internal/email.BuildEML` extracted; `SendUserAccounts` now delegates
  its message construction to it (zero behaviour change for the SMTP
  path).
- `internal/email` exposes test seams `nowFn` / `randRead` (package-
  level vars) for deterministic golden tests of RFC 5322 output.
- `internal/email.WriteTemplateScaffold` and sentinel
  `ErrTemplateDirExists` added.

### Behaviour changes (visible)

- None for existing commands.
```

## 上線前 dry-run checklist（作者於 PR review 填寫）

- [ ] 對乾淨 demo station 跑既有五條指令（v1.1.1 parity）：
  - `domjudge-tool gen-config ...` → `domserver.json` byte-identical
    （`prefer_scraper` 欄位 omitempty 故不出現）。
  - `domjudge-tool import-users-teams-example` → byte-identical。
  - `domjudge-tool import-users-teams ./testdata/importer/input_5_rows.csv`
    → `import-users-teams-out.csv` 與 v1.1.1 byte-identical。
  - `domjudge-tool download-scoreboard --cid 2` → byte-identical。
  - `domjudge-tool rm-teams-and-users --include alice,bob` → 行為相同
    （仍 scraper、QUIRK 仍生效）。
- [ ] 新增三條指令：
  - `domjudge-tool email-template-example -o /tmp/t` → byte-identical
    against `testdata/email/scaffold_golden/`。
  - `domjudge-tool email-template-validate /tmp/t` → exit 0，
    stdout: `template OK: /tmp/t`。
  - `domjudge-tool render-user-accounts
    ./testdata/importer/input_5_rows.csv /tmp/t -d /tmp/out` →
    5 個 `.eml`、`net/mail.ReadMessage` 全部讀得回。
- [ ] `goreleaser release --snapshot --clean` 產出
      darwin/linux × amd64/arm64 四個 binary。

## Definition of done

- [ ] `CHANGELOG.md` / `README.md` / `ROADMAP.md` / `ARCHITECTURE.md`
      （/ `CONVENTIONS.md` 視需要）已更新。
- [ ] `goreleaser` snapshot 綠。
- [ ] `git tag v1.2.0`（本地）。
- [ ] PR commit message：
      `chore(release): goreleaser + readme polish (phase 1.5 v1.2.0)`
      （對齊 Phase 1 PR-8 / Phase 1.5 PR-5 的格式）。

## Cross-references

- `docs/phase1.5/PR-5-release.md` — 直接沿用的模板（改版號與 changelog 內容）
- `docs/phase1.5/PR-6-email-template-tooling.md` — 本次 release 所含功能的需求源頭
- Phase 1 的 PR-8（`docs/phase1/PR-8-release.md`）— 同一份 release 慣例
- `CHANGELOG.md` — `## [1.2.0]` section 由本 PR 寫入
