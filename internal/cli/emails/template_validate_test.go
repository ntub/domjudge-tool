package emails_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/cli/emails"
)

func runValidateCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := emails.NewTemplateValidateCmd()
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs(append([]string{"email-template-validate"}, args...))
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestTemplateValidateCmd_ValidDir_Exit0(t *testing.T) {
	validDir := filepath.Join("..", "..", "..", "testdata", "email", "valid")
	out, err := runValidateCmd(t, validDir)
	if err != nil {
		t.Fatalf("validate valid dir: %v", err)
	}
	if !strings.Contains(out, "template OK") {
		t.Errorf("expected 'template OK' in output, got: %q", out)
	}
}

func TestTemplateValidateCmd_NoSubject_Exit1(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "email", "no_subject")
	_, err := runValidateCmd(t, dir)
	if err == nil {
		t.Error("expected error for no_subject dir, got nil")
	}
}

func TestTemplateValidateCmd_UnknownVar_Exit1(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "email", "unknown_var")
	_, err := runValidateCmd(t, dir)
	if err == nil {
		t.Error("expected error for unknown_var dir, got nil")
	}
}

func TestTemplateValidateCmd_MissingDir_Exit1(t *testing.T) {
	_, err := runValidateCmd(t, "/no/such/dir/"+t.Name())
	if err == nil {
		t.Error("expected error for missing dir, got nil")
	}
}

func TestTemplateValidateCmd_OutputContainsDirOnSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "subject.txt"), []byte("Hi $username"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "body.txt"), []byte("pw=$password name=$name email=$email aff=$affiliation host=$server_host"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runValidateCmd(t, dir)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.Contains(out, dir) {
		t.Errorf("output should contain dir path %q; got %q", dir, out)
	}
}
