package email

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// nowFn and randRead are package-level seams replaced by tests via
// SetDeterministicSeams to make buildMessage byte-deterministic.
var (
	nowFn    = time.Now
	randRead = rand.Read
)

// SetDeterministicSeams replaces nowFn and randRead for deterministic
// testing. Returns a restore function; call from t.Cleanup.
func SetDeterministicSeams(now func() time.Time, rnd func([]byte) (int, error)) func() {
	prevNow, prevRand := nowFn, randRead
	nowFn, randRead = now, rnd
	return func() { nowFn, randRead = prevNow, prevRand }
}

// SendOptions configures a SendUserAccounts call.
type SendOptions struct {
	FromEmail string // From: header value and recipient-fallback domain source

	// Reporter is the optional progress sink. Library code never
	// constructs a Reporter; the CLI root PreRunE is the only
	// production construct site. Nil is treated as [progress.Nop] so
	// existing callers (and Phase 1 tests that pre-date PR-9) need no
	// change.
	Reporter progress.Reporter
}

// SendUserAccounts sends a templated credential email to each user in users.
//
// Recipient resolution: u.Email when non-nil and non-empty; otherwise
// "{u.Username}@{domain}" where domain is the suffix of opts.FromEmail
// after its last "@". A malformed FromEmail (no "@" or trailing "@")
// falls back to domain="localhost".
//
// Template variables (fixed keys): username, name, email, affiliation,
// password, server_host. Nil Optional pointers expand to "".
//
// Abort-on-first-error: returns on the first sender.Send failure,
// wrapping the error with the failing user's username via %w.
//
// Honours ctx cancellation: returns ctx.Err() (wrapped) before any
// further send if ctx is done at the start of an iteration.
func SendUserAccounts(
	ctx context.Context,
	sender Sender,
	tmpl *Template,
	users []model.CreateUser,
	cfg *config.DomServerClient,
	opts SendOptions,
) error {
	if err := tmpl.Validate(); err != nil {
		return err
	}
	rep := opts.Reporter
	if rep == nil {
		rep = progress.Nop()
	}
	rep.Start(len(users), "sending")
	defer rep.Finish()
	for _, user := range users {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("email: send: %w", err)
		}
		msg, recipient, err := BuildEML(tmpl, user, cfg, opts)
		if err != nil {
			return fmt.Errorf("email: build message for %s: %w", user.Username, err)
		}
		if err := sender.Send(ctx, opts.FromEmail, []string{recipient}, msg); err != nil {
			return fmt.Errorf("email: send to %s: %w", user.Username, err)
		}
		rep.Tick()
	}
	return nil
}

// templateVars builds the fixed variable map consumed by Template.Render.
func templateVars(u model.CreateUser, cfg *config.DomServerClient) map[string]string {
	vars := map[string]string{
		"username":    u.Username,
		"name":        u.Name,
		"email":       "",
		"affiliation": "",
		"password":    "",
		"server_host": cfg.Host,
	}
	if u.Email != nil {
		vars["email"] = *u.Email
	}
	if u.Affiliation != nil {
		vars["affiliation"] = *u.Affiliation
	}
	if u.Password != nil {
		vars["password"] = *u.Password
	}
	return vars
}

// recipientFor resolves the To: address for u, falling back to
// "<username>@<fallbackDomain>" when Email is nil or empty.
func recipientFor(u model.CreateUser, fallbackDomain string) string {
	if u.Email != nil && *u.Email != "" {
		return *u.Email
	}
	return u.Username + "@" + fallbackDomain
}

// domainFromEmail extracts the domain portion of from. Returns
// "localhost" when from has no "@" or ends with "@".
func domainFromEmail(from string) string {
	idx := strings.LastIndex(from, "@")
	if idx == -1 || idx == len(from)-1 {
		return "localhost"
	}
	return from[idx+1:]
}

// buildMessage assembles a complete RFC 5322 message: required headers
// + base64-encoded body wrapped to 76-char lines per RFC 2045 §6.8.
//
// net/smtp does NOT inject Date or Message-ID — they are mandatory and
// added explicitly here.
func buildMessage(from, to, subject, body string, isHTML bool, domain string) ([]byte, error) {
	contentType := "text/plain; charset=UTF-8"
	if isHTML {
		contentType = "text/html; charset=UTF-8"
	}
	msgID, err := makeMessageID(domain)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "From: %s\r\n", from)
	fmt.Fprintf(&sb, "To: %s\r\n", to)
	fmt.Fprintf(&sb, "Subject: %s\r\n", subject)
	fmt.Fprintf(&sb, "Date: %s\r\n", nowFn().Format(time.RFC1123Z))
	fmt.Fprintf(&sb, "Message-ID: %s\r\n", msgID)
	sb.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&sb, "Content-Type: %s\r\n", contentType)
	sb.WriteString("Content-Transfer-Encoding: base64\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(wrapBase64(body))
	return []byte(sb.String()), nil
}

// wrapBase64 base64-encodes body and folds the output to 76-char lines
// (RFC 2045 §6.8). Each line, including the final one, is terminated
// with CRLF.
func wrapBase64(body string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	const lineLen = 76
	var sb strings.Builder
	for i := 0; i < len(encoded); i += lineLen {
		end := i + lineLen
		if end > len(encoded) {
			end = len(encoded)
		}
		sb.WriteString(encoded[i:end])
		sb.WriteString("\r\n")
	}
	return sb.String()
}

// makeMessageID returns "<<random_hex>.<unix_nano>@<domain>>".
func makeMessageID(domain string) (string, error) {
	var b [8]byte
	if _, err := randRead(b[:]); err != nil {
		return "", fmt.Errorf("email: random bytes: %w", err)
	}
	return fmt.Sprintf("<%s.%d@%s>", hex.EncodeToString(b[:]), nowFn().UnixNano(), domain), nil
}
