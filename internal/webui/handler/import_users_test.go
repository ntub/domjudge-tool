package handler_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/util/progress"
	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

func newImportHandler(t *testing.T, runImport handler.ImportFunc) http.Handler {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB, _ := gdb.DB()
	t.Cleanup(func() { sqlDB.Close() })

	pStore, err := profile.NewStore(gdb)
	if err != nil {
		t.Fatalf("profile.NewStore: %v", err)
	}
	oStore, err := oplog.NewStore(gdb)
	if err != nil {
		t.Fatalf("oplog.NewStore: %v", err)
	}

	// Pre-seed a server profile
	_, err = pStore.Create(context.Background(), profile.ServerProfile{
		Name:     "test-server",
		Host:     "https://test-station.example.org",
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create test profile: %v", err)
	}

	mux := handler.New(pStore)
	handler.RegisterImportUsersTeams(mux, pStore, oStore, runImport)
	return mux
}

func makeCSVUpload(t *testing.T, csvContent string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("csv_file", "users.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = io.WriteString(fw, csvContent)
	_ = w.WriteField("profile_id", "1")
	w.Close()
	return &buf, w.FormDataContentType()
}

func TestImportUsers_GetForm(t *testing.T) {
	h := newImportHandler(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/import-users-teams", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Import Users") {
		t.Error("body should contain 'Import Users'")
	}
	if !strings.Contains(body, "csv_file") {
		t.Error("body should contain file input 'csv_file'")
	}
}

func TestImportUsers_PostUpload_Redirect(t *testing.T) {
	h := newImportHandler(t, nil)

	buf, ct := makeCSVUpload(t, "username,name\nuser1,User One\n")
	req := httptest.NewRequest(http.MethodPost, "/import-users-teams", buf)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/import-users-teams/") || !strings.HasSuffix(loc, "/run") {
		t.Errorf("Location: got %q, want /import-users-teams/{id}/run", loc)
	}
}

func TestImportUsers_GetRunPage(t *testing.T) {
	h := newImportHandler(t, nil)

	// Upload CSV first to create an oplog entry
	buf, ct := makeCSVUpload(t, "username,name\nuser1,User One\n")
	req := httptest.NewRequest(http.MethodPost, "/import-users-teams", buf)
	req.Header.Set("Content-Type", ct)
	h.ServeHTTP(httptest.NewRecorder(), req)

	// Now GET the run page
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/import-users-teams/1/run", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "import-progress") {
		t.Error("run page should contain element id 'import-progress'")
	}
}

func TestImportUsers_Stream_Success(t *testing.T) {
	const rows = 3
	mockImport := func(_ context.Context, _ profile.ServerProfile, _ io.Reader, rep progress.Reporter) (importer.ImportUsersTeamsSummary, error) {
		rep.Start(rows, "importing")
		for range rows {
			rep.Tick()
		}
		rep.Finish()
		return importer.ImportUsersTeamsSummary{InputRows: rows, ProcessedRows: rows}, nil
	}

	h := newImportHandler(t, mockImport)

	// Upload first
	buf, ct := makeCSVUpload(t, "username,name\nuser1,User One\nuser2,User Two\nuser3,User Three\n")
	req := httptest.NewRequest(http.MethodPost, "/import-users-teams", buf)
	req.Header.Set("Content-Type", ct)
	h.ServeHTTP(httptest.NewRecorder(), req)

	// Now hit the SSE stream
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/import-users-teams/1/stream", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct2 := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct2, "text/event-stream") {
		t.Errorf("Content-Type: got %q, want text/event-stream", ct2)
	}
	assertOperationStreamResult(t, rec, "import", "alert-success", true)
}

func TestImportUsers_Stream_ImportError(t *testing.T) {
	mockImport := func(_ context.Context, _ profile.ServerProfile, _ io.Reader, rep progress.Reporter) (importer.ImportUsersTeamsSummary, error) {
		rep.Start(5, "importing")
		rep.Tick()
		return importer.ImportUsersTeamsSummary{}, errors.New("connection refused")
	}

	h := newImportHandler(t, mockImport)

	buf, ct := makeCSVUpload(t, "username,name\nuser1,User One\n")
	req := httptest.NewRequest(http.MethodPost, "/import-users-teams", buf)
	req.Header.Set("Content-Type", ct)
	h.ServeHTTP(httptest.NewRecorder(), req)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/import-users-teams/1/stream", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (SSE stays open even on error)", rec.Code)
	}
	assertOperationStreamResult(t, rec, "import", "alert-error", true)
	body := rec.Body.String()
	if !strings.Contains(body, "connection refused") {
		t.Errorf("body should contain error message; got: %s", body[:min(len(body), 300)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestImportUsers_RunPage(t *testing.T) {
	h := newImportHandler(t, nil)

	for _, id := range []string{"7", "42"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/import-users-teams/"+id+"/run", nil))
		assertOperationRunPage(t, rec, "/import-users-teams/"+id+"/stream", "import")
	}

	recBad := httptest.NewRecorder()
	h.ServeHTTP(recBad, httptest.NewRequest(http.MethodGet, "/import-users-teams/bad/run", nil))
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", recBad.Code)
	}
}

func TestImportUsers_Upload_Errors(t *testing.T) {
	h := newImportHandler(t, nil)

	// Bad multipart form
	req1 := httptest.NewRequest(http.MethodPost, "/import-users-teams", strings.NewReader("bad"))
	req1.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec1.Code)
	}

	// Invalid profile_id
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("profile_id", "xyz")
	_ = w.Close()
	req2 := httptest.NewRequest(http.MethodPost, "/import-users-teams", &buf)
	req2.Header.Set("Content-Type", w.FormDataContentType())
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec2.Code)
	}

	// Profile not found
	var buf2 bytes.Buffer
	w2 := multipart.NewWriter(&buf2)
	_ = w2.WriteField("profile_id", "9999")
	_ = w2.Close()
	req3 := httptest.NewRequest(http.MethodPost, "/import-users-teams", &buf2)
	req3.Header.Set("Content-Type", w2.FormDataContentType())
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec3.Code)
	}

	// Missing csv_file
	var buf3 bytes.Buffer
	w3 := multipart.NewWriter(&buf3)
	_ = w3.WriteField("profile_id", "1")
	_ = w3.Close()
	req4 := httptest.NewRequest(http.MethodPost, "/import-users-teams", &buf3)
	req4.Header.Set("Content-Type", w3.FormDataContentType())
	rec4 := httptest.NewRecorder()
	h.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec4.Code)
	}
}

func TestImportUsers_Stream_Errors(t *testing.T) {
	h := newImportHandler(t, nil)

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/import-users-teams/bad/stream", nil))
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/import-users-teams/9999/stream", nil))
	if rec2.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", rec2.Code)
	}
}
