package email_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/email"
)

// TDD ANCHOR #1 — name must match exactly.
func TestLoadTemplate_PrefersHTMLOverText(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "email", "both_body")
	tmpl, err := email.LoadTemplate(dir)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if !tmpl.BodyHTML {
		t.Error("want BodyHTML=true when both body.html and body.txt exist")
	}
	if !strings.Contains(tmpl.Body, "<p>") {
		t.Errorf("want HTML content in Body, got %q", tmpl.Body)
	}
}

func TestLoadTemplate_HTMLOnly(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "email", "valid")
	tmpl, err := email.LoadTemplate(dir)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if !tmpl.BodyHTML {
		t.Error("want BodyHTML=true when only body.html exists")
	}
	if !strings.Contains(tmpl.Body, "<p>") {
		t.Errorf("want HTML content in Body, got %q", tmpl.Body)
	}
	if !strings.Contains(tmpl.Subject, "$username") {
		t.Errorf("want raw $-substitution in Subject, got %q", tmpl.Subject)
	}
}

func TestLoadTemplate_TextOnly(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "email", "text_only")
	tmpl, err := email.LoadTemplate(dir)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if tmpl.BodyHTML {
		t.Error("want BodyHTML=false when only body.txt exists")
	}
	if !strings.Contains(tmpl.Body, "Hello $name") {
		t.Errorf("want plain-text body content, got %q", tmpl.Body)
	}
	if strings.Contains(tmpl.Body, "<p>") {
		t.Errorf("body should not contain HTML markers, got %q", tmpl.Body)
	}
}

func TestLoadTemplate_MissingSubject(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "email", "no_subject")
	if _, err := email.LoadTemplate(dir); err == nil {
		t.Fatal("want error when subject.txt is missing, got nil")
	}
}

func TestLoadTemplate_MissingBody(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "email", "no_body")
	if _, err := email.LoadTemplate(dir); err == nil {
		t.Fatal("want error when both body.html and body.txt are missing, got nil")
	}
}

func TestLoadTemplate_DirNotExist(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "email", "does_not_exist")
	if _, err := email.LoadTemplate(dir); err == nil {
		t.Fatal("want error when dir does not exist, got nil")
	}
}

func TestRender_AllVars(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "email", "valid")
	tmpl, err := email.LoadTemplate(dir)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	vars := map[string]string{
		"username":    "alice",
		"password":    "s3cret",
		"server_host": "judge.example.com",
		"name":        "Alice",
		"affiliation": "NTUB",
		"email":       "alice@example.com",
	}
	subject, body := tmpl.Render(vars)
	wantSubject := "Hello alice, your password is s3cret on judge.example.com"
	if subject != wantSubject {
		t.Errorf("subject = %q, want %q", subject, wantSubject)
	}
	if !strings.Contains(body, "Hello Alice (NTUB), email: alice@example.com") {
		t.Errorf("body = %q, missing rendered content", body)
	}
}

func TestRender_SubjectQEncoded(t *testing.T) {
	tmpl := &email.Template{
		Subject:  "Hello $name",
		Body:     "ignored",
		BodyHTML: false,
	}
	vars := map[string]string{"name": "世界"}
	subject, _ := tmpl.Render(vars)
	if !strings.HasPrefix(subject, "=?UTF-8?q?") && !strings.HasPrefix(subject, "=?utf-8?q?") {
		t.Errorf("want Q-encoded subject for non-ASCII content, got %q", subject)
	}
	if strings.Contains(subject, "世界") {
		t.Errorf("subject must encode non-ASCII chars, got %q", subject)
	}
}

func TestRender_MissingVar_ExpandsToEmpty(t *testing.T) {
	tmpl := &email.Template{
		Subject:  "User $username has $missing",
		Body:     "Body $also_missing here",
		BodyHTML: false,
	}
	vars := map[string]string{"username": "alice"}
	subject, body := tmpl.Render(vars)
	wantSubject := "User alice has "
	if subject != wantSubject {
		t.Errorf("subject = %q, want %q", subject, wantSubject)
	}
	wantBody := "Body  here"
	if body != wantBody {
		t.Errorf("body = %q, want %q", body, wantBody)
	}
}

func TestLoadTemplate_HTMLUnreadable_NoFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes cannot make a file unreadable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root — chmod-based permission test is meaningless")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "subject.txt"), []byte("subj"), 0o644); err != nil {
		t.Fatal(err)
	}
	htmlPath := filepath.Join(dir, "body.html")
	if err := os.WriteFile(htmlPath, []byte("<p>html</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(htmlPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(htmlPath, 0o644) })
	if err := os.WriteFile(filepath.Join(dir, "body.txt"), []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := email.LoadTemplate(dir)
	if err == nil {
		t.Fatal("want error when body.html is unreadable, got nil")
	}
}

func TestRender_DollarBraceSyntax(t *testing.T) {
	tmpl := &email.Template{
		Subject:  "User ${username}!",
		Body:     "Hello ${name}_suffix",
		BodyHTML: false,
	}
	vars := map[string]string{"username": "alice", "name": "Alice"}
	subject, body := tmpl.Render(vars)
	wantSubject := "User alice!"
	if subject != wantSubject {
		t.Errorf("subject = %q, want %q", subject, wantSubject)
	}
	wantBody := "Hello Alice_suffix"
	if body != wantBody {
		t.Errorf("body = %q, want %q", body, wantBody)
	}
}

func TestValidate_UnknownPlaceholder(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		body    string
	}{
		{"typo in subject", "Hello $username, your $pasword", "body"},
		{"unknown in body", "hello $username", "body $unknown_var"},
		{"unknown in both", "$typo_subj", "$typo_body"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := &email.Template{Subject: tc.subject, Body: tc.body}
			if err := tmpl.Validate(); err == nil {
				t.Fatalf("want error for unknown placeholder, got nil")
			}
		})
	}
}

func TestValidate_AllKnownVars_OK(t *testing.T) {
	tmpl := &email.Template{
		Subject: "Hello $username on $server_host",
		Body:    "$name $email $affiliation $password",
	}
	if err := tmpl.Validate(); err != nil {
		t.Errorf("want nil for all-known vars, got %v", err)
	}
}

func TestValidate_EmptyTemplate_OK(t *testing.T) {
	tmpl := &email.Template{Subject: "no vars here", Body: "plain text"}
	if err := tmpl.Validate(); err != nil {
		t.Errorf("want nil for template with no vars, got %v", err)
	}
}
