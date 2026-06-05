package problems_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/cli/problems"
	"github.com/ntub/domjudge-tool/internal/config"
)

func runImportProblems(t *testing.T, cfg *config.DomServerClient, args ...string) (*cobra.Command, error) {
	t.Helper()
	cmd := problems.NewImportProblemsCmd(problems.DefaultDeps())
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(cmd)
	root.SetArgs(append([]string{"import-problems"}, args...))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	return root, root.ExecuteContext(cli.WithConfig(context.Background(), cfg))
}

func TestImportProblems_FolderUploadsSortedZIPs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "B_medium.zip"), []byte("zip-B"), 0o644); err != nil {
		t.Fatalf("write B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "A_easy.zip"), []byte("zip-A"), 0o644); err != nil {
		t.Fatalf("write A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	cap := &problemImportCapture{}
	srv := newProblemImportServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("29"),
	}

	if _, err := runImportProblems(t, cfg, dir); err != nil {
		t.Fatalf("import-problems: %v", err)
	}

	got := cap.calls()
	want := []problemImportCall{
		{CID: "29", ProblemID: "", Field: "zip", Filename: "A_easy.zip", Body: "zip-A"},
		{CID: "29", ProblemID: "", Field: "zip", Filename: "B_medium.zip", Body: "zip-B"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %+v, want %+v", got, want)
	}
}

func TestImportProblems_V7UsesZipArrayField(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "package.zip")
	if err := os.WriteFile(zipPath, []byte("zip-body"), 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	cap := &problemImportCapture{}
	srv := newProblemImportServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("1"),
		Version:   "7.3.4",
	}

	if _, err := runImportProblems(t, cfg, zipPath); err != nil {
		t.Fatalf("import-problems: %v", err)
	}

	got := cap.calls()
	if len(got) != 1 {
		t.Fatalf("calls = %d, want 1", len(got))
	}
	if got[0].Field != "zip[]" {
		t.Fatalf("field = %q, want zip[]", got[0].Field)
	}
}

func TestImportProblems_SingleZIPProblemIDFlag(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "package.zip")
	if err := os.WriteFile(zipPath, []byte("zip-body"), 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	cap := &problemImportCapture{}
	srv := newProblemImportServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("13"),
	}

	if _, err := runImportProblems(t, cfg, zipPath, "--problem-id", "explicit"); err != nil {
		t.Fatalf("import-problems: %v", err)
	}

	got := cap.calls()
	if len(got) != 1 {
		t.Fatalf("calls = %d, want 1", len(got))
	}
	if got[0].CID != "13" || got[0].ProblemID != "explicit" || got[0].Body != "zip-body" {
		t.Fatalf("call = %+v, want cid 13 explicit zip-body", got[0])
	}
}

func TestImportProblems_RequiresContestID(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "A.zip")
	if err := os.WriteFile(zipPath, []byte("zip"), 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	cfg := &config.DomServerClient{Host: "http://example.test", Username: "u", Password: "p"}

	_, err := runImportProblems(t, cfg, zipPath)
	if err == nil {
		t.Fatal("expected missing contest_id error, got nil")
	}
	if !strings.Contains(err.Error(), "contest_id") {
		t.Fatalf("err = %v, want contest_id", err)
	}
}

func TestImportProblems_ProblemIDFlagRequiresSingleZIP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "A.zip"), []byte("zip"), 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	cfg := &config.DomServerClient{
		Host:      "http://example.test",
		Username:  "u",
		Password:  "p",
		ContestID: strPtr("13"),
	}

	_, err := runImportProblems(t, cfg, dir, "--problem-id", "A")
	if err == nil {
		t.Fatal("expected --problem-id folder error, got nil")
	}
	if !strings.Contains(err.Error(), "--problem-id") {
		t.Fatalf("err = %v, want --problem-id", err)
	}
}

type problemImportCall struct {
	CID       string
	ProblemID string
	Field     string
	Filename  string
	Body      string
}

type problemImportCapture struct {
	mu    sync.Mutex
	items []problemImportCall
}

func (c *problemImportCapture) calls() []problemImportCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]problemImportCall(nil), c.items...)
}

func (c *problemImportCapture) append(call problemImportCall) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, call)
}

func newProblemImportServer(t *testing.T, cap *problemImportCapture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
			http.NotFound(w, r)
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 6 || parts[1] != "api" || parts[2] != "v4" ||
			parts[3] != "contests" || parts[5] != "problems" {
			t.Errorf("path = %s, want /api/v4/contests/{cid}/problems", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		field := "zip"
		file, header, err := r.FormFile(field)
		if err != nil {
			field = "zip[]"
			file, header, err = r.FormFile(field)
		}
		if err != nil {
			t.Errorf("FormFile zip/zip[]: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			t.Errorf("read %s: %v", field, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if got := header.Header.Get("Content-Type"); got != "application/zip" {
			t.Errorf("zip[] content type = %q, want application/zip", got)
		}
		cap.append(problemImportCall{
			CID:       parts[4],
			ProblemID: r.FormValue("problem"),
			Field:     field,
			Filename:  header.Filename,
			Body:      string(body),
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`["ok"]`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func strPtr(s string) *string { return &s }
