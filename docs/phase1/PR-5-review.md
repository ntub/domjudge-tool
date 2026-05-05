# PR-5 Review — Email use-case

Adversarial review conducted via Codex on `feat/p1-pr5-email`.
Coverage: 84.6 % total; `template.go` 100 %, `send.go` 92 %, `smtp.go` 65 %.
TDD anchors: all three green.

## Findings

### High

**H1 — Unknown template vars silently expand to `""`, entire batch still sent**
(`internal/email/template.go:58-67`)

`Template.Render` maps missing keys to `""` via `os.Expand`; `SendUserAccounts`
does no pre-flight validation. Operator typo `$pasword` → every email is sent
without a password, function returns nil. Removes Python `Template.substitute`
fail-fast behaviour.

Fix: pre-validate all six fixed placeholders in `Render` or add a
`ValidateTemplate` step at the start of `SendUserAccounts`; add a test that a
misspelled `$password` aborts before any `sender.Send` call.

**H2 — `PlainAuth` rejects non-SSL remote SMTP with credentials**
(`internal/email/smtp.go:88-91`)

`smtp.PlainAuth` only transmits credentials over TLS or localhost; `UseSSL=false`
+ remote host → `"unencrypted connection"` error at `Auth` time. Existing tests
use `127.0.0.1` and miss this. Python `smtplib.SMTP.login` has no equivalent
restriction, so existing plain-SMTP+AUTH deployments break silently.

Fix: decide product policy — either implement a custom AUTH path for plain SMTP
or make `Dial` return an explicit error when `UseSSL=false` and credentials are
non-empty; document in CLI help text.

**H3 — `ctx` not propagated into `Sender.Send`; SMTP hang cannot be cancelled**
(`internal/email/send.go:46-56`)

`ctx.Err()` is checked only at loop start; `sender.Send` → `smtpSender.Send`
issues MAIL/RCPT/DATA as blocking `net/smtp` calls with no deadline. A stalled
server makes the entire batch uninterruptible even after CLI/Wails cancellation.
Existing cancellation test only covers the pre-first-send guard.

Fix: add `context.Context` to `Sender.Send`, or have `smtpSender` set
`SetDeadline` on the underlying `net.Conn` from ctx; add a test where the fake
SMTP server stalls at DATA and ctx cancellation returns within a deadline.

### Medium

**M1 — Non-`ErrNotExist` errors on `body.html` silently fall back to `body.txt`**
(`internal/email/template.go:39-46`)

Any read error (permission denied, I/O error) on `body.html` is swallowed and
`body.txt` is tried next. A misconfigured deployment with both files present
would silently send plain-text instead of HTML.

Fix: change the fallback condition to `errors.Is(err, os.ErrNotExist)` only;
propagate all other errors. Add a test for the "body.html exists but unreadable,
body.txt exists" scenario.

## Status

All findings fixed in TDD red→green pairs on `feat/p1-pr5-email`:

| Finding | Fix commit |
|---------|------------|
| M1 | `feat(email): green — M1 LoadTemplate only falls back to body.txt on ErrNotExist` |
| H1 | `feat(email): green — H1 Template.Validate + SendUserAccounts pre-flight check` |
| H2 | `feat(email): green — H2 custom plainAuth bypasses TLS restriction for Python parity` |
| H3 | `feat(email): green — H3 smtpSender.Send honours ctx via conn deadline` |

Ready to merge.
