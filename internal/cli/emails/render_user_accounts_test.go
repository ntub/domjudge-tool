package emails_test

import (
	"bytes"
	"context"
	"errors"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/cli/emails"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/email"
)

var (
	renderFixedTime      = time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	renderFixedRandBytes = [8]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22}
)

func renderFixedRandRead(b []byte) (int, error) {
	copy(b, renderFixedRandBytes[:])
	return len(b), nil
}

func makeRenderCSV(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(path, []byte(
		"username,name,email,affiliation,password\n"+
			"alice,Alice Wonderland,alice@example.org,NTUB,passA\n"+
			"bob,Bob Builder,,NTUB,passB\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runRenderCmd(t *testing.T, outDir string, args ...string) (string, error) {
	t.Helper()
	restore := email.SetDeterministicSeams(
		func() time.Time { return renderFixedTime },
		renderFixedRandRead,
	)
	t.Cleanup(restore)

	var out bytes.Buffer
	cmd := emails.NewRenderUserAccountsCmd()
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	allArgs := append([]string{"render-user-accounts"}, args...)
	allArgs = append(allArgs, "--output-dir", outDir)
	root.SetArgs(allArgs)
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "http://example.test"}
	ctx := cli.WithConfig(context.Background(), cfg)
	return out.String(), root.ExecuteContext(ctx)
}

func validTemplateDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "email", "valid")
}

func TestRenderUserAccountsCmd_TwoRows_TwoEMLFiles(t *testing.T) {
	csvPath := makeRenderCSV(t)
	outDir := t.TempDir()
	tmplDir := validTemplateDir(t)
	_, err := runRenderCmd(t, outDir, csvPath, tmplDir)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	emlFiles := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".eml") {
			emlFiles++
		}
	}
	if emlFiles != 2 {
		t.Errorf("got %d .eml files, want 2", emlFiles)
	}
}

func TestRenderUserAccountsCmd_EMLParsable_ByNetMail(t *testing.T) {
	csvPath := makeRenderCSV(t)
	outDir := t.TempDir()
	tmplDir := validTemplateDir(t)
	_, err := runRenderCmd(t, outDir, csvPath, tmplDir)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".eml") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(outDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		msg, err := mail.ReadMessage(bytes.NewReader(content))
		if err != nil {
			t.Errorf("%s is not valid RFC5322: %v", e.Name(), err)
			continue
		}
		if msg.Header.Get("From") == "" {
			t.Errorf("%s missing From header", e.Name())
		}
	}
}

func TestRenderUserAccountsCmd_ForceFlag(t *testing.T) {
	csvPath := makeRenderCSV(t)
	outDir := t.TempDir()
	tmplDir := validTemplateDir(t)

	if _, err := runRenderCmd(t, outDir, csvPath, tmplDir); err != nil {
		t.Fatalf("first render: %v", err)
	}
	if _, err := runRenderCmd(t, outDir, csvPath, tmplDir); err == nil {
		t.Error("second render without --force should fail")
	}
	if _, err := runRenderCmd(t, outDir, csvPath, tmplDir, "--force"); err != nil {
		t.Errorf("render with --force: %v", err)
	}
}

func TestRenderUserAccountsCmd_RecipientFallback(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(csvPath, []byte(
		"username,name,email,affiliation,password\n"+
			"carol,Carol,,NTUB,passC\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	tmplDir := validTemplateDir(t)
	_, err := runRenderCmd(t, outDir, csvPath, tmplDir, "--from-email", "noreply@example.com")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(outDir, "carol.eml"))
	if err != nil {
		t.Fatalf("read carol.eml: %v", err)
	}
	if !bytes.Contains(content, []byte("To: carol@example.com\r\n")) {
		t.Errorf("expected fallback recipient carol@example.com in:\n%s", content)
	}
}

func TestRenderUserAccountsCmd_GoldenByteIdentical(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(csvPath, []byte(
		"username,name,email,affiliation,password\n"+
			"testuser,Test User,testuser@example.org,NTUB,testpass\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	tmplDir := validTemplateDir(t)
	_, err := runRenderCmd(t, outDir, csvPath, tmplDir, "--from-email", "noreply@test.example.com")
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := filepath.Join("..", "..", "..", "testdata", "email", "rendered", "testuser.eml")
	got, err := os.ReadFile(filepath.Join(outDir, "testuser.eml"))
	if err != nil {
		t.Fatalf("read testuser.eml: %v", err)
	}
	golden, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden captured to %s", goldenPath)
		return
	}
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, golden) {
		t.Errorf("output not byte-identical to golden\ngot:\n%s\nwant:\n%s", got, golden)
	}
}

// TestRenderUserAccounts_CtxCancelled_AbortsBeforeNextRow exercises the
// `if err := ctx.Err(); err != nil` guard at the top of the per-user
// render loop. With a pre-cancelled ctx the loop must abort before any
// .eml is written and surface a context.Canceled-wrapped error.
func TestRenderUserAccounts_CtxCancelled_AbortsBeforeNextRow(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(csvPath, []byte(
		"username,name,email,affiliation,password\n"+
			"alice,Alice,alice@example.org,NTUB,passA\n"+
			"bob,Bob,bob@example.org,NTUB,passB\n"+
			"carol,Carol,carol@example.org,NTUB,passC\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	tmplDir := validTemplateDir(t)

	cmd := emails.NewRenderUserAccountsCmd()
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{
		"render-user-accounts", csvPath, tmplDir,
		"--output-dir", outDir,
	})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "http://example.test"}
	ctx, cancel := context.WithCancel(cli.WithConfig(context.Background(), cfg))
	cancel()
	err := root.ExecuteContext(ctx)
	if err == nil {
		t.Fatal("expected ctx-cancelled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read outDir: %v", err)
	}
	emlCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".eml") {
			emlCount++
		}
	}
	if emlCount != 0 {
		t.Errorf("got %d .eml files written before/after ctx-cancel; want 0", emlCount)
	}
}
