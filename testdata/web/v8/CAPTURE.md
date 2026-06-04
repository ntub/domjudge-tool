# DOMjudge 8.3.1 Jury HTML 擷取紀錄

## 擷取頁面

- `login`
- `post_login_landing`
- `problems`
- `users`
- `teams`
- `team_delete_confirm`
- `user_delete_confirm`
- `problem_export_response.headers`

## 擷取方式

使用已驗證登入的 jury session 進行 GET-only 擷取。刪除確認頁只讀取頁面，
沒有送出任何刪除 POST，也沒有執行 teams 或 users 的刪除動作。

## Sanitization

- 主機名稱改為 `test-station.example.org`。
- CSRF token 改為 `TEST_CSRF_TOKEN`。
- Email 全部改為 `user1@example.org`；Cloudflare `data-cfemail` 也重新編碼
  為解碼後的 `user1@example.org`。
- IP 位址改為 `192.0.2.1`。
- CJK 個人、學校、課程或隊伍資訊改為通用文字。
- 8 位以上數字 ID 已移除。
- 表格 `tbody` 已截短為 3 列。

## DOMjudge 8 Jury 結構事實

- Problem export 使用 `GET /jury/problems/{id}/export`，回應內容型態為
  `application/zip`。
- Teams 與 users 刪除流程先透過 GET 取得確認頁，再 POST 到
  `/jury/{teams,users}/{id}/delete`。
