# 0013. Desktop Frontend: Wails, templ, and Datastar

- **Status:** Accepted
- **Date:** 2026-06-20
- **Deciders:** @project-maintainer

## Context

The native Desktop preview is a single-user operator tool built around
forms, tables, file dialogs, and long-running batch operations. Its
business logic already exists in Go packages shared with the CLI. A
separate SPA state model and JavaScript business layer would duplicate
those responsibilities.

Server-rendered screens fit the workload, while SSE carries per-operation
progress without polling. The frontend still runs in a platform webview:
WebKit on macOS, WebView2 on Windows, and WebKitGTK on Linux. Rendering
compatibility and native SDK requirements remain platform-specific.

## Decision

Use a **server-driven frontend** with Wails v3, `net/http`, templ, and
Datastar. This is the implemented Desktop architecture.

### Current composition

1. **One Go module.** `cmd/domjudge-tool` and `cmd/desktop` share
   `github.com/ntub/domjudge-tool` and its `internal/` operation packages.
   The Desktop calls those packages directly; it does not start the CLI
   for normal screen operations or duplicate domain logic in TypeScript.
   Keep the canonical module path stable.
2. **Wails v3 native shell.** `cmd/desktop` owns the window and native
   dialogs behind the `desktop` build tag. The selected Wails dependency
   is a v3 beta, not a stable release; exact versions are in `go.mod`.
3. **An in-process loopback HTTP server.** Production binds
   `127.0.0.1:9900` before opening the webview. `net/http.ServeMux` owns page,
   action, fragment, static-asset, and SSE routes. It does not depend on
   Wails asset-server buffering for progress streams.
4. **templ-rendered HTML.** Synchronous screens use ordinary forms and
   Post/Redirect/Get where appropriate. Batch workflows create an operation
   log, redirect to a run page, and stream progress/results through SSE.
   Attachment responses remain downloads rather than navigating forms.
5. **Datastar.** The browser asset and
   `github.com/starfederation/datastar-go/datastar` provide hypermedia
   interactions and Go-side SSE DOM patches. There is no separate React,
   TanStack, htmx, or Alpine application layer.
6. **Locally embedded styling and scripts.** Tailwind v4 compiles utility
   CSS from templ sources using the npm-installed `@tailwindcss/cli`.
   DaisyUI 5 CSS and the hand-written `desktop.css` provide components,
   themes, and layout. CSS and Datastar JavaScript are embedded in the Go
   binary; pages do not depend on CDN availability.
7. **Persisted settings.** Dark is the initial theme. Settings offers Dark,
   Light, and System choices; SQLite stores the selection, while
   `localStorage` applies the current selection immediately and restores it
   during page rendering. This is not a Datastar-signals-only state model.

Node.js and npm are **asset-build dependencies only**, with the runtime
policy in [ADR 0015](0015-security-runtime-and-dependency-upgrades.md).
They are not required to run the packaged Desktop or CLI. The frontend has
no React/TypeScript application build or JavaScript test-runner dependency.
Generated templ Go files and embedded assets are committed together with
their sources and rebuilt through the existing tasks.

### Alternatives considered

- **React, TanStack, Vite, TypeScript, and a component framework.**
  Rejected: their client routing, state, and application-build machinery
  exceed this form-and-table tool's needs and create a second place for
  operation logic.
- **htmx plus Alpine.** A viable server-rendered design, but two separate
  interaction/reactivity layers are unnecessary alongside Datastar's
  streaming and patching model.
- **Hotwire Turbo plus Stimulus.** Its page-replacement and Rails-shaped
  conventions fit the Go batch-progress screens less directly.
- **Unpoly.** Its navigation and overlay model does not provide the chosen
  SSE-first progress model.
- **CDN-hosted assets.** Rejected: a local operator tool must not need a
  third-party asset request to render or operate.

## Consequences

- Shared Go operations remain the source of behavior for CLI and Desktop.
  The independent bundled CLI described by
  [ADR 0016](0016-desktop-bundled-cli.md) still runs without starting Wails.
- The renderer is HTML/CSS with a small embedded interaction layer, rather
  than a separately maintained SPA.
- The same HTTP surface can serve an isolated preview for browser-driven
  checks. Native dialogs, downloads, webview behavior, and packaging still
  need platform-native acceptance; browser checks alone do not establish it.
- SSE and operation logs are implemented contracts, not pending spikes.
  Keep streaming on the loopback server rather than assuming an embedded
  asset handler flushes events correctly.
- Datastar and pre-release Wails introduce upstream compatibility risk.
  Upgrades require their current API documentation, regenerated artifacts,
  and the checks appropriate to each build path.
- Modern CSS support depends on the target webview and Linux userspace.
  This decision does not promise compatibility with every older distro or
  replace native GUI acceptance with cross-compilation.

## References

- [Current architecture and package boundaries](../ARCHITECTURE.md).
- [Desktop workflows](../USER_GUIDE.md).
- [Desktop local datastore](0012-phase3-local-datastore.md).
- [Runtime and dependency upgrade contract](0015-security-runtime-and-dependency-upgrades.md).
- [Production server and window composition](../../cmd/desktop/main.go#L51)
  (`cmd/desktop/main.go:51`).
- [Layout and local asset loading](../../internal/desktop/templates/layout.templ#L3).
- [Datastar SSE usage](../../internal/desktop/handler/import_users.go#L155).
- [Settings persistence and immediate theme selection](../../internal/desktop/templates/settings.templ#L47).
- [Asset build task](../../mise.toml) and [frontend manifest](../../frontend/package.json).
- [Current Go dependency manifest](../../go.mod).
