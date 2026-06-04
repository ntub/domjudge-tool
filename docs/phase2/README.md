# Phase 2 — DOMjudge 8.3.1 支援

> 將 Phase 1.5（v1.4.0）已完成的 API-first CLI 擴充成同一個 binary 同時
> 支援 DOMjudge 7.3.x 與 **8.3.1**。Phase 2 不新增 CLI 功能；它的目標是
> 讓既有指令在 `domserver.json` 的既有 `version` 欄位設為 `"8.3.1"` 時，
> 走正確的 API decoder 與 `web/v8` scraper。

## 為什麼有 Phase 2

| | Phase 1.5（已完成） | **Phase 2（本次）** | Phase 3（規劃中） |
|---|---|---|---|
| 目標 | DOMjudge 7.3.x API-first + scraper fallback | **支援 DOMjudge 8.3.1，並保留 7.3.x** | 桌面 app |
| 對外介面 | CLI 子指令、flag、`domserver.json`、輸出檔名不變 | **不變** | 桌面 UI 另加 |
| 版本選擇 | 預設 `config.Version == "7.3.2"` | **config-driven：`config.Version == "8.3.1"` 時選 v8** | 沿用 CLI package |
| API 風險 | v7 fixture 已鎖 | **v8 response shape 有 hard break，需雙版本 fixture** | 不重做 HTTP client |
| scraper 範圍 | `web/v7` full `Gateway`，其中 5 active | **`web/v8` active 5 methods，其他 fallback 用 unsupported stubs 補齊完整 `Gateway`** | 不新增 scraper |

Phase 1.5 已經把大部分操作搬到 `/api/v4`，因此 Phase 2 不是重寫整個
CLI。真正需要做的是：

1. 讓 `internal/client/api/v4` 對 DOMjudge 7.3.x 與 8.3.1 的 JSON drift 都
   能 decode。
2. 新增 `internal/client/web/v8`，active surface 只處理 API 仍不能做的 5 個
   web 操作；其餘 fallback methods 以 unsupported sentinel stubs 補齊完整
   `web.Gateway`。
3. 透過既有 `config.Version` 選擇 v7 或 v8 scraper；不做自動偵測。

## 文件總覽

| 文件 | 用途 |
|------|------|
| [`SPEC.md`](SPEC.md) | 完整需求規格：每個 CLI 指令的 v8 transport、API drift、import body、live 驗證限制 |
| [`PLAN.md`](PLAN.md) | P2-0..P2-5 PR 切分、依賴圖、TDD 流程、fixtures-first hard gate、回退策略 |
| [`CONTRACT.md`](CONTRACT.md) | v8 API JSON 與 jury HTML fixture 的抓取、清洗、契約測試與 live write protocol |
| [`../adr/0008-domjudge-8.3.1-support-and-version-selection.md`](../adr/0008-domjudge-8.3.1-support-and-version-selection.md) | 版本選擇與 `web.Gateway` factory 的 ADR |
| [`../adr/0009-api-v4-cross-version-response-compatibility.md`](../adr/0009-api-v4-cross-version-response-compatibility.md) | API v4 decoder 跨 v7/v8 相容策略的 ADR |

## 關鍵的不變式（Phase 2 不能動）

依 `CLAUDE.md`、`docs/ROADMAP.md`、Phase 1.5 文件與既有測試：

- **`domserver.json` 欄位名 / 型別 / 預設值不變**。Phase 2 只使用既有
  `version` 欄位，把值設成 `"8.3.1"`；不新增 schema 欄位。
- **CLI 子指令名、flag 名、輸出檔名不變**。
- **`import-users-teams.csv` 既有欄位順序不變**：
  `username,name,email,affiliation,password`。可追加 optional 欄位，但舊
  CSV 必須維持可用。
- **`v7.py:161` inverted-filter quirk 保留 bit-for-bit**。`DeleteUsers` /
  `DeleteTeams` 的 v7 scraper 行為不能順手修；任何改變都需要 regression
  test 與新的 ADR。
- **測試只用 stdlib `testing` + `httptest`**；不引入 testify / ginkgo。
- **測試永遠不打真實 DOMjudge / SMTP**。所有 HTTP 行為用
  `httptest.Server` 與 sanitized fixture。
- **Go module directive 保持 Go 1.23**。`goquery` 維持 v1.10.3 pin，
  `cobra` 維持既有 pin，不因 Phase 2 升級。
- **library packages 不印 stdout/stderr**；只有 `cmd/` 與 `internal/cli/`
  可以印 user-facing output。Library diagnostic 走 structured `slog` 或回傳
  data/error。

## Phase 2 — Definition of Done

1. `testdata/api/v4/v8/` 與 `testdata/web/v8/` 先完成抓取、清洗、PII gate，
   且所有 v8 Go code 都以這些 fixture 為輸入。
2. `internal/client/api/v4` 可 decode v7.3.x 與 8.3.1 fixture：
   - `time_limit` 可接 v7 int 與 v8 float（live value `5.0`）。
   - `/api/v4/version` 的 `api_version` 可接 v7 string 與 v8 int。
   - additive / under-described 欄位不造成 decode fail。
3. `internal/client/web/v8` actively implements 5 個 `web.Gateway` 方法：
   `Login`、`GetProblems`、`Download`、`DeleteUsers`、`DeleteTeams`；其他
   fallback methods 回傳明確 unsupported sentinel，讓 v8 client 滿足完整
   `web.Gateway`。
4. `config.Version == "7.3.2"` 或其他 7.3.x 值時仍選 `web/v7`；
   `config.Version == "8.3.1"` 時選 `web/v8`。沒有 runtime auto-detection。
5. `import-users-teams` 的 v8 bulk import body 符合 live findings：
   groups / organizations / teams / accounts shape 正確，且 v8
   organizations import 不送 `url` / `twitter_hashtag`。
6. `rm-teams-and-users` 在 v8 的 delete correctness 由 `httptest` 鎖住；
   live writable contest 僅可在 `CONTRACT.md` 的 sign-off protocol 下執行。
7. `just test`、`just vet`、`just lint`、`just privacy-check` 全綠；
   release 前跑 `just check`。
8. ADR 0008 與 0009 被 review 後可從 `Proposed` 推進到 `Accepted`。

## 出 Phase 2 範圍的事

- 不新增、改名或刪除 CLI 指令與 flag。
- 不改 `domserver.json` schema；不新增 auto-detection flag。
- 不把 `rm-teams-and-users` 搬到 API。DOMjudge 8.3.1 live findings 仍沒有
  users / teams API DELETE。
- 不把 `download-problems` 搬到 API。DOMjudge 8.3.1 只有
  `/contests/{cid}/samples.zip`，沒有完整 problem-package ZIP export。
- 不修 v7 delete inverted-filter quirk。
- 不新增 Go dependency；尤其不要為 JSON drift 加第三方 decoder。
- 不在 repo 內保存 raw live capture、真實 host、credentials、cookies、CSRF、
  email、學生資料或 course identifiers。

## 給接手工程師的閱讀順序

1. 先讀 [`SPEC.md`](SPEC.md) §2 與 §6 — 弄清楚 v8 API drift 與 live write
   safety 為什麼是 Phase 2 最大風險。
2. 讀 [`CONTRACT.md`](CONTRACT.md) — P2-0 必須先完成 fixture capture，不能
   先寫 v8 parser。
3. 讀 [`../adr/0008-domjudge-8.3.1-support-and-version-selection.md`](../adr/0008-domjudge-8.3.1-support-and-version-selection.md)
   與 [`../adr/0009-api-v4-cross-version-response-compatibility.md`](../adr/0009-api-v4-cross-version-response-compatibility.md)
   — 知道版本選擇與 decoder 相容的決策。
4. 讀 [`PLAN.md`](PLAN.md) — 依 P2-0..P2-5 順序做；不要跳過
   fixtures-first hard gate。
5. 實作時對 Phase 1.5 routing 有疑問，回頭讀
   [`../phase1.5/SPEC.md`](../phase1.5/SPEC.md) 與 ADR 0006。
