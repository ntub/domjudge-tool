# Phase 1.5 — 執行計畫（PLAN）

> 給接手的初階工程師：**從上往下做、一個 PR 一個 PR**，不要跳。每個
> PR 都有自己的 spec（`PR-N-*.md`），裡面寫好 RED → GREEN → REFACTOR
> 三段。`docs/PHASE1.md` §"TDD discipline applied to every PR" 這個流
> 程在這裡完全沿用，先寫紅再寫綠，commit triplet：
>
> ```
> test(<scope>): red — <desc>
> feat(<scope>): green — <desc>
> refactor(<scope>): <desc>      # optional
> ```

## 進度狀態（2026-05-05 接手快照）

由 Codex 起手實作；接手時的真實狀態：

| PR | 狀態 | 備註 |
|----|------|------|
| PR-0 | 70% | scripts/、CAPTURE_API.md、Justfile `api-fixture-*` task 已寫；**真實 demo 抓的 fixture 與 §2.6 idempotency 三項實測仍 pending**（CAPTURE_API.md "Findings" 全是 "Pending demo capture"）。需要 throwaway DOMjudge 7.3.x station 才能補。 |
| PR-1 | 已實作（未 commit） | `multipart`/`imports`/`organizations`/`groups`/`contests`/`scoreboard` resource + tests 全在；`client.go` wiring + models 補齊。**Contract 測試使用 inline JSON literals 而非 `testdata/api/v4/contest_2/*.json` fixture**（fixture 目錄尚未建立，等 PR-0 demo capture）。PR review 以 inline 暫代是接手時可接受的折衷。 |
| PR-2 | 已實作（未 commit） | `internal/scoreboard/build.go` + `download.go` 已切走 API；CLI deps 改注入 `*apiv4.Client`。**未產出 `testdata/scoreboard/golden_v1_0.csv`**；現用 `parse_test.go` 既有 fixture 鎖行為。 |
| PR-3 | 未動工 | `importer.go::processRow` 仍是 scraper-only。需新建 `api_create.go` / `tsv.go` 以及 golden TSV/CSV。 |
| PR-4 | 已實作（未 commit） | `router.go` / `telemetry.go` / `cfg.PreferScraper` / `set-password` 走 router 全到位。**`router.go:43` `accountsTSVUpsertSafeOnPR0 = true` 為樂觀預設**，等 PR-0 demo 驗證；若 finding 翻轉為 non-upsert，將該常數改成 `false` 即可全面回退到 scraper。`cfg.PreferScraper` 為 ops 緊急 rollback 用。 |
| PR-5 | 未動工 | docs / CHANGELOG / `v1.1.0` tag。 |

**TDD trail 缺失說明**：PR-1/PR-2/PR-4 由 Codex 一次 bulk drop 進來，未保留 red→green commit 對；接手後這三批以單一 `feat()` commit 落地，並在 commit body 註明「TDD trail not preserved — imported from Codex bulk drop」。**PR-3 與 PR-5 仍嚴格遵循 TDD discipline**（PLAN §3 review checklist 第 1 項對 PR-1/2/4 retroactively 失效，由 PR-5 release note 帶過）。

## 0. 開工前清單

- [ ] `git switch main && git pull`，確認 `v1.0.0-rc1` tag 在當前分支祖
      先（`git tag --merged | grep v1.0.0`）。
- [ ] `just check` 全綠，否則先把問題修掉。
- [ ] 讀完 `docs/phase1.5/SPEC.md` §1–§4 與本檔。
- [ ] 讀完 `docs/adr/0006-api-first-with-scraper-fallback.md`。
- [ ] 透過 `ctx7` 確認下列 lib 在 Go 1.23 相容版有哪些 API（請各別查，
      不要一次塞同一個 query）：
  - `mime/multipart`（stdlib，理論上不變但確認 Writer.CreateFormFile 行為）
  - `net/http` SSE 處理（PR-2 解 contest scoreboard 不需 SSE，但 PR-N+1
    若有 event-feed 需求，用 `bufio.Scanner` 即可）
  - `cobra` v1.8 — 不需要動，但確認沒過時的 deprecation。
- [ ] 對你準備拿來測的 DOMjudge 7 instance 做一次「乾淨重置」（destroy
      容器後 `make demo` 起新的 — 千萬不要對 production contest 站台跑），
      確保不會打到真實學生資料。

## 1. PR 清單（共 6 個）

| # | Branch | 主要交付 | 預估工作量 |
|---|--------|---------|-----------|
| 0 | `feat/p15-fixtures` | `testdata/api/v4/` 下的 sanitized JSON fixture + `CAPTURE_API.md` | 1–2 天 |
| 1 | `feat/p15-imports-scaffold` | `internal/client/api/v4/imports.go` + `multipart.go` + `organizations.go` + `groups.go` + `contests.go` + `scoreboard.go`（API resource 層 + 全 RED→GREEN 測試） | 2–3 天 |
| 2 | `feat/p15-scoreboard` | `download-scoreboard` 改走 `/api/v4/contests/{cid}/scoreboard`；新增 `internal/scoreboard/build.go` | 1–2 天 |
| 3 | `feat/p15-importer-create` | `import-users-teams` create flow 改走 API；router 雛形（單獨切 PR-4） | 2–3 天 |
| 4 | `feat/p15-router` | `internal/importer/router.go` 統一決策；`set-password` 套用 router；fallback 機制；slog telemetry | 1–2 天 |
| 5 | `feat/p15-release` | docs / changelog / `v1.1.0` tag / goreleaser snapshot | 0.5 天 |

合併方式同 Phase 1（`docs/PHASE1.md` §"Merging a PR"）：

```
git switch feat/p15-<slug>
git rebase main          # 或 merge main，看當下衝突量
just check
git switch main
git merge --no-ff feat/p15-<slug> -m "Merge branch 'feat/p15-<slug>' (PR-N)"
```

每個 PR 的 commit triplet 都先 push 到本地分支再 rebase，**不要 force-
push 已經被 reviewer 看過的 commit**。

## 2. 依賴圖

```
PR-0  ─┬─→ PR-1  ─┬─→ PR-2  ─┐
       │          │           ├─→ PR-5
       │          └─→ PR-3  ──┤
       │                      │
       └──────────────────────┴─→ PR-4
```

PR-2 與 PR-3 在 PR-1 落地後可平行；PR-4 需要 PR-2 + PR-3 都已合併才能
重構成 router 模型；PR-5 是收尾，必須等 PR-4 合併。

實際操作建議：**一次只進一個 PR 的 review**。Phase 1 的
`docs/phase1/README.md` §"Workflow" 仍適用 — Codex 寫、Claude review、
無 GitHub PR、本地 `feat/p15-<slug>` 分支。

## 3. 每個 PR 的 review checklist（review 用）

每個 PR 收到 review 之前，作者自己要先過下列七項。Reviewer 也會逐項
檢查；任一項 fail，PR 不會被 merge。

1. **TDD 痕跡**：`git log --oneline feat/p15-<slug>` 顯示
   `test(...): red`、`feat(...): green` 兩種 commit 交替；不接受「一
   口氣 feat + test 一起進」。
2. **覆蓋率**：對 PR 觸碰的每個 package，`go test -cover ./...` 不退
   步；新檔 ≥ 80%。
3. **無 testify / ginkgo**：`grep -r "testify\|ginkgo" .` 應該無命中
   （除了 docs / fixtures）。
4. **API 呼叫 ctx-first**：每一個新方法簽名都是 `func (r *X) Foo(ctx
   context.Context, ...)`。
5. **library 不印**：`grep -rn "fmt.Print\|os.Stdout\|os.Stderr" internal/`
   除 `internal/cli/` 之外應無命中。新加 log 一律走 `slog`。
6. **Conventional Commit 格式**：commit-msg hook 必須通過。**禁止**
   `--no-verify`。
7. **Python parity / QUIRK**：CLI 子指令名 / flag 名沒被改、QUIRK 鎖
   測試仍綠、`domserver.json` schema 沒新增欄位（如真的需要新欄位需先
   寫 ADR）。

## 4. 失敗時的回退策略

每個 PR 都被設計成可單獨 revert：

- PR-0 revert：fixture 檔案被刪掉，後續 PR fail；不影響 main。
- PR-1 revert：新加的 resource 一起退掉；既有 scraper 路徑沒改，CLI 仍
  正常。
- PR-2 revert：scoreboard 改回 scraper；HTML parser 仍在沒被刪。
- PR-3 revert：importer create 路徑改回 scraper；router 雛形也一起退。
- PR-4 revert：router 退、`set-password` 退；importer 仍走 PR-3 的「直
  接 API」版本。
- PR-5 revert：只是 docs/release，純 metadata，可隨時 revert。

→ 如果 v1.1.0 釋出後線上有問題，**第一招永遠是「config flag 切回
scraper-only mode」**，不是 git revert。Router 必須提供
`cfg.PreferScraper bool`（預設 `false`），讓 ops 可以在 24 小時內把所
有 hybrid 指令切回 Phase 1 行為。詳見 PR-4 spec §"配置選項"。

## 5. 測試策略總覽

詳細請看 `CONTRACT.md`，這裡只列重點：

- **單元測試**：每個新方法都吃 `httptest.NewServer`，handler 內 inspect
  request method/path/headers/body。Multipart body 用
  `multipart.NewReader` parse 回 form fields，再對檔名、欄位名、檔案
  bytes 做斷言。
- **契約測試 (contract)**：每個 API 端點對應一份「real response」
  fixture，存在 `testdata/api/v4/<endpoint>/<case>.json`。同一份
  fixture 同時被 `api/v4` 客戶端的 decoder 與下游 `importer` /
  `scoreboard` 重組邏輯使用，確保「server 變了我會在 client 端就先看
  到 test fail」。
- **整合金測試**：DoD #4 列的 byte-identical 比對 — 同一份輸入 CSV，
  跑舊 scraper 路徑與新 API 路徑，產出 `import-users-teams-out.csv`
  必須相同。實作方式是切 router 用 env var `DJ_TOOL_FORCE_SCRAPER=1`
  跑一次、不切跑一次，diff 兩個輸出檔。
- **不打真實網路**：`go test ./...` 全程不能命中 `test-station.example.org`
  或任何外網。CI 會用 lint rule 抓 `_test.go` 裡的 `http.Get` 沒指向
  `httptest`。

## 6. CI 增量

Phase 1 的 `.github/workflows/ci.yml` 已涵蓋 `go test`、`go vet`、
`golangci-lint`、`gofumpt -l`。Phase 1.5 額外加：

- 一個 `make api-fixture-check` step：對 `testdata/api/v4/` 裡每個
  fixture 跑 `python3 scripts/check_pii.py`（見 `CONTRACT.md`），確保
  CI 不會因為某次合併把未 sanitize 的東西放進去。

## 7. 與 Phase 2 的接軌

Phase 1.5 完成後 `docs/ROADMAP.md` 需要小幅更新：

- Phase 2 §"Web-form differences from v7" 那段保留，但加一行：
  > **Phase 1.5 之後，下列差異中只有 *team delete 流程*（GET row → POST
  > `/jury/teams/{id}/delete`）需要在 `web/v8` 重做；其餘 form-field 差
  > 異已不再被 scraper 路徑使用。**
- Phase 2 「`web.Gateway` interface (Phase 1, used in Phase 2)」這段重
  寫成 5 個方法、註明 Phase 1.5 裁減過。

這個更新動作**寫進 PR-5**。

## 8. 名稱對照（Phase 1 命名 vs Phase 1.5 命名）

| 概念 | 對外（不變） | 內部新名 |
|------|-------------|---------|
| Affiliation / Organization | `model.Affiliation`、CLI flag `--affiliation-*` | `apiv4.Organization`、`apiv4.OrganizationsResource` |
| Team category / Group | `cfg.CategoryID`、CLI flag `--category-id` | `apiv4.Group`、`apiv4.GroupsResource` |
| Contest ID | `cfg.ContestID`（string）、CLI flag `--cid`（int） | `apiv4.Contest.ID`（string，server 給的） |
| Bulk import | (n/a — 之前沒有) | `apiv4.ImportsResource.{Groups,Organizations,Teams,Accounts,Contest,Problem}` |

## 9. 實作風格（給作者快速複習）

照舊（`docs/CONVENTIONS.md`），重點復習：

- 錯誤一律 `fmt.Errorf("doing X: %w", err)`，sentinel error 放在 package
  最上面 `var ErrXxx = errors.New("...")`。
- I/O 第一個參數 `ctx context.Context`。
- `init()` 只能用在 registry wiring（cobra group register 等），其他
  禁止。
- `gci` import grouping：stdlib → third-party → `github.com/ntub/...`。
- 庫不印；CLI 印用 `cmd.OutOrStdout()` / `cmd.ErrOrStderr()`。

## 10. 預期時程

如果作者熟悉 Go + 把 ctx7 用順，**~10 個工作天**完成。新人 +
unfamiliar codebase 估 **~3 週**。每天進度建議：

- D1–D2：PR-0（讀 SPEC、抓 fixture、寫 PII script）
- D3–D5：PR-1（client/api/v4 imports 系列）
- D6–D7：PR-2（scoreboard 切換）
- D8–D9：PR-3（importer create flow）
- D10–D11：PR-4（router 統合 + set-password）
- D12：PR-5（release）

中間任何一天卡住，找 reviewer / advisor 討論「是不是 §2.6 的假設破了 →
要不要把 PR-3 縮成 create-only」。**寧可降低 PR 範圍，也不要在錯的假設
上往下蓋程式碼**。
