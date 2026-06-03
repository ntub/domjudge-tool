# Phase 2 — 執行計畫（PLAN）

> Phase 2 一樣採 RED → GREEN → REFACTOR。和 Phase 1.5 不同的是：P2-0
> 是 hard gate。**沒有 sanitized v8 fixture，就不要寫任何 v8 Go code**。
> 這是避免 selector / decoder 靠猜的唯一辦法。

## 0. 開工前清單

- [ ] 確認分支是 `feat/phase2-domjudge-8.3.1`。
- [ ] `git status --short` 乾淨，或只含本任務相關 Markdown。
- [ ] 讀完 `docs/phase2/README.md`、`SPEC.md`、`CONTRACT.md`。
- [ ] 讀完 ADR 0008 / 0009。
- [ ] 讀完 `docs/adr/0006-api-first-with-scraper-fallback.md` 與
      `docs/adr/0007-scoreboard-cell-format-and-categories.md`。
- [ ] 確認 fixture capture 不會碰真實學生資料；raw capture 只放 `tmp/`
      或 repo 外。

TDD commit triplet 沿用 Phase 1.5：

```text
test(<scope>): red — <desc>
feat(<scope>): green — <desc>
refactor(<scope>): <desc>      # optional
```

## 1. PR 清單（P2-0..P2-5）

| # | 主要交付 | Entry files | Definition of Done |
|---|----------|-------------|--------------------|
| P2-0 | fixtures + drift confirmation | `docs/phase2/CONTRACT.md`, `testdata/api/v4/v8/`, `testdata/web/v8/`, capture scripts / notes | Sanitized v8 API JSON + jury HTML fixture 完成；先擴充 privacy gate 覆蓋 `testdata/web/v8/` `.html` / `.headers` 並 pass；`time_limit` / `api_version` / silent drift 寫入 findings；不含 Go v8 code。 |
| P2-1 | `api/v4` v8 compat | `internal/client/api/v4/`, `internal/model/` | 先寫 v8 fixture decode RED；修 `time_limit` float 與 `api_version` int；v7 fixture 仍綠；不使用 `DisallowUnknownFields`。 |
| P2-2 | `web/v8` scraper | `internal/client/web/gateway.go`, `internal/client/web/errors.go`, `internal/client/web/v8/` | `web/v8.Client` 滿足完整 `web.Gateway`；5 active methods 有 fixture-driven v8 行為；deprecated fallback methods 回傳 `web.ErrUnsupportedOnV8` sentinel；所有 parser test 吃 `testdata/web/v8/`；delete inverted-filter 行為在 v7 保持不變，v8 delete semantics 以 SPEC 鎖定。 |
| P2-3 | version selection + Gateway factory | `internal/client/web/`, `internal/cli/` wiring | Factory 回傳 `web.Gateway`；v7 / v8 都滿足同一介面，不拆 interface；`config.Version=="8.3.1"` 選 v8；7.3.x 選 v7；unsupported version 明確錯誤；不做 auto-detection。 |
| P2-4 | import body v8 compat | `internal/importer/`, `internal/client/api/v4/imports.go` payload layer | v8 organizations import drop `url` / `twitter_hashtag`；groups / teams / accounts body shape 由 fixture 與 httptest 鎖；v7 import fixture 仍綠。 |
| P2-5 | live validation + final docs + release | `docs/phase2/`, `docs/adr/`, `README.md` only if user-visible release notes need it | GET-only validation pass；sanctioned write protocol 若執行則有 sign-off / cleanup record；`just check` + `just privacy-check` pass；ADR 狀態可推 Accepted。 |

## 2. 依賴圖

```text
P2-0  ─┬─→ P2-1  ─┬─→ P2-4  ─┐
       │          │           │
       └─→ P2-2  ─┴─→ P2-3  ─┤
                              └─→ P2-5
```

- P2-0 是所有 code PR 的前置條件。
- P2-1 與 P2-2 可在 P2-0 後平行。
- P2-3 需要 P2-2 的 v8 implementation shape。
- P2-4 需要 P2-1 的 decoder / import compatibility。
- P2-5 必須最後做。

## 3. Fixtures-first hard gate

P2-0 必須先完成下列目錄，才可以開始 P2-1 / P2-2：

```text
testdata/
├── api/
│   └── v4/
│       └── v8/
│           ├── CAPTURE_API_V8.md
│           ├── version.json
│           ├── contests.json
│           ├── contest_<cid>/
│           │   ├── problems.json
│           │   ├── teams.json
│           │   ├── organizations.json
│           │   ├── groups.json
│           │   ├── scoreboard.json
│           │   ├── submissions.json
│           │   └── source_code.json
│           └── imports/
│               └── sanctioned_write_notes.md
└── web/
    └── v8/
        ├── CAPTURE.md
        ├── login.html
        ├── problems.html
        ├── problem_export_response.headers
        ├── users.html
        ├── user_delete_confirm.html
        ├── teams.html
        └── team_delete_confirm.html
```

檔名可依實際 parser 需要微調，但必須滿足：

- API JSON 與 HTML 都是 live response sanitized 後的 fixture。
- 不保存 real hostname / URL / credentials / cookies / CSRF / email / PII。
- 在任何 `testdata/web/v8/` HTML / headers fixture commit 前，P2-0 必須先
  擴充 `just privacy-check` 的 project-specific PII gate，讓它掃描
  `testdata/web/v8/` 的 `.html` 與 `.headers`。可延伸
  `scripts/check_pii_api.py` 接受 web dir 與這些 file types，或新增 sibling
  scanner 並接進 `Justfile` 的 `privacy-check` recipe。
- Web fixture gate 至少要抓 non-`example.org` email、CSRF-looking token、
  CJK names、8+ digit student-id-like numbers、raw hostnames、credentials。
- fixture commit message 或 capture note 寫清楚 sanitization rules 與 PII
  gate 結果。
- v8 Go code 的第一個 RED test 必須引用這些 fixture。

## 4. Per-PR 詳細計畫

### P2-0 — fixtures + drift confirmation

目的：把 live investigation 從 scratch note 轉成可測的 sanitized contract。

RED：

- 新增 contract tests skeleton，先指向尚不存在的 v8 fixture，確認 fail 的
  原因是 fixture missing。

GREEN：

- 擴充 privacy gate：`just privacy-check` 必須同時掃描 API fixture 與
  `testdata/web/v8/` `.html` / `.headers`，偵測 non-`example.org` email、
  CSRF-looking token、CJK names、8+ digit student-id-like numbers、raw
  hostnames、credentials。可延伸 `scripts/check_pii_api.py`，或新增 sibling
  scanner 並接進 `Justfile`。
- 抓 GET-only API fixture：version、contests、problems、teams、orgs、
  groups、scoreboard、submissions、source-code。
- 抓 jury HTML fixture：login、problems list、problem export、users list、
  teams list、delete confirm pages。
- 更新 capture notes，記錄：
  - `time_limit` live value `5.0`。
  - `/api/v4/version` `api_version` 是 int。
  - `source-code` 是 array。
  - problems 仍有 `ordinal` / `test_data_count` / `externalid`。
  - silent field drift 的實測結果。

DoD：

- `testdata/web/v8/` HTML / headers fixture commit 前，web-fixture PII gate
  已存在且會由 `just privacy-check` 執行。
- `just privacy-check` pass。
- repo 內沒有 raw live capture。
- 沒有新增 Go v8 implementation code。

### P2-1 — `api/v4` v8 compat

目的：讓既有 API client 對 v7 / v8 JSON 都 tolerant。

RED：

- `ProblemsResource.List` 對 v8 `time_limit: 5.0` fixture fail。
- `GeneralResource.Version` 對 v8 `{"api_version":4}` fixture fail。

GREEN：

- 為 `time_limit` 做 narrow numeric decoder，接受 int 與 integral float。
- 為 `api_version` 做 string-or-int decoder，回傳既有 string API。
- 補 silent drift fixture tests，確認 missing optional fields 不讓下游 panic。

REFACTOR：

- 避免把所有 model 改成 `map[string]any`。
- 不加 dependency；用 stdlib `encoding/json`。

DoD：

- v7 fixture tests 仍綠。
- v8 fixture tests 綠。
- 沒有 `DisallowUnknownFields`。

### P2-2 — `web/v8` scraper

目的：新增 DOMjudge 8.3.1 HTML scraper，只處理 API 不能做的 active methods。

RED：

- 用 `testdata/web/v8/` 寫 `Login`、`GetProblems`、`Download`、
  `DeleteUsers`、`DeleteTeams` parser / request tests。
- `web/v8.Client` 必須可指派給 `web.Gateway`；若只實作 5 active methods，
  compile 應 fail。
- 對 6 個 deprecated fallback 方法寫 `httptest` assertions，確認
  `GetAffiliations`、`GetAffiliation`、`CreateAffiliation`、
  `CreateTeamAndUser`、`UpdateTeam`、`SetUserPassword` 在 v8 回傳可用
  `errors.Is(err, web.ErrUnsupportedOnV8)` 判斷的錯誤。

GREEN：

- 新增 `internal/client/web/v8` implementation。
- 在 `internal/client/web/errors.go` 新增 `web.ErrUnsupportedOnV8` sentinel，
  風格比照既有 sentinel errors；v8 非 active fallback stubs wrap / return
  此錯誤。
- `Download` 支援 problem ZIP export path，並把 non-2xx 包成可用
  `errors.Is` 判斷的錯誤。
- `DeleteUsers` / `DeleteTeams` selector 以 v8 HTML fixture 鎖定。

DoD：

- v8 client 滿足完整 `web.Gateway`，不需要拆 interface。
- 5 active methods 用 `testdata/web/v8/` fixture-driven tests 鎖住。
- 6 個 deprecated fallback methods 回傳 `web.ErrUnsupportedOnV8`，且有
  `errors.Is` assertions。
- v7 QUIRK tests 不改、不刪。
- 測試只用 `httptest.Server`，不打 live station。

### P2-3 — version selection + Gateway factory

目的：把 version-driven scraper selection 放在 `internal/client/web` 邊界，讓
callers 不知道 v7 / v8 constructor 差異。

RED：

- `config.Version=="8.3.1"` 時 factory 應回傳 v8。
- `config.Version=="7.3.2"` 時 factory 應回傳 v7。
- unsupported version 應回清楚錯誤。

GREEN：

- 新增 `web.NewGateway(...)` 或等價 factory。
- Factory signature / return type 維持 `web.Gateway`；v7 與 v8 concrete
  clients 都滿足同一介面，callers 不需要知道版本差異，也不需要 interface
  split。
- CLI wiring 改用 factory。
- 不做 auto-detection，不呼叫 live `/api/v4/version`。

DoD：

- `domserver.json` schema 不變。
- `config.Default().Version` 仍是 `"7.3.2"`。
- library package 不印；錯誤回傳給 CLI。

### P2-4 — import body v8 compat

目的：讓 `import-users-teams` 在 v8 使用正確 bulk import body。

RED：

- v8 organizations import body 若含 `url` / `twitter_hashtag`，測試 fail。
- groups / teams / accounts JSON shape 不符合 SPEC §4 時 fail。

GREEN：

- 建立 version-aware import payload builder，或在 importer 層選擇 v8-safe DTO。
- v8 organizations body 僅送 `{id,name,formal_name,country}`。
- accounts body 依 P2-0 sanctioned write findings 選 JSON 或 TSV，並以
  `httptest` 鎖 request body。

DoD：

- v7 import tests 仍綠。
- v8 import tests 綠。
- 不改 `import-users-teams.csv` 欄位順序。

### P2-5 — live validation + final docs + release

目的：收尾、驗證、整理 release risk。

Validation：

- GET-only against the live 8.3.1 station：
  `export-users`、`download-scoreboard`、`download-submissions` metadata
  path、`download-problems` list path。
- 如需驗證 write/delete mechanics，只能用 `CONTRACT.md` 的 single sanctioned
  writable contest protocol。

Docs：

- 更新 Phase 2 docs 的實測結果。
- ADR 0008 / 0009 依 review 結論從 `Proposed` 改 `Accepted`。
- README 只在 user-visible release notes 需要時更新。

DoD：

- `just check` pass。
- `just privacy-check` pass。
- release notes 明確列出 DOMjudge 8.3.1 support、version selection、已知
  write/delete live validation scope。

## 5. Single-feature-branch commit strategy

Phase 2 在單一 feature branch `feat/phase2-domjudge-8.3.1` 上推進。每個 P2
PR slice 仍以 Conventional Commit 切清楚：

```text
chore(testdata): capture sanitized domjudge 8.3.1 fixtures
test(api): red — decode v8 time_limit float
feat(api): green — accept cross-version numeric problem limits
test(web): red — parse v8 team delete confirm page
feat(web): green — add v8 gateway active methods
```

不要把 P2-0 fixture、P2-1 decoder、P2-2 scraper 混成一個 commit。若需要
local merge commit，message 要能看出 P2 slice。

## 6. Rollback story

Phase 2 的 rollback 必須可以按 slice 做：

- P2-0 revert：只移除 fixture / notes；不影響 runtime。
- P2-1 revert：回到 v7-only decoder；v8 commands 會失敗，但 v7 不受影響。
- P2-2 revert：移除 `web/v8`；`config.Version=="8.3.1"` 的 web commands 不可用。
- P2-3 revert：factory 回到 v7-only wiring；v8 support 關閉。
- P2-4 revert：v8 import body 回到 v7 shape；必要時暫停 `import-users-teams`
  v8 write path。
- P2-5 revert：docs/release metadata，可單獨退。

Operational fallback：

- 7.3.x server 設定保持原版本值即可走 v7。
- 若 v8 write path 有疑慮，先停止 write commands；不要用 live delete 做
  疑難排解。
- `cfg.PreferScraper` 仍是 Phase 1.5 的 scraper fallback knob，但 Phase 2
  不能把它當成跨版本偵測機制。

## 7. Review checklist

每個 P2 slice review 前都要確認：

1. 有 RED → GREEN 痕跡，或 commit body 解釋不可避免的 fixture-only 例外。
2. 沒有真實 hostname / URL / credentials / cookies / CSRF / email / PII。
3. 沒有新增 dependency。
4. 沒有改 CLI command / flag / output filename。
5. 沒有改 `domserver.json` schema 或 default。
6. 沒有改 Go module directive、`goquery` pin、`cobra` pin。
7. Tests 只用 stdlib `testing` + `httptest`。
8. Library packages 不印 stdout/stderr。
9. v7 delete inverted-filter quirk tests 仍綠。
10. `just privacy-check` 在含 fixture 的 slice 必跑。
