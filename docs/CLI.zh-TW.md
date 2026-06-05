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

### 設定欄位語意

- `host`、`username`、`password` 同時供 REST API client 與 scraper
  session 使用。API 呼叫使用 Basic Auth；scraper 呼叫會登入 DOMjudge jury UI。
- `disable_ssl` 也可由全域 `--disable-ssl` 強制啟用。只應用於可信任、
  使用自簽憑證的測試站。
- `version` 選擇 scraper 實作。`7.3.x` family 使用 v7 gateway；
  `8.3.x` family 使用 v8 gateway。不支援的 family 會在任何 mutation
  前失敗。
- `api_version` 目前為 `v4`。API client 路徑以 `/api/v4` 為根。
- `timeout` 套用到 HTTP client。`null` 表示不設定明確逾時。
  `max_connections` 與 `max_keepalive_connections` 保留作為設定相容性；
  目前 HTTP client 不提供使用者可見的並行調校。
- `category_id` 是 `import-users-teams` 與 `set-password` API import
  使用的預設 DOMjudge team category/group id。CSV `category_id` cell
  會覆蓋該列的值。
- `affiliation_id` 是 CSV row 的 `affiliation` 留空時使用的備援
  affiliation id。若 `affiliation` 有值，importer 會改為解析或建立該
  affiliation。
- `affiliation_country` 用於 importer 建立缺少的 affiliation/organization。
- `user_roles` 是 scraper 密碼寫入的預設角色 id 清單。明確指定 roles
  會讓密碼更新走 scraper，因為 `accounts.tsv` 沒有 role 欄位。
- `contest_id` 用於需要 contest scope 的 API 讀取與 import。若省略，
  需要 contest 的指令只會在剛好有一個 active contest 時自動選用；
  否則會要求你設定此欄位。
- `prefer_scraper=true` 會在 scraper 實作存在時，強制混合操作走 scraper。
  API-only 讀取仍維持 API 路徑。

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
範本包含可選的 `category_id` 欄位；留空時會使用 `--category-id`
或 `domserver.json` 的預設值。

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
CSV 可包含可選的 `category_id` 欄位。若該列留空，或整份 CSV 省略此欄，
該列會使用既有的 `--category-id` / 設定檔備援機制。

詳細流程：

1. 載入 `domserver.json`、套用指令旗標、建立 API client，並建立依版本
   選擇的 web gateway。
2. 先透過 scraper gateway 登入。這會在 mutation 開始前攔截無效的 jury
   憑證。
3. 從 `/api/v4/users` 取得目前使用者清單並解析 CSV。重複 username 或
   格式錯誤會在 delete/create mutation 前中止。
4. 若指定 `--delete-existing`，透過 scraper 刪除符合的既有 users 與
   teams。DOMjudge 7.3.x 與 8.3.1 都沒有提供這些資源的 REST DELETE。
5. 將資料列分成 create、update、skip cohorts。`--ignore-existing` 會略過
   既有 username；否則既有資料列會依所選路由更新 team、password、email。
6. 解析 affiliation 資料。非空的 `affiliation` cell 會解析或建立該
   affiliation/organization；空白 cell 使用 `--affiliation-id` 或設定檔備援。
7. 解析 team category。非空的 CSV `category_id` cell 會套用到該列；
   否則使用 `--category-id` 或設定檔。
8. 建立/更新 users、設定密碼，並在需要時補寫 email。指令會寫出
   `import-users-teams-out.csv`，其中包含產生或實際使用的密碼。

輸出：

- `import-users-teams-out.csv` 會寫在目前工作目錄。
- 輸出 CSV 保持帳號寄送用格式，不包含 input-only 的可選 `category_id`
  欄位。
- 成功執行時會列印簡短摘要與輸出檔名。

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

詳細流程：

1. 載入設定檔，並以 `host`、`username`、`password`、`disable_ssl`、
   `timeout` 建立 REST API v4 client。
2. 讀取 `/api/v4/users`，並在 API 支援處傳入 `--ids` 與 `--team-id`。
3. 在記憶體中再次套用 `--ids` 與 `--team-id`。這可保留 DOMjudge 7.3.x
   伺服器忽略 `?team_id=` 時的正確行為。
4. 序列化為 CSV 或 JSON。未指定 `--output` 時輸出到 stdout；指定
   `--output` 時 stdout 保持空白，資料寫入檔案。

CSV 輸出包含 live DOMjudge user 欄位。JSON 輸出保留本工具使用的 model
欄位名稱。

#### `rm-teams-and-users`

刪除使用者及其隊伍。別名：`rm-teams`。

受保護帳號（`admin`、`judgehost` 及設定檔中的 `username`）一律略過。
`--exclude` / `--include` 旗標遵循與 Python CLI 相容的反向篩選慣例，
不可在無 ADR 的情況下修改。

```bash
domjudge-tool rm-teams-and-users
domjudge-tool rm-teams-and-users --exclude contestant1,contestant2
domjudge-tool rm-teams-and-users --include-csv import-users-teams.csv
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--exclude` | — | 要保留的帳號（除受保護帳號外） |
| `--include` | — | 要刪除的帳號（預設：所有非受保護帳號） |
| `--include-csv` | — | 讀取 import-users-teams CSV，刪除其中 `username` 欄位列出的帳號 |

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

使用設定的裁判憑證登入，並將選定題目的匯出 ZIP 檔案寫入磁碟。當
`domserver.json.contest_id` 已設定且未指定 `--only` 時，命令會先讀取
`/api/v4/contests/{cid}/problems`，用該 contest 的 problem ids 限縮
scraper 下載範圍，避免 DOMjudge 7.x 下載到全域 jury problem catalog。

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

#### `import-problems`

將 problem package ZIP 匯入設定檔指定的比賽。此命令使用
`domserver.json.contest_id` 作為目標 contest id。新建匯入時不送 optional
`problem` form field，讓 DOMjudge 從 package metadata 建立題目。
DOMjudge 7.x 使用 legacy multipart file field `zip[]`；DOMjudge 8.x
使用 `zip`。只有在匯入單一 ZIP 並要更新既有 problem id 時，才使用
`--problem-id`。

若 DOMjudge 回報 `problem.externalid: This value is already used`，代表
server 已有相同 external id 的全域 problem package。除非你明確要更新那個
既有 server problem，否則不要用 `--problem-id` 重試 create path。在
DOMjudge 8.3.1，
`DELETE /api/v4/contests/{cid}/problems/{id}` 只會將 problem 從 contest
unlink，不會刪除全域 problem package。

```bash
domjudge-tool import-problems ./export_problems
domjudge-tool import-problems ./zips/A_easy.zip --problem-id A
```

| 旗標 | 預設值 | 說明 |
| ---- | ------ | ---- |
| `--problem-id` | — | 單一 ZIP input 要更新的既有 problem id |

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
相同的帳號憑證 CSV 核心格式。檔案支援 BOM（UTF-8 BOM 會自動略過）。

| 欄位 | 必填 | 說明 |
| ---- | ---- | ---- |
| `username` | 是 | 唯一的登入帳號 |
| `name` | 是 | 顯示名稱 |
| `email` | 否 | 電子郵件（空白 = 不設定帳號 email） |
| `affiliation` | 是 | 所屬機構 / affiliation 名稱 |
| `password` | 否 | 明文密碼；空白時自動產生 |
| `category_id` | 否 | `import-users-teams` 的 input-only per-row team category override；email 指令會忽略 |

前五個欄位保留歷史順序：`username,name,email,affiliation,password`。
`category_id` 可追加在尾端。沒有 `category_id` 的舊 CSV 仍可使用。

`import-users-teams` 也接受兩種相容格式：

- `username,email,affiliation,name`
- `username,name,email,affiliation,password,is_exist`

`is_exist` 只用於讀取先前產生的輸出。不要把 `category_id` 寫入
`import-users-teams-out.csv`；它只屬於輸入。

使用下列指令產生範本：

```bash
domjudge-tool import-users-teams-example
```

## 操作路由

`domjudge-tool` 在 REST API v4 足夠時使用 API；DOMjudge 沒有安全 API
路徑時使用 HTML scraper。`--verbose` 會以結構化 log 顯示 router decision。

| 操作 | 預設路由 | 原因 |
| ---- | -------- | ---- |
| `version` | API | 讀取 `/api/v4/version` |
| `export-users` | API | 讀取 `/api/v4/users`，再於記憶體補篩選 |
| `download-submissions` | API | 使用 submission list 與 file/source endpoints |
| `download-scoreboard` | API | 讀取 scoreboard、teams、organizations、groups 後重建 CSV |
| `download-problems` contest scope | API | `contest_id` 已設定且未指定 `--only` 時讀取 `/api/v4/contests/{cid}/problems` |
| `download-problems` list | Scraper | 套用 API contest scoping 後使用版本化 jury UI problem listing |
| `download-problems` ZIP | Scraper | REST API 沒有 problem ZIP export endpoint |
| `import-problems` | API | 將 problem package ZIP POST 到 `/api/v4/contests/{cid}/problems` |
| `import-users-teams` create/update/password/email | v7 與 v8 預設 Scraper | v7 `accounts.tsv` user-to-team linkage 不安全；v8 write APIs 對這些欄位不可靠 |
| `import-users-teams` delete-existing | Scraper | REST API 沒有 user/team DELETE |
| `set-password` 未明確指定 `--user-roles` | v7 走 API，v8 走 scraper | v7 `accounts.tsv` password reset 安全；v8 fallback scraper |
| `set-password` 明確指定 `--user-roles` | Scraper | `accounts.tsv` 沒有 role 欄位 |
| `rm-teams-and-users` | Scraper | REST API 沒有 user/team DELETE |
| Email commands | Local / SMTP | 本機讀 CSV/templates；只有 `send-user-accounts` 會連 SMTP |

`domserver.json` 的 `prefer_scraper=true` 會強制混合操作走 scraper route。
它不會讓 scoreboard 或 submission download 這類 API-only 讀取離開 API。

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
