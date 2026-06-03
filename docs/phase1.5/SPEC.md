# Phase 1.5 — 規格書（SPEC）

> 這份文件是 Phase 1.5 的「合約」。每一個會被改動的指令、每一個會新增
> 或修改的內部 package、每一條 API 端點都在下面有對照欄位。實作工程
> 師遇到「這應該走 API 還是走 scraper」「這個欄位是哪裡來的」「這個
> 行為是 Phase 1 鎖死的還是可以順手改」這類問題時，從這裡找答案；找
> 不到就回來補，不要憑感覺修。

## 1. 目標與非目標

### 目標

1. **CLI 介面、輸入輸出格式、`domserver.json` 全部不變**。對外是無感
   的內部重構（semver patch / minor，不是 major）。
2. 把目前走 HTML scraping 的指令，能搬到 `/api/v4` 的全部搬過去。
3. 為留在 scraper 的指令（DELETE 流程、problem ZIP 下載）保留原本程
   式碼但縮小其調用面 — 一個 ADR 註明哪些操作仍依賴 scraper、為什麼。
4. 讓 Phase 2（DOMjudge 8）只需要再寫一個 `web/v8` scraper（針對沒搬
   走的少數操作），以及覆蓋同樣 endpoint 的 v8 fixture。
5. 不退步：覆蓋率、測試方法、commit 規範、Python parity 全保留。

### 非目標

- 不擴增 CLI 功能（不加新指令、不加新 flag）。
- 不調整 `domserver.json`（含預設值）。
- 不重構 `internal/email`、`internal/exporter`、`internal/util/*`、
  `internal/scoreboard/parse.go`（HTML 解析器）的實作 — 這些不碰
  DOMjudge。
- 不切換 `cobra` 或 `goquery` 大版本。
- 不增加新依賴；multipart、tsv 用 stdlib 寫。

## 2. 驗證紀錄（live test station，2026-05-05）

下列實測都是針對 `https://test-station.example.org`（DOMjudge **7.3.4**、
API v4，環境 prod）以 admin Basic Auth 直接打的結果。**全部紀錄都應在
PR-0 重跑並重新 capture 成 fixture，不要把 cred / hostname 寫進 commit
裡**（規範同 `testdata/web/v7/CAPTURE.md` §"Capture protocol"）。

### 2.1 已存在的 GET 端點（54 條 path、共 ~60 個 operation）

從 `GET /api/doc.json` 讀回 OpenAPI 2.0 spec，重要端點摘要如下；完整清
單在 `tmp/openapi.json` 抓到後直接看 `paths` map。

```
GET    /api/v4/version
GET    /api/v4/info
GET    /api/v4/user
GET    /api/v4/users                                 (有 ?team_id=, ?ids[]=)
GET    /api/v4/users/{id}
GET    /api/v4/contests
GET    /api/v4/contests/{cid}
GET    /api/v4/contests/{cid}/groups
GET    /api/v4/contests/{cid}/organizations          (有 ?country=)
GET    /api/v4/contests/{cid}/teams                  (有 ?category=, ?affiliation=, ?public=)
GET    /api/v4/contests/{cid}/problems
GET    /api/v4/contests/{cid}/problems/{id}
GET    /api/v4/contests/{cid}/scoreboard             (有 ?allteams=, ?category=, ?public=)
GET    /api/v4/contests/{cid}/contest-yaml
GET    /api/v4/contests/{cid}/event-feed             (Server-Sent Events)
GET    /api/v4/contests/{cid}/submissions
GET    /api/v4/contests/{cid}/submissions/{id}
GET    /api/v4/contests/{cid}/submissions/{id}/files       (回傳 ZIP)
GET    /api/v4/contests/{cid}/submissions/{id}/source-code (回傳 JSON)
GET    /api/v4/contests/{cid}/judgements
GET    /api/v4/contests/{cid}/judgement-types
GET    /api/v4/contests/{cid}/languages
GET    /api/v4/contests/{cid}/awards
GET    /api/v4/contests/{cid}/balloons
GET    /api/v4/contests/{cid}/clarifications
GET    /api/v4/contests/{cid}/state
GET    /api/v4/contests/{cid}/status
GET    /api/v4/contests/{cid}/runs
GET    /api/v4/judgehosts
GET    /api/v4/executables/{id}
GET    /api/v4/config                                 (有 PUT)
GET    /api/v4/config/check
```

### 2.2 寫入端點（POST 為主）

```
POST   /api/v4/users/groups            multipart  (tsv | json)
POST   /api/v4/users/organizations     multipart  (json   — 必填)
POST   /api/v4/users/teams             multipart  (tsv | json)
POST   /api/v4/users/accounts          multipart  (tsv    — 必填)
POST   /api/v4/contests                multipart  (yaml   — 必填)
POST   /api/v4/contests/{cid}/problems multipart  (zip[]  + problem)
POST   /api/v4/contests/{cid}/submissions  multipart  (problem, language, code[], entry_point)
PUT    /api/v4/config                  body
PATCH  /api/v4/contests/{cid}          body  (只能改 start_time)
```

`POST /api/v4/contests` 後回傳新 contest id；`POST /problems` 後回傳新
problem id 列表；`POST /users/{groups,orgs,teams,accounts}` 都接收一份
完整清單做 upsert。

### 2.3 **不存在**的動詞（已實測）

下列動詞對下列 path 全部回傳 **HTTP 405 Method Not Allowed**：

| Path | DELETE | PUT | PATCH |
|------|--------|-----|-------|
| `/api/v4/users/{id}` | 405 | 405 | 405 |
| `/api/v4/contests/{cid}/teams/{id}` | 405 | 405 | 405 |
| `/api/v4/contests/{cid}/organizations/{id}` | 405 | 405 | 405 |
| `/api/v4/contests/{cid}/problems/{id}` | 405 | 405 | 405 |

> **這就是為什麼 Phase 1.5 是 hybrid 而不是「全 API」**。
> DOMjudge 7 沒有提供任何刪除單一資源的 REST 端點。`rm-teams-and-users`
> 與 `import-users-teams --delete-existing` 都必須留在 scraper。

### 2.4 problem ZIP 匯出端點 — 不存在（已實測）

| 路徑嘗試 | 結果 |
|----------|------|
| `GET /api/v4/contests/{cid}/problems/{id}` | 200 但 CT=`application/json`（只有 metadata） |
| `GET /api/v4/contests/{cid}/problems/{id}.zip` | 200 但 CT=`application/json`（同上 metadata） |
| `GET /api/v4/contests/{cid}/problems/{id}/zip` | 404 |
| `GET /api/v4/problems/{id}` | 404 |
| `GET /api/v4/problems/{id}.zip` | 404 |
| `GET /api/v4/contests/{cid}/problems/{id}/statement` | 404 |

`download-problems` 仍只能透過 scraper 的 `/jury/problems/{id}/export`
路徑下載 ZIP（與 Phase 1 行為相同）。

### 2.5 scoreboard 不協商 CSV

`GET /api/v4/contests/{cid}/scoreboard` 不論 `Accept` header 為何
（試過 `application/json`、`text/csv`、不帶），永遠回傳 JSON。
**CSV 必須在 client 端從 JSON 重組**。

### 2.6 idempotency — 「未驗證 / 風險高」（PR-0 必須補做）

下列實驗 *未在本次研究中執行*，因為 live station 上有 production contest
資料，存在被覆寫的風險。**PR-0 將 station 換成 throwaway DOMjudge 7
後再驗證**：

1. `POST /api/v4/users/accounts` 用同一個 username + 不同 password 重
   送：是 (a) update password / (b) create duplicate / (c) error?
2. `POST /api/v4/users/teams`（JSON）用同一個 id + 改過 display_name
   重送：是 update / duplicate / error?
3. `POST /api/v4/users/organizations`（JSON）對既有 shortname 重送的行
   為。

**這三項的結果決定 SPEC §3 的「import-users-teams 是否能 100% 移到
API」是 yes / no**。

預設假設（保守）：DOMjudge 7 的 import 端點是 **upsert by 主要欄位**
（username / id），重送會更新而不會重複。歷史上 `misc-tools/import-
contest.sh` 的設計就建立在這個前提上。但在 PR-0 沒驗到之前，**程式
碼不能依賴這個假設** — PR-3 的測試必須證實。

## 3. 每個 CLI 指令的對應決策

> **v1.1.1 amendment（2026-05-05）：** live 驗證後 `OpCreateTeamAndUser`、
> `OpUpdateTeam`、`OpSetPassword(per-row)` 都從 ⚠️ 退回 🔒 — DOMjudge
> 7.3.x `accounts.tsv` 透過 `Team::find($pk)` 用 username regex 解析
> team-id，CCS 規格沒有 `team_id` 欄、server 端 auto-increment 我們無法
> 控制。新建立的 user 會被誤連到既存的 small-id team（觀察值是
> admin / Example team）。Phase 2 / DOMjudge 8 重啟用前須先驗證。
> 詳見 ADR 0006「v1.1.1 amendment」段。

### 圖例

- ✅ 走 API（內部完全 API、不再開 session）
- ⚠️ Hybrid（API 為主、scraper 補 API 做不到的部分）
- 🔒 Scraper-only（API 做不到，無法移）
- — 無關（local 邏輯，不接 DOMjudge）

| Cobra 指令 | 子操作 | Phase 1（現在） | **Phase 1.5（目標）** | 主要約束 |
|------------|-------|----------------|----------------------|---------|
| `gen-config` | 寫檔 | — | — | local-only |
| `version` | API.Version | ✅ `/version` | ✅ 不動 | 已是 API |
| `import-users-teams-example` | 寫 CSV | — | — | local-only |
| `export-users` | 取 user 清單 + 寫 csv/json | ✅ `/users` | ✅ 不動 | 已是 API |
| `import-users-teams` | List users | ✅ `/users` | ✅ 不動 | |
| | GetAffiliation / CreateAffiliation | scraper | ✅ `/contests/{cid}/organizations` (GET) + `POST /users/organizations` (JSON upsert) | 需 contest scope；舊 `/jury/affiliations` 是 global，新是 contest-scoped — 行為對齊在 §4 |
| | CreateTeamAndUser | scraper | ⚠️ `POST /users/groups` (確保 group exist) → `POST /users/teams` (TSV upsert) → `POST /users/accounts` (TSV upsert) | **取決於 §2.6 idempotency 實驗結果**；若 fail 則 fallback scraper |
| | UpdateTeam | scraper | ⚠️ 同上（重送 = upsert） | 同上 |
| | SetUserPassword（per user）| scraper | ⚠️ 隨 `accounts.tsv` upsert 一起完成 | 同上 |
| | UpdateUserEmail | scraper | 🔒 **無法移**：`accounts.tsv` 沒有 email 欄；CCS 規格沒這個欄位 | scraper 仍負責 |
| | DeleteExisting (pre-delete) | scraper | 🔒 同 `rm-teams-and-users` | 沒 DELETE |
| `rm-teams-and-users` | List users | ✅ `/users` | ✅ 不動 | |
| | DeleteUsers / DeleteTeams | scraper | 🔒 **無法移** | 沒 DELETE；QUIRK 仍鎖定 |
| `set-password` | List users | ✅ `/users` | ✅ 不動 | |
| | SetUserPassword | scraper | ⚠️ 用 `POST /users/accounts` 單行 TSV upsert；fallback scraper | idempotency 待證 |
| `download-problems` | List problems | scraper (`/jury/problems`) | ✅ `/contests/{cid}/problems` | |
| | Download ZIP | scraper | 🔒 **無法移**：API 沒這個端點 | scraper 仍負責 |
| `download-submissions` | List + Files + SourceCode | ✅ 已是 API | ✅ 不動 | |
| `download-scoreboard` | 抓公開頁 + parse HTML | scraper (`/public?static=1`) | ✅ `/contests/{cid}/scoreboard` (JSON) + `/contests/{cid}/teams` + `/contests/{cid}/organizations` 重組 CSV | |
| `send-user-accounts` | 讀 csv + SMTP | — | — | local-only |

預期結果：
- `web.Gateway` 的 11 個方法縮為 **5 個**：`Login`、`GetProblems`、
  `Download`（ZIP）、`DeleteUsers`、`DeleteTeams`。其餘 6 個會保留實
  作 + 測試但被標 deprecated（PR-3 註解），Phase 2 v8 不需實作。
- `internal/client/api/v4/` 新增 `ImportsResource`，提供 6 個方法包裝
  multipart POST。

## 4. 對外觀察行為的對齊

### 4.1 Affiliation → Organization 的詞彙統一

DOMjudge 7 的 web UI 用 "affiliation" 這個字（路徑也是 `/jury/affiliations`），
而 API 用 "organization"。它們是同一個資料表的兩種名字。

**內部命名規則**：
- 已公開的 `model.Affiliation`、`web.Gateway.GetAffiliation*`、CSV 欄位
  名 `affiliation`、CLI flag `--affiliation-id`、`--affiliation-country`
  **保留** — Python parity / 已上線。
- 新增的 API 對應命名為 `apiv4.ImportsResource.Organizations(...)` 與
  `apiv4.OrganizationsResource.List(...)`。
- Importer 內 `resolveAffiliation()` 重新實作後仍保留同名；只是內部呼
  叫從 `gw.GetAffiliation()` 換成 `api.Organizations.FindByName()`。

### 4.2 Affiliation 是 contest-scoped 還是 global

`/jury/affiliations` 顯示所有 affiliation（global），而 API 是
`/contests/{cid}/organizations`（contest-scoped）。實測：

```
GET /api/v4/contests/2/organizations    → 3 筆（id=1,2,3）
GET /api/v4/contests/1/organizations    → ?  (PR-0 抓 fixture 時補)
```

> **PR-0 要驗的事 #4**：API 看到的 organization 集合是 contest-scoped
> 還是其實 global？如果是 contest-scoped，那 importer 必須先決定要寫進
> 哪個 contest（用 `cfg.ContestID`）。如果是 global、只是路由要透過某
> 個 contest，那只要任選一個 active contest 就好。

實作時優先採用 **「使用 `cfg.ContestID` 指定的 contest」** 的設計；如果
PR-0 證實是 global，再簡化成「任意 active contest」。

`cfg.ContestID == ""` 時的 fallback：見 PR-3 §"邊界"。Phase 1 v7 scraper
是「不設 cookie，server 自選」；API 沒有此語意，改為「**單一 active
contest 自動選用、多個則明確報錯**」。這是 Phase 1.5 唯一的對外可見
行為差異。

### 4.3 group / category 對齊

- `cfg.CategoryID`（int，已存在）= DOMjudge "team category id" =
  API 的 `group.id`。
- `POST /users/groups` 接受 `groups.tsv`，欄位 `id\tname`。
- importer create-flow 在送 teams 之前要確保對應 group 存在；不存在時
  POST 一行 TSV 帶上 cfg.CategoryID + 預設名 `Phase1.5 created`（Phase
  1 並不會自己建立 category，這是「順便建好」的容錯）。

### 4.4 password 處理

CCS `accounts.tsv` 規格：

```
accounts<TAB>1
team<TAB>{name}<TAB>{username}<TAB>{password}
```

注意：DOMjudge import 邏輯只有 `team` / `judge` / `admin` 三種 type，
**沒有 role 自訂**。Phase 1 的 `--user-roles` flag 透過 scraper 才能設
定多個 role；移到 API 後 `--user-roles` **失去 v7 API 路徑**。

行為決議：
- `--user-roles` 沒被指定（flag 沒有 changed）→ 走 API、role 採 server
  預設（`team`）。
- `--user-roles` 被指定 → router 檢測後 fallback 走 scraper。

> **v1.1.1 amendment（2026-05-05）：** 上述「走 API」的兩條 set-password
> 路徑在 DOMjudge 7 的 `accounts.tsv` 處理裡也吃同一個 username regex
> linkage bug — 即使密碼欄寫對，user→team 連結仍會落到一個既存的
> small-id team。`OpCreateTeamAndUser` 已退回 scraper（v1.1.1）。
> 純 password update（不重建 team）的 v7 API 路徑仍存在 server 端
> idempotency 風險（PR-0 demo 鎖測試尚未完成），延至 Phase 2 重新評估。
- 在 PLAN PR-4 的 router 決策表寫清楚這條規則。

### 4.5 scoreboard JSON → CSV 的欄位對齊

Phase 1 的 CSV 欄位（從 HTML 表頭推得）：

```
rank,team_id,team_name,affiliation,solved,total_time,<problemA>,<problemB>,...
```

API JSON shape（已實測）：

```json
{
  "event_id": "...",
  "time": "...",
  "state": {...},
  "rows": [
    {
      "rank": 1, "team_id": "2",
      "score": {"num_solved": 0, "total_time": 0},
      "problems": [
        {"label": "boolfind", "problem_id": "3",
         "num_judged": 0, "num_pending": 0,
         "solved": false, "first_to_solve": false}
      ]
    }
  ]
}
```

**欠缺**：team_name、affiliation、country。重組 CSV 時必須額外打
`/contests/{cid}/teams` 與 `/contests/{cid}/organizations` 取得
display_name → team_id 的對照表，再 join。`internal/scoreboard/build.go`
（新檔）負責這個 transform 並對 Phase 1 的 CSV 做 byte-identical 比對。

> **更新（ADR 0007，取代上述 byte-identical 目標）**
>
> 實測對照真實 Python 輸出後，scoreboard 匯出做了刻意的行為變更，詳見
> `docs/adr/0007-scoreboard-cell-format-and-categories.md`：
>
> - 實際固定欄位為 title-case，且新增 `Category` 欄（在 `Score` 之後）：
>   `Rank,TeamAffiliation,TeamName,SolvedCount,Score,Category,<problem…>`。
> - 題目 cell 格式對齊 Python/`parse.go`：已解題 `"<time>/<tries> try(ies)"`
>   （例 `91/1 try`、`143/4 tries`）、試過未解 `"<tries> try(ies)"`、
>   未嘗試為空字串。
> - `Download` 依 team-category `sortorder` 分段，逐段以
>   `?sortorder=N&allteams=true` 抓取後合併，納入所有 category（含 hidden
>   的內建群組）；`--exclude-hidden` 可排除 hidden category。多 group 的
>   team 依「與該 section 相符的 group」標註 `Category` 與做 hidden 過濾。
> - 不再輸出 Python 版的 summary row。
> - 因此「與 Phase 1 byte-identical」不再成立（屬刻意變更，release note
>   需標註）。

### 4.6 cid 從 int 變成 string

Phase 1 `download-scoreboard --cid 2` 的 `cid` 是 int，因為它最終只是
塞 `domjudge_cid=2` cookie。API 路徑用的是 string。

對齊規則：
- CLI flag 維持 `IntVar`，因為 Python `--cid` 也是 int。Phase 2 不能
  改。
- 內部 `strconv.Itoa(cid)` 轉成 string 後拼進 path。
- `cid` 是 **DOMjudge 內部 id**（`/api/v4/contests` 結果裡的 `id`），不
  是 `external_id`。CLI 不變更語意。

## 5. 內部架構增量

```
internal/
├── client/
│   ├── api/v4/
│   │   ├── client.go              (改：在 Client struct 加 Imports / Contests / Organizations / Groups 欄位)
│   │   ├── imports.go             (新：ImportsResource，6 個 multipart POST 方法)
│   │   ├── multipart.go           (新：postMultipart 輔助 — 由 client.go 共用)
│   │   ├── organizations.go       (新：OrganizationsResource — List / Get / FindByName)
│   │   ├── groups.go              (新：GroupsResource — List / Get)
│   │   ├── contests.go            (新：ContestsResource — List / Get / 提供 cid 解析)
│   │   ├── scoreboard.go          (新：ScoreboardResource — Get JSON)
│   │   └── ...                    (其餘維持)
│   └── web/
│       ├── gateway.go             (改：加 // Deprecated 註解到將被搬走的方法)
│       └── v7/                    (不動 — 仍是 fallback)
├── importer/
│   ├── importer.go                (改：分裂 createFlow / updateFlow，注入 router)
│   ├── router.go                  (新：選擇 API 或 scraper，呼叫鏈式記錄)
│   ├── delete.go                  (不動 — 仍 100% 走 scraper)
│   └── parse.go                   (不動)
├── scoreboard/
│   ├── build.go                   (新：JSON -> CSV row builder)
│   ├── download.go                (改：呼叫 api.Scoreboard.Get / Teams.List / Organizations.List 後送 build)
│   ├── parse.go                   (不動 — 留作 fallback；測試保留)
│   └── parse_test.go              (不動)
└── cli/
    ├── scoreboard/                (改：deps.Download 簽名變更)
    ├── users/                     (改：deps 多接一個 ApiImports 介面)
    └── problems/                  (不動 — 仍 scraper)
```

唯一需要新增的測試類型是 **multipart contract 測試** — 對 `httptest.
Server` 預先安裝一個 handler，inspection 進來的 body / boundary /
filename / form fields 與預期 byte-identical。詳見 `CONTRACT.md`。

## 6. 預期 telemetry / log 變化

- 新增 `slog` group `api`，每次 API 呼叫吐 `{method, path, status, dur_ms}`。
- 保留 `web` group。
- Router 落點以 `slog.Info("router decision", "op", "...", "path", "api|scraper", "reason", "...")`
  記錄，方便在 user 報問題時直接看哪個 op 走哪邊。
- 不從 library 印 stdout/stderr — 規範未變。

## 7. 風險與緩解

| 風險 | 等級 | 緩解 |
|------|------|------|
| §2.6 idempotency 假設失敗 | 高 | PR-0 必須先驗；若失敗 PR-3 縮成「create only」、update 路徑保留 scraper |
| `accounts.tsv` 缺 email/role 欄 | 已知 | §4.4 明文：UpdateUserEmail / SetUserPassword(roles!=nil) 走 scraper |
| `/contests/{cid}/organizations` 是 contest-scoped 而非 global | 中 | §4.2 設計成 cfg.ContestID 指定；若是 global 也 backward compatible |
| API 端點 throw 5xx 時 client 缺 retry | 已知 | 不在這次範圍；ROADMAP 已標 Phase 1.5+ HTTP retry/backoff |
| Scoreboard CSV 與 Phase 1 byte-identical 對齊困難 | 中 | DoD #4 鎖死；PR-2 用 fixture 對比 |
| 因 `web.Gateway` 縮編而動到 QUIRK 鎖測試 | 中 | QUIRK 鎖定的 `DeleteUsers` / `DeleteTeams` 不被搬，QUIRK 自動保留 |
| Multipart 上傳 cred / fixture 含真實學生資料 | 高 | 沿用 `testdata/web/v7/CAPTURE.md` 的 15-rule + 8-grep gate；PR-0 文件化 |

## 8. 對外無感的證明 — 鎖定金測試

下列「金測試」存在的目的就是確保 §1「對外無感」這個目標。 PR 合進 main
之前每一個都得綠：

- `TestImportUsersTeams_OutputCSV_GoldenIdentical_v1_0_to_v1_1`
  （新增）— 用同一份輸入 CSV 跑舊路徑（scraper）與新路徑（router→API）
  比較 `import-users-teams-out.csv` byte-identical。
- `TestScoreboard_JSONToCSV_Golden`（新增）— 餵一份從 station 抓的
  scoreboard JSON + teams JSON + orgs JSON 進 build.go，產出與
  Phase 1 從 HTML 抽出的 golden CSV byte-identical。
- 既存 `TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk` /
  `TestDeleteTeams_PythonInvertedFilter_LocksKnownQuirk` — **不能改**。
  繼續驗證 scraper QUIRK；Phase 1.5 不重做 delete 路徑。
- 既存 `TestGenConfig_GoldenDomserverJSON` /
  `TestImportUsersTeamsExample_GoldenCSV` — **不能改**。仍 byte-identical。

## 9. 出 Phase 1.5 之後

Phase 2（DOMjudge 8 支援）只需要：

1. 抓 `testdata/api/v4/v8/` fixture（API 行為應該幾乎一樣，DOMjudge 8
   同樣是 API v4 但端點略有 v8 微差）。
2. 加 `internal/client/web/v8/`，**只實作仍走 scraper 的方法**（5 個）。
3. Router 加 v8 detection。

這條路會比 Phase 1 規劃中「6 個 web/v8 全方法」省 60% 工作量。把這個
槓桿寫在 `docs/ROADMAP.md` Phase 2 條目下，提醒未來不要走回頭路。
