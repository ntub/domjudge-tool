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
	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

type emailTestEnv struct {
	mux    http.Handler
	oStore *oplog.Store
	sStore *settings.Store
}

func buildEmailEnv(
	t *testing.T,
	scaffoldFn handler.EmailScaffoldFunc,
	validateFn handler.EmailValidateFunc,
	renderFn handler.EmailRenderFunc,
	sendFn handler.EmailSendFunc,
) emailTestEnv {
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
	sStore, err := settings.NewStore(gdb)
	if err != nil {
		t.Fatalf("settings.NewStore: %v", err)
	}

	mux := handler.New(pStore)
	handler.RegisterEmailTools(mux, sStore, oStore, scaffoldFn, validateFn, renderFn, sendFn)
	return emailTestEnv{mux: mux, oStore: oStore, sStore: sStore}
}

func nopScaffold(_ context.Context, _ string, _ bool) error               { return nil }
func nopValidate(_ context.Context, _ string) error                       { return nil }
func nopRender(_ context.Context, _ handler.EmailRenderOpts) (int, error) { return 3, nil }
func nopSend(_ context.Context, _ handler.EmailSendOpts, _ settings.AppSettings, _ progress.Reporter) (int, error) {
	return 2, nil
}

func TestEmailTools_Index(t *testing.T) {
	env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender, nopSend)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/email-tools", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Email Tools") {
		t.Error("body should contain 'Email Tools'")
	}
}

func TestEmailTools_Scaffold_GetForm(t *testing.T) {
	env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender, nopSend)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/email-tools/scaffold", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Scaffold") {
		t.Error("body should contain Scaffold")
	}
}

func TestEmailTools_Scaffold_Post_Success(t *testing.T) {
	env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender, nopSend)
	form := url.Values{"template_dir": {"/tmp/email-scaffold"}}
	req := httptest.NewRequest(http.MethodPost, "/email-tools/scaffold",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}
}

func TestEmailTools_Scaffold_Post_Error(t *testing.T) {
	errFn := func(_ context.Context, _ string, _ bool) error {
		return errors.New("dir already exists")
	}
	env := buildEmailEnv(t, errFn, nopValidate, nopRender, nopSend)
	form := url.Values{"template_dir": {"/tmp/exists"}}
	req := httptest.NewRequest(http.MethodPost, "/email-tools/scaffold",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303 (PRG with error flash)", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "flash=") {
		t.Errorf("Location should contain flash param; got %q", loc)
	}
}

func TestEmailTools_Validate_Post(t *testing.T) {
	env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender, nopSend)
	form := url.Values{"template_dir": {"/tmp/email-tmpl"}}
	req := httptest.NewRequest(http.MethodPost, "/email-tools/validate",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}
}

func TestEmailTools_Render_Post(t *testing.T) {
	env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender, nopSend)
	form := url.Values{
		"csv_path":     {"/tmp/users.csv"},
		"template_dir": {"/tmp/tmpl"},
		"output_dir":   {"/tmp/out"},
	}
	req := httptest.NewRequest(http.MethodPost, "/email-tools/render",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}
}

func TestEmailTools_Send_Post_CreatesOplog(t *testing.T) {
	env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender, nopSend)
	form := url.Values{
		"csv_path":     {"/tmp/users.csv"},
		"template_dir": {"/tmp/tmpl"},
	}
	req := httptest.NewRequest(http.MethodPost, "/email-tools/send",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}
	entries, _ := env.oStore.ListFiltered(context.Background(), oplog.TypeEmailSend, 0)
	if len(entries) != 1 {
		t.Errorf("oplog: got %d entries, want 1", len(entries))
	}
}
