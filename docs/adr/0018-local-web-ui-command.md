# 0018. Local web UI served by `domjudge-tool web`

- **Status:** Proposed
- **Date:** 2026-10-24
- **Deciders:** @ntub

## Context

The native Desktop app (Wails window plus a bundled CLI) cannot be
distributed as trusted software without Apple Developer ID signing and
notarization and a Windows Authenticode certificate. The project has
neither, and unsigned bundles are blocked or warned about by Gatekeeper
and SmartScreen. The Desktop UI itself is an ordinary HTTP application
(templ, Datastar, Tailwind; see [0013](0013-phase3-frontend-stack.md))
that the Wails window only displayed.

The CLI archives are already published with checksums and Sigstore
provenance attestations, need no OS signing to run, and are what users
install.

## Decision

Remove the Desktop app, its packaging, and its per-user CLI integration.
Serve the same UI from the CLI with a new `domjudge-tool web` command that
opens the user's default browser.

### Code layout

- `internal/webui/{assets,db,handler,oplog,profile,settings,templates}`
  hold the UI (formerly `internal/desktop/...`).
- `internal/webui/server` composes the stores, the real operation
  functions, the security middleware, and the HTTP server.
- `internal/cli/web` owns the Cobra command, flag validation, terminal
  output, and opening the browser.
- `cmd/e2e-server` keeps serving the UI with in-memory data for UI tests.
- Removed: `cmd/desktop`, `cmd/package-desktop`, `cmd/cli-launcher`,
  `internal/desktop/cliinstall`, the Settings CLI section, the native
  directory picker (`/api/pick-directory`), native download interception,
  `build/darwin`, the Wails dependency, and the Desktop CI/release jobs.

### Command contract

`domjudge-tool web [flags]` serves until interrupted (Ctrl+C or SIGTERM),
then shuts down gracefully.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--host` | `127.0.0.1` | Listen address (IP or hostname). |
| `--port` | `9900` | Listen port; `0` picks a free port. |
| `--auth` | `token` | `token`, `basic`, or `none`. |
| `--token` | random | Fixed access token for `--auth token`; env `DOMJUDGE_TOOL_WEB_TOKEN`. At least 16 characters. |
| `--username` | `admin` | User name for `--auth basic`. |
| `--password` | none | Password for `--auth basic`; env `DOMJUDGE_TOOL_WEB_PASSWORD`. Required for basic. |
| `--tls-cert`, `--tls-key` | none | Serve HTTPS with this PEM certificate and key; both or neither. |
| `--no-browser` | false | Do not open a browser. |
| `--db` | `<user config dir>/domjudge-tool/domjudge-tool.db` | SQLite store for profiles, settings, and history; the former Desktop path, so existing data carries over. |

Flags win over environment variables. Stdout prints, in order:

```text
domjudge-tool web is serving at http://127.0.0.1:9900/
Open in your browser: http://127.0.0.1:9900/?token=<token>
Press Ctrl+C to stop.
```

The `Open in your browser:` line is the stable, scriptable contract; for
`basic` and `none` it carries no token. A wildcard host (`0.0.0.0`, `::`)
is opened and printed through the matching loopback address. A failure to
open the browser is a warning; the server keeps running.

### Security model

- **Authentication.**
  - `token`: a 256-bit random token per run unless `--token` is set.
    `GET` with `?token=` sets an `HttpOnly`, `SameSite=Strict` session
    cookie named per port (`Secure` under TLS) and redirects to the same
    URL without the token. `Authorization: Bearer <token>` is also
    accepted. Comparisons are constant-time.
  - `basic`: HTTP Basic authentication with a constant-time comparison.
  - `none`: allowed only when `--host` is a loopback address.
- **DNS rebinding.** On a loopback listener the `Host` header must be
  `127.0.0.1`, `localhost`, or `[::1]` with the listening port.
- **CSRF.** Go's `http.CrossOriginProtection` rejects cross-origin
  state-changing requests.
- **Headers.** `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Content-Security-Policy: frame-ancestors 'none'`,
  `Referrer-Policy: same-origin` (`no-referrer` would turn the `Origin`
  header of form POSTs into `null`).
- **Exposure warning.** A non-loopback listener without TLS prints a
  stderr warning that credentials and DOMjudge data travel unencrypted.

Profiles store DOMjudge credentials, so the default stays loopback plus
token authentication.

### Alternatives considered

- **Keep shipping unsigned Desktop bundles.** Gatekeeper and SmartScreen
  block or warn on every install; users cannot judge authenticity.
- **Buy signing certificates.** Recurring cost and credential custody the
  project cannot take on.
- **Loopback-only server without authentication.** Any local process or a
  browser tab using DNS rebinding could drive DOMjudge admin operations
  with stored credentials.
- **Keep the native directory picker.** It opens a dialog on the server
  machine, which is wrong for a remote browser; output paths are typed.

## Consequences

- Positive: one signed-by-provenance artifact per platform, no CGO or
  WebKit/WebView2 runtime, and the UI works in any modern browser.
- Positive: the release pipeline builds only the CLI.
- Negative: users who installed the Desktop bundle must switch to the CLI
  archive. Data carries over through the shared `--db` default.
- Negative: output directories are typed instead of picked natively.
- Constraints: existing CLI commands, flags, `domserver.json`, and CSV
  column order are unchanged. The web UI stores data only in `--db`.

## References

- [0012](0012-phase3-local-datastore.md) local datastore.
- [0013](0013-phase3-frontend-stack.md) frontend stack.
- [0017](0017-mise-toolchain-and-tag-release.md) release pipeline.
- Go `net/http` [`CrossOriginProtection`](https://pkg.go.dev/net/http#CrossOriginProtection).
