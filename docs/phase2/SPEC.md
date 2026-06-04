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
4. 新增 `web/v8`，scraper surface 包含 `Login`、`GetProblems`、`Download`、
   `DeleteUsers`、`DeleteTeams`，以及 v8 import baseline 所需的
   `CreateTeamAndUser`、`UpdateTeam`、`SetUserPassword`、`UpdateUserEmail`、
   `GetAffiliations`、`GetAffiliation`、`CreateAffiliation`。
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
| `import-users-teams` | ⚠️ API import + scraper fallback（v7 create/update 仍有 fallback） | ❌ | v8 create/update/email/password/affiliation 走 `web/v8` scraper。Bulk `accounts.json` REST import drops email on 8.3.1 and is not used for the baseline。 |
| `rm-teams-and-users` | ⚠️ API list users + scraper delete users/teams | ❌ | 新增 `web/v8.DeleteUsers` / `DeleteTeams`；delete correctness 用 `httptest` 鎖。live write/delete 只可用 unique synthetic prefix + explicit sign-off。 |
| `download-problems` | 🔒 `web.Gateway.GetProblems` + `Download` problem ZIP | ❌ | 新增 `web/v8.GetProblems` 與 `Download`。v8 API 沒有完整 problem-package ZIP export；只有 samples zip 不足以替代。 |
| `download-scoreboard` | ✅ `/api/v4/contests/{cid}/scoreboard` + teams/orgs/groups | 待 P2-0 fixture 確認 | 抓 v8 scoreboard / teams / orgs / groups fixture；確認 `Category` 與 sortorder flow 不受 drift 影響。 |
| `download-submissions` | ✅ submissions / files / source-code API | ❌ | `indexProblems` 會讀 problems；v8 `time_limit: 5.0` 會讓 Go int decode fail。P2-1 修 `time_limit` 相容後才能 as-is。 |
| `set-password` | ⚠️ API accounts import 或 scraper fallback | ❌ | v8 password updates use `web/v8.SetUserPassword` so roles/enabled form state can be preserved from the edit snapshot。 |

結論：

- v8 仍不是「全 API」。`import-users-teams` 的 create/update/email/password/
  affiliation flows、`rm-teams-and-users`、`download-problems` 必須保留
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

## 4. v8 import transport

Live findings 來自 Python `domjudge_import.py` 對 8.x / 9.0 的 probing，加上
DOMjudge 8.3.1 sanctioned write/readback。Bulk import endpoints are still
documented because they explain why the P2-4 baseline does **not** route v8
user create/update/email/password through the REST import path.

```text
POST /api/v4/users/groups
POST /api/v4/users/organizations
POST /api/v4/users/teams
POST /api/v4/users/accounts
```

The bulk `accounts.json` path drops email on DOMjudge 8.3.1, so
`import-users-teams` uses the v8 scraper for create/update/email/password/
affiliation in the P2-4 baseline. Delete is also scraper-only because there is
no REST API DELETE for users/teams on 8.3.1.

| Endpoint | Live finding | Phase 2 requirement |
|----------|--------------|---------------------|
| `/users/groups` | Bulk JSON route exists. | Not used for v8 import baseline. |
| `/users/organizations` | Bulk JSON route drops `url` / `twitter_hashtag`. | Do not infer v8 user create/update routing from this route. |
| `/users/teams` | Bulk JSON route exists, but team auto-id is needed by the scraper create flow. | P2-4 creates teams through `web/v8.CreateTeamAndUser` / `UpdateTeam`. |
| `/users/accounts` | Bulk `accounts.json` drops email. | Not used for v8 baseline user create/update/email/password. |

Verified future opportunity: single-entity `POST /api/v4/users` on DOMjudge
8.3.1 honors `email`, `team_id`, `password`, and `roles`, and returns 201.
Future work may route only the v8 user-CREATE cohort through that endpoint
after scraper team creation. `PUT /api/v4/users/{id}` is broken on 8.3.1
(HTTP 400 "already exists", field not updated), so UPDATE stays scraper.

## 5. `web/v8` scope

Phase 2 的 `web/v8` scraper surface contains these `Gateway` methods:

```text
Login
GetProblems
Download
DeleteUsers
DeleteTeams
CreateTeamAndUser
UpdateTeam
SetUserPassword
UpdateUserEmail
GetAffiliations
GetAffiliation
CreateAffiliation
```

`web/v8.Client` 必須實作完整 `web.Gateway`。The import-related methods are
real scraper implementations in P2-4 because the bulk REST import path drops
email on 8.3.1, and `PUT /api/v4/users/{id}` is not usable for updates.

原因：

1. DOMjudge 8.3.1 live findings 仍 **沒有 API DELETE for users/teams**。只看
   到 banner / logo / photo / problemset / problem-unlink 類型的 delete；
   沒有可替代 `rm-teams-and-users` 的 REST endpoint。
2. DOMjudge 8.3.1 live findings 仍 **沒有 full problem-package ZIP export**。
   `/contests/{cid}/samples.zip` 只能下載 samples，不等於 jury web UI 的
   problem package export。
3. The bulk `accounts.json` path on 8.3.1 drops email, so the v8 import
   baseline uses scraper form flows for create/update/email/password/
   affiliation. Future API-first work may use single-entity
   `POST /api/v4/users` for creates only, with scraper fallback.

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
| bulk `accounts.json` import drops email | 高 | v8 import baseline uses scraper create/update/email/password flows；single-entity API create remains future work。 |
| auto-detection 誤判版本 | 中 | 不做 auto-detection；只讀 `config.Version`。 |
| v8 scraper form delta regressions | 中 | `web/v8` import methods use sanitized form fixtures and assert posted fields；ADR 0008 鎖。 |
