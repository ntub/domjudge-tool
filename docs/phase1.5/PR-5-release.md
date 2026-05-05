# PR-5 — Phase 1.5 release：docs / changelog / `v1.1.0`

## Goal

把 PR-1..PR-4 的成果包成可釋出的 `v1.1.0`：

- `CHANGELOG.md` 加 release note。
- `README.md` 更新「How it works」段，註明新版 internal architecture。
- `docs/ROADMAP.md` 更新 Phase 2 後續描述（v8 工作量縮小）。
- 確認 `goreleaser` snapshot 仍跨平台跑得起來。
- 本地 tag `v1.1.0`（不 push tag — 對齊 Phase 1 的本地 release 流程）。

## 為什麼獨立成 PR

跟 Phase 1 的 PR-8 一致，避免把 docs / release 跟功能 PR 混在一起。
`v1.1.0` 是本次唯一面向使用者的 deliverable，必須有清晰可追溯的單一
commit / merge 點。

## Files created / modified

### 修改

| Path | 改動 |
|------|------|
| `CHANGELOG.md` | 加 `## [1.1.0] - YYYY-MM-DD` section（見下） |
| `README.md` | 在 "How it works" / "Architecture" 段註明 hybrid 架構：API-first，scraper 仍負責 delete / problem zip / role-aware password |
| `docs/ROADMAP.md` | Phase 2 §"Web-form differences from v7" 補上 PR-1.5 註解；Phase 2 `web.Gateway` 條目重寫為 5 方法；Phase 1 §"Out of scope" 內「HTTP retry / backoff middleware → Phase 1.5 if real users hit transient failures」改寫為「Phase 1.5 已重新分配給 API 遷移；retry/backoff 視 ops 需要延後至 Phase 1.6 或 on-demand 處理」（避免兩個版本的 Phase 1.5 定義並存） |
| `docs/ARCHITECTURE.md` | §"Layers" 與 §"Package responsibilities" 加 `client/api/v4 ImportsResource`、`OrganizationsResource` 等；`web.Gateway` 條目註明 5 方法仍在 active、6 方法 deprecated |
| `docs/PHASE1.md` | 文末加一段「Phase 1.5 已完成於 v1.1.0；本檔保留作為 Phase 1 史料」 |
| `docs/CONVENTIONS.md` | 視需要：補 multipart 寫法的本 repo 慣例、TSV LF-only 註明 |
| `docs/TESTING.md` | 補「contract test」段，連到 `phase1.5/CONTRACT.md` |
| `.goreleaser.yaml` | 確認 version 從 `git describe` 抽，無寫死；不需改 |

### 新增

| Path | Role |
|------|------|
| `docs/adr/0006-api-first-with-scraper-fallback.md` | **PR-1 已建（Status: Proposed）**，PR-5 把 status 改成 `Accepted` |

## CHANGELOG 範本

```markdown
## [1.1.0] - 2026-XX-XX

Phase 1.5 — switch internal operations from HTML scraping to the
DOMjudge native REST API where supported. CLI flags and output formats
unchanged; this is an internal refactor.

### Internal

- New `internal/client/api/v4.ImportsResource` wrapping the bulk import
  endpoints (`/api/v4/users/{groups,organizations,teams,accounts}`,
  `/api/v4/contests`, `/api/v4/contests/{cid}/problems`).
- New `OrganizationsResource`, `GroupsResource`, `ContestsResource`,
  `ScoreboardResource` wrapping the corresponding read endpoints.
- New `internal/importer.Router` chooses between API and scraper
  per operation; emits structured `slog` decisions
  (`router decision op=… path=… reason=…`).
- `download-scoreboard` now drives the JSON scoreboard endpoint and
  reconstructs the CSV client-side (byte-identical to v1.0.0-rc1
  output for the same `--cid`).
- `import-users-teams` create-flow drives `Imports.Teams` /
  `Imports.Accounts` / `Imports.Organizations`; per-row email +
  role-aware password remain on the v7 scraper for parity.
- `set-password` defaults to API; falls back to scraper when
  `--user-roles` is supplied.
- `web.Gateway` retains 11 methods; 6 are now marked `// Deprecated:`
  but still implemented for fallback.

### Behaviour changes (visible)

- `import-users-teams` no longer silently picks a contest when
  `domserver.json.contest_id` is empty. With exactly one active
  contest the previous behaviour is preserved (auto-select). With zero
  or multiple active contests the command now errors with
  `cfg.contest_id is empty and N active contests exist (...); set contest_id in domserver.json`.
  Rationale: the v7 web UI accepted the empty `domjudge_cid` cookie
  and let the server choose; the API has no such fallback. This is the
  only user-visible change in v1.1.0.

### Configuration

- New optional `domserver.json` field `prefer_scraper` (bool). When
  `true`, all hybrid operations route through the v7 scraper, ignoring
  the API path. Existing files without this field load unchanged.

### Docs

- New `docs/phase1.5/` directory: SPEC, PLAN, CONTRACT, per-PR specs.
- ADR 0006: API-first with scraper fallback.

### Locked decisions preserved

- Python `services/web/v7.py:161` inverted-filter QUIRK on delete
  flow remains locked (we did not migrate delete; QUIRK still relevant).
- `domserver.json` schema is backward-compatible.
- `import-users-teams.csv` column order unchanged.
```

## 上線前 dry-run checklist（作者於 PR review 寫上）

- [ ] 對乾淨 demo station 跑：
  - `domjudge-tool gen-config ...` → 與 1.0.0 比較 `domserver.json`
    byte-identical（除 `prefer_scraper` 欄位是 omitempty 故不出現）。
  - `domjudge-tool import-users-teams-example` → byte-identical。
  - `domjudge-tool import-users-teams ./testdata/importer/input_5_rows.csv`
    → 產出 `import-users-teams-out.csv` 與 v1.0.0 在同 input 下 byte-
    identical。
  - `domjudge-tool download-scoreboard --cid 2` → 產出 byte-identical。
  - `domjudge-tool rm-teams-and-users --include alice,bob` → 行為與
    v1.0.0 完全相同（仍 scraper、QUIRK 仍生效）。
  - `domjudge-tool download-problems` → ZIP 仍下載成功（仍 scraper）。
  - `domjudge-tool download-submissions ...` → 仍 API。
- [ ] 在 `domserver.json` 加 `"prefer_scraper": true` 後，重跑 hybrid
      指令，stderr 確認看到 `path=scraper reason=cfg.prefer_scraper=true`。
- [ ] `goreleaser release --snapshot --clean` 產出 darwin/linux × amd64/arm64
      四個 binary。

## Definition of done

- [ ] CHANGELOG.md / README.md / ROADMAP.md / ARCHITECTURE.md /
      PHASE1.md / CONVENTIONS.md / TESTING.md 已更新。
- [ ] ADR 0006 status: Accepted。
- [ ] `goreleaser` snapshot 綠。
- [ ] `git tag v1.1.0`（本地）。
- [ ] PR commit message：`chore(release): goreleaser + adrs + readme polish (phase 1.5)`
      (對齊 Phase 1 PR-8 的格式)。

## Cross-references

- Phase 1 的 PR-8（`docs/phase1/PR-8-release.md`）— 同一份模板
- ROADMAP §Phase 2 — 因 Phase 1.5 而要更新的內容
- ADR 0006 — 由 Proposed 升 Accepted
