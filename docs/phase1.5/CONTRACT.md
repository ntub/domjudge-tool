# Phase 1.5 — 契約測試與 fixture 規範（CONTRACT）

> 這份文件平行於 `testdata/web/v7/CAPTURE.md`。CAPTURE.md 規範如何安全
> 地把 HTML scraper 的 fixture 從 production 抓進 repo；這份文件規範
> API JSON / multipart 端點的對應做法。**任何提交到 `testdata/api/v4/`
> 的檔案都必須遵守這裡的規則**。

## 1. 為什麼需要契約測試

API 客戶端常見的測試錯誤：

- 用一份「我以為的」JSON 測 decoder，跑得很開心。實際 server 在某個
  欄位回 `null` 而不是 `""`，client 上線就炸。
- 直接打 mock，從來沒看過真實 response，欄位名拼錯永遠看不出來。

契約測試（contract test）是把 **真實 server 回傳的 response 凍結成
fixture**，讓 client 的 decoder + 下游邏輯一起對著它跑。Server 改了
schema，fixture 重抓，diff 一看就知道哪個欄位變了。Phase 1.5 的整個
測試策略建立在這上面。

## 2. fixture 目錄規格

```
testdata/
├── api/
│   └── v4/
│       ├── CAPTURE_API.md         # 抓取程序 + sanitization rule list
│       ├── version.json
│       ├── user.json
│       ├── users_list.json
│       ├── team.json
│       ├── teams_list.json
│       ├── problem.json
│       ├── problems_list.json
│       ├── submission.json
│       ├── submissions_list.json
│       ├── submission_files.zip
│       ├── source_code.json
│       ├── judgement.json
│       ├── judgements_list.json
│       ├── judgement_type.json
│       └── judgement_types_list.json
```

## 3. 抓取程序（PR-0 必跑一次）

### 3.1 環境準備

**只能對「乾淨重置過的、無真實使用者資料」的 DOMjudge 7.3.x instance
跑**。建議流程：

```bash
# Option A: 跑官方提供的 docker-compose demo
git clone --depth=1 -b 7.3.4 https://github.com/DOMjudge/domjudge.git /tmp/dj-demo
cd /tmp/dj-demo
docker compose -f docker/docker-compose-demo.yml up -d
# 等 ~30s 後在 localhost:12345 拿到一個有 demo contest + admin/admin 的乾淨站台

# Option B: 使用內部 throwaway test station（例如 https://test-station.example.org）
# 只能放 demo / synthetic data；不要對 production contest server 抓 fixture。
```

設定環境變數（**永遠不要 commit `.env`**）：

```bash
export DJ_BASE="http://localhost:12345"
export DJ_USER="admin"
export DJ_PASS="<從 demo 容器 stdout 抓 / 自設>"
```

### 3.2 抓取腳本（放在 `scripts/capture_api_fixtures.sh`）

```bash
#!/usr/bin/env bash
set -euo pipefail
RAW="tmp/raw_api_v4"
mkdir -p "$RAW"

# auth 寫進 ~/.netrc，不要塞進 history 或 commit
trap 'rm -f "$HOME/.netrc.dj.bak"' EXIT
[ -f "$HOME/.netrc" ] && cp "$HOME/.netrc" "$HOME/.netrc.dj.bak"
cat > "$HOME/.netrc" <<EOF
machine $(echo "$DJ_BASE" | awk -F/ '{print $3}' | cut -d: -f1)
  login $DJ_USER
  password $DJ_PASS
EOF
chmod 600 "$HOME/.netrc"

curl_jq() { curl -s --netrc "$1" | jq . > "$2"; }
curl_bin() { curl -s --netrc "$1" -o "$2"; }

CID=2  # demo contest

curl_jq "$DJ_BASE/api/v4/version"                "$RAW/version.json"
curl_jq "$DJ_BASE/api/v4/info"                   "$RAW/info.json"
curl_jq "$DJ_BASE/api/v4/user"                   "$RAW/user.json"
curl_jq "$DJ_BASE/api/v4/users"                  "$RAW/users_list_default.json"
curl_jq "$DJ_BASE/api/v4/users?team_id=2"        "$RAW/users_list_with_team_id.json"
curl_jq "$DJ_BASE/api/v4/users/1"                "$RAW/users_single_admin.json"
curl_jq "$DJ_BASE/api/v4/contests"               "$RAW/contests_list.json"
mkdir -p "$RAW/contest_$CID"
for ep in teams organizations groups problems scoreboard submissions; do
  curl_jq "$DJ_BASE/api/v4/contests/$CID/$ep"    "$RAW/contest_$CID/$ep.json"
done
# binary stream
curl_bin "$DJ_BASE/api/v4/contests/$CID/submissions/1/files"  "$RAW/contest_$CID/submission_1_files.zip" || true

# restore netrc
[ -f "$HOME/.netrc.dj.bak" ] && mv "$HOME/.netrc.dj.bak" "$HOME/.netrc" || rm -f "$HOME/.netrc"
```

執行 → 你應該得到一份乾淨的 `tmp/raw_api_v4/`。

### 3.3 idempotency 三項測試（補做 SPEC §2.6）

**只能對 demo 站跑**。這些會 mutate state；務必在 PR-0 已新增 fixture
之後才動手，且把每個指令的 stdout 也存成 fixture：

```bash
# Test 1: accounts upsert
cat > /tmp/acc.tsv <<'TSV'
accounts	1
team	Demo Team	demoteam	pw1
TSV
curl --netrc -F tsv=@/tmp/acc.tsv "$DJ_BASE/api/v4/users/accounts" \
  > "$RAW/imports/accounts_post_first.json"
# verify
curl --netrc "$DJ_BASE/api/v4/users" | jq '.[] | select(.username=="demoteam")' \
  > "$RAW/imports/accounts_post_first_verify.json"

# 重送相同 username + 不同 password
sed -i.bak 's/pw1/pw2/' /tmp/acc.tsv
curl --netrc -F tsv=@/tmp/acc.tsv "$DJ_BASE/api/v4/users/accounts" \
  > "$RAW/imports/accounts_post_second.json"
curl --netrc "$DJ_BASE/api/v4/users" | jq '.[] | select(.username=="demoteam")' \
  > "$RAW/imports/accounts_post_second_verify.json"
```

從兩份 verify 對比：
- 如果 user_id 不變 + last_login_time 維持但能用新 password 登入，就是
  **upsert（最理想）**。
- 如果跑出第二筆 user_id 的 demoteam（duplicate），則 **API 不可用做
  update**，PR-3 的 update flow 必須走 scraper。
- 如果第二次 POST 回 400 / "already exists"，則 **API 是 strict-create**，
  update flow 也走 scraper。

**把實測結果寫進 `CAPTURE_API.md`**，下游 PR 才知道路口怎麼走。

### 3.4 sanitization rule list

對每個 fixture 跑下列替換（用 Python 寫成 `scripts/sanitize_api.py`，
參考 `testdata/web/v7/CAPTURE.md` 的 15 條規則改寫成 JSON-aware）：

| # | 規則 | 範例（before → after） |
|---|------|----------------------|
| 1 | FQDN 替換成 `example.org` | `test-station.example.org` → `example.org` |
| 2 | IPv4 替換成 RFC 5737 TEST-NET-1 | `203.0.113.10` → `192.0.2.1` |
| 3 | IPv6 替換成 `2001:db8::1` | `2001:0:fe00::123` → `2001:db8::1` |
| 4 | Email 統一成 `user{n}@example.org` | `chen@source.example.edu` → `user1@example.org` |
| 5 | CJK 字元（人名）統一成 `Sample###` | `陳宥辰` → `Sample123` |
| 6 | School-specific course code / cohort token → `course###` | `raw-course-a` → `course001` |
| 7 | Student id → `student###` | `raw-student-id` → `student001` |
| 8 | session token / CSRF token → `TEST_CSRF_TOKEN` | `_csrf_token: 9KZ3...` → `_csrf_token: TEST_CSRF_TOKEN` |
| 9 | 真實聯絡電話 → `+1-555-0100` | |
| 10 | 真實 affiliation 名（school domain / university name） → `Sample University` | |
| 11 | last_login_time / first_login_time 全部改 `2026-01-01T00:00:00+00:00` | |
| 12 | 真實密碼 hash / API key（如有） → `REDACTED` | |
| 13 | 真實 contest title / shortname → `ContestX` | |
| 14 | last_ip / ip → `192.0.2.1` | |
| 15 | URLs 中保留路徑、host 替換 | `http://test-station.example.org/api/...` → `http://example.org/api/...` |

### 3.5 PII 驗證（gate）

`scripts/check_pii_api.py` 對 sanitized fixture 跑 8 條 grep gate，**必
須全 0 命中** 才能 commit：

1. raw hostname / maintainer / school-domain tokens
2. `\b\d{8}\b`（學號 leak）
3. `[\p{Han}]+`（CJK，sanitization 漏掉的）
4. 私有 IP `(?:10|172\.(1[6-9]|2\d|3[0-1])|192\.168)\.`
5. `[a-zA-Z0-9._%+-]+@(?!example\.org)`（非 example.org email）
6. `(?i)bearer\s+[A-Za-z0-9_-]{20,}`
7. `(?i)password\s*[:=]\s*["']?[^"' \n]{6,}` 但允許 `REDACTED` / `TEST_*`
8. CSRF 樣式 `_csrf_token:\s*[A-Za-z0-9_-]{8,}` 但允許 `TEST_CSRF_TOKEN`

→ 任何 1 命中、`make api-fixture-check` 紅 → CI 紅。

### 3.6 commit checklist

每次新增 fixture 都在 commit message 列：

```
chore(testdata/api/v4): capture <ENDPOINT> from demo station

Source: <DOMjudge demo, version 7.3.4>
Sanitization: applied 15 rules from docs/phase1.5/CONTRACT.md §3.4
PII gates: 8/8 pass
```

把 `tmp/raw_api_v4/` 加進 `.gitignore`（Phase 1 的 `tmp/` 已經有了，確
認還在）。

## 4. 客戶端契約測試怎麼寫

### 4.1 GET 端點 — JSON decoder + downstream 一次測

```go
// internal/client/api/v4/scoreboard_test.go
func TestScoreboardResource_Get_DecodesContractFixture(t *testing.T) {
    raw := mustReadFixture(t, "../../../../testdata/api/v4/contest_2/scoreboard.json")
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet || r.URL.Path != "/api/v4/contests/2/scoreboard" {
            t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write(raw)
    }))
    defer srv.Close()

    c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "x"})
    sb, err := c.Scoreboard.Get(t.Context(), "2", apiv4.ScoreboardListOpts{})
    if err != nil {
        t.Fatalf("unexpected err: %v", err)
    }
    if sb.EventID == "" {
        t.Errorf("event_id should not be empty in fixture")
    }
    if len(sb.Rows) == 0 {
        t.Errorf("rows should not be empty in fixture")
    }
}
```

### 4.2 POST multipart 端點 — Inspect request body

```go
// internal/client/api/v4/imports_test.go
func TestImports_Accounts_SendsExpectedTSV(t *testing.T) {
    var got struct {
        method      string
        contentType string
        formFile    string
        formField   string
    }
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        got.method = r.Method
        got.contentType = r.Header.Get("Content-Type")
        require.NoError(t, r.ParseMultipartForm(1<<20))
        for name := range r.MultipartForm.File {
            got.formField = name // expect "tsv"
            f, _, err := r.FormFile(name)
            require.NoError(t, err)
            b, _ := io.ReadAll(f)
            got.formFile = string(b)
        }
        // mimic real DOMjudge response
        _, _ = w.Write([]byte(`["demoteam"]`))
    }))
    defer srv.Close()

    c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "x"})
    tsv := "accounts\t1\nteam\tDemo Team\tdemoteam\tpw1\n"
    ids, err := c.Imports.Accounts(t.Context(), strings.NewReader(tsv))
    if err != nil {
        t.Fatalf("unexpected err: %v", err)
    }
    if got.method != "POST" {
        t.Errorf("method = %q, want POST", got.method)
    }
    if !strings.HasPrefix(got.contentType, "multipart/form-data; boundary=") {
        t.Errorf("CT = %q, want multipart", got.contentType)
    }
    if got.formField != "tsv" {
        t.Errorf("form field = %q, want tsv", got.formField)
    }
    if got.formFile != tsv {
        t.Errorf("form file body mismatch")
    }
    if !reflect.DeepEqual(ids, []string{"demoteam"}) {
        t.Errorf("ids = %v, want [demoteam]", ids)
    }
}
```

> ⚠️ 注意：本 repo **不引入 testify**。上面的 `require.NoError` 換成
> `if err != nil { t.Fatal(err) }`。範例為了短才寫 require。

### 4.3 router 測試 — 用 fake API + fake gateway

`internal/importer/router_test.go` 用兩個 fake：一個 mock API
client、一個 mock web gateway，斷言 router 在不同 input 下選擇了哪一條
路。同一個 input case 只會命中其中一邊；測試遍歷組合表確保所有 §3
SPEC 的決策都被覆蓋。

範本：

```go
type fakeAPI struct{ accountsCalled int }
func (f *fakeAPI) Accounts(ctx context.Context, r io.Reader) ([]string, error) {
    f.accountsCalled++
    return []string{"demoteam"}, nil
}
type fakeGW struct{ setPwCalled int }
func (f *fakeGW) SetUserPassword(ctx context.Context, id, pw string, roles []int, en bool) error {
    f.setPwCalled++
    return nil
}

func TestRouter_SetPassword_NoRoles_GoesAPI(t *testing.T) { ... }
func TestRouter_SetPassword_WithRoles_FallsBackToScraper(t *testing.T) { ... }
```

### 4.4 「金檔案」byte-identical 測試（DoD #4）

```go
// internal/importer/golden_test.go
func TestImportUsersTeams_OutputCSV_GoldenIdentical_ScraperVsAPI(t *testing.T) {
    in := mustReadFixture(t, "testdata/importer/input_5_rows.csv")
    expect := mustReadFixture(t, "testdata/importer/golden_out.csv")

    for _, mode := range []string{"scraper", "api"} {
        t.Run(mode, func(t *testing.T) {
            srv, gw, api := newTestServerAndGateway(t, mode)
            t.Cleanup(srv.Close)
            var out bytes.Buffer
            users, err := importer.ImportUsersTeams(t.Context(), gw, api.Users, bytes.NewReader(in), opts)
            if err != nil { t.Fatal(err) }
            if err := exporter.CreateUsers(users, exporter.FormatCSV, &out); err != nil { t.Fatal(err) }
            if !bytes.Equal(out.Bytes(), expect) {
                t.Errorf("mode=%s diff:\n--got--\n%s--want--\n%s", mode, out.String(), expect)
            }
        })
    }
}
```

`newTestServerAndGateway` 是 helper，依 mode 配出兩種 wiring，確保兩條
路徑跑同一份輸入產出完全一樣的 CSV。

## 5. 不要做的事

- **不要**在測試裡命中真實 server。CI 會抓沒寫 `httptest.URL` 的測試。
- **不要**把 `RYtE0yA2wCo_tZCO` 或任何真 password / API key commit
  進 repo。即使是 fixture 的 metadata 註解也不行。
- **不要**用 `t.TempDir()` 跑 fixture 比對之後忘了清；`t.Cleanup` 一定
  要寫。
- **不要**把 `testdata/api/v4/` 跟 `testdata/web/v7/` 的 fixture 混
  著用。後者是 HTML，前者是 JSON / multipart。decoder 不一樣，混了會
  在 review 階段就被退。

## 6. 後續維護

- 上游 DOMjudge 7.x 出新版 (7.3.5, 7.3.6, ...)：執行 §3 的腳本，diff
  `tmp/raw_api_v4/` vs commit 過的 fixture。沒差 → 沒事。有差 → 評估
  該欄位 client 是否關心 → 必要時更新 fixture（走 PR）。
- 升級到 DOMjudge 8 → 開新目錄 `testdata/api/v4/v8/`（API 還是 v4 但 v8
  server 行為微差）；不要直接覆蓋 v7 的 fixture。
- 任何時候對著 fixture 加 `// TODO: server changed since 2026-...`
  都比悶頭蓋掉好。
