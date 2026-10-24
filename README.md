# domjudge-tool

Manage **DOMjudge 7.3.x and 8.3.1**: users, teams, problem packages,
scoreboards, submission sources, and account emails.

- **CLI:** fast, scriptable interface for automation and CI/CD.
- **Web UI:** local browser interface served by `domjudge-tool web`,
  with server profiles, operation progress, and history.
- **New here?** Follow the install and quick start below. Contributors can jump
  to [Work on this repository](#work-on-this-repository).

## Install

Copy the line for your platform and run it. No Go, Python, Homebrew, Scoop,
jq, Git, administrator rights, or sudo is needed.

**macOS / Linux (Terminal):**

```sh
curl -fsSL https://raw.githubusercontent.com/ntub/domjudge-tool/main/scripts/install.sh | sh
```

**Linux without `curl`:**

```sh
wget -qO- https://raw.githubusercontent.com/ntub/domjudge-tool/main/scripts/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/ntub/domjudge-tool/main/scripts/install.ps1 | iex
```

**Windows (Command Prompt):**

```cmd
powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/ntub/domjudge-tool/main/scripts/install.ps1 | iex"
```

The installer detects the OS and CPU, downloads the matching release archive
and `checksums.txt`, verifies the SHA-256 checksum, installs the binary to
`$HOME/.local/bin` (macOS/Linux) or
`%LOCALAPPDATA%\Programs\domjudge-tool\bin` (Windows), and adds that directory
to your user PATH when missing. It uses only tools shipped with the OS:
`sh`, `curl` or `wget`, `tar`, and `shasum` or `sha256sum` on macOS/Linux;
built-in Windows PowerShell 5.1+ on Windows.

Open a new terminal, then check the install:

```sh
domjudge-tool version
domjudge-tool web
```

### Environment overrides

- `DOMJUDGE_TOOL_VERSION`: pin an exact release tag (for example `v1.8.0`; defaults to the latest release).
- `DOMJUDGE_TOOL_INSTALL_DIR`: override the destination directory.
- `DOMJUDGE_TOOL_NO_MODIFY_PATH=1`: skip shell profile or registry PATH modifications.

### Manual download and verification

You can also download archives directly from
[Releases](https://github.com/ntub/domjudge-tool/releases)
(`domjudge-tool_{linux,darwin}_{amd64,arm64}.tar.gz`, `domjudge-tool_windows_{amd64,arm64}.zip`,
and `checksums.txt`). Verify the download before running:

```bash
sha256sum --ignore-missing -c checksums.txt     # Linux (or shasum -a 256 on macOS)
gh attestation verify <archive> --repo ntub/domjudge-tool
```

Extract the archive and place `domjudge-tool` (or `domjudge-tool.exe`) in a directory on your PATH.
Binaries are not OS-code-signed; integrity and authenticity come from the SHA-256 checksum and
optional Sigstore build-provenance attestation. Downloads via curl or PowerShell are not
quarantined, so Gatekeeper and SmartScreen do not block the CLI.

Alternatively, with Go **1.27.3+**:

```bash
go install github.com/ntub/domjudge-tool/cmd/domjudge-tool@latest
domjudge-tool --help
```

## Quick start: CLI

Create a local configuration. Replace the example host, password, version,
and contest ID with your server's values:

```bash
domjudge-tool gen-config \
  --host https://test-station.example.org \
  --username admin --password 'REPLACE_LOCALLY' \
  --version 8.3.1 --contest-id 1

# First live operation: read users without modifying the server.
domjudge-tool export-users -o users.csv
```

`gen-config` writes `domserver.json` without contacting DOMjudge. For a
7.3.x server, set its actual version, for example `--version 7.3.2`.
Use `--config /path/to/domserver.json` to select another configuration.

To import users, generate the CSV, edit its rows, and then import it:

```bash
domjudge-tool import-users-teams-example
# Edit import-users-teams.csv before running the next command.
domjudge-tool import-users-teams import-users-teams.csv
```

Importing **changes the server** and writes `import-users-teams-out.csv`.
Keep configuration, credential CSVs, exports, and rendered emails private;
never commit them. Inspect `domjudge-tool <command> --help` before running
writes or deletion commands.

## Web UI

Serve the local web interface and open it in your browser:

```bash
domjudge-tool web
```

This starts the server on `http://127.0.0.1:9900/`, generates a 256-bit random
access token, prints the launch URL, and opens your default browser. Server profiles,
settings, and operation history are stored in a local SQLite database (`--db`),
separated from the CLI's `domserver.json`.

### Flags and options

| Flag | Default | Description |
| --- | --- | --- |
| `--host` | `127.0.0.1` | Listen address (IP or hostname). |
| `--port` | `9900` | Listen port; `0` picks an available free port. |
| `--auth` | `token` | Authentication mode: `token`, `basic`, or `none`. |
| `--token` | random | Fixed access token for `--auth token`; env `DOMJUDGE_TOOL_WEB_TOKEN` (≥ 16 chars). |
| `--username` | `admin` | Username for `--auth basic`. |
| `--password` | none | Password for `--auth basic`; env `DOMJUDGE_TOOL_WEB_PASSWORD`. Required for basic. |
| `--tls-cert`, `--tls-key` | none | PEM certificate and private key files for HTTPS; both or neither. |
| `--no-browser` | `false` | Do not automatically open the default browser. |
| `--db` | `<user config dir>/domjudge-tool/domjudge-tool.db` | Path to SQLite store for profiles, settings, and history. |

### Usage examples

**Headless server with an automatic free port:**

```bash
domjudge-tool web --no-browser --port 0
```

**LAN or remote access with Basic auth and TLS:**

```bash
export DOMJUDGE_TOOL_WEB_PASSWORD="choose-a-strong-password"
domjudge-tool web --host 0.0.0.0 --auth basic --username admin \
  --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem
```

### Security model

- **Authentication:** `token` generates a 256-bit secret per run. Opening the browser URL
  with `?token=...` sets an `HttpOnly`, `SameSite=Strict` session cookie and redirects to the clean URL.
  `basic` enforces HTTP Basic authentication with constant-time comparison.
- **Loopback-only `none`:** `--auth none` is permitted only on loopback addresses (`127.0.0.1`,
  `localhost`, `[::1]`). Binding non-loopback with `--auth none` is rejected.
- **Plaintext exposure warning:** If `--host` is not a loopback address and TLS is not configured,
  a warning is printed to stderr because DOMjudge credentials and contest data travel unencrypted.
- **DNS rebinding & CSRF:** On loopback listeners, requests must supply a valid loopback `Host` header.
  Cross-origin mutating requests are rejected via Go's `http.CrossOriginProtection`.
- **Security headers:** The server sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Content-Security-Policy: frame-ancestors 'none'`, and `Referrer-Policy: same-origin`.

## Choose the next task

| Task | Start here |
| --- | --- |
| Illustrated Web UI + CLI guide | [English](docs/USER_GUIDE.md) · [繁體中文](docs/USER_GUIDE.zh-TW.md) |
| Users and teams | `import-users-teams`, `export-users`, `set-password` |
| Problem packages | `download-problems`, `import-problems --help` |
| Contest data | `download-scoreboard --help`, `download-submissions --help` |
| Account emails | Render with `render-user-accounts` before sending with `send-user-accounts` |
| Complete CLI reference | [English](docs/CLI.md) · [繁體中文](docs/CLI.zh-TW.md) |
| Copy-and-edit examples | [English](docs/SNIPPETS.md) · [繁體中文](docs/SNIPPETS.zh-TW.md) |
## Work on this repository

Install [mise](https://mise.jdx.dev/getting-started.html). It pins every
runtime and tool (Go, Node.js, Python, linters, scanners, release tools) in
`mise.toml`, with checksums in `mise.lock`; CI uses the same pins.

```bash
mise trust
mise install
mise run setup
mise run build
./bin/domjudge-tool --help
mise run check
```

`mise tasks` lists all tasks. Tests use sanitized fixtures and local test
servers; **never point tests at a real DOMjudge or SMTP server**. Run
`mise run privacy-check` before contributing documentation or screenshots.
Do not change CLI names, config keys, CSV column order, or locked scraper
behavior without an explicit compatibility decision.

Releases are published only by GitHub Actions, when a maintainer pushes an
annotated `vMAJOR.MINOR.PATCH` tag on `main`; the release notes are that
version's `CHANGELOG.md` section. On the 1st of each month a workflow opens
one issue summarizing open Dependabot alerts; it never opens pull requests or
changes dependencies.

Read [CONTRIBUTING.md](CONTRIBUTING.md) for setup, hooks, commits, and
releases, and [AGENTS.md](AGENTS.md) for the compatibility and privacy rules.
Design references: [architecture](docs/ARCHITECTURE.md) and
[decision records](docs/adr/README.md).

## License

[MIT](LICENSE) © Jason Xie / NTUB.
