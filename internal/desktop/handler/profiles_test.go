package handler_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	store, err := profile.NewStore(gdb)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return handler.New(store)
}

func TestProfiles_List_Empty(t *testing.T) {
	h := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profiles", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Server Profiles") {
		t.Error("body should contain 'Server Profiles'")
	}
}

func TestProfiles_New_Form(t *testing.T) {
	h := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profiles/new", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Add Server") {
		t.Error("body should contain 'Add Server'")
	}
}

func TestProfiles_Create_Redirect(t *testing.T) {
	h := newTestHandler(t)
	form := url.Values{
		"name":     {"prod"},
		"host":     {"https://test-station.example.org"},
		"username": {"admin"},
		"password": {"secret"},
		"version":  {"7.3.2"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profiles",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/profiles") {
		t.Errorf("Location: got %q, want prefix /profiles", loc)
	}
}

func TestProfiles_Create_ListShows(t *testing.T) {
	h := newTestHandler(t)
	form := url.Values{
		"name":     {"prod"},
		"host":     {"https://test-station.example.org"},
		"username": {"admin"},
		"password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profiles",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(httptest.NewRecorder(), req)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profiles", nil))
	if !strings.Contains(rec.Body.String(), "prod") {
		t.Error("list should show created profile name")
	}
}

func TestProfiles_Delete_Redirect(t *testing.T) {
	h := newTestHandler(t)

	form := url.Values{
		"name": {"to-delete"}, "host": {"https://test-station.example.org"},
		"username": {"admin"}, "password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profiles",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(httptest.NewRecorder(), req)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/profiles/1/delete", nil))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("delete status: got %d, want 303", rec.Code)
	}
}

func TestProfiles_Edit_Form(t *testing.T) {
	h := newTestHandler(t)

	form := url.Values{
		"name": {"prod"}, "host": {"https://test-station.example.org"},
		"username": {"admin"}, "password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profiles",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(httptest.NewRecorder(), req)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profiles/1/edit", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "prod") {
		t.Error("edit form should pre-fill profile name")
	}
}
