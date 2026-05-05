package problems_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/cli/problems"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/model"
)

// runWithStubGateway wires deps + injected gateway, sets a config on
// ctx so LoadConfigPreRunE no-ops, runs cmd with args, returns the
// post-run mock for assertions.
func runWithStubGateway(t *testing.T, m *mockGateway, args ...string) (*cobra.Command, error) {
	t.Helper()
	deps := problems.Deps{
		NewGateway: func(_ context.Context, _ *config.DomServerClient) (web.Gateway, error) {
			return m, nil
		},
	}
	cmd := problems.NewDownloadProblemsCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs(append([]string{"download-problems"}, args...))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "http://example.test", Username: "u", Password: "p"}
	ctx := cli.WithConfig(context.Background(), cfg)
	return root, root.ExecuteContext(ctx)
}

func TestDownloadProblems_HappyPath_DefaultFolder(t *testing.T) {
	dir := t.TempDir()
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	exportA := "/jury/problems/A/export"
	exportB := "/jury/problems/B/export"
	m := &mockGateway{
		getProblemsRes: []model.ProblemItem{
			{ID: "A", Name: "easy", ExportFilePath: &exportA},
			{ID: "B", Name: "medium", ExportFilePath: &exportB},
		},
		downloadBodies: map[string][]byte{
			exportA: []byte("zip-A"),
			exportB: []byte("zip-B"),
		},
	}

	if _, err := runWithStubGateway(t, m); err != nil {
		t.Fatalf("execute: %v", err)
	}
	wantA := filepath.Join("export_problems", "A_easy.zip")
	wantB := filepath.Join("export_problems", "B_medium.zip")
	if b, err := os.ReadFile(wantA); err != nil || string(b) != "zip-A" {
		t.Errorf("file A: body=%q err=%v", string(b), err)
	}
	if b, err := os.ReadFile(wantB); err != nil || string(b) != "zip-B" {
		t.Errorf("file B: body=%q err=%v", string(b), err)
	}
	if m.loginCalls.Load() != 1 {
		t.Errorf("login calls = %d, want 1", m.loginCalls.Load())
	}
}

func TestDownloadProblems_FolderFlag(t *testing.T) {
	dir := t.TempDir()
	customFolder := filepath.Join(dir, "myproblems")

	exportA := "/jury/problems/A/export"
	m := &mockGateway{
		getProblemsRes: []model.ProblemItem{{ID: "A", Name: "easy", ExportFilePath: &exportA}},
		downloadBodies: map[string][]byte{exportA: []byte("zip-A")},
	}
	if _, err := runWithStubGateway(t, m, "--folder", customFolder); err != nil {
		t.Fatalf("execute: %v", err)
	}
	wantPath := filepath.Join(customFolder, "A_easy.zip")
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("expected file at %s: %v", wantPath, err)
	}
}

func TestDownloadProblems_ExcludeOnlyForwarded(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	m := &mockGateway{
		getProblemsRes: []model.ProblemItem{},
	}
	if _, err := runWithStubGateway(t, m,
		"--exclude", "a,b",
		"--only", "c",
	); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, want := m.getProblemsArg.Exclude, []string{"a", "b"}; !equalStrSlice(got, want) {
		t.Errorf("Exclude = %v, want %v", got, want)
	}
	if got, want := m.getProblemsArg.Only, []string{"c"}; !equalStrSlice(got, want) {
		t.Errorf("Only = %v, want %v", got, want)
	}
}

func TestDownloadProblems_SkipsEmptyExportPath(t *testing.T) {
	dir := t.TempDir()
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	export := "/x"
	empty := ""
	m := &mockGateway{
		getProblemsRes: []model.ProblemItem{
			{ID: "A", Name: "skip-me", ExportFilePath: nil},
			{ID: "B", Name: "skip-empty", ExportFilePath: &empty},
			{ID: "C", Name: "ok", ExportFilePath: &export},
		},
		downloadBodies: map[string][]byte{export: []byte("zip-C")},
	}
	if _, err := runWithStubGateway(t, m); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(m.downloadCalls) != 1 {
		t.Errorf("download calls = %v, want 1 call", m.downloadCalls)
	}
	if _, err := os.Stat(filepath.Join("export_problems", "C_ok.zip")); err != nil {
		t.Errorf("expected C zip written: %v", err)
	}
	for _, name := range []string{"A_skip-me.zip", "B_skip-empty.zip"} {
		if _, err := os.Stat(filepath.Join("export_problems", name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("did not expect %s; err = %v", name, err)
		}
	}
}

func TestDownloadProblems_FilenameSpacesToDash(t *testing.T) {
	dir := t.TempDir()
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	export := "/x"
	m := &mockGateway{
		getProblemsRes: []model.ProblemItem{{ID: "P", Name: "foo bar baz", ExportFilePath: &export}},
		downloadBodies: map[string][]byte{export: []byte("zip")},
	}
	if _, err := runWithStubGateway(t, m); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := filepath.Join("export_problems", "P_foo-bar-baz.zip")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s: %v", want, err)
	}
}

func TestDownloadProblems_SanitizesFilename(t *testing.T) {
	dir := t.TempDir()
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	export := "/x"
	m := &mockGateway{
		getProblemsRes: []model.ProblemItem{{ID: "P/../X", Name: "../bad/name", ExportFilePath: &export}},
		downloadBodies: map[string][]byte{export: []byte("zip")},
	}
	if _, err := runWithStubGateway(t, m); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := filepath.Join("export_problems", "P-..-X_..-bad-name.zip")
	if b, err := os.ReadFile(want); err != nil || string(b) != "zip" {
		t.Errorf("sanitized file: body=%q err=%v", string(b), err)
	}
	if _, err := os.Stat(filepath.Join("export_problems", "P")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("unexpected nested path from unsafe id; err = %v", err)
	}
}

func TestDownloadProblems_CtxCancelled(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	export := "/x"
	m := &mockGateway{
		getProblemsRes: []model.ProblemItem{{ID: "A", Name: "x", ExportFilePath: &export}},
		downloadBodies: map[string][]byte{export: []byte("zip")},
	}
	deps := problems.Deps{
		NewGateway: func(_ context.Context, _ *config.DomServerClient) (web.Gateway, error) {
			return m, nil
		},
	}
	cmd := problems.NewDownloadProblemsCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{"download-problems"})
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
}

func TestDefaultDeps_ReturnsRealGatewayConstructor(t *testing.T) {
	deps := problems.DefaultDeps()
	if deps.NewGateway == nil {
		t.Fatal("DefaultDeps.NewGateway is nil")
	}
}

func equalStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
