package handler_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
)

func newExportHandler(t *testing.T, runExport handler.ExportFunc) http.Handler {
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
	handler.RegisterExportUsers(mux, pStore, runExport)
	return mux
}

func postExportForm(t *testing.T, h http.Handler, format string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{
		"profile_id": {"1"},
		"format":     {format},
	}
	req := httptest.NewRequest(http.MethodPost, "/export-users",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestExportUsers_GetForm(t *testing.T) {
	h := newExportHandler(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/export-users", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Export Users") {
		t.Error("body should contain 'Export Users'")
	}
	if !strings.Contains(body, "format") {
		t.Error("body should contain format selector")
	}
}

func TestExportUsers_Post_CSV(t *testing.T) {
	csvContent := "id,username,name\n1,user1,User One\n"
	mockExport := func(_ context.Context, _ profile.ServerProfile, _ handler.ExportOpts, w io.Writer) error {
		_, _ = io.WriteString(w, csvContent)
		return nil
	}

	rec := postExportForm(t, newExportHandler(t, mockExport), "csv")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type: got %q, want text/csv", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".csv") {
		t.Errorf("Content-Disposition: got %q, want attachment with .csv", cd)
	}
	if body := rec.Body.String(); body != csvContent {
		t.Errorf("body: got %q, want %q", body, csvContent)
	}
}

func TestExportUsers_Post_JSON(t *testing.T) {
	jsonContent := `[{"id":"1","username":"user1"}]`
	mockExport := func(_ context.Context, _ profile.ServerProfile, _ handler.ExportOpts, w io.Writer) error {
		_, _ = io.WriteString(w, jsonContent)
		return nil
	}

	rec := postExportForm(t, newExportHandler(t, mockExport), "json")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, ".json") {
		t.Errorf("Content-Disposition: got %q, want .json filename", cd)
	}
}

func TestExportUsers_Post_Error(t *testing.T) {
	mockExport := func(_ context.Context, _ profile.ServerProfile, _ handler.ExportOpts, _ io.Writer) error {
		return errors.New("connection refused")
	}

	rec := postExportForm(t, newExportHandler(t, mockExport), "csv")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("body should contain error message; got: %s", rec.Body.String())
	}
}
