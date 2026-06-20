# 0013. Phase 3 Desktop Frontend Stack

- **Status:** Proposed
- **Date:** 2026-06-20
- **Deciders:** @project-maintainer

## Context

Phase 3 ships a cross-platform desktop app (Wails v3) that wraps the
existing Go CLI. The original Phase 3 sketch in `docs/ROADMAP.md` paired
Wails with React 19 + TanStack (Query/Router/Table) + TS strict + Vite +
shadcn/ui + Tailwind v4. On review that stack is over-scoped for what the
app actually is: a single-user, local operator tool whose screens are
forms, data tables, modals, and — notably — batch operations with live
progress (the seven batch commands hardened in Phase 1.5:
`import-users-teams`, `send-user-accounts`, `render-user-accounts`,
`download-problems`, `download-submissions`, `download-scoreboard`).

Forces in play:

- **Operator workload, not a SPA.** Form/CRUD plus tables plus dialogs.
  There is no multi-user, no offline sync, no rich client routing need.
- **Single Go/DevOps developer.** A heavy JS toolchain (Node, Vite, a
  TS build, a component framework) is cost without payoff here. Keeping
  logic in Go and reusing `internal/` packages is the goal.
- **Batch progress is a first-class screen.** Several commands stream
  per-row progress; a server-push transport maps onto this directly.
- **System webview, not a controlled browser.** Wails uses WebKit
  (macOS), WebView2 (Windows), and WebKitGTK (Linux); the Linux engine
  version tracks the distro and can lag.
- **Go 1.23 module lock** and the no-casual-dependency rule from
  `CLAUDE.md` still apply.

Four server-driven (hypermedia) options were evaluated: htmx, Unpoly,
Hotwire Turbo, and Datastar. The question is which frontend stack to
build Phase 3 on.

## Decision

We decided to build the Phase 3 frontend as a **server-driven Datastar
stack**, replacing the original React/TanStack plan:

- **Wails v3** as the native shell (window, menus, file dialogs).
- **An in-process Go HTTP server** (`net/http` or `chi`) for page routes,
  action endpoints, and the SSE stream.
- **templ** for type-safe, server-rendered HTML.
- **Datastar** plus the official **Datastar Go SDK**
  (`github.com/starfederation/datastar/sdk/go/datastar`,
  `datastar.NewSSE(w, r)`) for hypermedia, client reactivity (signals),
  and SSE-driven DOM patching. This single library replaces the
  htmx + Alpine pair.
- **Tailwind v4 via the standalone CLI** (Node-free) plus **DaisyUI 5**
  for pure-CSS components and theming. DaisyUI ships an `.mjs` bundle
  loaded with `@plugin` so the standalone binary needs no npm.
- **Theme** follows the OS appearance via `prefers-color-scheme`, with a
  manual Light/Dark/System override persisted in the SQLite store from
  ADR 0012. Datastar signals drive the toggle.

The status stays `Proposed` until the spikes in *Consequences* pass.
Because Datastar is the youngest option, htmx (+ Alpine) is the named
fallback if those spikes fail.

### Alternatives considered

- **React 19 + TanStack + shadcn/ui + TS strict (original plan).**
  Rejected as over-engineered for a form-and-table operator tool. The
  toolchain and client-state machinery solve problems this app does not
  have, and they pull the project away from its Go center of gravity.
- **htmx (+ Alpine.js).** Mature, largest community, lowest bus-factor,
  and a proven templ + htmx path. Not chosen as primary only because
  Datastar folds htmx and Alpine into one dependency and its SSE-first
  model fits the batch-progress requirement more directly. Retained as
  the conservative fallback if the Datastar spikes fail.
- **Hotwire Turbo (+ Stimulus).** Backend-agnostic in principle, but its
  idioms, docs, and Turbo Streams format are Rails-shaped; a Go backend
  swims upstream, and Turbo Drive's page-replacement model is a poor fit
  for a desktop app. Rejected.
- **Unpoly.** Batteries-included overlays, forms, and navigation that a
  dialog-heavy operator UI would use well; mature and server-agnostic.
  Rejected as primary because it lacks Datastar's streaming model for
  batch progress and still wants a client-reactivity sprinkle. Kept as a
  secondary alternative if built-in overlays/forms later outweigh
  streaming.
- **Keep Alpine.js as a separate client-state layer.** Dropped:
  Datastar's signals provide client reactivity, so a separate Alpine
  dependency is redundant.

## Consequences

- Positive: one hypermedia + reactivity library (Datastar) instead of
  htmx + Alpine, shrinking the JS surface and the build.
- Positive: SSE-first patching maps onto the existing batch commands —
  a handler streams `datastar.NewSSE` patches per completed row, which is
  cleaner than htmx's SSE extension plus a separate reactivity layer.
- Positive: a first-class Go SDK and templ integration keep logic in Go
  and reuse `internal/`.
- Positive: the Tailwind standalone CLI plus DaisyUI keeps the build
  Node-free (verified), and DaisyUI's pure-CSS components do not conflict
  with Datastar's JS.
- Positive: theming follows the OS with a persisted manual override.
- Negative: Datastar is the youngest option (v1.0, small community,
  fast-moving). For a long-lived project this is real bus-factor risk.
- Negative: SSE transport through Wails is unverified. The Wails v3
  `AssetsHandler` is an `http.Handler`, but flushing for `text/event-
  stream` is not confirmed and embedded asset servers often buffer.
  Mitigation: run an in-process localhost HTTP listener dedicated to SSE
  and point the webview's `EventSource` at it, bypassing `AssetsHandler`.
- Negative: Tailwind v4's modern-CSS baseline (Safari 16.4+, Chrome
  111+, Firefox 128+) may exceed older Linux WebKitGTK builds; the
  fallback is Tailwind v3.4.
- Negative: Wails v3 is alpha; that risk is tracked separately at Phase 3
  start.

### Acceptance criteria (spikes before flipping to Accepted)

1. SSE works end to end inside the Wails webview — preferably via the
   localhost listener — demonstrated by an `import-users-teams`
   live-progress screen.
2. Tailwind v4 + DaisyUI render correctly on every target webview
   (macOS WebKit, Windows WebView2, Linux WebKitGTK on Ubuntu 24.04+);
   otherwise fall back to Tailwind v3.4.
3. Datastar v1.x API stability is acceptable for the project's lifetime,
   or the htmx (+ Alpine) fallback is confirmed viable.

- Follow-up: if a spike fails, supersede this ADR with an htmx (+ Alpine)
  or Unpoly decision rather than forcing Datastar.
- Follow-up: verify Wails v3, Datastar, templ, Tailwind v4, and DaisyUI
  against current docs with `ctx7` at implementation start, per
  `CLAUDE.md`.

## References

- [0012. Phase 3 Desktop Local Datastore](0012-phase3-local-datastore.md)
- [0014. Go Toolchain Bump for Phase 3](0014-go-toolchain-bump-for-phase3.md)
- [`docs/ROADMAP.md`](../ROADMAP.md) — Phase 3 scope and pre-research
  checklist.
- [`docs/phase3/SPEC.md`](../phase3/SPEC.md) — Phase 3 specification.
- [Datastar (GitHub)](https://github.com/starfederation/datastar) and the
  [Go SDK](https://pkg.go.dev/github.com/starfederation/datastar/sdk/go).
- [Datastar backend requests and SSE events](https://data-star.dev/guide/backend_requests_sse_events).
- [templ × Datastar](https://templ.guide/server-side-rendering/datastar/).
- [daisyUI with the Tailwind standalone CLI](https://daisyui.com/docs/install/standalone/).
- [Tailwind CSS compatibility (v4 browser baseline)](https://tailwindcss.com/docs/compatibility).
- [Wails v3 dynamic assets / AssetsHandler](https://wails.io/docs/guides/dynamic-assets/).
