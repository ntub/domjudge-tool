package submissions_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestDownloadSubmissions_HTTPToFiles_Integration(t *testing.T) {
	const source = "print(\"hello\")\n"
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	entry, err := zw.Create("main.py")
	if err != nil {
		t.Fatalf("create archive entry: %v", err)
	}
	if _, err := io.WriteString(entry, source); err != nil {
		t.Fatalf("write archive entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	timeout := 2.0
	tests := []struct {
		name       string
		timeout    *float64
		args       []string
		wantStrict string
		wantFile   string
		wantBody   []byte
	}{
		{
			name:     "preserve archive",
			args:     []string{"--extract=false"},
			wantFile: "accepted_S1.zip",
			wantBody: archive.Bytes(),
		},
		{
			name:       "extract source with explicit non-strict query",
			timeout:    &timeout,
			args:       []string{"--strict=false"},
			wantStrict: "false",
			wantFile:   "accepted_S1.py",
			wantBody:   []byte(source),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				username, password, ok := r.BasicAuth()
				if !ok || username != "jury" || password != "secret" {
					t.Errorf("BasicAuth = (%q, %q, %v), want configured credentials", username, password, ok)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v4/contests/contest-1/teams":
					_, _ = io.WriteString(w, `[{"id":"1","name":"alpha"}]`)
				case "/api/v4/contests/contest-1/problems":
					_, _ = io.WriteString(w, `[{"id":"P","short_name":"easy"}]`)
				case "/api/v4/contests/contest-1/judgement-types":
					_, _ = io.WriteString(w, `[{"id":"AC","name":"Accepted"}]`)
				case "/api/v4/contests/contest-1/judgements":
					_, _ = io.WriteString(w, `[{"id":"J1","submission_id":"S1","judgement_type_id":"AC"}]`)
				case "/api/v4/contests/contest-1/submissions":
					if got := r.URL.Query().Get("strict"); got != tc.wantStrict {
						t.Errorf("strict query = %q, want %q", got, tc.wantStrict)
					}
					_, _ = io.WriteString(w, `[{"id":"S1","team_id":"1","problem_id":"P"}]`)
				case "/api/v4/contests/contest-1/submissions/S1/files":
					w.Header().Set("Content-Type", "application/zip")
					w.Header().Set("Content-Disposition", `attachment; filename="server-name.zip"`)
					_, _ = w.Write(archive.Bytes())
				default:
					t.Errorf("unexpected endpoint: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(srv.Close)

			dir := t.TempDir()
			root := cli.NewRootCmd(cli.Deps{})
			root.AddCommand(clisubs.NewDownloadSubmissionsCmd(clisubs.DefaultDeps()))
			args := []string{"download-submissions", "contest-1", "--path", dir}
			root.SetArgs(append(args, tc.args...))
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			cfg := &config.DomServerClient{Host: srv.URL, Username: "jury", Password: "secret", Timeout: tc.timeout}
			if err := root.ExecuteContext(cli.WithConfig(context.Background(), cfg)); err != nil {
				t.Fatalf("execute: %v", err)
			}

			folder := filepath.Join(dir, "problem_easy", "team_alpha")
			body, err := os.ReadFile(filepath.Join(folder, tc.wantFile))
			if err != nil {
				t.Fatalf("read downloaded file: %v", err)
			}
			if !bytes.Equal(body, tc.wantBody) {
				t.Errorf("downloaded contents = %q, want %q", body, tc.wantBody)
			}
			files, err := os.ReadDir(folder)
			if err != nil {
				t.Fatalf("read output directory: %v", err)
			}
			if len(files) != 1 || files[0].Name() != tc.wantFile {
				t.Errorf("output directory = %v, want only %s", files, tc.wantFile)
			}
		})
	}
}

func TestDownloadSubmissions_HTTPFailureDoesNotReportSuccess_Integration(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"authentication rejected", http.StatusUnauthorized, "credentials rejected", apiv4.ErrUnauthorized},
		{"server failure", http.StatusServiceUnavailable, "temporarily unavailable", apiv4.ErrServerError},
		{"invalid response", http.StatusOK, `[{"id":`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(srv.Close)
			dir := t.TempDir()
			var stdout bytes.Buffer
			root := cli.NewRootCmd(cli.Deps{})
			root.AddCommand(clisubs.NewDownloadSubmissionsCmd(clisubs.DefaultDeps()))
			root.SetArgs([]string{"download-submissions", "contest-1", "--path", dir})
			root.SetOut(&stdout)
			root.SetErr(&bytes.Buffer{})
			cfg := &config.DomServerClient{Host: srv.URL}
			err := root.ExecuteContext(cli.WithConfig(context.Background(), cfg))
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("execute error = %v, want %v", err, tc.want)
				}
				var httpErr *apiv4.HTTPError
				if !errors.As(err, &httpErr) || httpErr.StatusCode != tc.status || string(httpErr.Body) != tc.body {
					t.Fatalf("execute error = %v, want original HTTP status and body", err)
				}
			} else {
				var decodeErr *json.SyntaxError
				if !errors.As(err, &decodeErr) && !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("execute error = %v, want JSON decoding failure", err)
				}
			}
			files, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read output directory: %v", err)
			}
			if len(files) != 0 {
				t.Errorf("output after failed download = %v, want no files", files)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout after failed download = %q, want no success report", stdout.String())
			}
		})
	}
}
