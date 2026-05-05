# PR-1 — `internal/client/api/v4` ImportsResource 與其他 API resource

## Goal

擴充 `internal/client/api/v4`，加入：

1. 一個 multipart helper（`postMultipart`），未來所有 import / upload 端
   點都共用。
2. **`ImportsResource`** — 對應 6 個 bulk import POST 端點。
3. **`OrganizationsResource`** / **`GroupsResource`** / **`ContestsResource`** /
   **`ScoreboardResource`** — 對應目前還在 scraper 拿但其實 API 有
   GET 端點的資源。

PR-1 **不動任何 CLI 行為，也不動 importer / scoreboard 的 use-case**。
這 PR 純粹補完 Layer-2 的 API client，讓 PR-2、PR-3 可以直接拿來用。

## Files created / modified

### 新增

| Path | Role |
|------|------|
| `internal/client/api/v4/multipart.go` | `postMultipart(ctx, path, fields)` — 共用的 multipart/form-data POST 輔助。`fields` 是 `map[string]multipartField`，value 可以是 `multipartFile`（含 reader/filename/content-type）或 `multipartValue`（普通 form field）。boundary 用 `mime/multipart` 預設。 |
| `internal/client/api/v4/multipart_test.go` | 對 helper 跑：(a) 一個 file field 的 boundary 與 Content-Disposition 正確、(b) 多個 file/value field 一起送、(c) ctx 取消傳遞、(d) 5xx 回 HTTPError |
| `internal/client/api/v4/imports.go` | `ImportsResource` 與 6 個方法。簽名表見下 |
| `internal/client/api/v4/imports_test.go` | 6 個方法每個一個 happy path + 至少一個 error path 的測試 |
| `internal/client/api/v4/organizations.go` | `OrganizationsResource` — `List(ctx, cid)`、`Get(ctx, cid, id)`、`FindByName(ctx, cid, name)` |
| `internal/client/api/v4/organizations_test.go` | List / Get / FindByName 的 happy 與 not-found |
| `internal/client/api/v4/groups.go` | `GroupsResource` — `List(ctx, cid)`、`Get(ctx, cid, id)` |
| `internal/client/api/v4/groups_test.go` | 同上 |
| `internal/client/api/v4/contests.go` | `ContestsResource` — `List(ctx)`、`Get(ctx, cid)`，`Resolve(ctx, hint string) (string, error)`（hint 可以是 numeric id 或 shortname，回傳 numeric id） |
| `internal/client/api/v4/contests_test.go` | 上述三方法 |
| `internal/client/api/v4/scoreboard.go` | `ScoreboardResource` — `Get(ctx, cid, opts ScoreboardListOpts)` |
| `internal/client/api/v4/scoreboard_test.go` | Decoder 對 fixture 的契約測試（CONTRACT.md §4.1） |
| `internal/model/scoreboard_api.go` | `ScoreboardSnapshot`、`ScoreboardRow`、`ScoreboardProblem` 三個 DTO（API JSON shape；不要與既有 HTML-parser scoreboard 結構共用） |
| `internal/model/contest.go` | `Contest`、`ContestState` |
| `internal/model/organization.go` | `Organization` |
| `internal/model/group.go` | `Group` |

### 修改

| Path | 改動 |
|------|------|
| `internal/client/api/v4/client.go` | 在 `Client` struct 加 `Imports *ImportsResource`、`Organizations *OrganizationsResource`、`Groups *GroupsResource`、`Contests *ContestsResource`、`Scoreboard *ScoreboardResource` 欄位；在 `New(...)` 內 wiring |
| `internal/client/api/v4/errors.go` | 新增 `ErrImportRejected`（server 回 4xx 但帶 JSON 錯誤陣列） |
| `internal/client/httpx/httpx.go` | 視情況加 `POST(ctx, url, ct string, body io.Reader)` —目前只有 GET / GETStream，PR-1 補一個泛用 POST 出來 |
| `internal/client/httpx/httpx_test.go` | POST 行為（Basic Auth header、Content-Type、ctx cancel） |

## API 方法簽名

```go
// internal/client/api/v4/imports.go

// ImportsResource wraps the bulk import endpoints under /api/v4.
//
// All methods return the server's parsed response (typically a list of
// IDs created/updated, or a 4xx ErrImportRejected with details).
//
// Format references:
//  - groups       : groups.tsv (CCS) or array<Group> json
//  - organizations: array<Organization> json (no TSV variant on v7.3)
//  - teams        : teams2.tsv (CCS) or array<TeamImport> json
//  - accounts     : accounts.tsv (CCS, no JSON variant on v7.3)
//  - contest      : combined contest.yaml + problemset.yaml
//  - problem      : ICPC problem ZIP
type ImportsResource struct{ c *Client }

func (r *ImportsResource) GroupsTSV(ctx context.Context, tsv io.Reader) ([]string, error)
func (r *ImportsResource) GroupsJSON(ctx context.Context, body []model.Group) ([]string, error)

func (r *ImportsResource) Organizations(ctx context.Context, body []model.Organization) ([]string, error)

func (r *ImportsResource) TeamsTSV(ctx context.Context, tsv io.Reader) ([]string, error)
func (r *ImportsResource) TeamsJSON(ctx context.Context, body []model.TeamImport) ([]string, error)

func (r *ImportsResource) Accounts(ctx context.Context, tsv io.Reader) ([]string, error)

func (r *ImportsResource) Contest(ctx context.Context, yaml io.Reader) (cid string, err error)
func (r *ImportsResource) Problem(ctx context.Context, cid, problemID string, zip io.Reader) (newID string, err error)
```

> `model.TeamImport` 不是 `model.Team` —前者是 import payload 用的精
> 簡 shape（id、icpc_id、group_ids、name、display_name、organization_id），
> 後者保留現有讀取結構。Phase 1.5 不為了「讓兩邊看起來一樣」而合併。

## Tests written first (RED)

按 `docs/PHASE1.md` §"TDD discipline" 的規範，**先寫紅再寫綠**。每個方
法配一個對應 `_test.go`：

| Test | RED commit message |
|------|-------------------|
| `TestPostMultipart_OneFile_BoundaryAndContentDisposition` | `test(api/v4): red — multipart one file boundary` |
| `TestPostMultipart_MultiFieldsAndFile` | `test(api/v4): red — multipart multi field` |
| `TestPostMultipart_ContextCanceled` | `test(api/v4): red — multipart ctx cancel` |
| `TestImports_GroupsTSV_HappyPath` | `test(api/v4): red — imports groups tsv` |
| `TestImports_GroupsTSV_RejectedReturnsErrImportRejected` | |
| `TestImports_OrganizationsJSON_HappyPath` | |
| `TestImports_TeamsTSV_HappyPath` | |
| `TestImports_TeamsJSON_HappyPath` | |
| `TestImports_Accounts_HappyPath` | |
| `TestImports_Contest_ReturnsNewCID` | |
| `TestImports_Problem_PassesContestAndProblemFields` | 確認 form field `problem=<id>` 與 `zip[]=<file>` 都送出 |
| `TestOrganizationsResource_List_DecodesContractFixture` | 用 `testdata/api/v4/contest_2/organizations.json` |
| `TestOrganizationsResource_FindByName_ReturnsNilOnMiss` | matches Python Optional |
| `TestGroupsResource_List_DecodesContractFixture` | |
| `TestContestsResource_Resolve_ByNumericID_PassesThrough` | |
| `TestContestsResource_Resolve_ByShortname_ResolvesToID` | |
| `TestScoreboardResource_Get_DecodesContractFixture_NoFreeze` | |
| `TestScoreboardResource_Get_DecodesContractFixture_WithFreeze` | |

每個 test 寫法都是 `httptest.NewServer` + handler 檢查 method/path/headers/
body，再做 JSON decode 比對，**完全不打外網**。

## 邊界情境

- multipart helper：`io.Reader` 可以是 `bytes.Reader`、`*os.File`、
  `strings.Reader`，PR-1 都需要工作。`postMultipart` 內部不做 `io.ReadAll`
  buffering（保留串流），但 boundary 計算需要事先決定 → 用 `multipart.Writer`
  + `io.Pipe` 模式。**注意 ctx 取消時要關掉 pipe 不漏 goroutine**，這個
  在 `TestPostMultipart_ContextCanceled` 會驗。
- TSV reader 不能在 helper 內被覆寫（例如 strip BOM）。BOM 處理是匯入
  人的責任 — 但 PR-3 寫 importer 端時會用到 `internal/util/csvio`，這
  個 PR 不碰。
- `Imports.Problem(zip[])` 要送出 array form key — Go `mime/multipart`
  寫成 `CreateFormFile("zip[]", ...)` 即可，DOMjudge 會解。
- `ScoreboardResource.Get` 必須先把 cid 從 int → string 轉好（CLI flag
  仍是 int）。轉換在 `cli/scoreboard/download.go`（PR-2 才動），這個
  PR 的 API resource 一律收 string。
- `ContestsResource.Resolve(hint)`：
  - `hint == ""` → 回 `ErrInvalidArgument`。
  - `hint` 為純 digit → 直接回；不打 server。
  - 否則 → `List(ctx)`、找 shortname == hint 的，回它的 `id`。找不到
    回 `ErrNotFound`。
- 401/403：`HTTPError` 的 `Is(ErrUnauthorized)` 必須 true（已存在
  `errors.go` 機制，沿用）。
- 500：`Is(ErrServerError)` true。

## Definition of done

- [ ] 上表所有檔案存在、`gci` import 排序通過。
- [ ] 上表所有 RED→GREEN commit 在 `git log` 看到（每方法至少一對）。
- [ ] `go test ./internal/client/api/v4/... -cover` 報 ≥ 80%。
- [ ] `go test ./internal/client/httpx/... -cover` 不退步。
- [ ] `golangci-lint run` 綠（含 `gofumpt`）。
- [ ] **無 CLI 行為改動** — `bin/domjudge-tool` 跑既有 e2e（Phase 1
      原有的 `gen-config` / `import-users-teams-example` 對 golden 比對）
      仍綠。
- [ ] commit message：
      `feat(api/v4): green — imports + organizations + groups + contests + scoreboard resources`

## Cross-references

- SPEC §2.2 `POST` 端點清單
- SPEC §5 內部架構增量
- CONTRACT §4.1, §4.2 契約測試範本
