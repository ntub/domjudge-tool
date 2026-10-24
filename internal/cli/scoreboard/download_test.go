package scoreboard_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	cliscb "github.com/ntub/domjudge-tool/internal/cli/scoreboard"
	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/config"
)

type capturedDownload struct {
	Host          string
	Cid           string
	ExcludeHidden bool
}

func stubFn(captured *capturedDownload) cliscb.DownloadFn {
	return func(_ context.Context, _ *apiv4.Client, cid string, excludeHidden bool) ([]string, [][]string, error) {
		captured.Cid = cid
		captured.ExcludeHidden = excludeHidden
		return []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score"},
			[][]string{{"1", "NTUB", "Alpha", "2", "100"}}, nil
	}
}

func TestDownloadScoreboard_ExcludeHiddenFlag(t *testing.T) {
	pwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	captured := &capturedDownload{}
	if err := runWithStub(t, captured, "--cid", "1", "--exclude-hidden"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !captured.ExcludeHidden {
		t.Fatal("ExcludeHidden = false, want true")
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
		Download: func(_ context.Context, _ *apiv4.Client, _ string, _ bool) ([]string, [][]string, error) {
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
		Download: func(_ context.Context, _ *apiv4.Client, _ string, _ bool) ([]string, [][]string, error) {
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

func TestDownloadScoreboard_HTTPToCSV_Integration(t *testing.T) {
	timeout := 2.0
	tests := []struct {
		name    string
		deps    cliscb.Deps
		timeout *float64
	}{
		{"default dependencies", cliscb.DefaultDeps(), nil},
		{"implicit dependencies with configured timeout", cliscb.Deps{}, &timeout},
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
				case "/api/v4/contests/2":
					_, _ = io.WriteString(w, `{"id":"2","shortname":"demo","name":"Demo","state":{}}`)
				case "/api/v4/contests/2/teams":
					_, _ = io.WriteString(w, `[{"id":"1","name":"Team Alpha","group_ids":["main"],"organization_id":"org"}]`)
				case "/api/v4/contests/2/organizations":
					_, _ = io.WriteString(w, `[{"id":"org","name":"Example University"}]`)
				case "/api/v4/contests/2/groups":
					_, _ = io.WriteString(w, `[{"id":"main","name":"Main","sortorder":0,"hidden":false}]`)
				case "/api/v4/contests/2/scoreboard":
					if r.URL.Query().Get("sortorder") != "0" || r.URL.Query().Get("allteams") != "true" {
						t.Errorf("scoreboard query = %q, want allteams=true&sortorder=0", r.URL.RawQuery)
					}
					_, _ = io.WriteString(w, `{"rows":[{"rank":1,"team_id":"1","score":{"num_solved":1,"total_time":30},"problems":[{"label":"A","problem_id":"a","num_judged":1,"solved":true,"time":30}]}]}`)
				default:
					t.Errorf("unexpected endpoint: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(srv.Close)

			dir := t.TempDir()
			root := cli.NewRootCmd(cli.Deps{})
			root.AddCommand(cliscb.NewDownloadScoreboardCmd(tc.deps))
			root.SetArgs([]string{"download-scoreboard", "--cid", "2", "--path", dir, "--filename", "results"})
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			cfg := &config.DomServerClient{Host: srv.URL, Username: "jury", Password: "secret", Timeout: tc.timeout}
			if err := root.ExecuteContext(cli.WithConfig(context.Background(), cfg)); err != nil {
				t.Fatalf("execute: %v", err)
			}

			body, err := os.ReadFile(filepath.Join(dir, "results.csv"))
			if err != nil {
				t.Fatalf("read CSV: %v", err)
			}
			got, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
			if err != nil {
				t.Fatalf("parse exported CSV: %v", err)
			}
			want := [][]string{
				{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score", "Category", "A"},
				{"1", "Example University", "Alpha", "1", "30", "Main", "30/1 try"},
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("CSV records = %#v, want %#v", got, want)
			}
			if bytes.Count(body, []byte("\r\n")) != len(want) || bytes.Count(body, []byte("\n")) != len(want) {
				t.Errorf("CSV must use CRLF line endings: %q", body)
			}
		})
	}
}

func TestDownloadScoreboard_HTTPFailureDoesNotCreateCSV_Integration(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"authentication rejected", http.StatusUnauthorized, "credentials rejected", apiv4.ErrUnauthorized},
		{"server failure", http.StatusServiceUnavailable, "temporarily unavailable", apiv4.ErrServerError},
		{"invalid response", http.StatusOK, `{"id":`, nil},
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
			root.AddCommand(cliscb.NewDownloadScoreboardCmd(cliscb.DefaultDeps()))
			root.SetArgs([]string{"download-scoreboard", "--cid", "2", "--path", dir})
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
			if _, err := os.Stat(filepath.Join(dir, "export.csv")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("CSV after failed download: stat error = %v, want not exist", err)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout after failed download = %q, want no success report", stdout.String())
			}
		})
	}
}
