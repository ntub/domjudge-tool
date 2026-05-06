# PR-8 — Import quality follow-ups：UPDATE auto-recovery + CLI progress bar

## Goal

兩條 Phase 1.5 follow-up 軌道，分兩個 PR 出貨。

**Track A（PR-8 / v1.2.1 patch）— `import-users-teams` UPDATE 在 API
snapshot 回 `team_id == nil` 時的自動修復。**v1.2.0 在真實 v7 站點 dry-
run 時觸發 `web: update team: TeamID is required`：被修人 `11446001` 是
v1.1.0 silent-corruption bug（v1.1.1 已修）的孤兒受害者，team 仍存在，
但 `/api/v4/users` 列表 snapshot 在 `team_id` 上回 `null`。當前 importer
沒有 fallback 路徑，operator 必須走 web UI 或加 `--delete-existing` 才能
recover。

**Track B（PR-9 / v1.3.0 minor）— 7 條 batch 指令補上 progress bar。**
`import-users-teams`、`send-user-accounts`、`render-user-accounts`、
`download-problems`、`download-submissions`、`download-scoreboard`、以及
（重新檢視時順便補上）受影響的 ctx-cancel 漏洞修補，都包在 PR-9 出貨。
50 列以上 CSV 沒進度回饋的「靜默不安感」是 PR-6 / PR-7 dry-run 收到的
operator UX 抱怨。

兩 PR 共用本份 spec 文件，但 release 分開兩條 tag（`v1.2.1` patch 與
`v1.3.0` minor），對齊 SemVer。

## 為什麼現在做

- PR-7 (`v1.2.0`) dry-run 在真實 NTUB v7.3.4 站點上觸發 import
  失敗，是 `[1.1.1]` silent-corruption bug 修復後遺留的 prod state
  唯一未 cover 的 recovery path（CHANGELOG `[1.1.1]` 第一條 P0
  bug-fix 開頭）。Operator 沒有自助 recovery 工具就必須回去動 web UI；
  patch release 把這條補齊。
- PR-6 的 render loop（5 列 CSV）是第一次有 operator 在 `for _, user
  := range users` 上看不到任何視覺回饋；PR-7 dry-run 的 NTUB 50+ 列
  跑下去更明顯。Phase 1.5 結尾把 7 條 batch 指令一次補齊比拖到 Phase
  2 才做更划算（PR-9 同時 sweep ctx-cancel 漏洞，pure parity work，
  順手）。

## Track A — `import-users-teams` UPDATE handles `team_id == nil`

> 出貨 PR-8（branch `feat/pr8-update-team-id-recovery`）→ 釋出
> `v1.2.1`。

### 兩段式 lookup contract

當 importer.update path 看到 `existing.TeamID == nil` 或 `*existing.
TeamID == ""`：

1. **Singular endpoint**：`GET /api/v4/users/{user.ID}`。已知 v7 build
   在 list endpoint 略掉 `team_id`，但 singular endpoint 會回。命中即
   結束。
2. **Team-list-by-name fallback**：呼叫
   `GET /api/v4/contests/{contestID}/teams`，逐筆比對 `team.Name ==
   *snapshot.Team` **與** `team.DisplayName != nil &&
   *team.DisplayName == *snapshot.Team`（v7 在哪欄回對人名取決於
   server 設定，比兩欄是廉價的防禦性寫法）。第一筆命中即結束；多筆
   命中印 `slog.Warn` 但仍回第一筆 ID。
3. 兩段都失敗 → 回 wrap 過 `web.ErrTeamIDMissing` 的錯誤；caller
   responsible for adding row context (username) on top。

`existing.Team == nil || *existing.Team == ""`（snapshot 沒帶 team
display name）→ 跳過 step 2，直接走 sentinel。`opts.API == nil` 或
`opts.ContestHint == ""` 也跳過 step 2（沒 contest hint 不能列 team
clip）。

### Sentinel error

```go
// internal/client/web/errors.go
var ErrTeamIDMissing = errors.New("web: team_id missing on snapshot")
```

放在 `web` package 的理由：caller `internal/importer` 已 import 這個
package（line 10 of `importer.go`）；不再多開一個 package 就能拿到
sentinel；雖然 lookup 邏輯不在 web package 內，但這個 error 表達的是
「v7 web/api snapshot 在 team_id 欄位上不可信」這個 _live-station 行
為差異_，跟其他 `Err*` sentinel 同一語意層。

### File ownership

| Path | 改動 |
|------|------|
| `internal/client/web/errors.go` | 加一個 sentinel `ErrTeamIDMissing` |
| `internal/client/web/errors_test.go`（不存在則新增） | sentinel 存在性 + `errors.Is` round-trip |
| `internal/importer/team_lookup.go`（新增） | `resolveTeamID` 兩段式 lookup helper |
| `internal/importer/team_lookup_test.go`（新增） | 4 case：singular hit、name fallback、雙失敗、`Team==nil` 跳 step 2 |
| `internal/importer/importer.go`（line 230-237） | 進 `gw.UpdateTeam` 之前插入 nil-TeamID guard，呼叫 `resolveTeamID` |
| `internal/importer/importer_test.go` | 加 2 個 regression：auto-recover 走 fallback、unrecoverable 路徑 surface 出 sentinel |
| `CHANGELOG.md` | `## [1.2.1] - YYYY-MM-DD` patch section |

### Wire-in shape（importer.go：line 231-237 上方）

```go
if existing.TeamID == nil || *existing.TeamID == "" {
    var teamsRes *apiv4.TeamsResource
    if opts.API != nil {
        teamsRes = opts.API.Teams
    }
    tid, err := resolveTeamID(ctx, users, teamsRes, opts.ContestHint, existing)
    if err != nil {
        return model.CreateUser{}, fmt.Errorf(
            "importer: update team %s: %w", cu.Username, err)
    }
    existing.TeamID = &tid
}
```

`users` 已經是 `ImportUsersTeams` 的 positional 參數，直接傳；
`opts.ContestHint` 已由 CLI 從 `cfg.ContestID` 餵入（見
`internal/cli/users/import_users_teams.go:99`）。**不新增** top-level
option type、**不改** `ImportUsersTeams` signature。

### Helper signature

```go
// internal/importer/team_lookup.go

// resolveTeamID returns the numeric DOMjudge team ID for a user whose
// API snapshot has team_id == nil. Two-step strategy:
//
//   1. GET /api/v4/users/{user.ID}. Some v7 builds populate team_id on
//      the singular endpoint even when /users (list) omits it.
//   2. If still nil and snapshot.Team (display name) is non-empty, list
//      /api/v4/contests/{contestID}/teams and match by team.Name OR
//      team.DisplayName. First match wins; logs a slog warning if
//      multiple matches found.
//
// Returns "" wrapped over web.ErrTeamIDMissing when neither succeeds,
// or when the second-step preconditions (teams resource / contestID /
// snapshot team display name) are missing.
//
// The caller is responsible for adding row context (username) to the
// returned error so the operator sees which row triggered the recovery
// failure.
func resolveTeamID(
    ctx context.Context,
    users *apiv4.UsersResource,
    teams *apiv4.TeamsResource,
    contestID string,
    snapshot model.User,
) (string, error)
```

### 邊界情境

- **多筆 name 撞名**：印
  `slog.Warn("team_id resolved via team-name fallback returned >1 match",
  "username", snapshot.Username, "team_name", *snapshot.Team, "first_id", id)`
  並回第一筆。撞名是 station-side data-quality 問題，不是 PR-8 要修
  的範圍。
- **api 兩段都不可達（網路死）**：兩段 lookup 都會 propagate `*HTTPError`
  / 相關 wrapped error；只在「兩段都成功但都沒給 team_id」這條路徑才
  回 `ErrTeamIDMissing`。網路錯不會被吞成 sentinel。
- **`opts.API == nil`** （Phase 1 簡單測試）：跳過 step 1（`users` 是 v4
  resource，仍可呼叫；事實上 `users` 一定有，所以 step 1 不需要這個
  guard），跳過 step 2（沒 teams resource），lookup 必然落到 sentinel。
- **CONVENTIONS.md "library packages use slog, never print"**：所有
  `slog.Warn` 走預設 logger（root cmd PreRunE 已 rebind via
  `slog.SetDefault`，PR-7 §"Internal" `[1.1.1]` 修過）。

### RED → GREEN commit order

1. `test(web): red — ErrTeamIDMissing sentinel exists and is errors.Is-comparable`
   - file: `internal/client/web/errors_test.go`
   - 唯一 case：`errors.Is(ErrTeamIDMissing, ErrTeamIDMissing)` 與
     `Error()` 字串穩定。
2. `test(importer): red — resolveTeamID two-step lookup`
   - file: `internal/importer/team_lookup_test.go`
   - cases：
     - `TestResolveTeamID_SingularEndpointPopulatesTeamID`
     - `TestResolveTeamID_FallsBackToTeamListByName`
     - `TestResolveTeamID_FallsBackMatchesDisplayName`（match `t.DisplayName` 而非 `t.Name`）
     - `TestResolveTeamID_BothLookupsFail_ReturnsErrTeamIDMissing`
     - `TestResolveTeamID_TeamNameNil_SkipsListLookup`
3. `test(importer): red — update row with nil team_id auto-recovers`
   - file: `internal/importer/importer_test.go`
   - case：`TestImportUsersTeams_UpdateRow_NilTeamID_AutoRecovers`
   - **不**改既有 `usersAPIServer` helper；新增 `teamRecoveryServer(t, ...)` 處理三條 endpoint。
4. `test(importer): red — update row with unrecoverable nil team_id surfaces sentinel`
   - case：`TestImportUsersTeams_UpdateRow_LookupFails_ReturnsErrTeamIDMissing`
5. `feat(web): green — add ErrTeamIDMissing sentinel`
6. `feat(importer): green — resolveTeamID + nil-TeamID guard`
7. `fix(importer): green — auto-recover team_id when api snapshot omits it`
   （final feat commit；squash-source for merge title）
8. **squash + merge --no-ff**
   - merge commit：`Merge branch 'feat/pr8-update-team-id-recovery' (PR-8)`
9. `chore(release): goreleaser + readme polish (phase 1.5 v1.2.1)`
   - 直接落在 main 上、不用獨立 PR-7b（patch release 比 PR-7 minor
     輕，不切兩條 PR）。
   - 含 `CHANGELOG.md [1.2.1]`、本地 `git tag -a v1.2.1`。

> Spec doc commit `docs(phase1.5): add PR-8 import quality followups specification`
> 沿襲 PR-6 / PR-7 慣例，**先進 main**，feature branch 從含 spec 的
> commit 切出。

### v1.2.1 dry-run checklist

- [ ] `just check` 綠。
- [ ] 對曾 fail 的 NTUB 站點重跑 `import-users-teams ...`，預期 update
      `11446001` 由 team-name fallback 自動接住，exit 0、出 CSV 正常。
- [ ] 在 fresh test station 把某 user 的 team 砍掉但留 user（人造
      orphan），預期錯誤含 `web: team_id missing on snapshot` 與
      該 username。
- [ ] PR-7 v1.2.0 byte-identical anchor 五條再跑一次：`gen-config`、
      `import-users-teams-example`、`download-scoreboard`、
      `email-template-example`、`render-user-accounts` — 全部 byte-
      identical（PR-8 沒碰這些 path）。
- [ ] `goreleaser release --snapshot --clean` 出 4 binary。

### Track A definition of done

- [ ] 上表全部檔案存在、`gci` import 排序通過。
- [ ] 每個 RED→GREEN 都看得到 commit 對。
- [ ] `go test ./internal/importer/... ./internal/client/web/... -cover`
      跑 PR-8 新增 case 全綠。
- [ ] `golangci-lint run` / `just check` 綠。
- [ ] **既有 e2e 不退步**：v1.2.0 byte-identical anchor 全部仍 pass。
- [ ] `CHANGELOG.md` `## [1.2.1]` 段已寫、本地 `git tag -a v1.2.1`。

---

## Track B — CLI progress bar via `schollz/progressbar/v3`

> 出貨 PR-9（branch `feat/pr9-cli-progress-bar`）→ 釋出 `v1.3.0`。
> 從 PR-8 merge 後的 main 切出。

### 套件選擇

`github.com/schollz/progressbar/v3`（MIT, Go 1.23+ 相容）。Phase 1
之後**第一個新增的 production runtime dependency**，需在 spec 與
CHANGELOG 都寫清楚理由：

- 純 stdlib 自實作會花掉 PR-9 一半篇幅在處理 CR-overwrite、TTY
  detection、SIGWINCH、carriage return 等細節，且自實作易在 CI 容器
  中行為不一致。schollz 已內建 `OptionSetWriter`、`OptionThrottle`、
  `OptionClearOnFinish`、`OptionShowCount`、`DefaultSilent` 等 ergonomics。
- 同類選項中 schollz 維護最活躍、test 覆蓋穩定、無外部依賴。
- v3 是穩定 major（已釋出超過 5 年）。
- pin 到 latest stable tag；CI `go mod verify` / `gosum` 把關。

不選的：
- `vbauerster/mpb` — 多支 bar 能力對單條 batch 過頭。
- `cheggaaa/pb` — v3 後維護放緩，且單條 batch 沒比 schollz 強。

### Reporter 介面與 package 形狀

新 package：`internal/util/progress`（leaf util，符合 ARCHITECTURE.md
分層）。

```go
// internal/util/progress/reporter.go
package progress

// Reporter is implemented by code that wants visual feedback for a
// known-length loop. Library packages (internal/email, internal/importer,
// internal/submissions) accept Reporter as an optional dependency;
// Nop() is silent and safe in tests / non-TTY pipelines.
type Reporter interface {
    Start(total int, label string)
    Tick()
    Finish()
}

// New returns a Reporter that writes a single-line bar to w when w is
// a TTY, NO_COLOR is unset, TERM != "dumb", and noProgress is false.
// Otherwise returns Nop(). The progress bar writer for CLI subcommands
// is conventionally cmd.ErrOrStderr() so that stdout-as-data pipes
// stay byte-identical to v1.2.x output.
func New(w io.Writer, noProgress bool) Reporter

// Nop returns a no-op Reporter; safe in tests and non-TTY pipelines.
func Nop() Reporter
```

實作細節：
- Internal `realReporter` 包 schollz `*progressbar.ProgressBar`，
  `Tick` 走 `bar.Add(1)`、`Finish` 走 `bar.Finish()`，
  `OptionThrottle(50ms)` 避免高頻 tick 把終端打爆。
- TTY/環境檢測在 `New` 一次決定：`isTTY(w)` 用
  `golang.org/x/term.IsTerminal(int(f.Fd()))` 當 `w` 是 `*os.File`、
  其他寫入器一律視為 non-TTY。**新增 dependency 限定 `x/term`**（已是
  Go 標準擴充，傳遞自 `golang.org/x/net`，不是新外部依賴）。
- 環境變數 gate：`NO_COLOR` 設且非空 → Nop；`TERM=dumb` → Nop。
- `noProgress=true` 任何狀況 → Nop（明確壓過 TTY 偵測）。
- **不暴露**測試專用的 `WithForceTTY()` API；測試直接走 package-
  internal `newRealBar(w)` 與 `gateForTesting(w, isTTY, noProgress, env)`，
  保持 public surface 只有 `New` / `Nop` / `Reporter`。

### CLI 持續性 flag

```go
// internal/cli/root.go — within NewRootCmd
cmd.PersistentFlags().Bool("no-progress", false,
    "disable the progress bar (auto-disabled when stderr is not a TTY)")
```

PreRunE 補一段：

```go
noProg, _ := c.Flags().GetBool("no-progress")
rep := progress.New(c.ErrOrStderr(), noProg)
c.SetContext(WithReporter(c.Context(), rep))
```

`WithReporter` / `ReporterFromContext` pair 放 `internal/cli/context.go`，
緊鄰既有 `WithConfig` / `WithLogger`：

```go
// internal/cli/context.go (append)
func WithReporter(ctx context.Context, r progress.Reporter) context.Context
func ReporterFromContext(ctx context.Context) progress.Reporter // never nil; defaults to progress.Nop()
```

`ReporterFromContext` 永不回 nil — fallback `progress.Nop()`，這樣 lib
caller `r.Tick()` 不需 nil-check。

### 7 條 batch 指令 wire-in

| Command | Loop site | Plumbing |
|---------|-----------|----------|
| `import-users-teams` | `internal/importer/importer.go` create cohort scraper loop（line 169-176）+ update loop（line 141-147）| `ImportOptions.Reporter Reporter` |
| `send-user-accounts` | `internal/email/send.go:62`（`for _, user := range users`）| `SendOptions.Reporter Reporter` |
| `render-user-accounts` | `internal/cli/emails/render_user_accounts.go:66`（cli loop）| 直接從 `ReporterFromContext(ctx)` |
| `download-problems` | `internal/cli/problems/download.go:76`（cli loop）| 直接從 ctx |
| `download-submissions` | `internal/submissions/download.go:48`（lib loop）| `submissions.Options.Reporter Reporter` |
| `download-scoreboard` | `internal/cli/scoreboard/download.go:73`（cli writer loop）| 直接從 ctx |
| 同步補強：`render-user-accounts` 與 `download-scoreboard` 兩個 cli loop **目前沒 honour ctx.Done**（CONVENTIONS.md §"Honour ctx.Done()" 違反），順手補 — 見 §"Bundled cancellation fixes"。

> import 的 `apiCreateFlow`（bulk endpoints）會被 v1.1.1 hot-fix 後的
> default router 跳過，create cohort 走 scraper loop。Reporter 在
> default router 下的語意是 「N 列 / N 列」，跟 operator 心智模型一致。
> 若未來 router 切回 API path，bar 一次跳到 finish — 接受這條 trade-off
> 換取 N=N 的可讀性。

### Library no-print rule

- `internal/email`、`internal/importer`、`internal/submissions` 仍走
  CONVENTIONS.md "library packages use slog, never print"。Reporter 是
  optional 的 opaque interface，library 永遠**不**自己 construct
  `progress.New`；只接 caller 給的 Reporter（nil 或 `Nop()`）。
- 唯一 construct site 是 `internal/cli` 的 root PreRunE。CLI 子指令
  從 ctx 拿、傳給 lib options 即可。

### Bundled cancellation fixes

| Site | 補丁 |
|------|------|
| `internal/cli/emails/render_user_accounts.go:66`（loop top） | `if err := ctx.Err(); err != nil { return err }` |
| `internal/cli/scoreboard/download.go:73`（writer loop top） | 同上 |

附帶 RED 測試（confirm cancel propagates before next iteration）：
- `internal/cli/emails/render_user_accounts_test.go` —
  `TestRenderUserAccounts_CtxCancelled_AbortsBeforeNextRow`
- `internal/cli/scoreboard/download_test.go` —
  `TestDownloadScoreboard_CtxCancelled_AbortsBeforeNextRow`

> 不順手 patch `cli/problems/download.go:77`、`submissions/download.go:50`、
> `email/send.go:63` — 這三條已經有 ctx.Err() 檢查（既有測試 cover）。

### CONVENTIONS.md 補一行

於 docs/CONVENTIONS.md "I/O & logging" 段補：

> Progress bars write to **stderr** via `cmd.ErrOrStderr()` — stdout
> reserved for data so `2>/dev/null` 或 `| ...` pipe 不混入 control
> sequences。Library 不自己 construct `progress.New`；CLI root PreRunE
> 是唯一 construct site。

### RED → GREEN commit order

1. `test(progress): red — Reporter nop on non-tty + flag-disabled + env`
   - file: `internal/util/progress/reporter_test.go`
   - cases：`TestReporter_NonTTYWriter_NoOutput`、
     `TestReporter_NoProgressFlag_NoOutput`、
     `TestReporter_NO_COLOR_NoOutput`、`TestReporter_TERMDumb_NoOutput`、
     `TestReporter_TickIncrements_FinishWritesNewline`、
     `TestReporter_NopFromTopFunc_IsSafeToCall`。
2. `feat(progress): green — Reporter wrapping schollz/progressbar/v3`
   - 加 `internal/util/progress/reporter.go`、`go.mod` / `go.sum`
     （`schollz/progressbar/v3`、`golang.org/x/term`）。
3. `test(emails,scoreboard): red — render + scoreboard honour ctx.Done`
   - 兩個既有 cli test file 各加一個 case。
4. `feat(emails,scoreboard): green — ctx checks at loop boundaries`
5. `feat(cli): green — persistent --no-progress flag + Reporter ctx helper`
   - `internal/cli/root.go`、`internal/cli/context.go`、新增 root
     test 驗證 flag 註冊與 PreRunE 注入。
6. `feat(importer,email,submissions): green — Reporter optional in lib opts`
   - `ImportOptions.Reporter`、`SendOptions.Reporter`、
     `submissions.Options.Reporter`。Library 預設 `Nop()`（透過 nil-
     guard）。
7. `feat(cli): green — wire Reporter through 7 batch commands`
8. **squash + merge --no-ff**
   - merge commit：`Merge branch 'feat/pr9-cli-progress-bar' (PR-9)`
9. `chore(release): goreleaser + readme polish (phase 1.5 v1.3.0)`
   - mirror PR-7 shape：
     - `CHANGELOG.md ## [1.3.0]`
     - `README.md` Commands 段加 progress bar / `--no-progress` 註解
     - `docs/ROADMAP.md` Phase 1.5 完成清單追加 v1.3.0 row
     - `docs/ARCHITECTURE.md` package 表加 `internal/util/progress`
     - `docs/CONVENTIONS.md` 補上述 stderr-progress 規則
     - 本地 `git tag -a v1.3.0`

### v1.3.0 dry-run checklist

- [ ] `just check` 綠。
- [ ] `bin/domjudge-tool import-users-teams ./testdata/importer/input_5_rows.csv`
      在 TTY 終端 — stderr 出進度條，stdout 與 v1.2.1 byte-identical。
- [ ] 同上 `2>/dev/null` — 無進度噪音。
- [ ] 同上 `--no-progress` — TTY 也不出。
- [ ] `NO_COLOR=1` env — 不出。
- [ ] `TERM=dumb` env — 不出。
- [ ] 7 條 batch 指令各跑一次（含 send/render/download-*），都看見 bar。
- [ ] `goreleaser release --snapshot --clean` 出 4 binary。

### Track B definition of done

- [ ] 上表全部檔案存在、`gci` import 排序通過。
- [ ] 每個 RED→GREEN 都看得到 commit 對。
- [ ] `go test ./internal/util/progress/... ./internal/cli/...` 綠。
- [ ] `golangci-lint run` / `just check` 綠。
- [ ] **既有 e2e 不退步**：PR-7 v1.2.0 與 PR-8 v1.2.1 byte-identical
      anchor 在 `2>/dev/null` 與 `--no-progress` 任一條件下仍 pass。
- [ ] `CHANGELOG.md` `## [1.3.0]` 段已寫、本地 `git tag -a v1.3.0`。
- [ ] `go.mod` / `go.sum` 含 `schollz/progressbar/v3` + `golang.org/x/term`，
      `go mod verify` 綠。

---

## Files created / modified（合併）

### 新增

| Path | PR | Role |
|------|----|------|
| `docs/phase1.5/PR-8-import-quality-followups.md` | spec | 本文件 |
| `internal/importer/team_lookup.go` | PR-8 | `resolveTeamID` 兩段式 lookup |
| `internal/importer/team_lookup_test.go` | PR-8 | helper 五個 case |
| `internal/client/web/errors_test.go`（若不存在） | PR-8 | sentinel round-trip |
| `internal/util/progress/reporter.go` | PR-9 | Reporter + schollz wrap |
| `internal/util/progress/reporter_test.go` | PR-9 | TTY/env/flag gate cases |

### 修改

| Path | PR | 改動 |
|------|----|------|
| `internal/client/web/errors.go` | PR-8 | 加 `ErrTeamIDMissing` sentinel |
| `internal/importer/importer.go`（line 230-237 上方） | PR-8 | nil-TeamID guard 呼叫 `resolveTeamID` |
| `internal/importer/importer_test.go` | PR-8 | 加 2 個 regression case；新增 `teamRecoveryServer` helper（不改既有 `usersAPIServer`） |
| `CHANGELOG.md` | PR-8 / PR-9 | `[1.2.1]` patch 段、`[1.3.0]` minor 段 |
| `internal/cli/root.go` | PR-9 | `--no-progress` persistent flag、PreRunE 注入 Reporter |
| `internal/cli/context.go` | PR-9 | `WithReporter` / `ReporterFromContext` |
| `internal/importer/importer.go` | PR-9 | `ImportOptions.Reporter Reporter` 欄位 + `Tick` 在 update / scraper-create 兩條 loop |
| `internal/email/send.go` | PR-9 | `SendOptions.Reporter`、loop tick |
| `internal/submissions/download.go` | PR-9 | `Options.Reporter`、loop tick |
| `internal/cli/emails/render_user_accounts.go` | PR-9 | ctx-cancel 補 + reporter Tick |
| `internal/cli/scoreboard/download.go` | PR-9 | 同上 |
| `internal/cli/problems/download.go` | PR-9 | reporter Tick（ctx 已 OK） |
| `internal/cli/submissions/download.go` | PR-9 | reporter 從 ctx 取出傳進 lib |
| `internal/cli/users/import_users_teams.go` | PR-9 | 把 reporter 餵進 `ImportOptions` |
| `internal/cli/emails/send_user_accounts.go` | PR-9 | 把 reporter 餵進 `SendOptions` |
| `go.mod` / `go.sum` | PR-9 | `schollz/progressbar/v3`、`golang.org/x/term` |
| `README.md` / `docs/ROADMAP.md` / `docs/ARCHITECTURE.md` / `docs/CONVENTIONS.md` | PR-9 release commit | 同 PR-7 模板 |

---

## Cross-references

- ADR `docs/adr/0006-api-first-with-scraper-fallback.md:140-160` —
  為何 update path 仍走 scraper（PR-8 fallback 落在 update path 之上，
  不影響該決議）。
- `internal/importer/router.go:113-119`（`OpCreateTeamAndUser` v1.1.1
  scraper 鎖定）— 與 PR-8 不重疊：team-id 修復是 _upstream_ of the
  router decision，看到的是 update cohort、router 此時不被諮詢。
- `CHANGELOG.md ## [1.1.1]` — silent-corruption hot-fix；其遺留的
  arphan team-id state 是 PR-8 要 cover 的 prod state。
- `docs/phase1.5/PR-6-email-template-tooling.md` / `PR-7-release-v1.2.0.md`
  — spec 模板與 squash-merge / release 慣例。
- `docs/CONVENTIONS.md` — `Honour ctx.Done()` 規則（PR-9 §"Bundled
  cancellation fixes" 兩處違反）；I/O & logging 段（PR-9 release 補
  stderr-progress 規則）。
- schollz/progressbar/v3 docs（fetched via `ctx7`）— APIs：
  `progressbar.NewOptions(max, ...OptionFn)`、`bar.Add(n)`、
  `bar.Finish()`、`OptionSetWriter`、`OptionThrottle`、
  `OptionClearOnFinish`、`OptionShowCount`、`DefaultSilent`。
