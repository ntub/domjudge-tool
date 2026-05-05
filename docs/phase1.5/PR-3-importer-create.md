# PR-3 — `import-users-teams` create flow 改走 API

## Goal

把 `import-users-teams` 的 **create-new-user** 路徑（CSV 第一次匯入新
名單）從 `web.Gateway.CreateTeamAndUser` + `SetUserPassword` +
`UpdateUserEmail` 三段 scraper 呼叫，換成兩到三段 API bulk POST。

> **前置假設**：PR-0 在 `CAPTURE_API.md` §"Findings" 已驗證
> `POST /api/v4/users/accounts` 是 upsert by username。如果是
> create-only / duplicate，本 PR 縮減為「只搬 *純新建* 的 row、其餘
> 還是走 scraper」並把這個事實寫入 `SPEC.md` §3 修訂記錄。

## Files created / modified

### 新增

| Path | Role |
|------|------|
| `internal/importer/api_create.go` | `apiCreateFlow(ctx, api, rows, opts) ([]model.CreateUser, error)` — 接管 create-only 路徑，內部呼叫 `api.Imports.GroupsTSV` / `OrganizationsJSON` / `TeamsTSV` / `Accounts` |
| `internal/importer/api_create_test.go` | 多 case：empty input、單筆、多筆、重複 username（依 PR-0 結論決定 upsert/skip/err）、ctx cancel |
| `internal/importer/tsv.go` | `formatGroupsTSV(...)`、`formatTeamsTSV(...)`、`formatAccountsTSV(...)` — 純函式 builder，不接 reader/writer。回傳字串方便比對 |
| `internal/importer/tsv_test.go` | 純 string 比對：給定 model 切片，產出指定 byte-identical TSV |
| `testdata/importer/golden_groups.tsv` | `File_Version\t1\n13337\tCompanies\n` 之類 |
| `testdata/importer/golden_teams2.tsv` | 仿 import.html §"Importing teams" 範例 |
| `testdata/importer/golden_accounts.tsv` | 仿 import.html §"Importing accounts" 範例 |
| `testdata/importer/input_5_rows.csv` | DoD #4 用的金輸入 CSV |
| `testdata/importer/golden_out.csv` | 對應的金輸出 |

### 修改

| Path | 改動 |
|------|------|
| `internal/importer/importer.go` | 把現有 `processRow` 切成 `processRowAPI` + `processRowScraper`；`ImportUsersTeams` 在 loop 前呼叫 `decideTransport(opts)`，依結果分派 |
| `internal/importer/importer_test.go` | 新增「同一份 input 走 API 路徑與 scraper 路徑都產同一份輸出 CSV」的 golden test |
| `internal/cli/users/import_users_teams.go` | `deps.NewAPI(cfg)` 已存在；確認傳給 `ImportUsersTeams` 的方式不變 |

### 暫不動（PR-4 才動）

- `internal/importer/router.go` — PR-3 還沒到。PR-3 用一個簡單 `if
  opts.UpdateUserEmail || opts.UserRoles != nil → scraper else api` 函
  式內聯式做 transport 選擇，後續 PR-4 抽出來。

## API create flow 演算法

```
inputs:
  rows []model.CreateUser   from ParseUsersCSV
  opts ImportOptions
  api  *apiv4.Client
  cfg  *config.DomServerClient (for cid resolution)

  cid, err := resolveContestID(ctx, api, cfg.ContestID)
  // 1. ensure target group exists
  if opts.CategoryID != nil:
      api.Imports.GroupsTSV(ctx, formatGroupsTSV([{ID: *opts.CategoryID, Name: "p1.5-imported"}]))
      // server should upsert by id; ignore "already exists" 4xx (verify in PR-0)
  // 2. ensure organizations exist (one POST per unique CSV affiliation column)
  uniqueAffs := dedupeAffiliations(rows)
  if uniqueAffs != []:
      orgs, err := api.Organizations.List(ctx, cid)            // existing
      missing := orgsNotIn(uniqueAffs, orgs)
      if missing != []:
          payload := buildOrgsPayload(missing, opts.AffiliationCountry)
          api.Imports.Organizations(ctx, payload)
  // 3. teams + accounts
  api.Imports.TeamsTSV(ctx, formatTeamsTSV(rows, cid, indexedOrgs, indexedGroups))
  api.Imports.Accounts(ctx, formatAccountsTSV(rows, opts))     // populates passwords

  // 4. populate per-row passwords (already in TSV) and email — but
  //    accounts.tsv has no email column → email update path stays
  //    on scraper. PR-3 detects rows with non-empty email and falls
  //    back to scraper for those specific rows (UpdateUserEmail only;
  //    skip CreateTeamAndUser since API already handled that).

  // 5. snapshot updated user list to retrieve user IDs and team IDs
  snap, _ := api.Users.List(ctx, apiv4.UsersListOpts{})
  // map back to per-row CreateUser with .Password populated; output CSV
  return rows, nil
```

### 邊界

- **`cfg.ContestID == ""`（Phase 1 合法、預設值）**：必須維持
  backward-compat（ROADMAP §"Backward compatibility — must preserve
  bit-for-bit"）。實作於 `internal/importer/api_create.go` 的
  `resolveContestID`：

  ```go
  func resolveContestID(ctx context.Context, api *apiv4.Client, hint string) (string, error) {
      if hint != "" {
          return api.Contests.Resolve(ctx, hint)
      }
      contests, err := api.Contests.List(ctx)
      if err != nil {
          return "", fmt.Errorf("importer: list contests: %w", err)
      }
      active := filterActive(contests)        // 由 contest.State 判斷 started && !ended
      switch len(active) {
      case 0:
          return "", fmt.Errorf("importer: cfg.contest_id is empty and no active contest exists; set contest_id in domserver.json")
      case 1:
          return active[0].ID, nil
      default:
          ids := contestIDs(active)
          return "", fmt.Errorf("importer: cfg.contest_id is empty and %d active contests exist (%s); set contest_id in domserver.json", len(active), strings.Join(ids, ","))
      }
  }
  ```

  Phase 1 的 v7 scraper 在 `cfg.ContestID == ""` 時是「不設
  `domjudge_cid` cookie，由 server 自選 contest」。由於 API 沒有「自動
  選 contest」的概念，這個 fallback 改成「**只有一個 active contest 時
  自動採用，否則明確報錯**」 — 對單一 contest 的常見場景無感、對多
  contest 的場景行為從「靜默選一個」變成「強制要求明示」。

  把這個變化寫進 PR-3 的 PR description **與** PR-5 CHANGELOG 的
  "Behaviour changes (visible)" 區。**這是 Phase 1.5 唯一的對外行為差
  異**（advisor review #2 在這裡偵測到）。

- **`opts.IgnoreExisting`**：在送 TSV 前，從 `api.Users.List` 過濾掉
  username 已存在 row。
- **`opts.DeleteExisting`**：API 沒辦法刪。在送 TSV **之前**呼叫
  `gw.DeleteUsers` / `gw.DeleteTeams`（scraper），把 existing 砍掉再走
  API create。**這是混合的關鍵 case** — 確保 PR-3 的測試明確覆蓋這個
  路徑，且 telemetry 記下來「create flow 用 API、pre-delete 用 scraper」。
- **`opts.NewPassword=false` + CSV 有 password 欄**：API 路徑同樣用 CSV
  password；TSV builder 把它放進 `accounts.tsv` 第四欄。
- **`opts.NewPassword=true` 或 CSV 缺 password**：呼叫
  `password.Generate` 後填進 TSV；同時填回 `[]model.CreateUser` 給上層
  寫 out.csv。
- **`cu.Email != nil`**：保持 Phase 1 行為 — 在 API create 完成後對該
  user 走 scraper `UpdateUserEmail`。**不要因為 API 沒 email 欄而靜默
  drop**。

## TSV format（CCS-compliant）

`groups.tsv`：

```
File_Version\t1\n
{id}\t{name}\n
{id}\t{name}\n
```

`teams2.tsv`（**File_Version=2**，跟 groups 不同）：

```
File_Version\t2\n
{teamID}\t{externalID|""}\t{categoryID}\t{teamName}\t{instName}\t{instShortName}\t{country|""}\t{externalInstID|""}\n
```

`accounts.tsv`：

```
accounts\t1\n
team\t{fullName}\t{username}\t{password}\n
```

> 注意：DOMjudge 7 的 import 期望以 LF 結尾、UTF-8 不帶 BOM。`tsv.go`
> 寫死 `\n`，**不要**用 `\r\n`（這會被 server reject）。
> 這跟 PR-2 scoreboard CSV 用 CRLF 是兩件事 — CSV 是 csv.Writer 預設、
> TSV 是按 CCS 標準。

## Tests written first (RED)

| Test | What |
|------|------|
| `TestFormatGroupsTSV_GoldenIdentical` | 給 `[{1,"a"},{2,"b"}]` 產出與 golden 完全相符 |
| `TestFormatTeamsTSV_GoldenIdentical` | 含 unicode 字元的 team 名 |
| `TestFormatAccountsTSV_RespectsPasswordOverride` | password 為空 / 非空 兩種 |
| `TestResolveContestID_HintProvided_PassThrough` | hint="2" → 不打 List、回 "2" |
| `TestResolveContestID_EmptyHint_OneActive_AutoSelect` | List 回一個 active → 用該 id |
| `TestResolveContestID_EmptyHint_NoActive_Error` | List 回空或全 ended → 錯誤訊息含「set contest_id」 |
| `TestResolveContestID_EmptyHint_MultipleActive_Error` | List 回多個 active → 錯誤訊息列出 ids |
| `TestApiCreateFlow_HappyPath_3Rows` | httptest server 倒回三個 ImportsResource POST + 一個 List |
| `TestApiCreateFlow_IgnoreExisting_SkipsKnownRow` | snapshot 含一筆 username，IgnoreExisting=true → TSV 不含這筆 |
| `TestApiCreateFlow_DeleteExisting_CallsScraperFirst` | mock gateway 計 DeleteUsers 被呼叫，TSV 之後才送 |
| `TestApiCreateFlow_EmailRow_FallsBackToScraper` | snapshot 中該 user 有 email；確認 gw.UpdateUserEmail 被呼叫 1 次（API path 完成後） |
| `TestApiCreateFlow_GenerateNewPassword_PopulatesOutput` | output `[]CreateUser` 的 .Password 不為空 |
| `TestImportUsersTeams_OutputCSV_Golden_API_vs_Scraper` | DoD #4 金測試（CONTRACT.md §4.4） |

## Definition of done

- [ ] 上表全部檔案在；TSV golden 與 PR 的測試 byte-identical。
- [ ] `go test ./internal/importer/... -cover` ≥ 85%（既有 importer 是
      85%；不退步）。
- [ ] `TestImportUsersTeams_OutputCSV_Golden_API_vs_Scraper` 兩種 mode
      都產出與 `testdata/importer/golden_out.csv` byte-identical。
- [ ] PR 描述列出對 PR-0 idempotency 假設的依賴（具體哪一個 finding
      被引用、若 finding flip 會怎麼降級）。
- [ ] `web.Gateway` 的 `CreateTeamAndUser` / `UpdateTeam` /
      `SetUserPassword` 加 `// Deprecated: prefer
      apiv4.ImportsResource since Phase 1.5; kept for fallback.` 註解
      （**不刪實作**，scraper 仍是 fallback）。
- [ ] commit triplet：
      ```
      test(importer): red — api create flow tsv golden
      feat(importer): green — api create flow + tsv builders
      refactor(importer): split processRow into api/scraper variants
      ```

## Cross-references

- SPEC §3 對照表「`import-users-teams`」三個子操作
- SPEC §4.4 password 處理
- import.html "Importing teams" / "Importing accounts" 章節
- ADR 0006 §Decision 條 2（per-user email/role 不能用 bulk API）
