package problems_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/ntub/domjudge-tool/internal/model"
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
	if got[0].CID != "13" || got[0].ProblemID != "explicit" ||
		got[0].Field != "zip" || got[0].Body != "zip-body" {
		t.Fatalf("call = %+v, want cid 13 explicit zip zip-body", got[0])
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

func TestImportProblems_UpsertListsOnceUpdatesExistingAndCreatesMissing_Integration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alpha.zip"), []byte("zip-alpha"), 0o644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta.zip"), []byte("zip-beta"), 0o644); err != nil {
		t.Fatalf("write beta: %v", err)
	}

	cap := &problemUpsertCapture{
		problems: []model.Problem{
			{ID: "srv-alpha", ShortName: "alpha", ExternalID: "alpha", Name: "Alpha"},
		},
	}
	srv := newProblemUpsertServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("29"),
		Version:   "8.3.1",
	}

	if _, err := runImportProblems(t, cfg, dir, "--upsert"); err != nil {
		t.Fatalf("import-problems --upsert: %v", err)
	}

	if got := cap.listCallCount(); got != 1 {
		t.Fatalf("list calls = %d, want 1", got)
	}
	got := cap.importCalls()
	want := []problemImportCall{
		{CID: "29", ProblemID: "srv-alpha", Field: "zip", Filename: "alpha.zip", Body: "zip-alpha"},
		{CID: "29", ProblemID: "", Field: "zip", Filename: "beta.zip", Body: "zip-beta"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imports = %+v, want %+v", got, want)
	}
}

func TestImportProblems_UpsertAndProblemIDAreMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "alpha.zip")
	if err := os.WriteFile(zipPath, []byte("zip-alpha"), 0o644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}

	cap := &problemUpsertCapture{}
	srv := newProblemUpsertServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("29"),
		Version:   "8.3.1",
	}

	_, err := runImportProblems(t, cfg, zipPath, "--upsert", "--problem-id", "srv-alpha")
	if err == nil {
		t.Fatal("expected mutually exclusive --upsert/--problem-id error, got nil")
	}
	if !strings.Contains(err.Error(), "--upsert") || !strings.Contains(err.Error(), "--problem-id") {
		t.Fatalf("err = %v, want --upsert and --problem-id", err)
	}
	if got := cap.listCallCount(); got != 0 {
		t.Fatalf("list calls = %d, want 0", got)
	}
	if got := cap.importCalls(); len(got) != 0 {
		t.Fatalf("imports = %+v, want none", got)
	}
}

func TestImportProblems_UpsertCreatesNewWithVersionSpecificPath_Integration(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantField string
	}{
		{name: "domjudge 7 uses legacy zip array", version: "7.3.4", wantField: "zip[]"},
		{name: "domjudge 8 uses package zip", version: "8.3.1", wantField: "zip"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			zipPath := filepath.Join(dir, "gamma.zip")
			if err := os.WriteFile(zipPath, []byte("zip-gamma"), 0o644); err != nil {
				t.Fatalf("write gamma: %v", err)
			}

			cap := &problemUpsertCapture{}
			srv := newProblemUpsertServer(t, cap)
			cfg := &config.DomServerClient{
				Host:      srv.URL,
				Username:  "admin",
				Password:  "secret",
				ContestID: strPtr("29"),
				Version:   tc.version,
			}

			if _, err := runImportProblems(t, cfg, zipPath, "--upsert"); err != nil {
				t.Fatalf("import-problems --upsert: %v", err)
			}

			if got := cap.listCallCount(); got != 1 {
				t.Fatalf("list calls = %d, want 1", got)
			}
			got := cap.importCalls()
			want := []problemImportCall{
				{CID: "29", ProblemID: "", Field: tc.wantField, Filename: "gamma.zip", Body: "zip-gamma"},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("imports = %+v, want %+v", got, want)
			}
		})
	}
}

func TestImportProblems_UpsertMatchesDownloadedFilenamePrefix_Integration(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "A_easy.zip")
	if err := os.WriteFile(zipPath, []byte("zip-A"), 0o644); err != nil {
		t.Fatalf("write A: %v", err)
	}

	cap := &problemUpsertCapture{
		problems: []model.Problem{
			{ID: "A", ExternalID: "problem-A", Name: "easy"},
		},
	}
	srv := newProblemUpsertServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("29"),
		Version:   "7.3.4",
	}

	if _, err := runImportProblems(t, cfg, zipPath, "--upsert"); err != nil {
		t.Fatalf("import-problems --upsert: %v", err)
	}

	got := cap.importCalls()
	want := []problemImportCall{
		{CID: "29", ProblemID: "A", Field: "zip[]", Filename: "A.zip", Body: "zip-A"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imports = %+v, want %+v", got, want)
	}
}

func TestImportProblems_UpsertTrimsUnsupportedFilenamePrefix_Integration(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "有問題沒上-H8-lc-772.zip")
	if err := os.WriteFile(zipPath, []byte("zip-H8"), 0o644); err != nil {
		t.Fatalf("write H8: %v", err)
	}

	cap := &problemUpsertCapture{
		problems: []model.Problem{
			{ID: "H8", ExternalID: "H8-lc-772", Name: "expr"},
		},
	}
	srv := newProblemUpsertServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("29"),
		Version:   "8.3.1",
	}

	if _, err := runImportProblems(t, cfg, zipPath, "--upsert"); err != nil {
		t.Fatalf("import-problems --upsert: %v", err)
	}

	got := cap.importCalls()
	want := []problemImportCall{
		{CID: "29", ProblemID: "H8", Field: "zip", Filename: "H8-lc-772.zip", Body: "zip-H8"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imports = %+v, want %+v", got, want)
	}
}

func TestImportProblems_UpsertRejectsAmbiguousProblemIdentity_Integration(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "A.zip")
	if err := os.WriteFile(zipPath, []byte("zip-A"), 0o644); err != nil {
		t.Fatalf("write A: %v", err)
	}

	cap := &problemUpsertCapture{
		problems: []model.Problem{
			{ID: "A", ExternalID: "problem-A"},
			{ID: "B", ExternalID: "A"},
		},
	}
	srv := newProblemUpsertServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("29"),
		Version:   "8.3.1",
	}

	_, err := runImportProblems(t, cfg, zipPath, "--upsert")
	if err == nil {
		t.Fatal("expected ambiguous identity error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous problem identity") {
		t.Fatalf("err = %v, want ambiguous problem identity", err)
	}
	if got := cap.importCalls(); len(got) != 0 {
		t.Fatalf("imports = %+v, want none", got)
	}
}

func TestImportProblems_UpsertInfersIdentityFromV8ProblemINIExternalID_Integration(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "package.zip")
	writeProblemPackageZip(t, zipPath, map[string]string{
		"domjudge-problem.ini": "externalid = ext-v8\n",
		"problem.yaml":         "name: Package\n",
	})

	cap := &problemUpsertCapture{
		problems: []model.Problem{
			{ID: "server-ext", ShortName: "different", ExternalID: "ext-v8", Name: "Package"},
		},
	}
	srv := newProblemUpsertServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("29"),
		Version:   "8.3.1",
	}

	if _, err := runImportProblems(t, cfg, zipPath, "--upsert"); err != nil {
		t.Fatalf("import-problems --upsert: %v", err)
	}

	got := cap.importCalls()
	if len(got) != 1 {
		t.Fatalf("imports = %d, want 1", len(got))
	}
	if got[0].ProblemID != "server-ext" {
		t.Fatalf("problem id = %q, want server-ext", got[0].ProblemID)
	}
	if got[0].Field != "zip" {
		t.Fatalf("field = %q, want zip update path", got[0].Field)
	}
	if got[0].Filename != "ext-v8.zip" {
		t.Fatalf("filename = %q, want matched externalid filename", got[0].Filename)
	}
}

func TestImportProblems_UpsertPreservesDuplicateExternalIDAPIErrorBody_Integration(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "dupe.zip")
	if err := os.WriteFile(zipPath, []byte("zip-dupe"), 0o644); err != nil {
		t.Fatalf("write dupe: %v", err)
	}

	const body = "problem.externalid: This value is already used"
	cap := &problemUpsertCapture{postStatus: http.StatusBadRequest, postBody: body}
	srv := newProblemUpsertServer(t, cap)
	cfg := &config.DomServerClient{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		ContestID: strPtr("29"),
		Version:   "8.3.1",
	}

	_, err := runImportProblems(t, cfg, zipPath, "--upsert")
	if err == nil {
		t.Fatal("expected duplicate externalid error, got nil")
	}
	if !strings.Contains(err.Error(), body) {
		t.Fatalf("err = %v, want body suffix %q", err, body)
	}
	if got := cap.listCallCount(); got != 1 {
		t.Fatalf("list calls = %d, want 1", got)
	}
	got := cap.importCalls()
	want := []problemImportCall{
		{CID: "29", ProblemID: "", Field: "zip", Filename: "dupe.zip", Body: "zip-dupe"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imports = %+v, want %+v", got, want)
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

type problemUpsertCapture struct {
	mu         sync.Mutex
	problems   []model.Problem
	listCalls  int
	imports    []problemImportCall
	postStatus int
	postBody   string
}

func (c *problemUpsertCapture) listCallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.listCalls
}

func (c *problemUpsertCapture) importCalls() []problemImportCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]problemImportCall(nil), c.imports...)
}

func (c *problemUpsertCapture) recordList() []model.Problem {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listCalls++
	return append([]model.Problem(nil), c.problems...)
}

func (c *problemUpsertCapture) recordImport(call problemImportCall) (int, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.imports = append(c.imports, call)
	return c.postStatus, c.postBody
}

func newProblemUpsertServer(t *testing.T, cap *problemUpsertCapture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 6 || parts[1] != "api" || parts[2] != "v4" ||
			parts[3] != "contests" || parts[5] != "problems" {
			t.Errorf("path = %s, want /api/v4/contests/{cid}/problems", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(cap.recordList()); err != nil {
				t.Errorf("encode problems: %v", err)
			}
		case http.MethodPost:
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
				t.Errorf("%s content type = %q, want application/zip", field, got)
			}
			status, responseBody := cap.recordImport(problemImportCall{
				CID:       parts[4],
				ProblemID: r.FormValue("problem"),
				Field:     field,
				Filename:  header.Filename,
				Body:      string(body),
			})
			if status != 0 {
				http.Error(w, responseBody, status)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["ok"]`))
		default:
			t.Errorf("method = %s, want GET or POST", r.Method)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeProblemPackageZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip member %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write zip member %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func strPtr(s string) *string { return &s }
