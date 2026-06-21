package problems_test

import (
	"context"
	"encoding/json"
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
