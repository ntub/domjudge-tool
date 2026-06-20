# CLI Snippets

`domjudge-tool` 常用工作流程的可複製命令片段。完整旗標參考請見
[`CLI.zh-TW.md`](CLI.zh-TW.md)。

共享筆記與提交內容只使用保留範例值。不要提交真實 DOMjudge URL、憑證、
production captures 或學生資料。

## Generate Config

建立 DOMjudge 8.3.1 server 的 `domserver.json`：

```bash
domjudge-tool gen-config \
  --host https://judge.example.org \
  --username admin \
  --password secret \
  --version 8.3.1 \
  --contest-id 2
```

將設定檔寫到指定路徑：

```bash
domjudge-tool gen-config \
  --host https://judge.example.org \
  --username admin \
  --password secret \
  --output /path/to/domjudge-tool/tmp/live-test/domserver.json
```

在任何指令指定設定檔：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  version
```

## Import Users And Teams

產生匯入 CSV 範本：

```bash
domjudge-tool import-users-teams-example
```

從填好的 CSV 匯入 users 與 teams：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-users-teams import-users-teams.csv
```

指令會在目前目錄寫出 `import-users-teams-out.csv`，內含產生的帳號憑證。
請將這個輸出放在 ignored storage。

匯入既有資料列時重新產生密碼：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-users-teams import-users-teams.csv \
  --new-password
```

## Password Pattern

`--password-pattern` 是字元集，不是 regular expression。密碼產生器會從
這個字串中的字元隨機挑選每一個密碼字元。

使用數字、英文大寫與英文小寫，同時排除列印或口頭念出時容易混淆的字元。
這個範例排除 `0`、`1`、`I`、`L`、`O`、`i`、`l`、`o`：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-users-teams import-users-teams.csv \
  --new-password \
  --password-length 12 \
  --password-pattern "23456789ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz"
```

重設單一帳號密碼時，也可以使用同一組易讀 pattern：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  set-password contestant1 \
  --password-length 12 \
  --password-pattern "23456789ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz"
```

如果現場需要更嚴格的口頭辨識規則，可以再從字元集中移除更多相似字元，
例如 `S` / `s` / `5`、`Z` / `z` / `2`、`B` / `8`、`G` / `6`、
`q` / `9`。請確保移除後的字元集仍足以支撐選定的密碼長度。

## Export Users

匯出 users 為 CSV：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  export-users \
  --output /path/to/domjudge-tool/tmp/live-test/users.csv
```

匯出 users 為 JSON：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  export-users \
  --format json \
  --output /path/to/domjudge-tool/tmp/live-test/users.json
```

## Delete Users And Teams

這些指令會刪除資料。執行前請先確認目標清單，live validation 輸出應放在
ignored storage。

刪除指定 users 及其 teams：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  rm-teams-and-users \
  --include contestant1,contestant2
```

刪除所有非保護帳號，但保留列出的帳號：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  rm-teams-and-users \
  --exclude contestant1,contestant2
```

刪除 `import-users-teams` 格式 CSV 中列出的 usernames：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  rm-teams-and-users \
  --include-csv /path/to/domjudge-tool/tmp/live-test/import-users-teams-out.csv
```

`--include-csv` 會讀取 CSV 的 `username` 欄。保護帳號（`admin`、
`judgehost` 與設定檔 username）一律略過。

## Download Contest Data

下載 contest id `2` 的 scoreboard：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  download-scoreboard \
  --cid 2 \
  --path /path/to/domjudge-tool/tmp/live-test \
  --filename scoreboard-cid-2
```

下載並解壓 contest id `2` 的 submissions：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  download-submissions 2 \
  --path /path/to/domjudge-tool/tmp/live-test/submissions-cid-2
```

下載 problem packages：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  download-problems \
  --path /path/to/domjudge-tool/tmp/live-test/problems
```

## Upload Problem Packages

`import-problems` 會將 problem package ZIP 上傳到
`domserver.json.contest_id` 指定的 contest。產生設定檔時請設定
`--contest-id`，或在執行前編輯 `contest_id`。

上傳單一 ZIP 作為新題目。DOMjudge 會從 package 讀取 problem id 與 metadata：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-problems /path/to/domjudge-tool/tmp/live-test/problem-zips/A_easy.zip
```

上傳單一 ZIP 以更新既有 problem id：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-problems /path/to/domjudge-tool/tmp/live-test/problem-zips/A_easy.zip \
  --problem-id A
```

批次上傳資料夾中所有 top-level `.zip` 檔：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  import-problems /path/to/domjudge-tool/tmp/live-test/problem-zips
```

`--problem-id` 只能搭配單一 ZIP 檔使用。資料夾批次上傳會依各 package 的
metadata 建立或匯入題目，不接受 `--problem-id`。

## Email Account Credentials

建立 email template 目錄：

```bash
domjudge-tool email-template-example \
  --output /path/to/domjudge-tool/tmp/live-test/email-template
```

驗證 template：

```bash
domjudge-tool email-template-validate \
  /path/to/domjudge-tool/tmp/live-test/email-template
```

只 render 帳號信件，不透過 SMTP 寄出：

```bash
domjudge-tool render-user-accounts \
  /path/to/domjudge-tool/tmp/live-test/import-users-teams-out.csv \
  /path/to/domjudge-tool/tmp/live-test/email-template \
  --output-dir /path/to/domjudge-tool/tmp/live-test/eml \
  --from-email contest@example.org
```

透過 SMTP 寄出帳號信件：

```bash
domjudge-tool send-user-accounts \
  /path/to/domjudge-tool/tmp/live-test/import-users-teams-out.csv \
  /path/to/domjudge-tool/tmp/live-test/email-template \
  --from-email contest@example.org \
  --smtp-host smtp.example.org \
  --smtp-port 587 \
  --smtp-username contest@example.org
```

## Live Test Pattern

將 live validation artifacts 放在 ignored 目錄：

```bash
mkdir -p /path/to/domjudge-tool/tmp/live-test
```

執行命令時明確指定 config 與 output path：

```bash
domjudge-tool \
  --config /path/to/domjudge-tool/tmp/live-test/domserver.json \
  --no-progress \
  export-users \
  --format json \
  --output /path/to/domjudge-tool/tmp/live-test/users-before.json
```

提交 docs、scripts、fixtures 或 generated outputs 前，先跑 privacy gate：

```bash
just privacy-check
```
