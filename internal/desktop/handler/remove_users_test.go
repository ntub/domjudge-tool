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
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

func newRemoveHandler(t *testing.T, runDelete handler.DeleteFunc) http.Handler {
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
	handler.RegisterRemoveUsersTeams(mux, pStore, oStore, runDelete)
	return mux
}

func postRemoveForm(t *testing.T, h http.Handler, include, exclude string, dryRun bool) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{
		"profile_id": {"1"},
		"include":    {include},
		"exclude":    {exclude},
	}
	if dryRun {
		form.Set("dry_run", "on")
	}
	req := httptest.NewRequest(http.MethodPost, "/remove-users-teams",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRemoveUsers_GetForm(t *testing.T) {
	h := newRemoveHandler(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/remove-users-teams", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Remove Users") {
		t.Error("body should contain 'Remove Users'")
	}
	if !strings.Contains(body, "dry_run") {
		t.Error("body should contain dry_run toggle")
	}
}

func TestRemoveUsers_PostForm_Redirect(t *testing.T) {
	h := newRemoveHandler(t, nil)
	rec := postRemoveForm(t, h, "user1,user2", "user3", false)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/remove-users-teams/") || !strings.HasSuffix(loc, "/run") {
		t.Errorf("Location: got %q, want /remove-users-teams/{id}/run", loc)
	}
}

func TestRemoveUsers_GetRunPage(t *testing.T) {
	h := newRemoveHandler(t, nil)
	postRemoveForm(t, h, "", "", false)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/remove-users-teams/1/run", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "remove-progress") {
		t.Error("run page should contain element id 'remove-progress'")
	}
}

func TestRemoveUsers_Stream_DryRun(t *testing.T) {
	mockDelete := func(_ context.Context, _ profile.ServerProfile, opts handler.DeleteOpts, rep progress.Reporter) (handler.DeleteResult, error) {
		rep.Start(2, "planning")
		rep.Tick()
		rep.Tick()
		rep.Finish()
		return handler.DeleteResult{
			PlannedUsers: []string{"user1", "user2"},
			PlannedTeams: []string{"team-1"},
			DryRun:       opts.DryRun,
		}, nil
	}

	h := newRemoveHandler(t, mockDelete)
	postRemoveForm(t, h, "user1,user2", "", true)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/remove-users-teams/1/stream", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type: got %q, want text/event-stream", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "datastar-patch-elements") {
		t.Errorf("body should contain SSE patch events; got: %.200s", body)
	}
}

func TestRemoveUsers_Stream_RealDelete(t *testing.T) {
	mockDelete := func(_ context.Context, _ profile.ServerProfile, _ handler.DeleteOpts, rep progress.Reporter) (handler.DeleteResult, error) {
		rep.Start(3, "removing")
		rep.Tick()
		rep.Tick()
		rep.Tick()
		rep.Finish()
		return handler.DeleteResult{
			DeletedUsers: 2,
			DeletedTeams: 2,
		}, nil
	}

	h := newRemoveHandler(t, mockDelete)
	postRemoveForm(t, h, "", "", false)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/remove-users-teams/1/stream", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "datastar-patch-elements") {
		t.Errorf("body should contain SSE patch events; got: %.200s", body)
	}
}

func TestRemoveUsers_Stream_Error(t *testing.T) {
	mockDelete := func(_ context.Context, _ profile.ServerProfile, _ handler.DeleteOpts, rep progress.Reporter) (handler.DeleteResult, error) {
		rep.Start(3, "removing")
		rep.Tick()
		return handler.DeleteResult{}, errors.New("login failed")
	}

	h := newRemoveHandler(t, mockDelete)
	postRemoveForm(t, h, "", "", false)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/remove-users-teams/1/stream", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (SSE keeps connection)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "login failed") {
		t.Errorf("body should contain error; got: %.200s", rec.Body.String())
	}
}
