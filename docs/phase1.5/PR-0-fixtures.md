# PR-0 — API fixture 抓取與 PII 清洗

## Goal

把 DOMjudge 7.3.x demo station 的 API 真實 response 抓進 `testdata/api/v4/`，
作為後續 PR-1..PR-4 所有契約測試 + decoder 測試的唯一資料源。

順便驗證 SPEC §2.6 列的 3 項 idempotency 假設，把結果寫進 `CAPTURE_API.md`。

## 為什麼放在 PR-0

PR-1 起所有測試都會直接 `mustReadFixture(t, "testdata/api/v4/...")`。
沒有 fixture，連 RED 測試都寫不出來；fixture 沒清洗乾淨，整批 PR 都
會帶 PII 進 commit history。把這件事獨立成 PR-0，讓 PR-1 起的作者可
以直接讀 fixture，不用每個 PR 都重新驗證 PII。

## Files created / modified

| Path | Role |
|------|------|
| `scripts/capture_api_fixtures.sh` | bash 抓取腳本（CONTRACT.md §3.2 完整版） |
| `scripts/sanitize_api.py` | JSON-aware sanitizer，吃 raw JSON 跑 15 條規則 |
| `scripts/check_pii_api.py` | 8 條 grep gate；任一 hit 回非 0 |
| `testdata/api/v4/CAPTURE_API.md` | 平行於 `testdata/web/v7/CAPTURE.md`：抓取程序、規則表、idempotency 三項實測結果 |
| `testdata/api/v4/version.json` | `GET /api/v4/version` 樣本 |
| `testdata/api/v4/info.json` | `GET /api/v4/info` 樣本 |
| `testdata/api/v4/user.json` | `GET /api/v4/user` 樣本（admin） |
| `testdata/api/v4/users/list_default.json` | 全 user 列表，含 `team`/`team_id`/`enabled`/`email` 各種變化 |
| `testdata/api/v4/users/list_with_team_id.json` | `?team_id=2` 過濾後 |
| `testdata/api/v4/users/single_admin.json` | `GET /users/1` |
| `testdata/api/v4/users/single_with_email.json` | 一個有 email 的 team user |
| `testdata/api/v4/contests/list.json` | 兩個 demo contest |
| `testdata/api/v4/contest_2/teams.json` | 含 affiliation/organization_id 的 team 列表 |
| `testdata/api/v4/contest_2/organizations.json` | 至少 3 筆，含 country=`TWN` 等 |
| `testdata/api/v4/contest_2/groups.json` | hidden=true / false 兩種 |
| `testdata/api/v4/contest_2/problems.json` | 至少 3 題 |
| `testdata/api/v4/contest_2/scoreboard.json` | 一個未 freeze 的版本 |
| `testdata/api/v4/contest_2/scoreboard_with_freeze.json` | 一個有 frozen state 的（如有 freeze 的 contest 可抓；否則手動構造一份 freezing 變體 fixture） |
| `testdata/api/v4/contest_2/submissions.json` | 至少一筆有 entry_point 的 submission |
| `testdata/api/v4/contest_2/submission_files.zip` | 真實 ZIP（注意：ZIP 內可能含學生原始碼 → 要確認沒 PII；建議用 demo 容器自送一份「Hello World」進去再抓出來） |
| `testdata/api/v4/imports/groups_post_response_ok.json` | 預期回應 |
| `testdata/api/v4/imports/teams_post_response_ok.json` | |
| `testdata/api/v4/imports/teams_post_response_validation_err.json` | 一筆失敗的回應，作 negative case |
| `testdata/api/v4/imports/organizations_post_response_ok.json` | |
| `testdata/api/v4/imports/accounts_post_response_ok.json` | |
| `testdata/api/v4/imports/accounts_post_first.json` | idempotency test 1 第一次 POST 結果 |
| `testdata/api/v4/imports/accounts_post_second.json` | 同 username 第二次 POST 結果 |
| `testdata/api/v4/imports/accounts_first_verify.json` | 第一次後 GET /users 過濾結果 |
| `testdata/api/v4/imports/accounts_second_verify.json` | 第二次後 GET /users 過濾結果 |
| `Justfile` | 加 task `api-fixture-capture`、`api-fixture-check`，分別呼叫上面兩支腳本 |
| `.github/workflows/ci.yml` | 加一步 `just api-fixture-check` |
| `.gitignore` | 確認 `tmp/raw_api_v4/` 已 ignore |

## 步驟（給作者）

1. 跑 `scripts/capture_api_fixtures.sh` 把 raw 抓進 `tmp/raw_api_v4/`。
2. 跑 `scripts/sanitize_api.py tmp/raw_api_v4/ testdata/api/v4/` 產 sanitized
   版到 testdata。
3. 跑 `scripts/check_pii_api.py testdata/api/v4/`，必須 8/8 pass。
4. 對 demo station 跑 §"Idempotency 三項實測"（CONTRACT.md §3.3），把
   結果填進 `CAPTURE_API.md` §"Findings"。
5. 刪 `tmp/raw_api_v4/`、commit。

## Tests written first (RED)

PR-0 只有 fixture + 腳本，沒有 production code。但**腳本本身要有測試**：

| Test | What it asserts |
|------|-----------------|
| `scripts/sanitize_api_test.py` | 對一份 hand-crafted 含 PII 的 sample JSON 跑 sanitizer，比對輸出 |
| `scripts/check_pii_api_test.py` | 對一份 hand-crafted 「乾淨」+ 「含一條 PII」的 sample，分別期望 exit 0 / 1 |

兩個 test script 用 stdlib `unittest`；放在同 `scripts/` 目錄。CI 跑
`python3 -m unittest discover scripts/`。

> 為什麼這樣寫：fixture 是後續所有測試的基礎，sanitizer / check 出錯
> 等於整批 PR 都不可信。先把這層工具測穩，後面才能高效。

## Idempotency 三項實測（必填）

把 `CAPTURE_API.md` 的這個區塊填好（範本，PR-0 作者照填）：

```markdown
## Findings — POST idempotency (PR-0, 202X-XX-XX)

Test 1: POST /api/v4/users/accounts re-upload with same username, new password
  Result: <upsert | duplicate | error>
  Evidence: imports/accounts_post_first.json,
            imports/accounts_first_verify.json,
            imports/accounts_post_second.json,
            imports/accounts_second_verify.json
  Verdict for PR-3 / PR-4:
    - upsert  → set-password and import-users-teams update path can use API
    - duplicate / error → both update paths must remain on scraper

Test 2: POST /api/v4/users/teams (json) re-upload with same id
  Result: ...

Test 3: POST /api/v4/users/organizations re-upload with same shortname
  Result: ...
```

## Edge cases to capture

確保以下「形狀變化」至少有一份 fixture：

- `team_id` 為 `null`（admin / judgehost user）vs string `"3"`。
- `email` 為 `null` vs `"foo@example.org"`。
- `roles` 同時含 multi-role（`admin` + `jury` + ...）vs 單 `team`。
- `team` 巢狀物件為 `null` vs 完整物件。
- organization 的 `formal_name` vs `name` vs `shortname` 三者異同的範
  例（API 上這三欄常常一樣，但 schema 允許不同）。
- problem `time_limit` 是 float（`5.0`）— 確保 decoder 用 `float64`，
  否則 JSON `int` decoder 會 panic。
- scoreboard `state.started` 為 ISO8601 + tz；frozen / thawed 在某些
  contest 上是 `null`。
- submissions empty array（demo contest 沒 submission 時的形狀）。

## Definition of done

- [ ] `tmp/raw_api_v4/` 已 capture、`.gitignore` 已含。
- [ ] `testdata/api/v4/` 全部檔案存在、structure 對齊上表。
- [ ] `just api-fixture-check` 綠。
- [ ] `python3 -m unittest discover scripts/` 綠。
- [ ] `CAPTURE_API.md` §"Findings" 三項都填了具體結果（不是「TBD」）。
- [ ] commit message：`chore(testdata/api/v4): capture sanitized fixtures from DOMjudge 7.3.x demo`
- [ ] PR 描述附上 idempotency 三項的明確結論（「Test 1: upsert」之類）
      — 後續 PR 直接以這個結論做設計輸入。

## Cross-references

- `docs/phase1.5/CONTRACT.md` §3 — 抓取與清洗詳細協議
- `testdata/web/v7/CAPTURE.md` — 平行範本（HTML 版本）
- `docs/phase1.5/SPEC.md` §2 — 為什麼這些 fixture 是必要的
