package email_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/email"
	"github.com/ntub/domjudge-tool/internal/model"
)

// Compile-time check: stubSender must satisfy email.Sender.
var _ email.Sender = (*stubSender)(nil)

// sendCall captures the arguments of one stubSender.Send invocation.
type sendCall struct {
	from string
	to   []string
	msg  []byte
}

// stubSender is a Sender implementation that records calls and can be
// scripted to return an error on a specific call index.
type stubSender struct {
	calls   []sendCall
	failOn  int   // 0-indexed call number to fail; -1 = never fail
	failErr error // returned on the failOn-th call
}

func newStubSender() *stubSender { return &stubSender{failOn: -1} }

func (s *stubSender) Send(_ context.Context, from string, to []string, msg []byte) error {
	toCopy := append([]string(nil), to...)
	msgCopy := append([]byte(nil), msg...)
	s.calls = append(s.calls, sendCall{from: from, to: toCopy, msg: msgCopy})
	if s.failOn >= 0 && len(s.calls)-1 == s.failOn {
		return s.failErr
	}
	return nil
}

func (s *stubSender) Close() error { return nil }

// strPtr returns a pointer to the given string for Optional fields.
func strPtr(v string) *string { return &v }

// decodeBody splits a built RFC 5322 message at the header/body boundary,
// strips CRLF from the base64 payload, and returns the decoded body.
func decodeBody(t *testing.T, msg []byte) string {
	t.Helper()
	parts := bytes.SplitN(msg, []byte("\r\n\r\n"), 2)
	if len(parts) != 2 {
		t.Fatalf("missing \\r\\n\\r\\n separator in %q", msg)
	}
	raw := strings.ReplaceAll(string(parts[1]), "\r\n", "")
	raw = strings.TrimSpace(raw)
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("base64 decode body: %v (raw=%q)", err, raw)
	}
	return string(decoded)
}

// TDD ANCHOR #2 — name must match exactly.
func TestSendUserAccounts_AbortOnFirstError(t *testing.T) {
	stub := newStubSender()
	stub.failOn = 0
	stub.failErr = errors.New("smtp boom")

	tmpl := &email.Template{Subject: "hi", Body: "hey", BodyHTML: false}
	users := []model.CreateUser{
		{Username: "alice", Name: "Alice"},
		{Username: "bob", Name: "Bob"},
	}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "alice") {
		t.Errorf("error should mention failing user %q; got %v", "alice", err)
	}
	if !errors.Is(err, stub.failErr) {
		t.Errorf("error should wrap original; got %v", err)
	}
	if len(stub.calls) != 1 {
		t.Errorf("call count after abort = %d, want 1", len(stub.calls))
	}
}

// TDD ANCHOR #3 — name must match exactly.
func TestSendUserAccounts_RecipientFallback(t *testing.T) {
	stub := newStubSender()
	tmpl := &email.Template{Subject: "hi", Body: "hey", BodyHTML: false}
	users := []model.CreateUser{
		{Username: "bob", Name: "Bob", Email: nil},
	}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	if err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts); err != nil {
		t.Fatalf("SendUserAccounts: %v", err)
	}
	if len(stub.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(stub.calls))
	}
	wantTo := []string{"bob@example.com"}
	if !reflect.DeepEqual(stub.calls[0].to, wantTo) {
		t.Errorf("recipient = %v, want %v", stub.calls[0].to, wantTo)
	}
}

func TestSendUserAccounts_EmailUsed(t *testing.T) {
	stub := newStubSender()
	tmpl := &email.Template{Subject: "hi", Body: "hey", BodyHTML: false}
	users := []model.CreateUser{
		{Username: "alice", Name: "Alice", Email: strPtr("alice@personal.com")},
	}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	if err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts); err != nil {
		t.Fatalf("SendUserAccounts: %v", err)
	}
	wantTo := []string{"alice@personal.com"}
	if !reflect.DeepEqual(stub.calls[0].to, wantTo) {
		t.Errorf("recipient = %v, want %v", stub.calls[0].to, wantTo)
	}
}

func TestSendUserAccounts_TemplateVarsPopulated(t *testing.T) {
	stub := newStubSender()
	tmpl := &email.Template{
		Subject:  "User $username on $server_host",
		Body:     "Hello $name ($affiliation) <$email>: pw=$password",
		BodyHTML: false,
	}
	users := []model.CreateUser{
		{
			Username:    "alice",
			Name:        "Alice Wonderland",
			Email:       strPtr("alice@wl.example"),
			Password:    strPtr("s3cret"),
			Affiliation: strPtr("NTUB"),
		},
	}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	if err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts); err != nil {
		t.Fatalf("SendUserAccounts: %v", err)
	}
	if len(stub.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(stub.calls))
	}
	got := stub.calls[0].msg

	// Subject is ASCII-only after substitution → not Q-encoded → appears
	// literally in the headers.
	if !bytes.Contains(got, []byte("Subject: User alice on judge.example.com\r\n")) {
		t.Errorf("missing rendered Subject in:\n%s", got)
	}
	// Required RFC 5322 / MIME headers.
	for _, want := range []string{
		"From: noreply@example.com\r\n",
		"To: alice@wl.example\r\n",
		"Date: ",
		"Message-ID: <",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"Content-Transfer-Encoding: base64\r\n",
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("missing header %q in:\n%s", want, got)
		}
	}
	// Body must round-trip through base64 with all template vars populated.
	body := decodeBody(t, got)
	want := "Hello Alice Wonderland (NTUB) <alice@wl.example>: pw=s3cret"
	if body != want {
		t.Errorf("decoded body = %q, want %q", body, want)
	}
}

func TestSendUserAccounts_TemplateVarsNilOptionalsExpandEmpty(t *testing.T) {
	// nil Affiliation/Password/Email pointers must render to "" rather
	// than panic or substitute the literal "$var".
	stub := newStubSender()
	tmpl := &email.Template{
		Subject:  "u=$username",
		Body:     "name=$name email=$email aff=$affiliation pw=$password host=$server_host",
		BodyHTML: false,
	}
	users := []model.CreateUser{
		{Username: "carol", Name: "Carol"},
	}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	if err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts); err != nil {
		t.Fatalf("SendUserAccounts: %v", err)
	}
	body := decodeBody(t, stub.calls[0].msg)
	want := "name=Carol email= aff= pw= host=judge.example.com"
	if body != want {
		t.Errorf("decoded body = %q, want %q", body, want)
	}
}

func TestSendUserAccounts_BodyHTMLContentType(t *testing.T) {
	stub := newStubSender()
	tmpl := &email.Template{Subject: "hi", Body: "<p>hi</p>", BodyHTML: true}
	users := []model.CreateUser{{Username: "alice", Name: "Alice"}}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	if err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts); err != nil {
		t.Fatalf("SendUserAccounts: %v", err)
	}
	if !bytes.Contains(stub.calls[0].msg, []byte("Content-Type: text/html; charset=UTF-8\r\n")) {
		t.Errorf("BodyHTML=true should produce text/html Content-Type; msg=\n%s", stub.calls[0].msg)
	}
}

func TestSendUserAccounts_MalformedFromEmail(t *testing.T) {
	tests := []struct {
		name      string
		fromEmail string
	}{
		{"no @ sign", "noatsign"},
		{"trailing @", "user@"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := newStubSender()
			tmpl := &email.Template{Subject: "hi", Body: "hey", BodyHTML: false}
			users := []model.CreateUser{
				{Username: "dave", Name: "Dave"},
			}
			cfg := &config.DomServerClient{Host: "judge.example.com"}
			opts := email.SendOptions{FromEmail: tc.fromEmail}

			if err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts); err != nil {
				t.Fatalf("SendUserAccounts: %v", err)
			}
			wantTo := []string{"dave@localhost"}
			if !reflect.DeepEqual(stub.calls[0].to, wantTo) {
				t.Errorf("recipient = %v, want %v", stub.calls[0].to, wantTo)
			}
		})
	}
}

func TestSendUserAccounts_EmptyUsers(t *testing.T) {
	stub := newStubSender()
	tmpl := &email.Template{Subject: "hi", Body: "hey", BodyHTML: false}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	if err := email.SendUserAccounts(context.Background(), stub, tmpl, nil, cfg, opts); err != nil {
		t.Fatalf("SendUserAccounts(nil users): %v", err)
	}
	if err := email.SendUserAccounts(context.Background(), stub, tmpl, []model.CreateUser{}, cfg, opts); err != nil {
		t.Fatalf("SendUserAccounts(empty users): %v", err)
	}
	if len(stub.calls) != 0 {
		t.Errorf("call count = %d, want 0", len(stub.calls))
	}
}

func TestSendUserAccounts_UnknownVarAbortsBeforeSend(t *testing.T) {
	stub := newStubSender()
	tmpl := &email.Template{
		Subject:  "Hello $username, your $pasword", // typo: $pasword not in fixed set
		Body:     "body",
		BodyHTML: false,
	}
	users := []model.CreateUser{{Username: "alice", Name: "Alice"}}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts)
	if err == nil {
		t.Fatal("want error for unknown template var, got nil")
	}
	if len(stub.calls) != 0 {
		t.Errorf("sender.Send must not be called before validation; got %d call(s)", len(stub.calls))
	}
}

func TestSendUserAccounts_ContextCancelled(t *testing.T) {
	stub := newStubSender()
	tmpl := &email.Template{Subject: "hi", Body: "hey", BodyHTML: false}
	users := []model.CreateUser{
		{Username: "alice", Name: "Alice"},
		{Username: "bob", Name: "Bob"},
	}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := email.SendUserAccounts(ctx, stub, tmpl, users, cfg, opts)
	if err == nil {
		t.Fatal("want error from cancelled ctx, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err should wrap context.Canceled; got %v", err)
	}
	if len(stub.calls) != 0 {
		t.Errorf("call count after cancellation = %d, want 0", len(stub.calls))
	}
}
