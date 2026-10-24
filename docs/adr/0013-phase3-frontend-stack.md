# 0013. Web UI Frontend: templ, Datastar, and Tailwind

- **Status:** Accepted
- **Date:** 2026-06-20
- **Deciders:** @project-maintainer

## Context

The UI is a single-user operator interface built around forms, tables, and
long-running batch operations. Its business logic already exists in Go packages
shared with the CLI. A separate SPA state model and JavaScript business layer
would duplicate those responsibilities.

Server-rendered screens fit the workload, while SSE carries per-operation
progress without polling. Formerly hosted inside a Wails v3 native desktop
webview, the UI is now served directly from the CLI via `domjudge-tool web`
(see [ADR 0018](0018-local-web-ui-command.md)), opening in any modern web
browser. This eliminates code-signing barriers, native window toolchains,
WebKit/WebView2 runtime dependencies, and platform-specific bundle packaging.

## Decision

Use a **server-driven web frontend** with `net/http`, templ, Datastar, and
Tailwind v4, served directly by `domjudge-tool web`.
### Current composition

1. **One Go module.** The CLI commands and the web UI share
   `github.com/ntub/domjudge-tool` and its `internal/` operation packages.
   The web UI calls those packages directly; it does not start a subshell
   for normal screen operations or duplicate domain logic in TypeScript.
   Keep the canonical module path stable.
2. **Browser-based presentation.** `internal/cli/web` starts the local web
   server and launches the user's default browser. Native desktop windows,
   Wails dependencies, and platform webview bindings have been removed.
3. **An in-process HTTP server.** Production binds loopback (`127.0.0.1:9900`
   by default) or a user-specified host/port. `internal/webui/server` composes
   security middleware (token or basic authentication, DNS rebinding
   protection, CSRF protection, and security headers) and mounts the
   `net/http.ServeMux` for pages, actions, fragments, static assets, and SSE.
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
   DaisyUI 5 CSS and the hand-written `webui.css` provide components,
   themes, and layout. CSS and Datastar JavaScript are embedded in the Go
   binary; pages do not depend on CDN availability.
7. **Persisted settings.** Dark is the initial theme. Settings offers Dark,
   Light, and System choices; SQLite stores the selection, while
   `localStorage` applies the current selection immediately and restores it
   during page rendering. This is not a Datastar-signals-only state model.
Node.js and npm are **asset-build dependencies only**, with the runtime
policy in [ADR 0015](0015-security-runtime-and-dependency-upgrades.md).
They are not required to run the CLI or web UI. The frontend has
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

- Shared Go operations remain the source of behavior for CLI and web UI.
- The renderer is HTML/CSS with a small embedded interaction layer, rather
  than a separately maintained SPA.
- The same HTTP surface serves both the user's browser in production and an
  isolated preview (`cmd/e2e-server`) for deterministic tests and screenshot
  captures.
- SSE and operation logs are implemented contracts, not pending spikes.
  Keep streaming on the local server rather than assuming an embedded
  asset handler flushes events correctly.
- Upgrades require current API documentation, regenerated templ and CSS
  artifacts, and running the project test suites.
- All modern desktop browsers (Chrome/Chromium, Firefox, Safari, Edge) are
  supported without requiring platform CGO or webview libraries.

## References

- [Current architecture and package boundaries](../ARCHITECTURE.md).
- [Web UI workflows in the User Guide](../USER_GUIDE.md).
- [Web UI local datastore](0012-phase3-local-datastore.md).
- [Runtime and dependency upgrade contract](0015-security-runtime-and-dependency-upgrades.md).
- [Local web UI command contract](0018-local-web-ui-command.md).
- [Layout and local asset loading](../../internal/webui/templates/layout.templ#L3).
- [Datastar SSE usage](../../internal/webui/handler/import_users.go#L155).
- [Settings persistence and immediate theme selection](../../internal/webui/templates/settings.templ#L47).
- [Asset build task](../../mise.toml) and [frontend manifest](../../frontend/package.json).
- [Current Go dependency manifest](../../go.mod).
