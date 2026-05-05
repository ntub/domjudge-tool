# Phase 1.5 — DOMjudge 原生 API 遷移

> 將 Phase 1（v1.0.0-rc1）內部對 DOMjudge 的操作從 HTML scraping 漸進
> 切換到 DOMjudge 7 原生 REST API（`/api/v4`），目的是降低未來版本升
> 級、頁面改版時的維護成本，並讓行為從「靠 HTML 結構穩定」改為「靠
> 文件化契約穩定」。

## 為什麼有 Phase 1.5（不直接做 Phase 2）

| | Phase 1（已完成） | **Phase 1.5（本次）** | Phase 2（規劃中） |
|---|---|---|---|
| 目標 | Python CLI 移植到 Go，DOMjudge 7.3.x | **同一個 7.3.x，換成 API-first** | 支援 DOMjudge 8.3.x |
| 對外介面 | CLI 子指令、flag、`domserver.json`、輸出 CSV/JSON 格式 | **不變** | 不變 |
| 對內介面 | `web.Gateway` 11 個方法 + `api/v4` 7 個 sub-resource | **新增 `api/v4` `ImportsResource` 與 router；`web.Gateway` 縮編成「API 不能做的」的後援** | `web/v8` 與 `api/v4`（v8 含同樣端點）並存 |
| 風險 | 中 — Python 對齊壓力大 | **低** — CLI 行為已被現有測試鎖住，本次只換實作 | 中 — 需從零捕捉 v8 fixture |
| 主要產出 | `v1.0.0-rc1` | **`v1.1.0`** | `v1.2.0` |

Phase 1.5 縮短 Phase 2 的範圍：v8 之後 `web.Gateway` 只剩 *delete /
problem zip download* 這幾個端點需要爬，剩下 80% 走 API、跟 v7 共用同
一份 client 程式碼。

## 文件總覽

| 文件 | 用途 |
|------|------|
| [`SPEC.md`](SPEC.md) | 完整需求規格：每個 CLI 指令的 API 對應表、缺口分析、live-server 探測證據 |
| [`PLAN.md`](PLAN.md) | PR 切分、TDD 流程、合併時序、回退策略 |
| [`CONTRACT.md`](CONTRACT.md) | API fixture 抓取與契約測試方法（與 `testdata/web/v7/CAPTURE.md` 平行） |
| [`PR-0-fixtures.md`](PR-0-fixtures.md) | 抓取 + 清洗 `testdata/api/v4/` |
| [`PR-1-imports-scaffold.md`](PR-1-imports-scaffold.md) | `internal/client/api/v4/ImportsResource` 與 multipart 輔助 |
| [`PR-2-scoreboard.md`](PR-2-scoreboard.md) | `download-scoreboard` 改用 `/api/v4/contests/{cid}/scoreboard` |
| [`PR-3-importer-create.md`](PR-3-importer-create.md) | `import-users-teams` create 路徑改走 API |
| [`PR-4-router.md`](PR-4-router.md) | API-first router 與 scraper fallback 的調度層 |
| [`PR-5-release.md`](PR-5-release.md) | docs / changelog / `v1.1.0` 釋出 |
| [`../adr/0006-api-first-with-scraper-fallback.md`](../adr/0006-api-first-with-scraper-fallback.md) | 此次架構決策的 ADR |

## 關鍵的不變式（這次不能動）

依 `docs/ROADMAP.md` 與 `CLAUDE.md`：

- **`domserver.json`** 欄位名 / 型別 / 預設值不變。新增欄位需提供
  migration 路徑。
- **CLI 子指令名與 flag 名**逐字保留 — Python parity 已上線。
- **`import-users-teams.csv` 欄位順序**保留：
  `username,name,email,affiliation,password`。
- **三個 Python bug regression test** 與 **`v7.py:161` QUIRK 鎖**保留 —
  本次不能「順便修掉」；若某個指令真的完全離開 scraper、QUIRK 因此消
  失，需在 PR 內增寫 ADR 明文說明（Phase 1 ROADMAP §"Quirks to
  preserve" 的硬性條款）。
- **不引入 testify / ginkgo**；`testing` + `httptest` 為唯一允許。
- **不從 library 印東西**；只能在 `cmd/` 與 `internal/cli/` 印。
- **`go 1.23`** module 宣告不動，`cobra` 與 `goquery` 維持 1.23-相容
  pin（cobra v1.8.x、goquery v1.10.3）。

## Phase 1.5 — Definition of Done

從 repo 根目錄執行：

1. `git log --oneline main` 看到 PR-0..PR-5 的 6 個 merge commits。
2. CI 在 `main` 綠燈。
3. `go test ./... -cover` 各層覆蓋率不退步：parser ≥ 80%、client ≥ 70%。
4. 對一台乾淨的 DOMjudge 7.3.x：
   - `import-users-teams` 完整流程結果與 Phase 1 byte-identical（用同一
     份 CSV 餵給 v1.0.0-rc1 與 v1.1.0，產出的
     `import-users-teams-out.csv` 必須一致）。
   - `download-scoreboard --cid <id>` 產出與 Phase 1 byte-identical。
   - `rm-teams-and-users` 行為不變（仍走 scraper，QUIRK 仍生效）。
   - `download-problems` 行為不變（ZIP 仍走 scraper）。
5. ADR 0006 已 accepted。
6. `goreleaser release --snapshot --clean` 產出 darwin/linux × amd64/arm64。
7. 本地 tag `v1.1.0`。

## 出 Phase 1.5 範圍的事

- DOMjudge 8.3.x 支援 → Phase 2。
- 把 `rm-teams-and-users` 移上 API → 不可能，v7 API 沒有 DELETE 動詞
  （見 SPEC §"驗證紀錄"）；Phase 2 視 v8 端點再評估。
- 把 `download-problems` ZIP 下載移上 API → 不可能，v7 沒有
  problem-zip-export 端點；同上。
- HTTP retry / backoff middleware → 仍是後續議題，不在這次。
- Wails / PocketBase / 桌面 App → Phase 3。

## 給接手工程師的閱讀順序

1. 先讀 [`SPEC.md`](SPEC.md) §"驗證紀錄" — 弄清楚為什麼是 hybrid 而不
   是「全 API」。
2. 讀 [`../adr/0006-api-first-with-scraper-fallback.md`](../adr/0006-api-first-with-scraper-fallback.md)
   — 知道架構決策與 alternatives。
3. 讀 [`PLAN.md`](PLAN.md) — 知道 PR 順序與每個 PR 的入口檔。
4. 從 [`PR-0-fixtures.md`](PR-0-fixtures.md) 開始實作；每個 PR 都有自
   己的「Files created/modified」「Tests written first (RED)」「DoD」
   區段。
5. 過程中對特定 endpoint 行為有疑問時，回去讀 [`CONTRACT.md`](CONTRACT.md)
   §"在 live test station 上驗證"。
