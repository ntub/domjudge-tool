# PR-5 — Email use-case

## Goal

Deliver the Layer 3 use-case package that sends templated credential emails:

- **`internal/email`** — load a template directory (subject + body), render
  per-user variable substitution, and drive an SMTP `Sender` to deliver a
  `[]model.CreateUser` credential batch.

This package unblocks **PR-6** (CLI command `send-user-accounts`) and no
other PR in Phase 1.

## Files created / modified

### Source

| Path | Role |
|------|------|
| `internal/email/email.go` | `Template`, `LoadTemplate`, `(*Template).Render`, `Sender` interface, `DialOptions`, `Dial`, `SMTPSender`, `SendOptions`, `SendUserAccounts`. |
| `internal/email/email_test.go` | Tests for all exported symbols. |

### Fixtures (committed with RED commits)

| Path | Purpose |
|------|---------|
| `testdata/email/valid/subject.txt` | `Hello $username, your password is $password on $server_host` |
| `testdata/email/valid/body.html` | `<p>Hello $name ($affiliation), email: $email</p>` |
| `testdata/email/text_only/subject.txt` | Same subject text as `valid/` |
| `testdata/email/text_only/body.txt` | Plain-text variant: `Hello $name ($affiliation), email: $email` |
| `testdata/email/both_body/subject.txt` | Same subject text as `valid/` |
| `testdata/email/both_body/body.html` | Same as `valid/body.html` |
| `testdata/email/both_body/body.txt` | Same as `text_only/body.txt` |
| `testdata/email/no_subject/body.html` | Body only — `subject.txt` absent |
| `testdata/email/no_body/subject.txt` | Subject only — both `body.html` and `body.txt` absent |

No PII, no real SMTP host, no live email addresses in any fixture.

## Architecture

### Package boundary

`internal/email` is a Layer 3 package. It depends on:

- Layer 1: `internal/model` (`model.CreateUser`) and `internal/config`
  (`config.DomServerClient.Host`).
- Standard library only for SMTP (`net/smtp`, `crypto/tls`, `net`,
  `mime`, `encoding/base64`, `os`).

It **must not** print to stdout — all output (errors, progress) is the
caller's responsibility. The CLI layer (`internal/cli`, PR-6) owns
printing. It never writes files — I/O flows through the `Sender` interface
and `net` connections already established by `Dial`.

### Function signatures

```go
// Package email provides the templated credential mailer use case.
package email

// Template holds the loaded and parsed templates for a single email batch.
type Template struct {
	Subject  string // raw $-substitution text for the email subject
	Body     string // raw $-substitution text for the email body
	BodyHTML bool   // true if body.html was loaded; false if body.txt
}

// LoadTemplate loads subject.txt and a body template from dir.
//
// HTML is preferred: if both body.html and body.txt exist, body.html is
// selected (mirrors Python list(glob("body.html")) + list(glob("body.txt"))
// picking index 0).
//
// Errors (all wrapped with %w):
//   - dir does not exist or is not a directory
//   - subject.txt not found in dir
//   - neither body.html nor body.txt found in dir
func LoadTemplate(dir string) (*Template, error)

// Render applies vars to t using $var / ${var} substitution (os.Expand
// semantics). Unknown keys expand to "" (safe_substitute semantics — no
// error on missing keys). Returns the Q-encoded subject and the raw body.
//
// Subject is encoded unconditionally via mime.QEncoding.Encode("UTF-8", …)
// for non-ASCII safety. Body is returned as-is; MIME assembly handles
// transfer encoding.
func (t *Template) Render(vars map[string]string) (subject, body string)

// DialOptions configures a Dial call.
type DialOptions struct {
	Host     string
	Port     int
	UseSSL   bool
	Timeout  time.Duration // 0 → no dial timeout
	Username string        // "" → skip AUTH
	Password string        // "" → skip AUTH
}

// Sender is the narrow interface for delivering a pre-built MIME message.
// *smtpSender satisfies this in production; tests substitute a stub.
type Sender interface {
	// Send delivers msg to the recipients. msg is a complete RFC 5322
	// message (headers + body) as bytes. ctx cancellation is honoured:
	// a cancelled ctx interrupts any blocking SMTP op (MAIL/RCPT/DATA).
	Send(ctx context.Context, from string, to []string, msg []byte) error
	// Close shuts down the SMTP connection cleanly (QUIT; fallback CLOSE
	// on error, mirroring Python SMTP.close()).
	Close() error
}

// Dial opens an SMTP connection and returns a Sender.
//
// SSL path (UseSSL=true): tls.DialWithDialer + smtp.NewClient.
// Plain path (UseSSL=false): net.Dialer.DialContext + smtp.NewClient.
// Both paths: EHLO after connection. STARTTLS is not implemented
// (mirrors Python's commented-out self.connection.starttls()).
// AUTH is performed only when both Username and Password are non-empty.
func Dial(ctx context.Context, opts DialOptions) (Sender, error)

// SendOptions configures a SendUserAccounts call.
type SendOptions struct {
	FromEmail string // From: header value and recipient-fallback domain source
}

// SendUserAccounts sends a templated credential email to each user in users.
//
// Recipient resolution: user.Email if non-nil and non-empty; otherwise
// "{user.Username}@{domain}" where domain is extracted via
// strings.LastIndex(opts.FromEmail, "@"). A malformed FromEmail (no "@"
// or trailing "@") falls back to domain="localhost".
//
// Template vars (fixed set): username, name, email (empty string if nil),
// affiliation (empty string if nil), password (empty string if nil),
// server_host (cfg.Host).
//
// Abort-on-first-error: returns on the first sender.Send failure,
// wrapping the error with the failing user's username via %w.
//
// The sender argument is injected so tests can substitute a stub without
// opening a real SMTP connection. The caller is responsible for calling
// sender.Close() in all paths (typically via defer).
func SendUserAccounts(
	ctx    context.Context,
	sender Sender,
	tmpl   *Template,
	users  []model.CreateUser,
	cfg    *config.DomServerClient,
	opts   SendOptions,
) error
```

### MIME assembly

Each outbound message is a raw RFC 5322 byte slice built inside
`SendUserAccounts` via an unexported helper. Headers in order:

```
From: <from_email>
To: <comma-separated recipients>
Subject: <mime.QEncoding.Encode("UTF-8", rendered_subject)>
Date: <time.Now().Format(time.RFC1123Z)>
Message-ID: <<random_hex>.<unix_nano>@<domain>>
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8   (or text/plain; charset=UTF-8)
Content-Transfer-Encoding: base64

<base64-encoded body, 76-char lines>
```

- `Date:` is included explicitly (RFC 5322 §3.6.1 mandatory field).
  `net/smtp` does NOT synthesise headers — the message bytes must contain
  all required fields. Mirrors Python `formatdate(localtime=True)`.
- `Message-ID:` domain is extracted from `opts.FromEmail` via the same
  `strings.LastIndex` rule used for recipient fallback.
- `Content-Type` is `text/html` when `tmpl.BodyHTML == true`, otherwise
  `text/plain`.
- Body is base64-encoded (76-char line wrap, RFC 2045 §6.8) so non-ASCII
  and multi-byte UTF-8 characters survive transit.

### SMTP lifecycle

```
Dial(ctx, opts)
    → net.Dialer.DialContext (plain) / tls.DialWithDialer (SSL)
    → smtp.NewClient(conn, host)
    → client.Hello("localhost")    // EHLO
    // STARTTLS not implemented (intentional parity with Python)
    → client.Auth(...)             // only when Username+Password non-empty
    → return &SMTPSender{client}

smtpSender.Send(ctx, from, to, msg)
    → goroutine: on ctx.Done() → conn.SetDeadline(time.Now()) // unblocks any op
    → client.Mail(from)
    → client.Rcpt(to[i]) for each recipient
    → wc = client.Data()  → io.Copy(wc, bytes.NewReader(msg))  → wc.Close()

SMTPSender.Close()
    → client.Quit()               // graceful
    → client.Close() on Quit error  // fallback, mirrors Python try/finally
    → connection = nil
```

## Behavioural deviations from Python

| Deviation | Python | Go |
|-----------|--------|----|
| Template substitution | `string.Template.substitute(**kwargs)` — raises `KeyError` on missing vars | `(*Template).Render` uses `os.Expand` safe-substitute semantics (unknown keys → `""`). `(*Template).Validate()` + `SendUserAccounts` pre-flight adds fail-fast: unknown placeholders abort the batch before any `Send` call. |
| Template variable scope | All CSV column keys forwarded via `**item` (`username`, `name`, `email`, `affiliation`, `password`, and any extra columns) | Fixed set: `username`, `name`, `email`, `affiliation`, `password`, `server_host`. `Validate()` rejects any placeholder outside this set. User-confirmed: 固定欄位，不需要傳入額外 CSV 欄位。 |
| Subject encoding | Raw string set on MIME header (Python `email` lib may encode internally) | `mime.QEncoding.Encode("UTF-8", subject)` applied unconditionally before header assembly |
| `Date:` header | `email.utils.formatdate(localtime=True)` — set by `MIMEText` automatically | `time.Now().Format(time.RFC1123Z)` added explicitly; `net/smtp` does not synthesise headers |
| SMTP connection class | `smtplib.SMTP_SSL` for SSL; `smtplib.SMTP` for plain | `tls.DialWithDialer` + `smtp.NewClient` for SSL; `net.Dialer.DialContext` + `smtp.NewClient` for plain |
| STARTTLS | Commented out: `# self.connection.starttls()` | Not implemented; `// STARTTLS not implemented` comment cross-references Python source |
| Recipient fallback source | `_, domain = from_email.split("@")` — raises `ValueError` on malformed input | `strings.LastIndex` — malformed `FromEmail` (no `@` or trailing `@`) → domain `"localhost"` |
| Progress bar | `typer.progressbar(dataset.dict)` | No progress in library code; CLI layer (PR-6) may add one |
| `is_exist` column | `item.pop("is_exist", None)` before template kwargs | `model.CreateUser.IsExist` is a pointer field not in the fixed template var map; no special handling needed |
| `email` sentinel | `item["email"] = None if not item.get("email")` before `CreateUser(**item)` | `model.CreateUser.Email == nil` (pointer); `SendUserAccounts` uses `""` in template vars if nil |
| Body content encoding | Python `MIMEText("utf-8")` manages encoding | Body base64-encoded; `Content-Transfer-Encoding: base64` header set explicitly |

## TDD anchors

| Test | Location | What it locks |
|------|----------|---------------|
| `TestLoadTemplate_PrefersHTMLOverText` | `email_test.go` | `testdata/email/both_body/` → `tmpl.BodyHTML == true` and `tmpl.Body` contains HTML content — mirrors Python `list(glob("body.html")) + list(glob("body.txt"))` picking index 0. |
| `TestSendUserAccounts_AbortOnFirstError` | `email_test.go` | Stub sender fails on first `Send` call → `SendUserAccounts` returns a non-nil error wrapping the failing username; second user's `Send` is never called. |
| `TestSendUserAccounts_RecipientFallback` | `email_test.go` | User with `Email == nil` → recipient is `"{username}@{domain}"` where domain is extracted from `opts.FromEmail`; verified via stub that captures the `to` argument. |

## Tests written first (RED) — per function

Per the confirmed TDD rule: each function lands as its own
red→green→refactor commit triple.

### `LoadTemplate`

| Test | Asserts |
|------|---------|
| **`TestLoadTemplate_PrefersHTMLOverText`** | `testdata/email/both_body/` → `BodyHTML==true`; `Body` contains HTML content (not text). |
| `TestLoadTemplate_HTMLOnly` | `testdata/email/valid/` → `BodyHTML==true`; no error. |
| `TestLoadTemplate_TextOnly` | `testdata/email/text_only/` → `BodyHTML==false`; no error. |
| `TestLoadTemplate_MissingSubject` | `testdata/email/no_subject/` → non-nil error. |
| `TestLoadTemplate_MissingBody` | `testdata/email/no_body/` → non-nil error. |
| `TestLoadTemplate_DirNotExist` | Non-existent path → non-nil error. |

### `(*Template).Render`

| Test | Asserts |
|------|---------|
| `TestRender_AllVars` | All six fixed vars supplied → subject and body contain substituted values. |
| `TestRender_SubjectQEncoded` | Non-ASCII subject text → returned subject string is in `=?UTF-8?q?...?=` form. |
| `TestRender_MissingVar_ExpandsToEmpty` | Template contains `$unknown` not in `vars` map → rendered to `""` without error. |
| `TestRender_DollarBraceSyntax` | `${username}` and `$username` both substitute correctly. |

### `Dial`

| Test | Asserts |
|------|---------|
| `TestDial_PlainConnects` | Local `net.Listen` goroutine serving minimal SMTP greeting + EHLO response → `Dial` returns non-nil `Sender` without error. |
| `TestDial_WithAuth` | Fake SMTP handles AUTH PLAIN handshake → `Dial` with non-empty Username+Password authenticates successfully. |
| `TestDial_SSLConnects` | Local TLS listener with self-signed cert → `Dial(UseSSL=true, InsecureSkipVerify)` connects and returns `Sender`. |

### `Sender` / `SMTPSender.Send`

`SMTPSender.Send` is exercised indirectly through `SendUserAccounts` tests
using a stub. Direct tests against a fake SMTP listener are optional if
the `TestDial_*` suite plus coverage from `SendUserAccounts` tests reaches
the ≥ 80 % threshold.

### `SendUserAccounts`

| Test | Asserts |
|------|---------|
| **`TestSendUserAccounts_AbortOnFirstError`** | Stub fails on call 1 → error returned; call 2 never made. Error wraps the failing username via `%w`. |
| **`TestSendUserAccounts_RecipientFallback`** | `user.Email == nil` → stub's `to` arg is `["{username}@{domain}"]`. |
| `TestSendUserAccounts_EmailUsed` | `user.Email` non-nil and non-empty → stub's `to` arg is `[*user.Email]`. |
| `TestSendUserAccounts_TemplateVarsPopulated` | Stub captures `msg` bytes; parsed headers confirm `Subject` contains rendered subject; body bytes contain rendered body. |
| `TestSendUserAccounts_MalformedFromEmail` | `opts.FromEmail` has no `@` → stub's `to` arg uses domain `"localhost"`; no panic. |
| `TestSendUserAccounts_EmptyUsers` | `users == nil` → returns nil without calling `sender.Send`. |
| `TestSendUserAccounts_ContextCancelled` | Context cancelled before first `Send` → function returns an error containing `context.Canceled`. |

## Edge cases covered

- **HTML preferred over text** — `testdata/email/both_body/` fixture; locked by TDD anchor #1.
- **Missing `subject.txt`** — `LoadTemplate` returns error, not panic.
- **Missing body (both absent)** — `LoadTemplate` returns error.
- **Unknown template vars** — expand to `""` (safe_substitute semantics); `Render` never returns an error.
- **nil `user.Email`** — recipient fallback to `{username}@{domain}`.
- **Malformed `FromEmail`** — no `@` or trailing `@` → domain `"localhost"`.
- **Abort-on-first-error** — loop halts at first `Send` failure; partial sends are not rolled back (mirrors Python).
- **Empty user list** — no `sender.Send` calls; returns nil.
- **STARTTLS intentionally absent** — documented in deviations; `// STARTTLS not implemented` comment in source.
- **`Date:` header required** — built explicitly in every outbound message.
- **Subject Q-encoding** — applied unconditionally; ASCII subjects round-trip safely through Q-encoding.
- **`is_exist` not in template vars** — `model.CreateUser.IsExist` is irrelevant to email sending; not included in the var map.
- **Context cancellation** — surfaced through first network call; test confirms error bubbles up.

## Definition of done

- [ ] One red→green→refactor commit triple per function (auditable in `git log`).
- [ ] `go test -race ./internal/email/...` green.
- [ ] Coverage ≥ 80 % on `internal/email`.
- [ ] `TestLoadTemplate_PrefersHTMLOverText`, `TestSendUserAccounts_AbortOnFirstError`,
      `TestSendUserAccounts_RecipientFallback` all pass (TDD anchors).
- [ ] `gofmt -s` / `go vet` / `golangci-lint run` clean (0 issues).
- [ ] No real SMTP host, credentials, or PII in any committed fixture.
- [ ] No `fmt.Print*` in `internal/email`; no `init()` outside registry wiring.
- [ ] Every exported symbol has a doc comment; `// Package email …` package-level comment present.
- [ ] `Sender` interface has exactly two methods; test stub requires only a 2-method struct.
- [ ] `Date:` header included explicitly in every outbound message (RFC 5322 §3.6.1).
- [ ] `// STARTTLS not implemented` comment in `Dial` references the Python commented-out line.
- [ ] `LoadTemplate` uses `os.ReadFile` / `os.Stat` so `testdata/email/` fixtures work without path magic.

## Cross-references

- Python source:
  - `domjudge_tool_cli/commands/emails/__init__.py` — `send_user_accounts` command (orchestration loop, CSV load, recipient fallback, `is_exist` pop)
  - `domjudge_tool_cli/utils/email/smtp.py` — `SMTP` class (`open`, `close`, `send_message`; EHLO lifecycle; STARTTLS commented-out)
  - `domjudge_tool_cli/utils/email/helper.py` — `load_template`, `EmailContext` (`__init__` glob ordering, `render_subject`, `render_body`, `mime` header assembly)
- Architecture: `docs/ARCHITECTURE.md` Layer 3 (email).
- Roadmap: `docs/ROADMAP.md` Phase 1.
- Conventions: `docs/CONVENTIONS.md`, `docs/TESTING.md`, `docs/COMMITS.md`.
- Reusable components:

| What | Path |
|------|------|
| `model.CreateUser` | `internal/model/user.go` |
| `config.DomServerClient.Host` | `internal/config/config.go` |
| Error sentinel pattern | `internal/client/web/errors.go` |

## Conventional Commit (merge)

```
feat(usecase): email
```

## Notes for downstream PRs

- **PR-6** (CLI users subtree) wires `email.LoadTemplate` + `email.Dial` +
  `email.SendUserAccounts` into the `send-user-accounts` subcommand. The
  CLI layer: (1) resolves the template dir from the CLI argument,
  (2) calls `Dial(ctx, dialOpts)` to open the SMTP connection,
  (3) defers `sender.Close()`, (4) reads `[]model.CreateUser` from a CSV
  file (using `csvio` from PR-1 + `exporter` from PR-4),
  (5) calls `SendUserAccounts(ctx, sender, tmpl, users, cfg, opts)`.
- `CreateUser.IsExist` is irrelevant to email sending; `SendUserAccounts`
  ignores it (no pop needed in Go — the field is simply absent from the
  template var map).
- No other Phase 1 PR depends on `internal/email`.
- **Phase 3** (Wails desktop app) will reuse `email.Dial` and
  `email.SendUserAccounts` directly through Wails bindings — the `Sender`
  interface injection avoids duplicating SMTP logic in TypeScript.
