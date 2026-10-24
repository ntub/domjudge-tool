# domjudge-tool

Manage **DOMjudge 7.3.x and 8.3.1**: users, teams, problem packages,
scoreboards, submission sources, and account emails.

- **CLI:** released and suitable for scripting.
- **Desktop:** native preview with server profiles, progress, and History.
  Release bundles for macOS, Linux, and Windows include the CLI.
- **New here?** Follow one quick start below. Contributors can jump to
  [Work on this repository](#work-on-this-repository).

## CLI: install and start

Download the CLI archive for your platform from
[Releases](https://github.com/ntub/domjudge-tool/releases)
(`domjudge-tool_<os>_<arch>.tar.gz`, or `.zip` on Windows), extract it, and
put `domjudge-tool` on your `PATH`. [Verify the download](#verify-a-download)
before running it. Alternatively, with Go **1.27.3+**:

```bash
go install github.com/ntub/domjudge-tool/cmd/domjudge-tool@latest
domjudge-tool --help
```

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

## Desktop: download or build

Each release attaches `domjudge-tool-desktop_<version>_<os>_<arch>` bundles
for macOS, Linux, and Windows on amd64 and arm64 (`.tar.gz` on Linux, `.zip`
elsewhere). Extract the bundle to a permanent, user-writable location and
keep its directory together; on macOS, move the `.app` out of Downloads
before opening it. The bundles are **unsigned and not notarized**, so the
operating system may warn on first launch. Linux needs GTK4 and
WebKitGTK 6.0 (baseline: Ubuntu 24.04); Windows needs WebView2.

To build from a checkout instead, install [mise](https://mise.jdx.dev/) and a
native CGO toolchain; mise installs the pinned Go, Node.js, and other tools:

```bash
mise install
mise run setup
mise run build-desktop
./bin/domjudge-tool-app
```

`mise run package-desktop` bundles the native Desktop and CLI together, as a
release does. macOS produces `bin/domjudge-tool.app`; Linux and Windows
produce a portable directory under `bin/desktop/domjudge-tool`. Archives and
checksums are under `dist/desktop`.

In the packaged app, use the **Command-Line Tool** card in **Settings** to
install the current-user command. Follow the displayed PATH instructions; the
app never edits PATH, shell configuration, or the Windows registry. CLI
commands work while Desktop is closed. Moving the app requires
**Relink CLI to This App**; deleting it makes its CLI target unavailable. See
the [installation guide](docs/USER_GUIDE.md#23-install-the-bundled-cli-command)
for conflicts, removal and platform prerequisites. A raw development build
without a bundle does not provide installation.

1. In **Servers**, select **+ Add Server** and enter the host, credentials,
   DOMjudge version, and contest ID. Desktop profiles are separate from
   the CLI's `domserver.json`.
2. Open an operation in the sidebar, choose the profile, and supply its
   CSV, problem package, contest, or output directory.
3. Review progress and **History**. File downloads may open a native Save
   dialog; cancelling leaves the form available for another attempt.

**Remove Users** defaults to dry-run. Review the planned removals before
turning dry-run off; permanent removal requires confirmation. The minimum
native window size is **800 × 560**.

## Verify a download

Every release publishes `checksums.txt` for the CLI archives, a `.sha256`
file next to each Desktop bundle, and a signed build-provenance attestation
for every archive. Check both before running a download:

```bash
sha256sum --ignore-missing -c checksums.txt     # CLI archives
sha256sum -c domjudge-tool-desktop_*.sha256     # Desktop bundles
gh attestation verify <archive> --repo ntub/domjudge-tool
```

On macOS, use `shasum -a 256` in place of `sha256sum`. A passing attestation
proves the archive was built by this repository's release workflow from the
tagged commit; it does not mean the bundle is code-signed.

## Choose the next task

| Task | Start here |
| --- | --- |
| Illustrated desktop + CLI guide | [English](docs/USER_GUIDE.md) · [繁體中文](docs/USER_GUIDE.zh-TW.md) |
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
