package email

import (
	"net/smtp"
	"testing"
)

// TestPlainAuth_NoTLSCheck verifies that plainAuth.Start succeeds even when
// the server is non-TLS and non-localhost — the restriction imposed by
// smtp.PlainAuth is absent. This matches Python smtplib.SMTP.login behaviour.
func TestPlainAuth_NoTLSCheck(t *testing.T) {
	auth := plainAuth{username: "user", password: "pass"}
	mech, resp, err := auth.Start(&smtp.ServerInfo{
		Name: "smtp.external.com",
		TLS:  false,
	})
	if err != nil {
		t.Fatalf("Start: %v (smtp.PlainAuth would return \"unencrypted connection\")", err)
	}
	if mech != "PLAIN" {
		t.Errorf("mechanism = %q, want PLAIN", mech)
	}
	want := "\x00user\x00pass"
	if string(resp) != want {
		t.Errorf("response = %q, want %q", string(resp), want)
	}
}

// TestPlainAuth_Next_NoChallenge verifies that Next returns nil when more=false
// and an error when more=true (unexpected server challenge).
func TestPlainAuth_Next(t *testing.T) {
	auth := plainAuth{username: "u", password: "p"}
	if _, err := auth.Next(nil, false); err != nil {
		t.Errorf("Next(more=false): want nil, got %v", err)
	}
	if _, err := auth.Next(nil, true); err == nil {
		t.Error("Next(more=true): want error for unexpected challenge, got nil")
	}
}
