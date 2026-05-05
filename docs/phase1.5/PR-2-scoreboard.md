# PR-2 — `download-scoreboard` 改走 API

## Goal

讓 `domjudge-tool download-scoreboard --cid N` 不再抓 `/public?static=1`
HTML 頁面，改打三個 API：

1. `GET /api/v4/contests/{cid}` — 解析 cid（int → string）。
2. `GET /api/v4/contests/{cid}/scoreboard` — JSON 排名。
3. `GET /api/v4/contests/{cid}/teams` + `/organizations` — 為 CSV 補上
   team_name / affiliation 兩欄。

最終輸出檔案 byte-identical 於 Phase 1（HTML scraping）版本。CLI flag
不動。

## 為什麼第一個動 scoreboard

- 它是 Phase 1 唯一一個需要 *公開讀* 的 scraper 路徑（不需要 jury
  cred），測試最容易；
- 它沒有寫操作，不會踩 idempotency 議題；
- 它的 API endpoint 100% 完整，沒有缺欄問題；
- 它能在 PR-3（importer create flow）開始前先把 ImportsResource 以外
  的 API resource（Contests / Scoreboard / Organizations / Teams） wiring
  跑起來，用最小改動驗證 PR-1 的東西真的可以 work。

## Files created / modified

### 新增

| Path | Role |
|------|------|
| `internal/scoreboard/build.go` | `Build(snap, teams, orgs) (header []string, rows [][]string, err error)` — JSON → CSV row builder |
| `internal/scoreboard/build_test.go` | 6 個 case：空 contest、滿 contest、freeze 中、有 first_to_solve、affiliation 是 nil、team display_name 是 nil |
| `testdata/scoreboard/golden_v1_0.csv` | Phase 1 輸出做的金檔案（複製自既有 `parse_test.go` 內 fixture，重新 normalize 成磁碟檔） |

### 修改

| Path | 改動 |
|------|------|
| `internal/scoreboard/download.go` | `Download(...)` 簽名變成 `func Download(ctx, c *apiv4.Client, cid string) (header []string, rows [][]string, err error)`。內部改打三個 API、呼叫 `Build(...)` |
| `internal/scoreboard/download_test.go` | 用 fixture 做契約測試；保留 `TestDownload_ContextCanceled` |
| `internal/cli/scoreboard/download.go` | CLI 層 `--cid int` 仍接 int；`strconv.Itoa(cid)` 後傳給新 `Download`；移除 `httpx.SessionClient` 依賴 |
| `internal/cli/scoreboard/register.go` | `Deps` struct 改成 `NewAPI func(*config.DomServerClient) *apiv4.Client`，去掉 `Download func(ctx, *httpx.SessionClient, int)` 那個 seam |
| `internal/cli/scoreboard/download_test.go` | mock 換成 fake `*apiv4.Client`（`httptest.Server` + `apiv4.New`） |

### 不動（但作者要看一眼確保沒踩到）

- `internal/scoreboard/parse.go` / `parse_test.go` — HTML parser 留下，
  作為「未來 v8 行為若 API 改了仍可以參考」的 reference；測試一起留。
  在 `parse.go` 頂部加註解：
  ```go
  // Package scoreboard's HTML parser is no longer used by the
  // download-scoreboard command since Phase 1.5; build.go drives the
  // CSV from the API JSON snapshot instead. The parser is retained for
  // possible scoreboard-export-from-public-mirror use cases and for
  // regression locking against Phase 1 output.
  ```
- `internal/client/httpx/session.go` — 仍是 v7 web scraper 用的
  transport；不刪。

## CSV 欄位對齊（最關鍵的工程細節）

Phase 1 的 CSV header（從 `parse.go` 推得，與 Python `download_scoreboard.py`
一致；title-case 是 Python parity 鎖死的）：

```
Rank,TeamAffiliation,TeamName,SolvedCount,Score,<problemA>,<problemB>,...
```

每筆 row：

```
1,Sample University,Example team,3,142,3/30,5/55,2/120,...
```

`build.go` 必須產出一模一樣的格式，不能有 `\n` 多空白、不能有額外
quoting、不能改順序。建議流程：

```go
func Build(snap model.ScoreboardSnapshot, teams []model.Team, orgs []model.Organization) (header []string, rows [][]string, err error) {
    teamByID := indexTeams(teams)            // map[string]model.Team
    orgByID  := indexOrgs(orgs)              // map[string]model.Organization

    header = []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score"}
    if len(snap.Rows) == 0 {
        return header, nil, nil
    }
    // problem labels are determined by row[0].Problems order — DOMjudge
    // emits them in the same order for every row, but we still validate
    // that all rows match (defensive; mismatch → ErrMalformedSnapshot).
    labels := make([]string, len(snap.Rows[0].Problems))
    for i, p := range snap.Rows[0].Problems {
        labels[i] = p.Label
        header = append(header, p.Label)
    }
    for _, row := range snap.Rows {
        team := teamByID[row.TeamID]                     // tolerate missing
        affName := ""
        if team.OrganizationID != nil {
            affName = orgByID[*team.OrganizationID].Name // tolerate missing
        }
        if affName == "" && team.Affiliation != nil {
            affName = *team.Affiliation                  // legacy fallback
        }
        if len(row.Problems) != len(labels) {
            return header, nil, ErrMalformedSnapshot
        }
        rec := []string{
            strconv.Itoa(row.Rank),
            row.TeamID,
            displayName(team),                           // see helper below
            affName,
            strconv.Itoa(row.Score.NumSolved),
            strconv.Itoa(row.Score.TotalTime),
        }
        for _, p := range row.Problems {
            rec = append(rec, formatScoreCell(p))        // see Phase 1 logic
        }
        rows = append(rows, rec)
    }
    return header, rows, nil
}
```

`displayName(team)`：先取 `team.DisplayName`、為 nil/空時取 `team.Name`。
`formatScoreCell(p)`：仿 Phase 1 HTML parser 的格式 — solved 顯示
`+<num_judged>` / `+`、failed 顯示 `-<num_judged>` / `-`、pending 顯示
`?<num_pending>`、無 attempt 為空字串。

> **PR-2 的金測試** `TestScoreboard_JSONToCSV_Golden` 從 `testdata/api/v4/`
> 三份 fixture 跑 Build，輸出與 `testdata/scoreboard/golden_v1_0.csv`
> byte-identical。如果一開始不一致，**修 `build.go` 不要修 golden**（除
> 非你能說明 Phase 1 HTML parser 在哪一格漏算了）。

## Tests written first (RED)

| Test | 注釋 |
|------|------|
| `TestBuild_EmptyScoreboard_HeaderOnly` | rows 為 nil 時不應 panic |
| `TestBuild_HappyPath_FromContractFixture` | scoreboard.json + teams.json + organizations.json fixture 餵進去；row 數量、第一筆與最後一筆完整斷言 |
| `TestBuild_FreezeState_DoesNotChangeOutput` | freeze state 在 snapshot.state 但不影響 rows；確認沒因為 frozen != null 而吐錯 |
| `TestBuild_TeamMissingFromMap_FallsBackToEmptyName` | rows 內某個 team_id 沒在 teams.json — 不能 panic、輸出空字串 |
| `TestBuild_OrgMissingFromMap_FallsBackToTeamAffiliationField` | team.OrganizationID 設了但 orgs.json 沒對應；要回退到 team.Affiliation |
| `TestBuild_MalformedRow_ReturnsErr` | 構造一個第二 row 比 header 少一題的 snapshot |
| `TestDownload_HitsAllThreeEndpoints` | httptest.Server 計數三個 path 各被打到一次 |
| `TestDownload_ContextCanceled` | mid-flight cancel 應當 ErrContext.Cancel propagate |
| `TestScoreboardCmd_Golden` | end-to-end：build cobra cmd、SetArgs、跑 → 對輸出 CSV 與 golden byte-equal |

## CLI 行為對照

| 行為 | Phase 1 | Phase 1.5 |
|------|---------|-----------|
| `--cid 2` | 設 `domjudge_cid=2` cookie + GET `/public?static=1` | GET `/api/v4/contests/2`、`/scoreboard`、`/teams`、`/organizations` |
| `--cid` 不帶值 | `MarkFlagRequired("cid")` 已在 v1.0.0 起被 cobra 擋下 | 不變 |
| `--filename` `--path` `--url` | unchanged | unchanged |
| 認證 | 公開頁，不需 jury cred | **需要** API Basic Auth（`cfg.Username` / `cfg.Password`）。雖然 scoreboard endpoint 對 admin Basic Auth 並非 strict，但 contests / teams / organizations 端點需要登入 |

> `download-scoreboard` 對使用者**沒有可觀察的行為差異**：CLI flag、輸
> 入合法性、輸出 CSV byte-identical（PR-2 的 golden 鎖死）、必填的
> `--cid` 也與 v1.0.0 相同（`MarkFlagRequired("cid")` 在
> `internal/cli/scoreboard/download.go:82` 已存在）。
> 唯一新增的是：scoreboard 內部現在會打 API（需要 Basic Auth），所以
> 在 `domserver.json.username/password` 為空 / 錯誤時會比 1.0.0 更早
> 失敗。這仍屬於「正常的 config error」，不是 breaking change。

## Definition of done

- [ ] 上表所有檔案存在；不動的檔案沒有 diff。
- [ ] `go test ./internal/scoreboard/... ./internal/cli/scoreboard/... -cover`
      ≥ 80%；既有 parser 測試仍綠。
- [ ] `bin/domjudge-tool download-scoreboard --cid 2` 對 demo station
      跑能成功，產出 `export.csv` byte-identical 於 v1.0.0-rc1 同樣 cid
      的輸出（手動驗一次，把證據貼進 PR review）。
- [ ] `bin/domjudge-tool download-scoreboard` 不帶 `--cid` 應失敗（行為
      與 v1.0.0 相同：cobra `MarkFlagRequired` 擋下；不是 PR-2 新增）。
- [ ] **無**任何 user-visible 行為差異 — PR-5 release note 不寫
      `download-scoreboard` 進 "Behaviour changes" 區。
- [ ] commit triplet：
      ```
      test(scoreboard): red — build snapshot to csv
      feat(scoreboard): green — JSON snapshot CSV builder
      refactor(cli/scoreboard): wire api.Scoreboard + drop session client
      ```

## Cross-references

- SPEC §3 對照表「`download-scoreboard`」列
- SPEC §4.5 CSV 欄位對齊
- SPEC §4.6 cid int↔string 轉換
- CONTRACT §4.1 GET 契約測試範本
