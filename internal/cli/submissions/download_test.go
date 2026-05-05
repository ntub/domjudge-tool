package submissions_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	clisubs "github.com/ntub/domjudge-tool/internal/cli/submissions"
	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/submissions"
)

type stubCall struct {
	Cid  string
	Opts submissions.Options
}

// stubDownloader records the call and returns count + nil err.
func stubDownloader(captured *stubCall) clisubs.DownloadFn {
	return func(_ context.Context, _ *apiv4.Client, cid string, opts submissions.Options) (int, error) {
		captured.Cid = cid
		captured.Opts = opts
		return 0, nil
	}
}

// runWithStubDownloader injects deps + a stubbed downloader, sets a
// config on ctx so PreRunE no-ops, runs the command.
func runWithStubDownloader(t *testing.T, captured *stubCall, args ...string) error {
	t.Helper()
	deps := clisubs.Deps{
		NewAPI: func(_ *config.DomServerClient) *apiv4.Client {
			return apiv4.New(apiv4.Options{BaseURL: "http://x", Username: "u", Password: "p"})
		},
		Download: stubDownloader(captured),
	}
	cmd := clisubs.NewDownloadSubmissionsCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs(append([]string{"download-submissions"}, args...))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "http://example.test"}
	ctx := cli.WithConfig(context.Background(), cfg)
	return root.ExecuteContext(ctx)
}

func TestDownloadSubmissions_DefaultsMatchPython(t *testing.T) {
	c := &stubCall{}
	if err := runWithStubDownloader(t, c, "contest-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if c.Cid != "contest-1" {
		t.Errorf("cid = %q, want contest-1", c.Cid)
	}
	if c.Opts.Mode != 2 {
		t.Errorf("Mode = %d, want 2 (Python default)", c.Opts.Mode)
	}
	if !c.Opts.Extract {
		t.Errorf("Extract = false, want true (Python default is_extract=True)")
	}
	if c.Opts.OutputPath != "" {
		t.Errorf("OutputPath = %q, want empty", c.Opts.OutputPath)
	}
	if c.Opts.Strict != nil {
		t.Errorf("Strict = %v, want nil (unset)", c.Opts.Strict)
	}
}

func TestDownloadSubmissions_FlagPropagation(t *testing.T) {
	c := &stubCall{}
	if err := runWithStubDownloader(t, c,
		"contest-9",
		"--mode", "1",
		"--path", "/tmp/grading",
		"--strict",
		"--extract=false",
	); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if c.Opts.Mode != 1 {
		t.Errorf("Mode = %d, want 1", c.Opts.Mode)
	}
	if c.Opts.OutputPath != "/tmp/grading" {
		t.Errorf("OutputPath = %q, want /tmp/grading", c.Opts.OutputPath)
	}
	if c.Opts.Strict == nil || !*c.Opts.Strict {
		t.Errorf("Strict = %v, want non-nil true", c.Opts.Strict)
	}
	if c.Opts.Extract {
		t.Errorf("Extract = true, want false (--extract=false)")
	}
}

func TestDownloadSubmissions_RequiresCid(t *testing.T) {
	c := &stubCall{}
	err := runWithStubDownloader(t, c) // no cid
	if err == nil {
		t.Fatal("expected ExactArgs(1) error, got nil")
	}
}

func TestDownloadSubmissions_AliasContestFiles(t *testing.T) {
	c := &stubCall{}
	deps := clisubs.Deps{
		NewAPI: func(_ *config.DomServerClient) *apiv4.Client {
			return apiv4.New(apiv4.Options{BaseURL: "http://x"})
		},
		Download: stubDownloader(c),
	}
	cmd := clisubs.NewDownloadSubmissionsCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{"contest-files", "contest-7"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "http://x"}
	if err := root.ExecuteContext(cli.WithConfig(context.Background(), cfg)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if c.Cid != "contest-7" {
		t.Errorf("alias path: cid = %q, want contest-7", c.Cid)
	}
}

func TestDownloadSubmissions_CtxCancelled(t *testing.T) {
	c := &stubCall{}
	deps := clisubs.Deps{
		NewAPI: func(_ *config.DomServerClient) *apiv4.Client {
			return apiv4.New(apiv4.Options{BaseURL: "http://x"})
		},
		// Downloader respects ctx.
		Download: func(ctx context.Context, _ *apiv4.Client, _ string, _ submissions.Options) (int, error) {
			return 0, ctx.Err()
		},
	}
	cmd := clisubs.NewDownloadSubmissionsCmd(deps)
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs([]string{"download-submissions", "contest-1"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cfg := &config.DomServerClient{Host: "http://x"}
	ctx, cancel := context.WithCancel(cli.WithConfig(context.Background(), cfg))
	cancel()
	err := root.ExecuteContext(ctx)
	if err == nil {
		t.Fatal("expected ctx error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
	_ = c
}
