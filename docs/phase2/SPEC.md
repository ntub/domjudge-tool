# Phase 2 — 規格書（SPEC）

> 這份文件是 Phase 2 的「合約」。它把 DOMjudge 8.3.1 live investigation
> 已確認的事實、還沒確認的 drift、每個 CLI 指令的 transport 決策、fixture
> 與 live write 安全規則都寫在一起。實作時如果遇到「v8 是否要走 API」
> 或「這個欄位 drift 可不可以忽略」，先查這裡；找不到就補文件，不要憑
> 感覺改。

## 1. 目標與非目標

### 目標

1. **同一個 binary 支援 DOMjudge 7.3.x 與 8.3.1**，由既有
   `config.Version` 選擇；`"8.3.1"` 走 v8 scraper，7.3.x 走 v7 scraper。
2. **CLI 介面、輸入輸出格式、`domserver.json` schema 全部不變**。
3. 讓 `internal/client/api/v4` 的 decoder 能吃 v7 與 v8 fixture，不因
   `time_limit` float 或 `api_version` int 這類跨版本 drift 失敗。
4. 新增 `web/v8`，active scraper surface 維持 5 個方法：
   `Login`、`GetProblems`、`Download`、`DeleteUsers`、`DeleteTeams`；
   但 `web/v8.Client` 必須滿足完整 `web.Gateway` 介面，非 active 的 legacy
   fallback 方法以明確 unsupported sentinel fail fast。
5. 先抓 sanitized v8 fixture，再寫 v8 Go code。API JSON 與 jury HTML 都
   需要 contract fixture。
6. 明確鎖住 live write/delete 安全規則：teams / users / groups / orgs 在
   DOMjudge 是 global resource，不是 contest-scoped。

### 非目標

- 不新增 CLI 功能、不新增 flag、不改輸出檔名。
- 不新增 `domserver.json` 欄位。Phase 2 只把既有 `version` 欄位設成
  `"8.3.1"`。
- 不做 runtime auto-detection；不呼叫 `/api/v4/version` 來決定 scraper。
- 不移除 `web/v7`，也不修 v7 delete inverted-filter quirk。
- 不新增第三方 dependency；測試仍只用 stdlib `testing` + `httptest`。
- 不在測試打真實 DOMjudge、SMTP 或 production endpoint。

## 2. 每個 CLI 指令的 v8 對應決策

圖例：

- ✅ API-only：v8 預期繼續走 `internal/client/api/v4`。
- ⚠️ Hybrid：API 為主，但仍需 `web.Gateway`。
- 🔒 Scraper-only：API 做不到，必須走 `web/v8`。
- — local-only：不碰 DOMjudge。

| CLI 指令 | 目前 transport（Phase 1.5） | v8 as-is? | v8-specific work needed |
|----------|-----------------------------|-----------|-------------------------|
| `gen-config` | — local-only | ✅ | 不改 schema；只允許 operator 手動把既有 `version` 設成 `"8.3.1"`。預設值仍是 `"7.3.2"`。 |
| `export-users` | ✅ `/api/v4/users` | 待 P2-0 fixture 確認 | 抓 v8 users fixture；確認 silent drift 不影響 `model.User` 與 exporter 欄位。 |
| `import-users-teams` | ⚠️ API import + scraper fallback（v7 create/update 仍有 fallback） | ❌ | v8 import body shape 需調整；organizations JSON 不送 `url` / `twitter_hashtag`；groups / teams / accounts 用 P2-0 live write protocol 驗證。 |
| `rm-teams-and-users` | ⚠️ API list users + scraper delete users/teams | ❌ | 新增 `web/v8.DeleteUsers` / `DeleteTeams`；delete correctness 用 `httptest` 鎖。live write/delete 只可用 unique synthetic prefix + explicit sign-off。 |
| `download-problems` | 🔒 `web.Gateway.GetProblems` + `Download` problem ZIP | ❌ | 新增 `web/v8.GetProblems` 與 `Download`。v8 API 沒有完整 problem-package ZIP export；只有 samples zip 不足以替代。 |
| `download-scoreboard` | ✅ `/api/v4/contests/{cid}/scoreboard` + teams/orgs/groups | 待 P2-0 fixture 確認 | 抓 v8 scoreboard / teams / orgs / groups fixture；確認 `Category` 與 sortorder flow 不受 drift 影響。 |
| `download-submissions` | ✅ submissions / files / source-code API | ❌ | `indexProblems` 會讀 problems；v8 `time_limit: 5.0` 會讓 Go int decode fail。P2-1 修 `time_limit` 相容後才能 as-is。 |
| `set-password` | ⚠️ API accounts import 或 scraper fallback | 待 live write 確認 | v8 accounts body shape 使用 `{id,username,password,type:"team",team_id,name}` 的 import mechanics；live write 只可在 sanctioned contest / synthetic prefix 下驗證。 |

結論：

- v8 仍不是「全 API」。`rm-teams-and-users` 與 `download-problems` 必須保留
  scraper。
- `download-submissions` 是已確認的真 break，因為它會透過
  `internal/client/api/v4.ProblemsResource` 讀 `time_limit`。
- `export-users`、`download-scoreboard` 目前沒有 live-confirmed hard break，
  但 P2-0 必須用 v8 fixture 鎖住。

## 3. v8 REST API drift

Live investigation 針對 **the live 8.3.1 station** 只做 GET-only probe。已
確認 `provider.version == 8.3.1`、`domjudge.apiversion == 4`、OpenAPI
3.0.0、85 paths。

### 3.1 CONFIRMED hard breaks

| Endpoint / field | v7 Go expectation | v8 live shape | Impact | Phase 2 requirement |
|------------------|-------------------|---------------|--------|---------------------|
| `/api/v4/contests/{cid}/problems` `time_limit` | `model.Problem.TimeLimit int` | JSON number with decimal, live value `5.0` | `download-submissions` 會 decode fail；`download-problems` 的 API list path 也可能受影響 | P2-1 新增跨版本 numeric decoder；保留對外 model 行為，不讓 float 進 CLI output。 |
| `/api/v4/version` `api_version` | `versionBody.APIVersion string` | `4`（int） | 目前沒有 command 直接呼叫，但會阻止 auto-detection；也是 decoder fragility | P2-1 接受 string / int，仍回傳 string `"4"` 或既有格式。 |

### 3.2 Live-confirmed non-breaks

| 疑點 | v8 live result | Phase 2 decision |
|------|----------------|------------------|
| `source-code` response | JSON array `[filename,id,source,submission_id]` | 符合 Go `[]model.SubmissionFile`，不需要改成 object decoder。 |
| `problems` OpenAPI under-described | Live response 仍有 `ordinal`、`test_data_count`、`externalid` | OpenAPI 少寫不代表 live 沒欄位。Fixture 要以 live GET 為準。 |

### 3.3 Silent field drift（P2-0 必須用 live GET 驗證）

這些 drift 不一定造成 decode fail，因為 `encoding/json` 預設忽略 unknown
fields，而且目前 client 沒有使用 `DisallowUnknownFields`。但如果下游讀的是
舊欄位，就會變成「decode 成功但值遺失」。

| Resource | Observed / suspected drift | 目前風險 |
|----------|----------------------------|----------|
| Contest | `externalid` → `external_id`，nested `state` moved out | `download-scoreboard` 主要讀 name / id；仍需 fixture 確認。 |
| Submission | `externalid` → `external_id` | `download-submissions` 可能不需要 external id；仍需 fixture 確認。 |
| Organization | dropped `url` / `twitter_hashtag` | Import body 必須在 v8 drop 這兩欄；list decoder 需接受 missing fields。 |
| Team | dropped `members` | Scoreboard join 不應依賴 `members`。 |
| Judgement | dropped `judgehost` | `download-submissions` 若不讀 judgehost，可 additive-safe。 |

P2-1 的 decoder 原則：

- 不使用 `Decoder.DisallowUnknownFields`。
- 對已知跨版本欄位提供 narrow custom decoder，不做全域 loose map parser。
- v7 與 v8 fixture 都要留在 testdata，避免修 v8 時弄壞 v7。

## 4. v8 import body shapes

Live findings 來自 Python `domjudge_import.py` 對 8.x / 9.0 的 probing。所有
bulk import 都是 `multipart/form-data`，主要 endpoint 是：

```text
POST /api/v4/users/groups
POST /api/v4/users/organizations
POST /api/v4/users/teams
POST /api/v4/users/accounts
```

Root CCS fallback 仍可作為參考，但 Phase 2 以 `/users/{...}` primary routes
為準。

| Endpoint | Field | v8 body shape | Phase 2 requirement |
|----------|-------|---------------|---------------------|
| `/users/groups` | `json` | `groups.json`: `{id,name,hidden,sortorder}` | 可用 JSON；確認 `hidden` / `sortorder` 型別。 |
| `/users/organizations` | `json` | `organizations.json`: `{id,name,formal_name,country}` | **v8 不送 `url` / `twitter_hashtag`**。這是 Phase 2 hard requirement。 |
| `/users/teams` | `json` | `teams.json`: `{id,name,group_ids:[catID],organization_id?}` | `group_ids` 保持 array；`organization_id` 可省略或 null，依 fixture 鎖。 |
| `/users/accounts` | `json` 或 TSV mechanics 待確認 | `accounts.json`: `{id,username,password,type:"team",team_id,name}` | P2-0 sanctioned write 驗證接受 shape；若 server 只接受 TSV，文件與 router 需明確記錄 fallback。 |

目前 `internal/client/api/v4.ImportsResource.Organizations` 送的是
`[]model.Organization`，而 `model.Organization` 含 `URL` / `Hashtag`
(`twitter_hashtag`)。Phase 2 不應為 v8 硬改 model schema；應在 import
payload 層建立 v8-safe body 或 encoder，讓 v7 import 仍可保留既有欄位。

## 5. `web/v8` scope

Phase 2 的 `web/v8` active scraper surface 只包含 5 個 `Gateway` 方法：

```text
Login
GetProblems
Download
DeleteUsers
DeleteTeams
```

但 Go compile contract 仍是完整 `web.Gateway`。`web/v8.Client` 必須實作整個
介面：上述 5 個方法提供真實 v8 行為；`GetAffiliations`、`GetAffiliation`、
`CreateAffiliation`、`CreateTeamAndUser`、`UpdateTeam`、
`SetUserPassword` 這 6 個 deprecated fallback 方法在 v8 回傳
`web.ErrUnsupportedOnV8`。若介面仍包含其他非 active fallback 方法，P2-2 也
必須用同一個 sentinel stub 補齊，確保 `web/v8.Client` 可指派給
`web.Gateway`。

原因：

1. DOMjudge 8.3.1 live findings 仍 **沒有 API DELETE for users/teams**。只看
   到 banner / logo / photo / problemset / problem-unlink 類型的 delete；
   沒有可替代 `rm-teams-and-users` 的 REST endpoint。
2. DOMjudge 8.3.1 live findings 仍 **沒有 full problem-package ZIP export**。
   `/contests/{cid}/samples.zip` 只能下載 samples，不等於 jury web UI 的
   problem package export。
3. Phase 1.5 之後 `GetAffiliations`、`GetAffiliation`、
   `CreateAffiliation`、`CreateTeamAndUser`、`UpdateTeam`、
   `SetUserPassword` 都是 deprecated fallback，不是 default active path。
   v8 不應重做這 6 個 legacy form flows；如果 `prefer_scraper` 在 v8 強制
   走到它們，必須以 `web.ErrUnsupportedOnV8` 明確失敗，不能 silent
   misbehave。

`web/v8` parser 必須以 `testdata/web/v8/` 的 sanitized jury HTML fixture 為
準，不接受只靠截圖或肉眼推測 selector。

## 6. Validation record 與 live write safety

### 6.1 目前 live validation 狀態

- 已對 **the live 8.3.1 station** 做 GET-only probe。
- 沒有把 hostname、credentials、cookies、CSRF、email、學生資料或 course
  identifiers 寫入 repo。
- 寫入 / 刪除 mechanics 尚未 live-verified；Phase 2 只允許在單一 sanctioned
  writable contest 做一次受控驗證。

### 6.2 CRITICAL safety finding

DOMjudge 的 teams / users / groups / organizations 是 **global resources**，
不是 contest-scoped resources。即使 CLI 指令帶了 `<cid>` 或 config 指向某個
contest，delete sweep 看到的仍可能是整個 server 的 teams / users。

因此：

- `DeleteUsers` / `DeleteTeams` correctness 主要由 `httptest` 與 sanitized
  HTML fixture 鎖定，不靠 live delete 來「試出來」。
- 任何 live write/delete 都必須先產生 predicted match set。
- predicted match set 必須只包含本次建立的 unique synthetic prefix，例如
  `phase2-synthetic-<timestamp>-...`；不能碰既有 row。
- operator 必須 explicit sign-off，且驗證後要 cleanup。

詳細 protocol 寫在 [`CONTRACT.md`](CONTRACT.md)。

## 7. Version-selection design

Phase 2 採 **config-driven version selection**：

- 讀既有 `config.DomServerClient.Version`。
- `Version == "8.3.1"` 時，`web.Gateway` factory 回傳 `web/v8` implementation。
- `Version == "7.3.2"` 或其他受支援 7.3.x 值時，factory 回傳 `web/v7`。
- 不做 auto-detection；不為了選版本去打 `/api/v4/version`。

理由：

1. `domserver.json` 已經有 `version` 欄位，Phase 2 不需要 schema 變更。
2. `/api/v4/version` 本身在 v8 有 `api_version` int drift；拿它當 detection
   source 會把相容性問題變成啟動問題。
3. Operator 在 contest 前已知道 server 版本；手動設定比隱式偵測更可審計。

## 8. 對外契約鎖

Phase 2 所有 PR 都必須保持：

- `domserver.json` field names / types / defaults unchanged。只允許既有
  `version` 值設為 `"8.3.1"`。
- CLI command names、flag names、output file names unchanged。
- `import-users-teams.csv` column order unchanged。
- `v7.py:161` inverted-filter quirk in `DeleteUsers` / `DeleteTeams`
  preserved bit-for-bit；任何改變需要 regression test + ADR。
- Tests use only stdlib `testing` + `httptest`；never hit a real DOMjudge /
  SMTP；no testify / ginkgo。
- Go 1.23 module directive unchanged；`goquery` pinned v1.10.3；`cobra`
  pinned。
- Library packages never print；only `cmd/` and `internal/cli/` print。

## 9. 風險與緩解

| 風險 | 等級 | 緩解 |
|------|------|------|
| v8 fixture 未先抓，parser 靠猜 | 高 | P2-0 fixtures-first hard gate；沒有 fixture 不寫 v8 Go code。 |
| `time_limit` float 修法破壞 v7 int | 高 | v7 + v8 dual fixture；narrow custom decoder；`just test` 鎖。 |
| live delete 誤刪 global teams/users | 極高 | 不做 broad live delete；predicted match set + synthetic prefix + sign-off + cleanup。 |
| organizations import 送 v7 欄位被 v8 reject | 中 | v8 import payload 明確 drop `url` / `twitter_hashtag`。 |
| auto-detection 誤判版本 | 中 | 不做 auto-detection；只讀 `config.Version`。 |
| deprecated `Gateway` 方法被 v8 重做 | 中 | `web/v8` actively implements 5 methods；deprecated fallback methods 只做 `web.ErrUnsupportedOnV8` stubs；ADR 0008 鎖。 |
