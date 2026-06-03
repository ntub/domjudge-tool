# Phase 2 — 契約測試與 fixture 規範（CONTRACT）

> 這份文件平行於 `docs/phase1.5/CONTRACT.md` 與
> `testdata/web/v7/CAPTURE.md`。Phase 2 的 fixture 分成兩類：
> DOMjudge 8.3.1 API JSON 與 jury HTML。兩者都必須先抓、清洗、通過 PII
> gate，才可以成為 v8 decoder / scraper 的測試輸入。

## 1. 為什麼 Phase 2 更需要 contract fixture

DOMjudge 8.3.1 live investigation 已確認 API v4 不是完全等形：

- `time_limit` 從 Go 目前期待的 int 變成 live JSON `5.0`。
- `/api/v4/version` 的 `api_version` 從 string expectation 變成 int。
- OpenAPI under-described：文件沒列出的欄位，live response 仍可能存在。
- 部分欄位 silent drift：decode 不炸，但值可能落在舊 struct 欄位之外。

因此 Phase 2 不能用「手寫一份看起來合理的 JSON」測 decoder。必須把 live
response sanitize 後存成 fixture，讓 decoder 與 downstream use-case 都對同
一份資料跑。

## 2. fixture 目錄規格

```text
testdata/
├── api/
│   └── v4/
│       ├── ...                    # v7 fixture，保留
│       └── v8/
│           ├── CAPTURE_API_V8.md
│           ├── version.json
│           ├── contests.json
│           ├── contest_<cid>/
│           │   ├── contest.json
│           │   ├── problems.json
│           │   ├── teams.json
│           │   ├── organizations.json
│           │   ├── groups.json
│           │   ├── scoreboard.json
│           │   ├── submissions.json
│           │   ├── submission_<id>.json
│           │   └── source_code_<id>.json
│           └── imports/
│               ├── groups_request.json
│               ├── organizations_request.json
│               ├── teams_request.json
│               ├── accounts_request.json
│               └── sanctioned_write_notes.md
└── web/
    └── v8/
        ├── CAPTURE.md
        ├── login.html
        ├── problems.html
        ├── users.html
        ├── user_delete_confirm.html
        ├── teams.html
        ├── team_delete_confirm.html
        └── problem_export_response.headers
```

`<cid>` 與 `<id>` 是 placeholder；committed fixture 可使用 generic numbers。
不要把 live contest title、shortname、host 或 raw identifier 寫進檔名。

## 3. API JSON capture protocol

### 3.1 環境變數

設定只放 shell / `.env.local`，不要 commit：

```bash
export DJ_BASE="https://test-station.example.org"
export DJ_USER="<username>"
export DJ_PASS="<password>"
export DJ_CID="<cid>"
```

`test-station.example.org` 是保留範例；實際 live host 不得寫進 repo。任何
capture note 只能稱為 **the live 8.3.1 station**。

### 3.2 GET-only capture

P2-0 預設只允許 GET：

```bash
RAW="tmp/raw_api_v4_v8"
mkdir -p "$RAW/contest_$DJ_CID"

curl_json() {
  curl -sS --fail --user "$DJ_USER:$DJ_PASS" "$DJ_BASE$1" | jq . > "$2"
}

curl_json "/api/v4/version" "$RAW/version.json"
curl_json "/api/v4/contests" "$RAW/contests.json"
curl_json "/api/v4/contests/$DJ_CID" "$RAW/contest_$DJ_CID/contest.json"
curl_json "/api/v4/contests/$DJ_CID/problems" "$RAW/contest_$DJ_CID/problems.json"
curl_json "/api/v4/contests/$DJ_CID/teams" "$RAW/contest_$DJ_CID/teams.json"
curl_json "/api/v4/contests/$DJ_CID/organizations" "$RAW/contest_$DJ_CID/organizations.json"
curl_json "/api/v4/contests/$DJ_CID/groups" "$RAW/contest_$DJ_CID/groups.json"
curl_json "/api/v4/contests/$DJ_CID/scoreboard?allteams=true" "$RAW/contest_$DJ_CID/scoreboard.json"
curl_json "/api/v4/contests/$DJ_CID/submissions" "$RAW/contest_$DJ_CID/submissions.json"
```

若抓 `source-code`，只選 synthetic / sample submission，不抓真實學生提交：

```bash
curl_json "/api/v4/contests/$DJ_CID/submissions/<id>/source-code" \
  "$RAW/contest_$DJ_CID/source_code_<id>.json"
```

### 3.3 drift confirmation checklist

把下列結果寫進 `testdata/api/v4/v8/CAPTURE_API_V8.md`：

- `provider.version` / `domjudge.apiversion`。
- `/api/v4/version` `api_version` 的 JSON type。
- `problems[].time_limit` 的 JSON type 與 sample value。
- `source-code` 是 array 還是 object。
- `problems` 是否有 `ordinal`、`test_data_count`、`externalid`。
- Contest / submission 是否使用 `external_id`。
- Organization 是否缺 `url` / `twitter_hashtag`。
- Team 是否缺 `members`。
- Judgement 是否缺 `judgehost`。

## 4. jury HTML capture protocol

HTML fixture 必須從 authenticated jury flow 抓，但 committed file 不得含
cookie、CSRF、real host 或 PII。

需要的頁面：

| Fixture | 用途 |
|---------|------|
| `login.html` | `web/v8.Login` form / CSRF parser |
| `problems.html` | `GetProblems` list parser |
| `problem_export_response.headers` | `Download` problem ZIP metadata / content disposition |
| `users.html` | `DeleteUsers` row selector 與 delete link parser |
| `user_delete_confirm.html` | user delete confirm POST form |
| `teams.html` | `DeleteTeams` row selector 與 delete link parser |
| `team_delete_confirm.html` | team delete confirm POST form |

Capture 時可用 browser devtools、`curl --cookie-jar` 或小腳本，但 raw output
只放 `tmp/raw_web_v8/`。Sanitized fixture 才能放進 `testdata/web/v8/`。

## 5. Sanitization checklist

所有 API JSON 與 HTML fixture 都跑同一份 checklist：

| # | 規則 | Sanitized value |
|---|------|-----------------|
| 1 | 真實 hostname / URL | `example.org`、`test-station.example.org`、`prod-station.example.org` |
| 2 | credentials / password / API key | `REDACTED` |
| 3 | cookies / session id | `TEST_COOKIE` 或移除 |
| 4 | CSRF token | `TEST_CSRF_TOKEN` |
| 5 | email | `user1@example.org`、`user2@example.org` |
| 6 | private / real IP | `192.0.2.1` 或 `203.0.113.10` |
| 7 | student id / course id / cohort token | `student001`、`course001` |
| 8 | CJK real names / school-specific names | `Sample001`、`Sample University` |
| 9 | phone / contact info | `+1-555-0100` 或移除 |
| 10 | live contest title / shortname | `ContestX` |
| 11 | timestamps tied to people | `2026-01-01T00:00:00+00:00` |
| 12 | source code from real submissions | 不 commit；只允許 synthetic sample |

PII gate 至少檢查：

```text
non-example.org email
private IP ranges
raw live hostname tokens
Bearer / Basic credentials
password-like key/value pairs
CSRF-looking random tokens not equal TEST_CSRF_TOKEN
CJK text that was not intentionally kept as generic prose
8+ digit student-id-like numbers
```

P2-0 的 project-specific PII gate 必須同時掃描：

```text
testdata/api/v4/v8/     # JSON and existing API fixture types
testdata/web/v8/        # .html and .headers jury web fixtures
```

目前 `scripts/check_pii_api.py` 只由 `Justfile` 的 `api-fixture-check` 掃
`testdata/api/v4`，且 fixture suffix 沒有涵蓋 `.html` / `.headers`。因此在
任何 `testdata/web/v8/` HTML / headers fixture commit 前，P2-0 必須先擴充
這個 scanner 讓它接受 web dir 與這些 file types，或新增 sibling scanner
並接進 `privacy-check` recipe。Web gate 必須涵蓋 non-`example.org`
emails、CSRF-looking tokens、CJK names、8+ digit student-id-like numbers、
raw hostnames、credentials。

任何命中都不能 commit。

## 6. Live contest write protocol

### 6.1 何時允許 write

Phase 2 預設 GET-only。只有下列情況可以 write：

- P2-0 或 P2-5 需要驗證 v8 import body / delete mechanics。
- Operator 指定 single sanctioned writable contest。
- 本次測試資料使用 unique synthetic prefix。
- 已產生 predicted match set，且 operator explicit sign-off。

### 6.2 Synthetic prefix

所有 live-created resource 都必須使用同一個 prefix：

```text
phase2-synthetic-<yyyymmddhhmm>-<shortcase>
```

例如：

```text
phase2-synthetic-202606040930-team01
phase2-synthetic-202606040930-user01
phase2-synthetic-202606040930-org01
phase2-synthetic-202606040930-group01
```

不要使用真實姓名、學號、課程代碼或 email。Email 如必要只用
`user1@example.org`。

### 6.3 Predicted match set

在任何 delete POST 前，先用 GET / HTML parse 產生 predicted match set：

```text
operation: DeleteTeams
include: [phase2-synthetic-202606040930]
exclude: []
predicted rows:
- id: <id>, name: phase2-synthetic-202606040930-team01
- id: <id>, name: phase2-synthetic-202606040930-team02
non-matching rows observed: <count>
```

Sign-off 前必須確認：

- predicted rows 全部都是本次 synthetic prefix。
- 沒有任何 real user / team / group / org 在 predicted rows。
- include / exclude 條件和 `v7.py:161` quirk 的 preserved semantics 不衝突。
- cleanup plan 已寫好。

### 6.4 Operator sign-off

Capture note 使用這種格式：

```text
Live write sign-off
Station: the live 8.3.1 station
Contest: <cid>
Synthetic prefix: phase2-synthetic-<timestamp>
Allowed operations: import groups/orgs/teams/accounts, delete synthetic users/teams
Predicted match set reviewed: yes
Operator sign-off: yes, by @project-maintainer, YYYY-MM-DD HH:MM local time
Cleanup completed: yes/no
```

不要寫真實 hostname、帳號、email、學生資料。

### 6.5 Cleanup

Write test 結束後立即 cleanup：

1. Delete synthetic users。
2. Delete synthetic teams。
3. 若 DOMjudge UI / API 支援，清掉 synthetic groups / organizations；若不支
   援，capture note 要記錄 residual synthetic rows。
4. 再跑一次 GET / HTML list，確認 predicted synthetic rows 為 0 或只剩已
   記錄的 residual rows。

## 7. 客戶端 contract tests

API decoder test 模式：

```go
func TestProblemsResource_List_DecodesV8Fixture(t *testing.T) {
    raw := mustReadFixture(t, "../../../../testdata/api/v4/v8/contest_1/problems.json")
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet || r.URL.Path != "/api/v4/contests/1/problems" {
            t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write(raw)
    }))
    t.Cleanup(srv.Close)

    c := v4.New(v4.Options{BaseURL: srv.URL, Username: "example-user", Password: "REDACTED"})
    problems, err := c.Problems.List(t.Context(), "1")
    if err != nil {
        t.Fatalf("List: %v", err)
    }
    if len(problems) == 0 {
        t.Fatalf("problems should not be empty")
    }
}
```

HTML scraper test 模式：

- `httptest.Server` 回傳 sanitized HTML。
- Test 斷言 method / path / form fields / CSRF placeholder。
- 不連 live station。
- 不用 testify；錯誤用 `if err != nil { t.Fatal(err) }`。

## 8. 不要做的事

- 不要把 raw capture 從 `tmp/` commit。
- 不要在 fixture 檔名或 Markdown note 寫真實 station hostname。
- 不要用 live delete 取代 `httptest` correctness test。
- 不要把 v7 fixture 覆蓋成 v8 fixture；v8 一律放 `testdata/api/v4/v8/` 與
  `testdata/web/v8/`。
- 不要在 tests 裡呼叫 real DOMjudge URL。
- 不要新增 testify / ginkgo。

## 9. 後續維護

- DOMjudge 8.3.x patch release 出現時，重跑 GET-only capture，比對 v8
  fixture diff。
- 如果 upstream 加了 users / teams DELETE 或 full problem-package ZIP API，
  先寫新的 ADR，再決定是否縮小 `web/v8` scope。
- 如果 `accounts.json` / import mechanics 在 v8 live write 中被證實不同，
  更新本文件 §6 的 write note 與 `SPEC.md` §4，再改 code。
