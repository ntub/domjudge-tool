# `testdata/api/v4/v8/` API Fixture Capture Log

這些 fixture 鎖定 DOMjudge 8.3.1 在 Contest API v4 的回應形狀，用於 Phase 2
contract 與 drift 測試。所有 fixture 都是從 live 8.3.1 station
`test-station.example.org` 擷取的 GET-only responses；raw responses 必須只留在
本機 ignored 路徑，只有清理後的衍生檔可進入 git。

## Source

| Field | Value |
|-------|-------|
| Capture target | DOMjudge 8.3.1 live test station |
| Station hostname | `test-station.example.org` |
| Required credentials | 本機環境變數，不寫入 fixture 或文件 |
| Raw output | ignored local capture directory |
| Sanitized output | `testdata/api/v4/v8/` |
| PII gate | `scripts/check_pii_api.py` |

不要從 production contest server 擷取。若 production data 是唯一來源，raw files
必須放在 repo 外或 ignored 路徑，立即清理，並在 review 前跑完 privacy gate。

## Procedure

```bash
python3 scripts/check_pii_api.py testdata/api/v4/v8 testdata/web/v8
mise run privacy-check
```

## Expected Fixture Layout

```text
testdata/api/v4/v8/
  CAPTURE_API_V8.md
  contests.json
  info.json
  users.json
  version.json
```

目前只提交已清理並通過 gate 的 GET-only JSON fixture。新增 fixture 前，先確認
raw capture 不在 tracked files，並把 fixture 清單同步更新。

## Sanitization Rules

1. Hostname 與 URL host 一律改成 `test-station.example.org`。
2. Email 一律改成 `user@example.org`。
3. IPv4 一律改成 `192.0.2.1`。
4. `login_time` 改成 generic timestamp。
5. Names、usernames、source content、CJK text 一律 genericise。
6. Lists truncate to 3 items，避免帶入不必要的 live data。

## Confirmed Drift Checklist

- `/api/v4/version` 的 `api_version` 是 int `4`。
- `problems[].time_limit` 是 float，例如 `5.0`。
- `source-code` endpoint 回傳 JSON array，元素形狀為
  `{id, submission_id, filename, source}`。
- `problems` 有 `ordinal`、`test_data_count`、`externalid`。
- `contest` 與 `submission` 使用 `external_id`，不是 `externalid`。
- `organizations` 沒有 `url` / `twitter_hashtag`，但有 `formal_name`。
- `team` 沒有 `members`。
- `judgement` 沒有 `judgehost`。

## Fixture Files

- `contests.json`
- `info.json`
- `users.json`
- `version.json`

## Review Checklist

- Raw live captures 只存在 ignored local path 或 repo 外。
- Sanitized fixtures committed under `testdata/api/v4/v8/`。
- `python3 scripts/check_pii_api.py testdata/api/v4/v8 testdata/web/v8` passes。
- `mise run privacy-check` passes。
