package problems_test

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/problems"
)

// importServer captures POST /api/v4/contests/{cid}/problems multipart uploads
// and optionally serves a GET problem list for upsert tests.
type importServer struct {
	mu          sync.Mutex
	existingPro []model.Problem
	listCalls   int
	imports     []importCall
	postStatus  int
	postBody    string
}

type importCall struct {
	cid       string
	problemID string
	field     string
	filename  string
}

func (s *importServer) recordList() []model.Problem {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCalls++
	return append([]model.Problem(nil), s.existingPro...)
}

func (s *importServer) recordImport(call importCall) (int, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imports = append(s.imports, call)
	return s.postStatus, s.postBody
}

func newImportHTTPServer(t *testing.T, s *importServer) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		// /api/v4/contests/{cid}/problems → parts[0]="" [1]="api" [2]="v4" [3]="contests" [4]=cid [5]="problems"
		if len(parts) != 6 || parts[5] != "problems" {
			http.NotFound(w, r)
			return
		}
		cid := parts[4]

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(s.recordList())

		case http.MethodPost:
			if err := r.ParseMultipartForm(1 << 20); err != nil {
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
				http.Error(w, "missing zip", http.StatusBadRequest)
				return
			}
			_, _ = io.Copy(io.Discard, file)
			_ = file.Close()
			status, body := s.recordImport(importCall{
				cid:       cid,
				problemID: r.FormValue("problem"),
				field:     field,
				filename:  header.Filename,
			})
			if status != 0 {
				http.Error(w, body, status)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["ok"]`))

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func makeTempZip(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp zip %s: %v", name, err)
	}
	return path
}

func newAPIPair(srv *httptest.Server) (*apiv4.ImportsResource, *apiv4.ProblemsResource) {
	api := apiv4.New(apiv4.Options{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: "secret",
	})
	return api.Imports, api.Problems
}

func TestImport_HappyPath(t *testing.T) {
	s := &importServer{}
	srv := newImportHTTPServer(t, s)
	importer, lister := newAPIPair(srv)

	zipPath := makeTempZip(t, "A_easy.zip", "fake-zip-content")
	inputs := []problems.ImportInput{
		{Path: zipPath, Name: "A_easy.zip"},
	}

	rep := &nopReporter{}
	result, err := problems.Import(context.Background(), importer, lister, "8.3.1", "29", inputs, false, rep)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Imported != 1 {
		t.Errorf("Imported=%d, want 1", result.Imported)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listCalls != 0 {
		t.Errorf("list calls=%d, want 0 (no upsert)", s.listCalls)
	}
	if len(s.imports) != 1 {
		t.Fatalf("import calls=%d, want 1", len(s.imports))
	}
	if s.imports[0].field != "zip" {
		t.Errorf("field=%q, want zip (v8 path)", s.imports[0].field)
	}
	if s.imports[0].filename != "A_easy.zip" {
		t.Errorf("filename=%q, want A_easy.zip", s.imports[0].filename)
	}
}

func TestImport_UpsertMatchesExistingProblem(t *testing.T) {
	s := &importServer{
		existingPro: []model.Problem{
			{ID: "srv-alpha", ShortName: "alpha", ExternalID: "alpha"},
		},
	}
	srv := newImportHTTPServer(t, s)
	importer, lister := newAPIPair(srv)

	zipPath := makeTempZip(t, "alpha.zip", "fake-zip-alpha")
	inputs := []problems.ImportInput{
		{Path: zipPath, Name: "alpha.zip"},
	}

	rep := &nopReporter{}
	result, err := problems.Import(context.Background(), importer, lister, "8.3.1", "29", inputs, true, rep)
	if err != nil {
		t.Fatalf("Import upsert: %v", err)
	}
	if result.Imported != 1 {
		t.Errorf("Imported=%d, want 1", result.Imported)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listCalls != 1 {
		t.Errorf("list calls=%d, want 1", s.listCalls)
	}
	if len(s.imports) != 1 {
		t.Fatalf("import calls=%d, want 1", len(s.imports))
	}
	if s.imports[0].problemID != "srv-alpha" {
		t.Errorf("problemID=%q, want srv-alpha (matched existing)", s.imports[0].problemID)
	}
}

func TestIsDOMjudge7(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"7.3.2", true},
		{"7.0.0", true},
		{"8.3.1", false},
		{"", false},
		{" 7.3.2 ", true},
	}
	for _, c := range cases {
		if got := problems.IsDOMjudge7(c.version); got != c.want {
			t.Errorf("IsDOMjudge7(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

func TestSortInputs(t *testing.T) {
	inputs := []problems.ImportInput{
		{Path: "z/c.zip"},
		{Path: "a/b.zip"},
		{Path: "m/m.zip"},
	}
	problems.SortInputs(inputs)
	if inputs[0].Path != "a/b.zip" || inputs[1].Path != "m/m.zip" || inputs[2].Path != "z/c.zip" {
		t.Errorf("SortInputs unexpected order: %+v", inputs)
	}
}

func createProblemZip(t *testing.T, filename, iniContent string) string {
	t.Helper()
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, filename)
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	zw := zip.NewWriter(f)
	if iniContent != "" {
		w, err := zw.Create("domjudge-problem.ini")
		if err != nil {
			t.Fatalf("create ini inside zip: %v", err)
		}
		if _, err := io.WriteString(w, iniContent); err != nil {
			t.Fatalf("write ini: %v", err)
		}
	} else {
		w, err := zw.Create("other.txt")
		if err != nil {
			t.Fatalf("create other inside zip: %v", err)
		}
		if _, err := io.WriteString(w, "nothing"); err != nil {
			t.Fatalf("write other entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zw: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
	return zipPath
}

func TestImport_UpsertIdentitySelection_Integration(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		filename string
		ini      string
		existing model.Problem
		want     importCall
	}{
		{
			name:     "v8 matches INI external ID",
			version:  "8.3.1",
			filename: "unrelated-name.zip",
			ini:      "# comment\n; another comment\n\nexternalid = \"from-ini\"\n",
			existing: model.Problem{ID: "prob-10", ShortName: "not-the-identity", ExternalID: "from-ini"},
			want:     importCall{cid: "1", problemID: "prob-10", field: "zip", filename: "from-ini.zip"},
		},
		{
			name:     "v7 matches filename to server ID",
			version:  "7.3.2",
			filename: "prob-20.zip",
			existing: model.Problem{ID: "prob-20", ShortName: "different-short-name"},
			want:     importCall{cid: "1", problemID: "prob-20", field: "zip[]", filename: "prob-20.zip"},
		},
		{
			name:     "v7 matches filename external ID and ignores INI",
			version:  "7.3.2",
			filename: "from-filename.zip",
			ini:      "externalid = different-identity\n",
			existing: model.Problem{ID: "prob-30", ShortName: "different-short-name", ExternalID: "from-filename"},
			want:     importCall{cid: "1", problemID: "prob-30", field: "zip[]", filename: "prob-30.zip"},
		},
		{
			name:     "v8 falls back to exported filename prefix",
			version:  "8.3.1",
			filename: "42_Hello World.zip",
			existing: model.Problem{ID: "42"},
			want:     importCall{cid: "1", problemID: "42", field: "zip", filename: "42.zip"},
		},
		{
			name:     "v7 normalizes filename punctuation and padding",
			version:  "7.3.2",
			filename: "__alpha!!_suffix.zip",
			existing: model.Problem{ID: "prob-40", ExternalID: "alpha"},
			want:     importCall{cid: "1", problemID: "prob-40", field: "zip[]", filename: "prob-40.zip"},
		},
		{
			name:     "v7 preserves uppercase filename identity",
			version:  "7.3.2",
			filename: "ABC.zip",
			existing: model.Problem{ID: "ABC"},
			want:     importCall{cid: "1", problemID: "ABC", field: "zip[]", filename: "ABC.zip"},
		},
		{
			name:     "v7 does not update a short-name-only match",
			version:  "7.3.2",
			filename: "simple.zip",
			existing: model.Problem{ID: "prob-50", ShortName: "simple"},
			want:     importCall{cid: "1", field: "zip[]", filename: "simple.zip"},
		},
		{
			name:     "v8 ignores existing rows without server IDs",
			version:  "8.3.1",
			filename: "orphan.zip",
			existing: model.Problem{ExternalID: "orphan"},
			want:     importCall{cid: "1", field: "zip", filename: "orphan.zip"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &importServer{existingPro: []model.Problem{tc.existing}}
			srv := newImportHTTPServer(t, s)
			importer, lister := newAPIPair(srv)
			path := createProblemZip(t, tc.filename, tc.ini)
			inputs := []problems.ImportInput{{Path: path, Name: tc.filename}}

			result, err := problems.Import(context.Background(), importer, lister, tc.version, "1", inputs, true, &nopReporter{})
			if err != nil {
				t.Fatalf("Import upsert: %v", err)
			}
			if result.Imported != 1 {
				t.Errorf("Imported = %d, want 1", result.Imported)
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.listCalls != 1 {
				t.Errorf("problem list requests = %d, want 1", s.listCalls)
			}
			if len(s.imports) != 1 {
				t.Fatalf("imports = %+v, want exactly one upload", s.imports)
			}
			if s.imports[0] != tc.want {
				t.Errorf("multipart upload = %+v, want %+v", s.imports[0], tc.want)
			}
		})
	}
}

func TestImport_HTTPFailurePreservesInputAndResponse_Integration(t *testing.T) {
	tests := []struct {
		name    string
		version string
		id      string
	}{
		{"create modern package", "8.3.1", ""},
		{"update modern package", "8.3.1", "existing-problem"},
		{"create legacy package", "7.3.2", ""},
		{"update legacy package", "7.3.2", "existing-problem"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const rejection = "validation failed: package is not importable"
			s := &importServer{postStatus: http.StatusBadRequest, postBody: rejection}
			srv := newImportHTTPServer(t, s)
			importer, lister := newAPIPair(srv)
			path := makeTempZip(t, "rejected.zip", "data")
			inputs := []problems.ImportInput{{Path: path, Name: "rejected.zip", ID: tc.id}}
			rep := &nopReporter{}

			result, err := problems.Import(context.Background(), importer, lister, tc.version, "1", inputs, false, rep)
			var httpErr *apiv4.HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest || string(httpErr.Body) != rejection+"\n" {
				t.Fatalf("Import error = %v, want original HTTP status and response body", err)
			}
			for _, detail := range []string{path, tc.id, rejection} {
				if detail != "" && !strings.Contains(err.Error(), detail) {
					t.Errorf("error = %q, want input or server diagnostic %q", err, detail)
				}
			}
			if result.Imported != 0 || rep.ticks != 0 {
				t.Errorf("failed import result = %+v, ticks = %d, want no completed imports", result, rep.ticks)
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			if len(s.imports) != 1 || s.imports[0].problemID != tc.id {
				t.Errorf("attempted uploads = %+v, want one attempt targeting %q", s.imports, tc.id)
			}
		})
	}
}

func TestImport_UpsertRejectsAmbiguousOrMissingIdentity_Integration(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		existing []model.Problem
		details  []string
	}{
		{
			name:     "conflicting external IDs",
			filename: "shared.zip",
			existing: []model.Problem{{ID: "p1", ExternalID: "shared"}, {ID: "p2", ExternalID: "shared"}},
			details:  []string{"shared", "p1", "p2"},
		},
		{
			name:     "no usable filename identity",
			filename: "!!!.zip",
			details:  []string{"!!!.zip"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &importServer{existingPro: tc.existing}
			srv := newImportHTTPServer(t, s)
			importer, lister := newAPIPair(srv)
			path := makeTempZip(t, tc.filename, "not an archive")
			rep := &nopReporter{}
			result, err := problems.Import(context.Background(), importer, lister, "8.3.1", "1",
				[]problems.ImportInput{{Path: path, Name: tc.filename}}, true, rep)
			if err == nil {
				t.Fatal("Import succeeded without an unambiguous input identity")
			}
			for _, detail := range tc.details {
				if !strings.Contains(err.Error(), detail) {
					t.Errorf("error = %q, want offending identity information %q", err, detail)
				}
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			if result.Imported != 0 || rep.ticks != 0 || len(s.imports) != 0 || s.listCalls != 1 {
				t.Errorf("rejected upsert: result = %+v, ticks = %d, uploads = %+v, list calls = %d", result, rep.ticks, s.imports, s.listCalls)
			}
		})
	}
}

func TestImport_UpsertPreservesProblemListError_Integration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v4/contests/1/problems" {
			t.Errorf("unexpected request = %s %s", r.Method, r.URL.Path)
		}
		http.Error(w, "problem list unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	importer, lister := newAPIPair(srv)
	rep := &nopReporter{}
	result, err := problems.Import(context.Background(), importer, lister, "8.3.1", "1",
		[]problems.ImportInput{{Path: "unused.zip", Name: "unused.zip"}}, true, rep)
	if !errors.Is(err, apiv4.ErrServerError) {
		t.Fatalf("Import error = %v, want ErrServerError", err)
	}
	if result.Imported != 0 || rep.ticks != 0 {
		t.Errorf("failed listing result = %+v, ticks = %d, want no imported problems", result, rep.ticks)
	}
}

func TestImport_InputFailureDoesNotUpload_Integration(t *testing.T) {
	tests := []struct {
		name   string
		cancel bool
		want   error
	}{
		{"missing archive", false, os.ErrNotExist},
		{"canceled before opening archive", true, context.Canceled},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			if tc.cancel {
				cancel()
			}
			path := filepath.Join(t.TempDir(), "missing.zip")
			rep := &nopReporter{}
			result, err := problems.Import(ctx, nil, nil, "8.3.1", "1",
				[]problems.ImportInput{{Path: path, Name: "missing.zip"}}, false, rep)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Import error = %v, want %v", err, tc.want)
			}
			if result.Imported != 0 || rep.ticks != 0 {
				t.Errorf("input failure result = %+v, ticks = %d, want no completed imports", result, rep.ticks)
			}
		})
	}
}
