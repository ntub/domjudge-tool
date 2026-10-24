# ADR 0016: Desktop-bundled CLI and per-user command integration

- **Status:** Proposed (implemented in the working tree; not merged)
- **Scope:** macOS, Linux, Windows; no publishing or signing credentials

## Context

Desktop and CLI have independent entry points and share operation packages.
Installing the Desktop must also deliver the existing CLI, without starting
Wails for terminal commands. Installation must not require administrator
privileges, overwrite an unrelated command, modify shell startup files or
registry PATH, or synchronize credentials. A TUI is out of scope.

The local bundle/installation implementation is present, and native
acceptance has passed on macOS, Linux, and Windows. The feature changes
remain uncommitted, and the new six-job Desktop artifact CI workflow has
not run. Local acceptance does not establish merged code, CI success, or a
public release.

## Decision

Ship the native Desktop, native CLI, and bundle metadata together. Keep the
existing CLI command names, flags, output, config defaults, and version
format. Build all bundle components from the same source and version.

### Package layout

- macOS: `domjudge-tool.app/Contents/MacOS/domjudge-tool-app`,
  `Contents/Resources/cli/domjudge-tool`, and
  `Contents/Resources/bundle.json`.
- Linux: `domjudge-tool/domjudge-tool-app`, `cli/domjudge-tool`, and
  `bundle.json` inside that root.
- Windows: the same portable directory with `.exe` suffixes, plus
  `resources/cli-launcher.exe`.
- Metadata records schema version 1, version, revision, GOOS, and GOARCH.
  It declares build identity, not authenticity. It never supplies paths.
- Archive names include version/platform; internal directory names remain
  stable for in-place upgrades. Unsigned artifacts are not a public release.

### Per-user integration

On macOS and Linux, `~/.local/bin/domjudge-tool` links to the private state
file `target`, which is itself a symlink to the bundled CLI. State lives in
`os.UserConfigDir()/domjudge-tool/cli-install`. Relinking atomically updates
only the private target.

On Windows, install a native console launcher at
`%LOCALAPPDATA%/domjudge-tool/cli-install/bin/domjudge-tool.exe`; its sibling
state directory holds `target.json`, with a single absolute `path` field.
The launcher executes that exact target, never a PATH fallback or a shell.
Preserve arguments, empty arguments, Unicode, stdin/stdout/stderr, working
directory, environment, exit status, and console cancellation. A missing
App produces a clear nonzero failure. The launcher never starts Desktop.

The App does not change PATH. Show exact command paths, current-session
examples and permanent setup instructions. GUI environment observations
must not claim that another terminal's PATH, aliases, or command cache has
been checked.

### Lifecycle and safety

Settings provides Inspect, Install, Relink, and Uninstall, independently of
General/SMTP Save Settings. No nested forms. Mutations require explicit user
action and show the affected path. Same-location App upgrades update the
CLI automatically. A moved App requires Relink; deleting the App leaves a
broken target. Uninstall removes only integration objects whose ownership
can be verified; never App binaries, profiles, config files or unrelated
commands. Other installations may remain usable through PATH.

An ownership receipt plus actual filesystem identity determines ownership.
Receipt alone never authorizes overwrite. Foreign files, directories,
symlinks and modified launchers are conflicts, with no force action.
Read-only inspection does not create directories or execute discovered
programs. Missing source, corrupt state, permission errors, stale forms and
busy files remain visible. Cross-process locking serializes this tool's
mutations; revision checking rejects stale forms. Public command creation
uses no-replace operations. Do not promise protection against malicious
programs already running as the same user.

An inaccessible old CLI source is a broken target, not an ownership failure.
Once command, private target and receipt ownership are verified, allow
Relink from a valid new bundle and owned-only Uninstall while showing the
source permission error. Inaccessible integration state still fails closed.

Reject known macOS App Translocation locations. A raw development executable
without the packaged CLI/metadata reports integration unavailable. Preview
mode explicitly disables installation and never resolves the real home.

### Interfaces and package boundaries

`internal/desktop/cliinstall` owns filesystem state, discovery, platform
operations and the Windows launcher runtime. It does not import Wails,
HTTP, templ, GORM or the Cobra command tree. `cmd/desktop` composes it.
The handler consumes these methods:

```go
Inspect(context.Context) (cliinstall.Status, error)
Install(context.Context, string) (cliinstall.Status, error)
Relink(context.Context, string) (cliinstall.Status, error)
Uninstall(context.Context, string) (cliinstall.Status, error)
```

The mutation string is the inspected revision, not a filesystem path.
`Config` has `ExecutablePath`, `HomeDir`, `StateDir` strings; native paths are
resolved only by production composition. `New(Config) (*Manager, error)`
validates configuration without modifying disk. Status fields are strings
`State`, `BundlePath`, `CLIPath`, `CommandPath`, `TargetPath`, `Version`,
`Revision`, `Reason`, `PathNote`, and booleans `CanInstall`, `CanRelink`,
`CanUninstall`. Missing or invalid bundles have an explanatory unavailable
status rather than a fake installed state. Tests inject temporary paths.

### Request boundary

Protect the new CLI mutation routes and token-bearing Settings GET with an
exact listener Host check. Require process-local random CSRF tokens on POST,
constant-time comparison, same-origin checks where Origin is present, and
Fetch Metadata protection. Reject null/cross-site origins and unknown form
fields; accept no source, destination or executable path from HTTP. Do not
trust User-Agent. Use no-store and anti-framing headers. Missing Origin is
allowed only with the other checks and valid token. Preview mutations fail
closed. Bind the listener synchronously before opening the native WebView,
so it cannot navigate to another process after a bind failure.

Use `Referrer-Policy: same-origin`: it suppresses cross-origin referrers
without turning native HTML form POST origins into `null`. `no-referrer`
would make legitimate installation forms fail the strict Origin check.
Keep rejecting `null` rather than weakening the request boundary.

## Packaging and acceptance contract

The artifact workflow defines six native package jobs: macOS, Linux, and
Windows on amd64 and arm64. This describes configured coverage, not executed
CI evidence. Linux targets Ubuntu 24.04-compatible GTK4/WebKitGTK 6.0
userspace, not every distribution. Windows Desktop uses the GUI subsystem;
CLI and launcher use the console subsystem. Standalone CLI release
configuration remains independent, with Windows included. CI has read-only
repository permissions and is configured to upload artifacts/checksums
without publishing releases.

Packaging replaces only an absent, empty, or recognizable prior bundle
directory. Resolve existing path ancestors before checking repository,
input, output and artifact overlap. If replacement and rollback both fail,
keep the previous bundle outside temporary-stage cleanup and report its
recovery path; never delete the only remaining copy.

Deterministic stdlib tests cover lifecycle, ownership, stale revisions,
permission failures, hostile origins, and launcher argument/I/O/exit
behavior. Packaging smoke checks exercise actual unpacked CLI
help/version/config generation and installed-command lookup in an isolated
PATH. Generated templ files stay paired with their sources. Acceptance
uses full affected test packages, not only newly added cases.

Native acceptance remains distinct from workflow configuration and
cross-compilation. Windows acceptance covers ARM64 and x86_64 in separate
native guests kept disconnected from the network. It includes standard
users without Developer Mode, cmd/PowerShell, spaces and Unicode paths,
Ctrl+C, upgrades, move/relink, missing targets, and owned-only removal.
macOS and Linux also need actual GUI and command behavior, not just binary
production. The local native acceptance recorded above does not substitute
for running the new artifact workflow.

Keep machine identifiers, ISO paths, credentials, screenshots, and raw
native-test logs out of tracked records. Human approval remains required
for license acceptance, account/security changes, and permission grants.

## Consequences

A Windows launcher adds a small executable and console-behavior maintenance.
Unix indirection avoids rewriting shared command paths during Relink. Manual
PATH setup is explicit; installing the link alone does not guarantee a bare
command resolves in every terminal. Bundle removal does not clean abandoned
integration state automatically. App and system runtime installation remain
separate from per-user CLI integration. Code signing, notarization, automatic
updates and a TUI are not provided by this decision.

## References

- [Current architecture](../ARCHITECTURE.md#desktop-bundled-command-integration).
- [Desktop workflows and manual PATH guidance](../USER_GUIDE.md).
- [Installer configuration and observed status](../../internal/desktop/cliinstall/manager.go#L21)
  (`internal/desktop/cliinstall/manager.go:21`).
- [Scoped request guards](../../internal/desktop/handler/cli_install.go#L50).
- [Production listener binding](../../cmd/desktop/main.go#L70).
- [Bundle packager](../../cmd/package-desktop/main.go).
- [Configured native artifact matrix](../../.github/workflows/desktop-artifacts.yml#L17).
