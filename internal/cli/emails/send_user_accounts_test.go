package emails_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/cli/emails"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/email"
)

// stubSender records every Send call. Close is a no-op.
type stubSender struct {
	calls []sendCall
}

type sendCall struct {
	From string
	To   []string
	Body string
}

func (s *stubSender) Send(_ context.Context, from string, to []string, msg []byte) error {
	s.calls = append(s.calls, sendCall{From: from, To: append([]string(nil), to...), Body: string(msg)})
	return nil
}

func (s *stubSender) Close() error { return nil }

// makeFixtures writes a CSV + a minimal template directory under
// t.TempDir() and returns their paths.
func makeFixtures(t *testing.T) (csvPath, tmplDir string) {
	t.Helper()
	dir := t.TempDir()
	csvPath = filepath.Join(dir, "in.csv")
	if err := os.WriteFile(csvPath, []byte(
		"username,name,email,affiliation,password\n"+
			"alice,Alice,alice@example.org,NTUB,passA\n"+
			"bob,Bob,,NTUB,passB\n",
	), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	tmplDir = filepath.Join(dir, "tmpl")
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatalf("mkdir tmpl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "subject.txt"), []byte("Hi $username"), 0o644); err != nil {
		t.Fatalf("write subject: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "body.txt"), []byte("password=$password"), 0o644); err != nil {
		t.Fatalf("write body: %v", err)
	}
	return csvPath, tmplDir
}

func makeOutputCSVFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(path, []byte(
		"username,name,email,affiliation,password,is_exist\n"+
			"alice,Alice,alice@example.org,NTUB,passA,true\n"+
			"bob,Bob,,NTUB,passB,false\n",
	), 0o644); err != nil {
		t.Fatalf("write output csv: %v", err)
	}
	return path
}

func runWithStubSender(t *testing.T, args ...string) (*stubSender, *emails.DialedDeps, error) {
	t.Helper()
	stub := &stubSender{}
	dialed := &emails.DialedDeps{}
	deps := emails.Deps{
		Dial: func(_ context.Context, opts email.DialOptions) (email.Sender, error) {
			dialed.Opts = opts
			return stub, nil
		},
	}
	cmd := emails.NewSendUserAccountsCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs(append([]string{"send-user-accounts"}, args...))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "http://example.test"}
	ctx := cli.WithConfig(context.Background(), cfg)
	return stub, dialed, root.ExecuteContext(ctx)
}

func TestSendUserAccounts_HappyPath_TwoRows(t *testing.T) {
	csv, tmpl := makeFixtures(t)
	stub, _, err := runWithStubSender(t, csv, tmpl,
		"--from-email", "noreply@example.org",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(stub.calls) != 2 {
		t.Fatalf("got %d sends, want 2", len(stub.calls))
	}
	// Recipient resolution: alice has email; bob falls back to bob@example.org.
	if stub.calls[0].To[0] != "alice@example.org" {
		t.Errorf("alice To = %v, want alice@example.org", stub.calls[0].To)
	}
	if stub.calls[1].To[0] != "bob@example.org" {
		t.Errorf("bob To = %v, want bob@example.org", stub.calls[1].To)
	}
}

func TestSendUserAccounts_FlagPropagation(t *testing.T) {
	csv, tmpl := makeFixtures(t)
	_, dialed, err := runWithStubSender(t, csv, tmpl,
		"--host", "smtp.example.com",
		"--port", "587",
		"--use-ssl",
		"--smtp-username", "user",
		"--smtp-password", "pw",
		"--timeout", "10s",
		"--from-email", "noreply@example.org",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if dialed.Opts.Host != "smtp.example.com" {
		t.Errorf("Host = %q, want smtp.example.com", dialed.Opts.Host)
	}
	if dialed.Opts.Port != 587 {
		t.Errorf("Port = %d, want 587", dialed.Opts.Port)
	}
	if !dialed.Opts.UseSSL {
		t.Errorf("UseSSL = false, want true")
	}
	if dialed.Opts.Username != "user" {
		t.Errorf("Username = %q, want user", dialed.Opts.Username)
	}
	if dialed.Opts.Password != "pw" {
		t.Errorf("Password = %q, want pw", dialed.Opts.Password)
	}
	if dialed.Opts.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", dialed.Opts.Timeout)
	}
}

func TestSendUserAccounts_RequiresPositionals(t *testing.T) {
	cases := [][]string{
		{},                   // 0 args
		{"only-csv.csv"},     // 1 arg
		{"a.csv", "b/", "c"}, // 3 args
	}
	for i, args := range cases {
		_, _, err := runWithStubSender(t, args...)
		if err == nil {
			t.Errorf("case %d: expected ExactArgs(2) error, got nil", i)
		}
	}
}

func TestSendUserAccounts_TemplateDirNotFound(t *testing.T) {
	csv, _ := makeFixtures(t)
	_, _, err := runWithStubSender(t, csv, "/nope/does/not/exist", "--from-email", "x@example.org")
	if err == nil {
		t.Fatal("expected error for missing template dir, got nil")
	}
}

func TestSendUserAccounts_CSVNotFound(t *testing.T) {
	_, tmpl := makeFixtures(t)
	_, _, err := runWithStubSender(t, "/nope/csv.csv", tmpl, "--from-email", "x@example.org")
	if err == nil {
		t.Fatal("expected error for missing csv, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("errors.Is(err, os.ErrNotExist) = false; err = %v", err)
	}
}

func TestSendUserAccounts_AcceptsImportUsersTeamsOutput(t *testing.T) {
	csv := makeOutputCSVFixture(t)
	_, tmpl := makeFixtures(t)
	stub, _, err := runWithStubSender(t, csv, tmpl, "--from-email", "noreply@example.org")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(stub.calls) != 2 {
		t.Fatalf("got %d sends, want 2", len(stub.calls))
	}
}

func TestSendUserAccounts_CtxCancelled(t *testing.T) {
	csv, tmpl := makeFixtures(t)
	stub := &stubSender{}
	deps := emails.Deps{
		Dial: func(_ context.Context, _ email.DialOptions) (email.Sender, error) {
			return stub, nil
		},
	}
	cmd := emails.NewSendUserAccountsCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{"send-user-accounts", csv, tmpl, "--from-email", "x@example.org"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "http://x"}
	ctx, cancel := context.WithCancel(cli.WithConfig(context.Background(), cfg))
	cancel()
	err := root.ExecuteContext(ctx)
	if err == nil {
		t.Fatal("expected ctx-cancelled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
	if len(stub.calls) != 0 {
		t.Errorf("ctx cancel: stub called %d times, want 0", len(stub.calls))
	}
}
