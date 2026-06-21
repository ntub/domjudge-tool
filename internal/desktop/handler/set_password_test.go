package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
)

func newSetPasswordEnv(t *testing.T, fn handler.SetPasswordFunc) http.Handler {
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
		Name: "test-server",
		Host: "https://test-station.example.org",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	mux := handler.New(pStore)
	handler.RegisterSetPassword(mux, pStore, fn)
	return mux
}

func TestSetPassword_GetForm(t *testing.T) {
	h := newSetPasswordEnv(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/set-password", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Set Password") {
		t.Error("body should contain 'Set Password'")
	}
	if !strings.Contains(body, "username") {
		t.Error("body should contain username field")
	}
}

func TestSetPassword_Post_Success(t *testing.T) {
	mockFn := func(_ context.Context, _ profile.ServerProfile, _ handler.SetPasswordOpts) (handler.SetPasswordResult, error) {
		return handler.SetPasswordResult{Username: "user1", Password: "secret123", Generated: false}, nil
	}
	h := newSetPasswordEnv(t, mockFn)

	form := url.Values{
		"profile_id": {"1"},
		"username":   {"user1"},
		"password":   {"secret123"},
	}
	req := httptest.NewRequest(http.MethodPost, "/set-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "user1") {
		t.Error("result page should show username")
	}
}

func TestSetPassword_Post_Generate(t *testing.T) {
	mockFn := func(_ context.Context, _ profile.ServerProfile, opts handler.SetPasswordOpts) (handler.SetPasswordResult, error) {
		if opts.Password != "" {
			return handler.SetPasswordResult{}, errors.New("expected empty password for generate")
		}
		return handler.SetPasswordResult{Username: "user1", Password: "genABC123", Generated: true}, nil
	}
	h := newSetPasswordEnv(t, mockFn)

	form := url.Values{
		"profile_id": {"1"},
		"username":   {"user1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/set-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "genABC123") {
		t.Error("result page should show generated password")
	}
}

func TestSetPassword_Post_MissingUsername(t *testing.T) {
	h := newSetPasswordEnv(t, nil)

	form := url.Values{
		"profile_id": {"1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/set-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestSetPassword_Post_Error(t *testing.T) {
	mockFn := func(_ context.Context, _ profile.ServerProfile, _ handler.SetPasswordOpts) (handler.SetPasswordResult, error) {
		return handler.SetPasswordResult{}, errors.New("user not found")
	}
	h := newSetPasswordEnv(t, mockFn)

	form := url.Values{
		"profile_id": {"1"},
		"username":   {"nobody"},
		"password":   {"pw"},
	}
	req := httptest.NewRequest(http.MethodPost, "/set-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (form re-render)", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "user not found") {
		t.Error("form re-render should show error")
	}
}
