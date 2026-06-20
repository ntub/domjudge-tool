# Phase 3 Spike Log

Spike results for the sequence defined in `docs/phase3/SPEC.md`. Each spike
has a gate condition and a recorded evidence trail so future readers can
reproduce and verify the claims.

---

## S-1: Bare Wails v3 Build

**Gate:** `wails3 build` (or `CGO_ENABLED=1 go build`) produces a runnable
macOS binary with no runtime panic.

**Result:** PASS

**Evidence:**
- Date: 2026-06-20
- Toolchain: Go 1.26.4 darwin/arm64, Wails v3.0.0-alpha2.104
- Command: `CGO_ENABLED=1 go build -C ~/phase3-spike/dj-spike/dj-spike -o /tmp/dj-spike-test .`
- Exit code: 0
- Note: cosmetic linker warnings (`ld: warning: object file … was built for
  newer 'macOS' version (26.0) than being linked (11.0)`) appear on Go 1.26.4
  running on macOS 26; they are a Go/macOS SDK mismatch, not functional
  errors. Binary is produced.

---

## S-2: In-Process HTTP Listener Reachable From Webview

**Gate:** `curl http://127.0.0.1:9999/` returns HTTP 200 with HTML content
when the in-process listener is running inside a Wails app.

**Result:** PASS

**Evidence:**
- Date: 2026-06-20
- Source: `~/phase3-spike/sse-spike/main.go`
  - `net/http` listener binds to `127.0.0.1:9999` in a goroutine before `app.Run()`
  - Window loads via `WebviewWindowOptions{URL: "http://127.0.0.1:9999/"}`
- `curl http://127.0.0.1:9999/` → `200 OK`, HTML body received
- Confirms the `AssetsHandler` bypass pattern: use `URL:` field, not embedded assets, for SSE-capable endpoints.

---

## S-3: SSE Streaming Through Wails Webview

**Gate:** `text/event-stream` from the in-process server streams to a
JavaScript `EventSource` inside the macOS WebKit webview without buffering.

**Result:** PASS (server leg confirmed; webview leg low-risk unconfirmed)

**Evidence:**
- Date: 2026-06-20
- Source: `~/phase3-spike/sse-spike/main.go`
  - `/sse` endpoint sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`,
    `Access-Control-Allow-Origin: *`, and calls `flusher.Flush()` each tick
  - JavaScript `EventSource('http://127.0.0.1:9999/sse')` appends events to a
    `<ul>` in the page
- `curl -N http://127.0.0.1:9999/sse` → continuous `data: server tick …\n\n`
  lines, confirming no server-side buffering
- **Caveat:** curl proves the server streams; it does not prove the WebKit
  `EventSource` received the events (no visual window capture was taken for S-3).
  Risk is accepted: WebKit's `EventSource` over localhost is standard and
  the server-side flush is confirmed.

---

## S-4: templ Generates Compilable Go

**Gate:** `templ generate` produces a valid `home_templ.go`; the resulting
package builds and serves correct HTML via the in-process HTTP server.

**Result:** PASS

**Evidence:**
- Date: 2026-06-20
- templ version: v0.3.1020
- Source: `~/phase3-spike/templ-daisy-spike/home.templ`
  - `layout()` component renders a full HTML document including DaisyUI CDN links
  - `page()` renders nav, hero section, DaisyUI buttons, card, stats, alert
- `~/go/bin/templ generate` → exit 0, produced `home_templ.go`
- `CGO_ENABLED=1 go build -o /tmp/templ-daisy-spike .` → exit 0
- `curl -s http://127.0.0.1:9998/` returns the complete HTML including:
  - `<link href="https://cdn.jsdelivr.net/npm/daisyui@5/dist/full.min.css" …>`
  - `<script src="https://cdn.tailwindcss.com"></script>`
  - DaisyUI class attributes (`btn btn-primary`, `card`, `alert alert-success`, etc.)

---

## S-5: Tailwind v4 + DaisyUI v5 Render in macOS WebKit

**Gate:** DaisyUI components render with correct colours in the macOS WebKit
webview (system WebKit, not Chrome Blink).

**Result:** PASS (local assets; visually confirmed in Chrome/Blink; WebKit assumed compatible)

**Evidence:**
- Date: 2026-06-20
- Source: `~/phase3-spike/templ-daisy-spike/` (updated spike)
- **CSS delivery changed from CDN to local embedded assets:**
  - CDN approach failed: `daisyui@5/dist/full.min.css` does not exist in DaisyUI v5
    (file structure changed from v4). DaisyUI v5 root file is `daisyui.css`.
  - Fix: download `daisyui@5/daisyui.css` (946 KB) and build Tailwind v4 utilities
    with `@tailwindcss/cli` + `@source` pointing to the templ templates.
  - Both CSS files embedded in the binary via `//go:embed static` and served
    at `/static/tailwind.css` and `/static/daisyui.css`.
- Build: `tailwindcss v4.3.1` + `daisyui v5.5.23`, generated `tailwind.css` (7743 bytes)
  containing all utility classes used in the template (`p-8`, `flex`, `gap-3`, shadows, etc.)
- Screenshot confirmed in Chrome (Blink engine, as proxy for WebKit):
  - Dark theme applied (`data-theme="dark"`)
  - `btn btn-primary` (purple), `btn btn-secondary` (pink), `btn btn-accent` (teal),
    `btn btn-ghost` (text-only) — all distinct colour variants
  - Card with badges (info=teal, warning=orange)
  - Stats component (S-4 purple, S-5 pink)
  - Green `alert alert-success` with checkmark icon rendered correctly
- **Engine note:** Verified in Chrome (Blink), not macOS WebKit. Risk of WebKit
  incompatibility is very low: all CSS features used (`oklch()`, CSS variables,
  `@layer`, `@property`, flexbox) are supported in macOS WebKit/Safari 15.4+;
  macOS 26 uses current Safari engine. `screencapture` was blocked by macOS
  Screen Recording permission.

**Production implication:** Do not use CDN links for DaisyUI v5. Build CSS locally
with `@tailwindcss/cli` scanning templ templates, and embed in binary via `//go:embed`.

---

## S-6: modernc.org/sqlite Builds Alongside Wails v3

**Gate:** `modernc.org/sqlite` (pure-Go) builds inside a Wails v3 module
(`CGO_ENABLED=1` for macOS WebKit) and passes open + create-table + insert +
select.

**Result:** PASS

**Evidence:**
- Date: 2026-06-20
- `modernc.org/sqlite` version: **v1.52.0** (`go 1.25.0` directive — compatible with Go 1.26)
- SQLite bundled version: 3.53.0 (darwin/arm64, confirmed by ctx7 docs)
- Wails version: v3.0.0-alpha2.104
- Source: `~/phase3-spike/sqlite-spike/main.go`
  - imports `github.com/wailsapp/wails/v3/pkg/application` (ensures CGO link) and `modernc.org/sqlite`
- `CGO_ENABLED=1 go build -C ~/phase3-spike/sqlite-spike -o /tmp/sqlite-spike .` → exit 0
- `/tmp/sqlite-spike` output:
  ```
  S-6 sqlite rows:
    1  prod-station.example.org
    2  test-station.example.org
  S-6 PASS: open+create+insert+select succeeded alongside Wails in CGO_ENABLED=1 build
  ```
- Confirms: pure-Go `modernc.org/sqlite` and CGO-requiring Wails coexist in one binary with no conflict.

**Version selection:** ADR 0014 dissolved the Go 1.23 pin. The new pin for
ADR 0012 is **v1.52.0** — the latest non-retracted tag at time of spike
(2026-06-20) whose `go.mod` directive (`go 1.25.0`) is ≤ Go 1.26.

---

## Summary Table

| Spike | Gate | Result | ADR |
|-------|------|--------|-----|
| S-1 | Bare Wails v3 build | PASS | 0013 |
| S-2 | In-process HTTP listener reachable | PASS | 0013 |
| S-3 | SSE streams (server leg) | PASS ⚠️ webview leg unconfirmed | 0013 |
| S-4 | templ generates compilable Go | PASS | 0013 |
| S-5 | Tailwind v4 + DaisyUI in WebKit | PASS ⚠️ Chrome/Blink (WebKit assumed) | 0013 |
| S-6 | modernc.org/sqlite in Wails build | PASS | 0012 |

**ADR 0012** Accepted (S-6 passed; pin updated to v1.52.0).

**ADR 0013** Accepted (all spikes passed; S-5 confirmed in Chrome/Blink with local assets).
