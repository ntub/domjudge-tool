# PR-6 — Email template tooling：scaffold / validate / render-user-accounts

## Goal

新增三個 CLI 子指令，補齊 email template 工作流的離線操作能力：

1. **`email-template-example`** — 寫出含全部 6 個白名單佔位符的範本目錄
   （對稱 `import-users-teams-example`）。
2. **`email-template-validate`** — 驗證既有 template 目錄合法性
   （走 `email.LoadTemplate` + `(*Template).Validate()`）。
3. **`render-user-accounts`** — 對 CSV 每列渲染後寫成 `.eml` 到磁碟，
   不寄信（對稱 `send-user-accounts`）。

Python 版套件雖捆綁 `domjudge_tool_cli/templates/email/` 三份檔案，但從
未有 CLI 指令把它們寫到使用者目錄（只有 `emails send-user-accounts` 在
執行時讀取）。新版 Go 同樣缺，這是**新增功能**，不是 Python parity 補位。

## 為什麼三個指令一個 PR

- 三個指令共用同一份 `internal/email` Layer-3 helper（`BuildEML`）、同一
  批 testdata fixture、同一個 `internal/cli/emails/` register。
- 拆三個 PR 反而拉高 review 成本，且 scaffold → validate → render 是
  同一 UX 流程，文件與測試同批處理更完整。
- release 動作另外切到 PR-7（與 PR-4→PR-5 同模式）。

## Files created / modified

### 新增

| Path | Role |
|------|------|
| `internal/email/scaffold.go` | `WriteTemplateScaffold(dir string, force bool) error` — 在 `dir` 下建立 `subject.txt`、`body.txt`、`body.html`。Go 版用含全部 6 個白名單佔位符的範例文字（scaffold 出來的東西必須能直接通過 `Validate()`，first-run 即可 round-trip）。`force=false` + dir 已存在任一目標檔 → 回 `ErrTemplateDirExists`；`force=true` 只覆寫 `subject.txt`/`body.txt`/`body.html` 三檔，**絕不刪除 dir 內任何其他檔案**（如 `body.html.bak`、`notes.md`）。 |
| `internal/email/scaffold_test.go` | (a) byte-identical golden 比對 (b) `force=false` + 既有檔回 `ErrTemplateDirExists` (c) `force=true` 同名覆寫 (d) `force=true` 不刪 `body.html.bak`（行為測試）(e) `LoadTemplate(dir).Validate()` 對 scaffold 輸出回 nil（round-trip）。 |
| `internal/email/render.go` | 抽 helper：`BuildEML(tmpl *Template, user model.CreateUser, cfg *config.DomServerClient, opts SendOptions) ([]byte, string, error)` 回 `(rfc5322Bytes, recipient, err)`。內部呼叫既有 `templateVars`、`Render`、`recipientFor`、`buildMessage` —**全部 reuse 既有 `internal/email/send.go:67-160` 邏輯，零複製貼上**。 |
| `internal/email/render_test.go` | text/html 兩種 body、空 email 走 fallback domain、subject Q-encoding、CRLF、base64 wrap 76-char 行長皆 lock。 |
| `internal/cli/emails/template_example.go` | `NewTemplateExampleCmd()` — `email-template-example`，flag `--output / -o` 預設 `./email-template`、`--force / -f`。 |
| `internal/cli/emails/template_example_test.go` | golden、`--force` 行為、`GroupID == "emails"`。 |
| `internal/cli/emails/template_validate.go` | `NewTemplateValidateCmd()` — `email-template-validate <dir>`，`email.LoadTemplate` + `(*Template).Validate()`，OK 印 `template OK: <dir>`，錯走 `RunE` return（`SilenceUsage:true SilenceErrors:true`）。 |
| `internal/cli/emails/template_validate_test.go` | (a) `testdata/email/valid` 通過 (b) `testdata/email/no_subject` 失敗 (c) `testdata/email/unknown_var/` 含 `$nonexistent` 失敗 (d) 不存在的 dir 失敗。 |
| `internal/cli/emails/render_user_accounts.go` | `NewRenderUserAccountsCmd()` — `render-user-accounts <csv> <template-dir>`，flag `--output-dir / -d` 預設 `./rendered-emails`、`--from-email` 預設 `noreply@localhost`（對齊 `send-user-accounts`）、`--force`。保留 `PreRunE: cli.LoadConfigPreRunE`（`templateVars()` 需要 `cfg.Host` 餵 `$server_host`，見 `internal/email/send.go:74`）。 |
| `internal/cli/emails/render_user_accounts_test.go` | (a) 對 `testdata/email/valid` + 兩列 CSV 產出兩個 `.eml` (b) 用 `net/mail.ReadMessage` 讀回每封，header & decoded body 與預期相符 (c) `--force` 行為 (d) recipient-fallback `<username>@<from-domain>` (e) golden byte-identical（靠 `nowFn` / `randRead` 注入，見§邊界情境）。 |
| `testdata/email/scaffold_golden/subject.txt` | scaffold 輸出 golden（subject）。 |
| `testdata/email/scaffold_golden/body.txt` | scaffold 輸出 golden（text body）。 |
| `testdata/email/scaffold_golden/body.html` | scaffold 輸出 golden（HTML body）。 |
| `testdata/email/unknown_var/subject.txt` | validate 負向案例：含 `$nonexistent`。 |
| `testdata/email/unknown_var/body.txt` | validate 負向案例：body（無 HTML，逼 LoadTemplate 走 txt 路徑）。 |
| `testdata/email/rendered/<username>.eml` | render-user-accounts 輸出 golden（針對 `testdata/email/valid` template + 一列固定 CSV，配合測試注入的 fixed clock & randRead）。 |

### 修改

| Path | 改動 |
|------|------|
| `internal/email/send.go` | (1) 把 `time.Now`（`send.go:125,159`）與 `crypto/rand.Read`（`send.go:156`）抽成 package-level vars `nowFn = time.Now`、`randRead = rand.Read`，testfile 用 `t.Cleanup` 暫時覆寫，拿到決定論的 `Date:` 與 `Message-ID:`。(2) **不改 export 任何 unexported helper**（`templateVars`/`recipientFor`/`domainFromEmail`/`buildMessage`）— `BuildEML` 走 same package 直接吃，外部 CLI 只認 `BuildEML`。(3) `SendUserAccounts` 內部從重複邏輯改成呼叫 `BuildEML`（移除 ~10 行重複），保證 send / render 走同一條 RFC 5322 路徑。 |
| `internal/email/errors.go`（不存在則新增） | 新增 sentinel `ErrTemplateDirExists`。 |
| `internal/cli/emails/register.go` | 在 `register(root)` 加 3 個新 cmd：`NewTemplateExampleCmd()`、`NewTemplateValidateCmd()`、`NewRenderUserAccountsCmd(DefaultDeps())`。 |

## CLI 簽名與行為

```
domjudge-tool email-template-example [-o <dir>] [--force]
  └─ 預設 dir = ./email-template
  └─ dir 不存在 → mkdir -p
  └─ subject.txt/body.txt/body.html 任一存在且 --force=false → 回 ErrTemplateDirExists
  └─ --force=true 只覆寫 3 檔，不動 dir 內其他任何檔
  └─ scaffold 出來的 3 檔含全部 6 個白名單佔位符，可直接通過 validate
  └─ exit 0；stdout 印 dir 路徑

domjudge-tool email-template-validate <dir>
  └─ 走 email.LoadTemplate(dir) + (*Template).Validate()
  └─ OK 印 "template OK: <dir>"
  └─ 任何錯誤經 RunE return → exit 1

domjudge-tool render-user-accounts <csv> <template-dir>
  [-d <output-dir>] [--from-email <addr>] [--force]
  └─ 預設 output-dir = ./rendered-emails、from-email = noreply@localhost
  └─ 與 send-user-accounts 行為對稱：CSV BOM-tolerant、template HTML-prefer
  └─ 保留 PreRunE: cli.LoadConfigPreRunE（templateVars 需 cfg.Host）
  └─ 每列產出 <pathx.SafeFilename(username)>.eml；空 username → unknown.eml
  └─ 同名 .eml 已存在且 --force=false → 拒絕（fail-fast）
  └─ 失敗 abort-on-first-error（同 send-user-accounts 語意）
  └─ stdout 印 "Rendered N emails to <dir>."
```

## API 方法簽名

```go
// internal/email/render.go

// BuildEML assembles a complete RFC 5322 message for user, using tmpl and cfg.
// Returns (rfc5322Bytes, recipient, err). Reuses templateVars, Render,
// recipientFor, and buildMessage from send.go; no logic is duplicated.
func BuildEML(
    tmpl *Template,
    user model.CreateUser,
    cfg *config.DomServerClient,
    opts SendOptions,
) ([]byte, string, error)

// internal/email/scaffold.go

// WriteTemplateScaffold writes subject.txt, body.txt, body.html into dir.
// All three files contain all six fixed placeholders so they pass Validate
// immediately. If force is false and any of the three target files already
// exists, returns ErrTemplateDirExists without writing anything.
// If force is true, only the three named files are overwritten; no other
// files in dir are removed or modified.
func WriteTemplateScaffold(dir string, force bool) error
```

## Tests written first (RED)

按 `docs/PHASE1.md` §"TDD discipline" 規範，先寫紅再寫綠。

| Test | RED commit message |
|------|--------------------|
| `TestWriteTemplateScaffold_GoldenOutput` | `test(email): red — scaffold golden output` |
| `TestWriteTemplateScaffold_ExistsNonForce_ReturnsErr` | `test(email): red — scaffold force=false rejects existing` |
| `TestWriteTemplateScaffold_ForceOverwrites` | `test(email): red — scaffold force=true overwrites` |
| `TestWriteTemplateScaffold_ForcePreservesOtherFiles` | `test(email): red — scaffold force=true keeps bak file` |
| `TestWriteTemplateScaffold_RoundTrip_ValidateReturnsNil` | `test(email): red — scaffold round-trip validate` |
| `TestBuildEML_TextBody_RFC5322Structure` | `test(email): red — BuildEML text body structure` |
| `TestBuildEML_HTMLBody_ContentType` | `test(email): red — BuildEML html content-type` |
| `TestBuildEML_EmptyEmail_FallbackDomain` | `test(email): red — BuildEML recipient fallback` |
| `TestBuildEML_SubjectQEncoding` | `test(email): red — BuildEML subject Q-encoding` |
| `TestBuildEML_Base64Wrap76Chars` | `test(email): red — BuildEML base64 line wrap` |
| `TestTemplateExampleCmd_GoldenDir` | `test(emails): red — email-template-example golden` |
| `TestTemplateExampleCmd_ForceFlag` | `test(emails): red — email-template-example --force` |
| `TestTemplateExampleCmd_GroupID` | `test(emails): red — email-template-example group id` |
| `TestTemplateValidateCmd_ValidDir_Exit0` | `test(emails): red — email-template-validate valid dir` |
| `TestTemplateValidateCmd_NoSubject_Exit1` | `test(emails): red — email-template-validate no subject` |
| `TestTemplateValidateCmd_UnknownVar_Exit1` | `test(emails): red — email-template-validate unknown var` |
| `TestTemplateValidateCmd_MissingDir_Exit1` | `test(emails): red — email-template-validate missing dir` |
| `TestRenderUserAccountsCmd_TwoRows_TwoEMLFiles` | `test(emails): red — render-user-accounts two rows` |
| `TestRenderUserAccountsCmd_EMLParsable_ByNetMail` | `test(emails): red — render-user-accounts eml parsable` |
| `TestRenderUserAccountsCmd_ForceFlag` | `test(emails): red — render-user-accounts --force` |
| `TestRenderUserAccountsCmd_RecipientFallback` | `test(emails): red — render-user-accounts recipient fallback` |
| `TestRenderUserAccountsCmd_GoldenByteIdentical` | `test(emails): red — render-user-accounts golden` |

## 邊界情境

- **決定論（#1 footgun）**：`buildMessage` 的 `time.Now()`
  （`internal/email/send.go:125,159`）與 `crypto/rand.Read`
  （`internal/email/send.go:156`）讓 golden byte-比對失敗。**解法**：
  在 `send.go` 抽兩個 package-level 變數
  ```go
  var nowFn = time.Now
  var randRead = rand.Read
  ```
  測試用 `t.Cleanup(func(){ nowFn = time.Now })` 暫時覆寫。**這是 PR-6
  必做的小重構，不是 nice-to-have**。

- **Filename safety**：使用既有 `internal/util/pathx.SafeFilename`
  （`internal/util/pathx/pathx.go:39`，無需新增）。空字串 → `"unknown"`。
  同批 CSV 兩列 sanitize 後撞名 → 第二筆遇到既有檔，由 `--force` 規則
  與 abort-on-first-error 共同接住（PR-6 不增加 dedupe suffix 邏輯）。

- **CRLF**：`buildMessage` 已 emit CRLF（`send.go:122-130`），不再額外
  處理。

- **Subject Q-encoding**：直接走既有 `Render()` 的
  `mime.QEncoding.Encode("UTF-8", ...)` （`template.go:107`）。

- **Content-Type / charset**：`buildMessage` 已分流（`send.go:113-116`）。

- **From / To recipient fallback**：reuse `recipientFor` /
  `domainFromEmail`（`send.go:88-105`）。

- **Empty CSV**：`Rendered 0 emails to ...`，exit 0（與 `send-user-
  accounts` 0 列同樣成功）。

- **scaffold force semantics**：`--force` 只清三個固定檔名，不 `rm -rf
  <dir>`，必須 cover 行為測試（含旁邊放 `body.html.bak` 驗證不動）。

- **validate fixedVars**：與 `internal/email/template.go:64-67` 完全一致，
  PR-6 不擴張白名單。

- **render 仍需 cfg**：`render-user-accounts` 即使 offline 仍需
  `domserver.json` 載入；無 `domserver.json` 時錯誤訊息對齊
  `send-user-accounts` 既有行為。

## Definition of done

- [ ] 上表所有檔案存在、`gci` import 排序通過。
- [ ] 全部 RED→GREEN commit 看得到（每指令至少一對）。
- [ ] `go test ./internal/email/... ./internal/cli/emails/... -cover` ≥ 80%。
- [ ] `golangci-lint run` / `just check` 綠。
- [ ] **既有 e2e 不退步**：`gen-config`、`import-users-teams-example`、
      `send-user-accounts`（接 mock SMTP）golden 與 v1.1.1 byte-identical。
- [ ] PR commit message：
      `feat(emails): green — email-template-example + validate + render-user-accounts`
- [ ] **CHANGELOG / `git tag` / `goreleaser snapshot` 不在 PR-6**，由 PR-7 處理。

## Cross-references

- `docs/phase1/PR-5-email.md` — `internal/email` 既有實作的源頭
- `internal/email/template.go:60-95` — `fixedVars` 白名單與 `collectUnknown`
  即 validate 的核心邏輯
- `internal/email/template.go:64-67` — `fixedVars` 定義（6 個 key）
- `internal/email/send.go:67-160` — `templateVars`、`recipientFor`、
  `domainFromEmail`、`buildMessage` 全 reuse；新 `BuildEML` 是它們的薄組合
- `internal/email/send.go:74` — `cfg.Host` 作為 `$server_host`，render 仍需載入 cfg
- `internal/email/send.go:88-105` — `recipientFor` / `domainFromEmail`
- `internal/email/send.go:112-160` — `buildMessage` / `wrapBase64` / `makeMessageID`
- `internal/email/send.go:125,156,159` — non-deterministic spots（`nowFn` / `randRead` seam）
- `internal/cli/users/import_users_teams_example.go` — scaffold 指令的
  flat-flag-pattern 範本
- `internal/util/pathx/pathx.go:39` — `SafeFilename` 已存在，直接吃
- `docs/phase1.5/PR-1-imports-scaffold.md` — PR 文件章節格式範本
- `docs/phase1.5/PR-5-release.md` — PR-7 的模板
- 舊 Python `domjudge_tool_cli/commands/emails/__init__.py:13-57` —
  template-driven email 行為對照源頭（已知差異：Python 直接 SMTP、無
  render-to-disk；Go 加碼）
