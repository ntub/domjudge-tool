# CLI 使用手冊

`domjudge-tool` 指令列工具完整旗標參考文件。快速入門與架構說明請見
[`README.md`](../README.md)。

英文版請見 [`CLI.md`](CLI.md)。

## 支援的 DOMjudge 版本

| 版本 | 傳輸方式 |
| ---- | -------- |
| 7.3.x | REST API v4（建立 / 設定密碼）+ HTML 爬蟲備援 |
| 8.3.1 | HTML 爬蟲（寫入操作）+ REST API v4（讀取操作） |

在 `domserver.json` 的 `version` 欄位（或 `gen-config` 的 `--version` 旗標）
指定版本以選擇對應的 web gateway。

## 全域旗標

以下旗標適用於所有指令。

| 旗標 | 縮寫 | 預設值 | 說明 |
| ---- | ---- | ------ | ---- |
| `--config` | `-c` | `domserver.json` | 設定檔路徑 |
| `--verbose` | `-v` | — | 日誌詳細程度；`-v` = info，`-vv` = debug |
| `--disable-ssl` | — | `false` | 略過 TLS 憑證驗證（覆蓋設定檔） |
| `--no-progress` | — | `false` | 停用逐列進度條；當 stderr 非 TTY、設定 `NO_COLOR` 或 `TERM=dumb` 時自動停用 |

## 設定檔：`domserver.json`

由 `gen-config` 產生，JSON 欄位名稱使用 snake_case。

| 欄位 | 型別 | 預設值 | 說明 |
| ---- | ---- | ------ | ---- |
| `host` | string | — | DOMjudge 伺服器 URL（必填） |
| `username` | string | — | 管理員或 API 使用者（必填） |
| `password` | string | — | 使用者密碼（必填） |
| `disable_ssl` | bool | `false` | 略過 TLS 驗證 |
| `version` | string | `"7.3.2"` | DOMjudge 版本；控制 web gateway 選擇 |
| `api_version` | string | `"v4"` | REST API 版本 |
| `timeout` | float \| null | null | HTTP 逾時（秒）；null = 無逾時 |
| `max_connections` | int \| null | null | HTTP 最大連線數 |
| `max_keepalive_connections` | int \| null | null | HTTP 最大 keepalive 連線數 |
| `category_id` | int \| null | null | 預設隊伍類別 id |
| `affiliation_id` | int \| null | null | CSV 欄位空白時的備援 affiliation id |
| `affiliation_country` | string \| null | `"TWN"` | 新建 affiliation 的國家代碼 |
| `user_roles` | int[] | `[]` | 預設使用者角色 id |
| `contest_id` | string \| null | null | 指定比賽（數字 id 或 shortname）；null 自動偵測唯一進行中比賽 |
| `prefer_scraper` | bool \| null | null | 強制所有混合指令走 HTML 爬蟲路徑 |

## 指令

### 一般

#### `gen-config`

依旗標產生 `domserver.json`，不會連線至伺服器。

```bash
domjudge-tool gen-config --host https://judge.example.org \
    --username admin --password secret
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--host` | — | DOMjudge 主機 URL（必填） |
| `--username` | — | 管理員帳號（必填） |
| `--password` | — | 使用者密碼（必填） |
| `-o, --output` | `domserver.json` | 設定檔輸出路徑 |
| `--version` | `7.3.2` | DOMjudge 版本 |
| `--api-version` | `v4` | DOMjudge API 版本 |
| `--disable-ssl` | `false` | 略過 TLS 驗證 |
| `--timeout` | — | HTTP 逾時（秒） |
| `--max-connections` | — | HTTP 最大連線數 |
| `--max-keepalive-connections` | — | HTTP 最大 keepalive 連線數 |
| `--category-id` | `0` | 預設隊伍類別 id |
| `--affiliation-id` | `0` | 預設 affiliation id |
| `--affiliation-country` | `TWN` | 預設 affiliation 國家代碼 |
| `--user-roles` | — | 預設使用者角色 id（逗號分隔） |
| `--contest-id` | — | 預設比賽 id（數字或 shortname） |

#### `version`

列印建置版本後退出。

```bash
domjudge-tool version
```

無額外旗標。

---

### 使用者與隊伍

#### `import-users-teams-example`

將標準 CSV 範本寫入磁碟。填寫完畢後傳入 `import-users-teams`。

```bash
domjudge-tool import-users-teams-example
domjudge-tool import-users-teams-example -o my-contest.csv
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `-o, --output` | `import-users-teams.csv` | 範本輸出路徑 |

#### `import-users-teams`

將 CSV 檔案與伺服器上的使用者清單進行比對，建立缺少的使用者與隊伍；
可選擇更新或刪除現有資料列。

```bash
domjudge-tool import-users-teams import-users-teams.csv
domjudge-tool import-users-teams import-users-teams.csv \
    --delete-existing --category-id 3
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--category-id` | `0` | 預設隊伍類別 id |
| `--affiliation-id` | `0` | CSV 欄位空白時的備援 affiliation id |
| `--affiliation-country` | 取自設定檔 | 新建 affiliation 的國家代碼 |
| `--user-roles` | — | 使用者角色 id（逗號分隔） |
| `--enabled` | `true` | 保持每位建立的使用者為啟用狀態 |
| `--ignore-existing` | `false` | 略過帳號已存在的資料列 |
| `--delete-existing` | `false` | 刪除後重新建立帳號已存在的資料列 |
| `--password-length` | `0` | 產生密碼的長度（0 = 程式庫預設值） |
| `--password-pattern` | — | 產生密碼使用的字元集 |
| `--new-password` | `false` | 一律重新產生密碼，即使 CSV 已提供 |

#### `export-users`

將伺服器使用者清單匯出為 CSV 或 JSON。別名：`user-list`。

```bash
domjudge-tool export-users
domjudge-tool export-users --format json -o users.json
domjudge-tool export-users --ids 1,2,3
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--format` | `csv` | 輸出格式：`csv` 或 `json` |
| `-o, --output` | stdout | 輸出檔案 |
| `--ids` | — | 要包含的使用者 id（逗號分隔） |
| `--team-id` | — | 依隊伍 id 篩選 |

#### `rm-teams-and-users`

刪除使用者及其隊伍。別名：`rm-teams`。

受保護帳號（`admin`、`judgehost` 及設定檔中的 `username`）一律略過。
`--exclude` / `--include` 旗標遵循與 Python CLI 相容的反向篩選慣例，
不可在無 ADR 的情況下修改。

```bash
domjudge-tool rm-teams-and-users
domjudge-tool rm-teams-and-users --exclude contestant1,contestant2
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--exclude` | — | 要保留的帳號（除受保護帳號外） |
| `--include` | — | 要刪除的帳號（預設：所有非受保護帳號） |

#### `set-password`

重設單一使用者的密碼。

```bash
domjudge-tool set-password alice --password newsecret
domjudge-tool set-password alice --password-length 16
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--password` | — | 新密碼（省略則自動產生） |
| `--password-length` | `0` | 產生密碼的長度（0 = 程式庫預設值） |
| `--password-pattern` | — | 產生密碼使用的字元集 |
| `--user-roles` | — | 使用者角色 id（逗號分隔） |
| `--enabled` | `true` | 更新後保持使用者為啟用狀態 |

---

### 郵件

#### `email-template-example`

建立包含 `subject.txt`、`body.txt`、`body.html` 的範本目錄，
並填入全部六個支援的佔位符號。

```bash
domjudge-tool email-template-example
domjudge-tool email-template-example -o ./my-template --force
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `-o, --output` | `./email-template` | 範本輸出目錄 |
| `-f, --force` | `false` | 覆蓋現有範本檔案 |

#### `email-template-validate`

驗證範本目錄可正常載入，且僅使用六個支援的佔位符號。
有錯誤時以非零值退出。

```bash
domjudge-tool email-template-validate ./my-template
```

無額外旗標。

#### `render-user-accounts`

將每位收件人的 RFC 5322 `.eml` 檔案渲染至磁碟，不接觸 SMTP。
適用於審閱、批次作業及郵件合併工作流程。

```bash
domjudge-tool render-user-accounts users.csv ./my-template
domjudge-tool render-user-accounts users.csv ./my-template \
    -d ./output --from-email contest@example.org --force
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `-d, --output-dir` | `./rendered-emails` | `.eml` 檔案輸出目錄 |
| `--from-email` | `noreply@localhost` | `From:` 標頭值；同時作為收件人備援網域來源 |
| `--force` | `false` | 覆蓋現有 `.eml` 檔案 |

#### `send-user-accounts`

直接透過 SMTP 發送帳號憑證郵件。

SMTP 憑證（`--smtp-username` / `--smtp-password`）與 `domserver.json`
中的 DOMjudge 管理員驗證資訊分開設定。

```bash
domjudge-tool send-user-accounts users.csv ./my-template \
    --host smtp.example.org --port 587 \
    --smtp-username mailer --smtp-password secret \
    --from-email contest@example.org
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--host` | `localhost` | SMTP 伺服器主機 |
| `--port` | `25` | SMTP 伺服器連接埠 |
| `--from-email` | `noreply@localhost` | `From:` 標頭值 |
| `--use-ssl` | `false` | TLS-on-connect（非 STARTTLS） |
| `--smtp-username` | — | SMTP AUTH 帳號（空白略過驗證） |
| `--smtp-password` | — | SMTP AUTH 密碼（空白略過驗證） |
| `--timeout` | `0` | SMTP 連線逾時（`0` = 無逾時） |

---

### 題目

#### `download-problems`

使用設定的裁判憑證登入、列出比賽題目，並將每道題目的匯出 ZIP 檔案寫入磁碟。

```bash
domjudge-tool download-problems
domjudge-tool download-problems --folder ./zips --only A,B,C
domjudge-tool download-problems --exclude D,E
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--folder` | `export_problems` | 輸出目錄 |
| `--exclude` | — | 要排除的題目 id（逗號分隔） |
| `--only` | — | 要保留的題目 id（逗號分隔，排除其他所有題目） |

---

### 計分板

#### `download-scoreboard`

將公開計分板匯出為 CSV。別名：`scoreboard-export`。

輸出欄位：`Rank, TeamAffiliation, TeamName, SolvedCount, Score,
Category, <題目標籤…>`。自 v1.4.0 起，輸出涵蓋所有隊伍類別
（每個 `sortorder` 一個排名段落，`Rank` 在每段重新從 1 開始）。

```bash
domjudge-tool download-scoreboard --cid 1
domjudge-tool download-scoreboard --cid 1 \
    --filename results --path ./output --exclude-hidden
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--cid` | — | DOMjudge 比賽 id（必填） |
| `--filename` | `export` | 輸出檔名（不含副檔名，自動加上 `.csv`） |
| `--path` | 目前目錄 | CSV 檔案寫入目錄 |
| `--exclude-hidden` | `false` | 排除隱藏隊伍類別（例如 System） |
| `--url` | 設定檔 `host` | 覆蓋計分板主機位址 |

---

### 繳交

#### `download-submissions`

下載某場比賽所有繳交的原始碼壓縮檔。別名：`contest-files`。

```bash
domjudge-tool download-submissions 1
domjudge-tool download-submissions 1 --mode 1 --path ./dl --extract=false
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--mode` | `2` | 目錄結構：`1`=隊伍優先，`2`=題目優先，其他=`contest_<cid>` |
| `--path` | — | 輸出路徑前綴 |
| `--extract` | `true` | 在原位解壓縮每個下載的壓縮檔 |
| `--strict` | `false` | 傳遞至 API v4 繳交清單查詢（Python 相容性） |

---

## CSV 格式

`import-users-teams`、`render-user-accounts`、`send-user-accounts` 均使用
相同的 CSV 格式。檔案支援 BOM（UTF-8 BOM 會自動略過）。

| 欄位 | 必填 | 說明 |
| ---- | ---- | ---- |
| `username` | 是 | 唯一的登入帳號 |
| `name` | 是 | 顯示名稱 |
| `email` | 否 | 電子郵件（空白 = 不設定帳號 email） |
| `affiliation` | 是 | 所屬機構 / affiliation 名稱 |
| `password` | 否 | 明文密碼；空白時自動產生 |

欄位順序固定，不可新增或刪除欄位。

使用下列指令產生範本：

```bash
domjudge-tool import-users-teams-example
```

## 郵件範本佔位符號

`render-user-accounts` 和 `send-user-accounts` 在 `subject.txt`、
`body.txt`、`body.html` 中支援以下六個佔位符號：

| 佔位符號 | 來源 |
| -------- | ---- |
| `$username` | CSV `username` 欄位 |
| `$name` | CSV `name` 欄位 |
| `$email` | CSV `email` 欄位 |
| `$affiliation` | CSV `affiliation` 欄位 |
| `$password` | CSV `password` 欄位（或自動產生的值） |
| `$server_host` | `domserver.json` `host` 欄位 |

使用下列指令驗證範本目錄：

```bash
domjudge-tool email-template-validate ./my-template
```
