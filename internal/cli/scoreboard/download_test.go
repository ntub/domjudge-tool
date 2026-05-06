package scoreboard_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	cliscb "github.com/ntub/domjudge-tool/internal/cli/scoreboard"
	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/config"
)

type capturedDownload struct {
	Host string
	Cid  string
}

func stubFn(captured *capturedDownload) cliscb.DownloadFn {
	return func(_ context.Context, _ *apiv4.Client, cid string) ([]string, [][]string, error) {
		captured.Cid = cid
		return []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score"},
			[][]string{{"1", "NTUB", "Alpha", "2", "100"}}, nil
	}
}

func runWithStub(t *testing.T, captured *capturedDownload, args ...string) error {
	t.Helper()
	deps := cliscb.Deps{
		Download: stubFn(captured),
		NewAPI: func(cfg *config.DomServerClient) *apiv4.Client {
			captured.Host = cfg.Host
			return apiv4.New(apiv4.Options{BaseURL: cfg.Host})
		},
	}
	cmd := cliscb.NewDownloadScoreboardCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs(append([]string{"download-scoreboard"}, args...))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "https://judge.example.test"}
	ctx := cli.WithConfig(context.Background(), cfg)
	return root.ExecuteContext(ctx)
}

func TestDownloadScoreboard_DefaultFilename(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	captured := &capturedDownload{}
	if err := runWithStub(t, captured, "--cid", "1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := filepath.Join(tmp, "export.csv")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s, err = %v", want, err)
	}
}

func TestDownloadScoreboard_PathFlag(t *testing.T) {
	tmp := t.TempDir()
	captured := &capturedDownload{}
	if err := runWithStub(t, captured, "--cid", "2", "--path", tmp); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := filepath.Join(tmp, "export.csv")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s, err = %v", want, err)
	}
}

func TestDownloadScoreboard_FilenameFlag(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	captured := &capturedDownload{}
	if err := runWithStub(t, captured, "--cid", "3", "--filename", "round1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "round1.csv")); err != nil {
		t.Errorf("expected round1.csv, err = %v", err)
	}
}

func TestDownloadScoreboard_UrlOverride(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	captured := &capturedDownload{}
	if err := runWithStub(t, captured, "--cid", "5", "--url", "https://other/public"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if captured.Host != "https://other/public" {
		t.Errorf("Host = %q, want https://other/public", captured.Host)
	}
}

func TestDownloadScoreboard_UrlDefaultIsCfgHost(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	captured := &capturedDownload{}
	if err := runWithStub(t, captured, "--cid", "1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if captured.Host != "https://judge.example.test" {
		t.Errorf("Host = %q, want cfg.Host", captured.Host)
	}
}

func TestDownloadScoreboard_AliasScoreboardExport(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	captured := &capturedDownload{}
	deps := cliscb.Deps{
		Download: stubFn(captured),
		NewAPI: func(cfg *config.DomServerClient) *apiv4.Client {
			captured.Host = cfg.Host
			return apiv4.New(apiv4.Options{BaseURL: cfg.Host})
		},
	}
	cmd := cliscb.NewDownloadScoreboardCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{"scoreboard-export", "--cid", "9"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	cfg := &config.DomServerClient{Host: "https://judge.example.test"}
	if err := root.ExecuteContext(cli.WithConfig(context.Background(), cfg)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if captured.Cid != "9" {
		t.Errorf("alias path: cid = %q, want 9", captured.Cid)
	}
}

func TestDownloadScoreboard_RequiresCid(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	captured := &capturedDownload{}
	err := runWithStub(t, captured) // no --cid
	if err == nil {
		t.Fatal("expected MarkFlagRequired error, got nil")
	}
}

// TestDownloadScoreboard_CtxCancelled_AbortsBeforeNextRow exercises
// the `if err := ctx.Err(); err != nil` guard at the top of the CSV
// row-write loop. With a pre-cancelled ctx the cmd must surface a
// context.Canceled-wrapped error and the resulting CSV must not
// contain any data rows (header-only, empty, or absent are all OK).
func TestDownloadScoreboard_CtxCancelled_AbortsBeforeNextRow(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	deps := cliscb.Deps{
		Download: func(_ context.Context, _ *apiv4.Client, _ string) ([]string, [][]string, error) {
			return []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score"},
				[][]string{
					{"1", "NTUB", "Alpha", "2", "100"},
					{"2", "NTUB", "Beta", "1", "50"},
					{"3", "NTUB", "Gamma", "0", "0"},
				}, nil
		},
		NewAPI: func(cfg *config.DomServerClient) *apiv4.Client {
			return apiv4.New(apiv4.Options{BaseURL: cfg.Host})
		},
	}
	cmd := cliscb.NewDownloadScoreboardCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{"download-scoreboard", "--cid", "1"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "https://judge.example.test"}
	ctx, cancel := context.WithCancel(cli.WithConfig(context.Background(), cfg))
	cancel()
	err := root.ExecuteContext(ctx)
	if err == nil {
		t.Fatal("expected ctx-cancelled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}

	// The CSV may not exist, may be empty, or may contain just the
	// header (the loop's ctx.Err() guard runs after the writer is
	// created and the header is written but before any data rows).
	// Any data row would prove cancellation was missed.
	body, readErr := os.ReadFile(filepath.Join(tmp, "export.csv"))
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("read export.csv: %v", readErr)
	}
	if readErr == nil {
		// File exists; ensure no data row was written. Header is fine.
		// Data rows in this fixture all start with a digit; reject any
		// line beyond the header that contains "Alpha", "Beta", or
		// "Gamma" team names.
		text := string(body)
		for _, marker := range []string{"Alpha", "Beta", "Gamma"} {
			if strings.Contains(text, marker) {
				t.Errorf("export.csv contains data row %q after ctx-cancel; body:\n%s", marker, text)
			}
		}
	}
}

func TestDownloadScoreboard_DownloadError(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	deps := cliscb.Deps{
		Download: func(_ context.Context, _ *apiv4.Client, _ string) ([]string, [][]string, error) {
			return nil, nil, errors.New("upstream 500")
		},
		NewAPI: func(cfg *config.DomServerClient) *apiv4.Client {
			return apiv4.New(apiv4.Options{BaseURL: cfg.Host})
		},
	}
	cmd := cliscb.NewDownloadScoreboardCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{"download-scoreboard", "--cid", "1"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	cfg := &config.DomServerClient{Host: "https://judge.example.test"}
	if err := root.ExecuteContext(cli.WithConfig(context.Background(), cfg)); err == nil {
		t.Fatal("expected error, got nil")
	}
}
