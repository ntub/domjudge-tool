# PR-4 — Router 抽象與 `set-password` 切換 + escape hatch

## Goal

把 PR-3 在 `importer.go` 內聯式做的「API 還是 scraper」決策抽出來變成
獨立 `internal/importer/router.go`，並讓所有混合流程的指令都用同一個
router 做決策、留下一致的 telemetry。同時為 `set-password` 套上 router、
加進 `cfg.PreferScraper` 的 escape hatch。

PR-4 完成後，CLI 的調用面被分為三類：

1. **API-only**：`version`、`export-users`、`download-submissions`、
   `download-scoreboard`。
2. **Hybrid via router**：`import-users-teams`、`set-password`。
3. **Scraper-only**：`rm-teams-and-users`、`download-problems`。

## Files created / modified

### 新增

| Path | Role |
|------|------|
| `internal/importer/router.go` | `Router` interface + `DefaultRouter` 實作 + decision table |
| `internal/importer/router_test.go` | 對 decision table 每一個 case 跑一個 sub-test，斷言選對 |
| `internal/importer/telemetry.go` | `logDecision(slog.Logger, op, path, reason)` 統一格式；測試以 `slog.NewTextHandler` 收 buffer 比對輸出 |
| `internal/importer/telemetry_test.go` | log 格式鎖死 |

### 修改

| Path | 改動 |
|------|------|
| `internal/config/config.go` | 加 `PreferScraper *bool `json:"prefer_scraper,omitempty"`。**snake_case**、optional、預設行為等同 false。schema migration：對於沒有此欄位的舊 `domserver.json`，讀回時為 `nil`，等同 false |
| `internal/config/config_test.go` | 既有 round-trip 測試擴充 + 一個「舊檔少這個欄位仍能 load」的測試 |
| `internal/importer/importer.go` | 把 PR-3 內聯的 `decideTransport(...)` 改成 `router.Decide(...)`；保留 inline behaviour 的 sub-test 不退步 |
| `internal/importer/api_create.go` | 收 `Router` 注入，呼叫 `router.Decide` 後依結果走 API 或 scraper 邏輯 |
| `internal/cli/users/set_password.go` | 改成走 router：default API、若 `roles != nil` 或 `cfg.PreferScraper == true` 走 scraper |
| `internal/cli/users/set_password_test.go` | 兩條路徑都測；fake API + fake gateway |
| `internal/cli/users/import_users_teams.go` | 把 `deps.NewRouter(cfg)` 加進 wiring |
| `internal/cli/users/register.go` | `Deps.NewRouter` 介面 + `Default` 實作 |
| `cmd/domjudge-tool/main.go` | 不需動 |

## Decision table

`Router.Decide(ctx, op Op, ctx Context) Decision`：

```go
type Op int
const (
    OpCreateTeamAndUser Op = iota   // 1 row create
    OpUpdateTeam
    OpSetPassword
    OpUpdateEmail
    OpDeleteUsers
    OpDeleteTeams
    OpResolveAffiliation
    OpListProblems
    OpDownloadProblemZip
    OpScoreboardSnapshot
)

type Path string
const (
    PathAPI     Path = "api"
    PathScraper Path = "scraper"
)

type Decision struct {
    Path   Path
    Reason string
}
```

決策規則（按順序，先命中的 reason 為主）：

| 條件 | 路徑 | Reason |
|------|------|--------|
| `cfg.PreferScraper == true` 且 op 非 API-only | scraper | `cfg.prefer_scraper=true` |
| op == `OpDeleteUsers` / `OpDeleteTeams` | scraper | `api lacks DELETE on v7` |
| op == `OpDownloadProblemZip` | scraper | `api lacks problem zip export on v7` |
| op == `OpUpdateEmail` 且 ctx.Email != "" | scraper | `accounts.tsv has no email column` |
| op == `OpSetPassword` 且 ctx.Roles != nil | scraper | `accounts.tsv has no role column; explicit roles requested` |
| op == `OpResolveAffiliation` 且 PR-0 finding 為 "global"（非 contest-scoped） | scraper | `(若 PR-0 結論是 global)` |
| 否則 | api | `default api-first` |

> **每個 reason string 是 stable identifier**，會被 SREs 拿來搜 logs 並
> 寫 dashboards。修這個字串等同改 API；改時要寫 ADR 或 issue。

`Op` 取個短小好認的字串對應 — log 行格式：

```
level=INFO msg="router decision" op=set-password path=api reason="default api-first"
level=INFO msg="router decision" op=set-password path=scraper reason="cfg.prefer_scraper=true"
```

## `cfg.PreferScraper` 的設計理由

ADR 0006 §Consequences "follow-ups"：明列要有「24 小時內可切回 Phase
1 行為」的 escape hatch。實作上：

- `domserver.json` 加 `"prefer_scraper": true`（optional 欄、預設不寫）。
- 寫了之後，所有 hybrid 指令的 router 直接落 scraper 路徑（除 op 是
  scraper-impossible 例如 problem-zip 之外都是 scraper-impossible 的
  反操作）。
- 不影響 API-only 指令（它們沒有 scraper 等價物）。
- 不影響 scraper-only 指令（本來就走 scraper）。

CLI 不加 `--prefer-scraper`，避免把這變成日常 flag。它是 ops 緊急手動
編輯 `domserver.json` 的 toggle，不是 user-facing feature。

## Tests written first (RED)

`internal/importer/router_test.go`：

| Sub-test | Inputs | Expected |
|----------|--------|----------|
| `OpDeleteUsers_AlwaysScraper` | any cfg, any ctx | path=scraper, reason="api lacks DELETE on v7" |
| `OpDownloadProblemZip_AlwaysScraper` | | scraper |
| `OpUpdateEmail_WithEmail_Scraper` | ctx.Email="x@y" | scraper |
| `OpUpdateEmail_NoEmail_API` | ctx.Email="" | api |
| `OpSetPassword_NoRoles_API` | ctx.Roles=nil | api |
| `OpSetPassword_WithRoles_Scraper` | ctx.Roles=[1,2] | scraper |
| `PreferScraperTrue_OverridesAPI` | cfg.PreferScraper=true, op=OpSetPassword, no roles | scraper, reason="cfg.prefer_scraper=true" |
| `PreferScraperTrue_NoEffectOnAPIOnlyOp` | cfg.PreferScraper=true, op=OpResolveAffiliation 但 op 不是 hybrid（要先確認 router 會收到這個 op；如果不會，移到 OpScoreboardSnapshot） | api（API-only ops 無 scraper 對等） |

`internal/cli/users/set_password_test.go`：

- `TestSetPassword_DefaultGoesAPI` — 假設 PR-0 finding 為 upsert：mock
  api.Imports.Accounts 被打到 1 次，gw.SetUserPassword 0 次。
- `TestSetPassword_RolesFlag_GoesScraper` — `--user-roles 1,2` 之後，
  api.Accounts 0 次、gw.SetUserPassword 1 次。
- `TestSetPassword_PreferScraperConfig_GoesScraper` — cfg 帶
  prefer_scraper=true。
- `TestSetPassword_GeneratedPasswordPrintedOnce` — Phase 1 既有行為，
  不退步。

## Edge cases

- PR-0 結論是 "duplicate"（即 accounts upsert 不可用）：set-password
  的 default decision 強制變成 scraper。Decision table 加一條 hard
  override 在最頂端：
  ```
  if op == OpSetPassword and !pr0FindingsFlags["accountsUpsertSafe"]:
      return scraper, "PR-0 finding: accounts.tsv re-upload is not idempotent"
  ```
  這個 flag 由 `internal/importer/router.go` 內 const 寫死（PR-4 作者
  根據 PR-0 報告填）。**這就是「PR-0 結論成為 PR-4 的編譯時常數」的具
  體實作**。
- `set-password` API path：發一個只有一行的 accounts.tsv：
  `accounts\t1\nteam\t{name}\t{username}\t{password}\n`。**name** 取自
  `api.Users.Get(ctx, userID).Name`（PR-3 已有此 endpoint）。如果 List
  回來 name 是空 → fallback 用 username。
- `slog` handler：`internal/cli/preload.go` 已建好 logger；router 用
  `slog.Default()` 即可，不需自己 wire。但測試要用
  `slog.New(slog.NewTextHandler(buf, ...))` 換進 ctx。

## Definition of done

- [ ] 上表所有檔案存在。
- [ ] `go test ./internal/importer/... ./internal/cli/users/... -cover`
      ≥ 85%。
- [ ] `domserver.json` 寫入 `prefer_scraper: true` 後，重跑 `set-password`
      與 `import-users-teams` 都能看到 `router decision` log 標明
      `path=scraper reason=cfg.prefer_scraper=true`（手動驗一次，把 stderr
      貼進 PR review）。
- [ ] `TestDomServerClient_RoundTrip_Golden`（Phase 1 anchor）仍綠 —
      新欄位是 optional，golden 仍 byte-identical 因為 omitempty。
- [ ] `TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk` /
      `TestDeleteTeams_...` 仍綠 — 我們沒動 delete 路徑。
- [ ] commit triplet：
      ```
      test(importer): red — router decision table
      feat(importer): green — router + telemetry + prefer_scraper config
      refactor(cli/users): set-password through router
      ```

## Cross-references

- ADR 0006 — Decision 條 3、Consequences "follow-ups"
- SPEC §3 對照表（router 是該表的執行體）
- SPEC §6 telemetry 變化
- SPEC §7 風險表「§2.6 idempotency 假設失敗」
- PHASE1.md "Cross-cutting rules" — slog 使用、`init()` 規則
