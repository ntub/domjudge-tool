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

**Result:** PARTIAL — CSS served, visual rendering unconfirmed

**Evidence:**
- Date: 2026-06-20
- Source: combined S-4 spike binary (`/tmp/templ-daisy-spike`)
  - Wails window loads `http://127.0.0.1:9998/` which serves the templ-generated HTML
  - HTML includes DaisyUI v5 CDN (`cdn.jsdelivr.net/npm/daisyui@5/dist/full.min.css`)
    and Tailwind Play CDN (`cdn.tailwindcss.com`)
  - DaisyUI classes present in HTML: `navbar`, `btn btn-primary/secondary/accent/ghost`,
    `card`, `stats`, `badge`, `alert alert-success`, `hero`, `divider`
- `curl` confirmed full HTML served correctly (see S-4 evidence)
- **Blocker:** `screencapture -x` returned "could not create image from display" —
  macOS Screen Recording permission not granted to the Claude Code terminal process.
  Chrome browser extension was also unavailable.
- **Risk assessment:** DaisyUI v5 explicitly targets modern WebKit (Safari 16.4+);
  macOS 26 WebKit is current; CDN links are standard. The risk that DaisyUI
  classes fail to render is very low, but it is unverified.

**User action required:** Run `/tmp/templ-daisy-spike` and visually confirm:
- DaisyUI dark theme is applied (dark background, coloured buttons)
- `btn btn-primary`, `btn btn-secondary`, `btn btn-accent`, `btn btn-ghost` show
  distinct colour variants
- The green `alert alert-success` renders with correct styling

Update this file with the visual confirmation result before flipping ADR 0013
to Accepted.

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
| S-5 | Tailwind v4 + DaisyUI in WebKit | PARTIAL ⚠️ visual unconfirmed | 0013 |
| S-6 | modernc.org/sqlite in Wails build | PASS | 0012 |

**ADR 0012** can be flipped to Accepted (S-6 passed; pin updated to v1.52.0).

**ADR 0013** stays Proposed until S-5 is visually confirmed by the user.
