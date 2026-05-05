package email_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/email"
)

// scaffoldGoldenDir is captured on first test run when absent so the
// constants in scaffold.go remain the single source of truth.
const scaffoldGoldenDir = "../../testdata/email/scaffold_golden"

func TestWriteTemplateScaffold_GoldenOutput(t *testing.T) {
	dir := t.TempDir()
	if err := email.WriteTemplateScaffold(dir, false); err != nil {
		t.Fatalf("WriteTemplateScaffold: %v", err)
	}
	for _, name := range []string{"subject.txt", "body.txt", "body.html"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		goldenPath := filepath.Join(scaffoldGoldenDir, name)
		want, err := os.ReadFile(goldenPath)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(scaffoldGoldenDir, 0o755); err != nil {
				t.Fatalf("mkdir golden: %v", err)
			}
			if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
				t.Fatalf("write golden %s: %v", name, err)
			}
			t.Logf("golden captured to %s", goldenPath)
			continue
		}
		if err != nil {
			t.Fatalf("read golden %s: %v", name, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s mismatch:\ngot:  %q\nwant: %q", name, got, want)
		}
	}
}

func TestWriteTemplateScaffold_ExistsNonForce_ReturnsErr(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "subject.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := email.WriteTemplateScaffold(dir, false)
	if !errors.Is(err, email.ErrTemplateDirExists) {
		t.Errorf("got %v, want ErrTemplateDirExists", err)
	}
}

func TestWriteTemplateScaffold_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"subject.txt", "body.txt", "body.html"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := email.WriteTemplateScaffold(dir, true); err != nil {
		t.Fatalf("WriteTemplateScaffold force=true: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "subject.txt"))
	if err != nil {
		t.Fatalf("read subject.txt: %v", err)
	}
	if string(got) == "old" {
		t.Error("force=true did not overwrite subject.txt")
	}
}

func TestWriteTemplateScaffold_ForcePreservesOtherFiles(t *testing.T) {
	dir := t.TempDir()
	bakPath := filepath.Join(dir, "body.html.bak")
	if err := os.WriteFile(bakPath, []byte("backup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := email.WriteTemplateScaffold(dir, true); err != nil {
		t.Fatalf("WriteTemplateScaffold force=true: %v", err)
	}
	got, err := os.ReadFile(bakPath)
	if err != nil || string(got) != "backup" {
		t.Errorf("force=true must not touch body.html.bak; got %q, err=%v", got, err)
	}
}

func TestWriteTemplateScaffold_RoundTrip_ValidateReturnsNil(t *testing.T) {
	dir := t.TempDir()
	if err := email.WriteTemplateScaffold(dir, false); err != nil {
		t.Fatalf("WriteTemplateScaffold: %v", err)
	}
	tmpl, err := email.LoadTemplate(dir)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if err := tmpl.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}
