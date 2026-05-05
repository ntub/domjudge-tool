package email_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/email"
	"github.com/ntub/domjudge-tool/internal/model"
)

var (
	fixedTime      = time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	fixedRandBytes = [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
)

func fixedRandRead(b []byte) (int, error) {
	copy(b, fixedRandBytes[:])
	return len(b), nil
}

func TestBuildEML_TextBody_RFC5322Structure(t *testing.T) {
	restore := email.SetDeterministicSeams(func() time.Time { return fixedTime }, fixedRandRead)
	t.Cleanup(restore)

	tmpl := &email.Template{Subject: "Hi $username", Body: "pw=$password", BodyHTML: false}
	user := model.CreateUser{Username: "alice", Name: "Alice", Password: strPtr("s3cret")}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	got, recipient, err := email.BuildEML(tmpl, user, cfg, opts)
	if err != nil {
		t.Fatalf("BuildEML: %v", err)
	}
	if recipient != "alice@example.com" {
		t.Errorf("recipient = %q, want alice@example.com", recipient)
	}
	for _, want := range []string{
		"From: noreply@example.com\r\n",
		"To: alice@example.com\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"Content-Transfer-Encoding: base64\r\n",
		"MIME-Version: 1.0\r\n",
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestBuildEML_HTMLBody_ContentType(t *testing.T) {
	restore := email.SetDeterministicSeams(func() time.Time { return fixedTime }, fixedRandRead)
	t.Cleanup(restore)

	tmpl := &email.Template{Subject: "Hi", Body: "<p>hello</p>", BodyHTML: true}
	user := model.CreateUser{Username: "alice", Name: "Alice"}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	got, _, err := email.BuildEML(tmpl, user, cfg, opts)
	if err != nil {
		t.Fatalf("BuildEML: %v", err)
	}
	if !bytes.Contains(got, []byte("Content-Type: text/html; charset=UTF-8\r\n")) {
		t.Errorf("missing HTML content-type in:\n%s", got)
	}
}

func TestBuildEML_EmptyEmail_FallbackDomain(t *testing.T) {
	restore := email.SetDeterministicSeams(func() time.Time { return fixedTime }, fixedRandRead)
	t.Cleanup(restore)

	tmpl := &email.Template{Subject: "Hi", Body: "hey", BodyHTML: false}
	user := model.CreateUser{Username: "bob", Name: "Bob"}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	_, recipient, err := email.BuildEML(tmpl, user, cfg, opts)
	if err != nil {
		t.Fatalf("BuildEML: %v", err)
	}
	if recipient != "bob@example.com" {
		t.Errorf("recipient = %q, want bob@example.com (fallback)", recipient)
	}
}

func TestBuildEML_SubjectQEncoding(t *testing.T) {
	restore := email.SetDeterministicSeams(func() time.Time { return fixedTime }, fixedRandRead)
	t.Cleanup(restore)

	tmpl := &email.Template{Subject: "您好 $username", Body: "body", BodyHTML: false}
	user := model.CreateUser{Username: "alice", Name: "Alice"}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	got, _, err := email.BuildEML(tmpl, user, cfg, opts)
	if err != nil {
		t.Fatalf("BuildEML: %v", err)
	}
	if !bytes.Contains(got, []byte("=?UTF-8?")) {
		t.Errorf("expected Q-encoded subject in:\n%s", got)
	}
}

func TestBuildEML_Base64Wrap76Chars(t *testing.T) {
	restore := email.SetDeterministicSeams(func() time.Time { return fixedTime }, fixedRandRead)
	t.Cleanup(restore)

	body := strings.Repeat("Hello World! ", 50)
	tmpl := &email.Template{Subject: "Hi", Body: body, BodyHTML: false}
	user := model.CreateUser{Username: "alice", Name: "Alice"}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	got, _, err := email.BuildEML(tmpl, user, cfg, opts)
	if err != nil {
		t.Fatalf("BuildEML: %v", err)
	}
	parts := bytes.SplitN(got, []byte("\r\n\r\n"), 2)
	if len(parts) != 2 {
		t.Fatal("no header/body separator")
	}
	for _, line := range strings.Split(string(parts[1]), "\r\n") {
		if len(line) > 76 {
			t.Errorf("base64 line too long (%d chars): %q", len(line), line)
		}
	}
}
