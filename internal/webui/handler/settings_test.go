package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
	"github.com/ntub/domjudge-tool/internal/webui/settings"
)

func newSettingsEnv(t *testing.T) (http.Handler, *settings.Store) {
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
	sStore, err := settings.NewStore(gdb)
	if err != nil {
		t.Fatalf("settings.NewStore: %v", err)
	}
	_, _ = pStore.Create(context.Background(), profile.ServerProfile{
		Name: "test-server",
		Host: "https://test-station.example.org",
	})

	mux := handler.New(pStore)
	handler.RegisterSettings(mux, sStore, pStore)
	return mux, sStore
}

func TestSettings_GetForm(t *testing.T) {
	h, _ := newSettingsEnv(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Error("body should contain 'Settings'")
	}
	if !strings.Contains(body, "SMTP") {
		t.Error("body should contain SMTP section")
	}
}

func TestSettings_SaveAndReload(t *testing.T) {
	h, sStore := newSettingsEnv(t)
	ctx := context.Background()

	form := url.Values{
		"theme":           {"light"},
		"smtp_host":       {"smtp.example.org"},
		"smtp_port":       {"587"},
		"smtp_from_email": {"noreply@example.org"},
		"smtp_use_tls":    {"on"},
		"smtp_username":   {"user"},
		"smtp_password":   {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}

	got, err := sStore.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Theme != "light" {
		t.Errorf("Theme: got %q, want %q", got.Theme, "light")
	}
	if got.SMTPHost != "smtp.example.org" {
		t.Errorf("SMTPHost: got %q", got.SMTPHost)
	}
	if got.SMTPPort != 587 {
		t.Errorf("SMTPPort: got %d, want 587", got.SMTPPort)
	}
	if !got.SMTPUseTLS {
		t.Error("SMTPUseTLS should be true")
	}
}

func TestSettings_SaveDefaultProfile(t *testing.T) {
	h, sStore := newSettingsEnv(t)
	ctx := context.Background()

	form := url.Values{
		"theme":              {"dark"},
		"default_profile_id": {"1"},
		"smtp_port":          {"25"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}

	got, err := sStore.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DefaultProfileID != 1 {
		t.Errorf("DefaultProfileID: got %d, want 1", got.DefaultProfileID)
	}
}

func TestSettings_ThemeFallbackAndSetDefault(t *testing.T) {
	h, sStore := newSettingsEnv(t)
	ctx := context.Background()

	form := url.Values{
		"theme":              {"invalid-theme"},
		"default_profile_id": {"1"},
		"smtp_port":          {"invalid-port"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}

	got, err := sStore.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Theme != "dark" {
		t.Errorf("Theme: got %q, want dark", got.Theme)
	}
	if got.SMTPPort != 25 {
		t.Errorf("SMTPPort: got %d, want default 25", got.SMTPPort)
	}
}
