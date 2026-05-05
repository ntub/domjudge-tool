package emails_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/cli/emails"
	"github.com/ntub/domjudge-tool/internal/email"
)

func TestTemplateExampleCmd_GoldenDir(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "email-template")
	var out bytes.Buffer
	cmd := emails.NewTemplateExampleCmd()
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{"email-template-example", "--output", outDir})
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, name := range []string{"subject.txt", "body.txt", "body.html"} {
		got, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		golden, err := os.ReadFile(filepath.Join("../../../testdata/email/scaffold_golden", name))
		if err != nil {
			t.Fatalf("read golden %s: %v", name, err)
		}
		if string(got) != string(golden) {
			t.Errorf("%s mismatch:\ngot:  %q\nwant: %q", name, got, golden)
		}
	}
}

func TestTemplateExampleCmd_ForceFlag(t *testing.T) {
	dir := t.TempDir()
	cmd := emails.NewTemplateExampleCmd()
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{"email-template-example", "--output", dir})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	cmd2 := emails.NewTemplateExampleCmd()
	root2 := cli.NewRootCmd(cli.Deps{})
	root2.AddCommand(cmd2)
	root2.SetArgs([]string{"email-template-example", "--output", dir})
	root2.SetOut(&bytes.Buffer{})
	root2.SetErr(&bytes.Buffer{})
	if err := root2.ExecuteContext(context.Background()); err == nil {
		t.Error("second run without --force should fail")
	}
	cmd3 := emails.NewTemplateExampleCmd()
	root3 := cli.NewRootCmd(cli.Deps{})
	root3.AddCommand(cmd3)
	root3.SetArgs([]string{"email-template-example", "--output", dir, "--force"})
	root3.SetOut(&bytes.Buffer{})
	root3.SetErr(&bytes.Buffer{})
	if err := root3.ExecuteContext(context.Background()); err != nil {
		t.Errorf("third run with --force: %v", err)
	}
}

func TestTemplateExampleCmd_GroupID(t *testing.T) {
	cmd := emails.NewTemplateExampleCmd()
	if cmd.GroupID != "emails" {
		t.Errorf("GroupID = %q, want emails", cmd.GroupID)
	}
}

var _ = email.ErrTemplateDirExists
