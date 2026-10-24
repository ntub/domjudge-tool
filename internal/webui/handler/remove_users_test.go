package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/util/progress"
	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
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
	if !strings.Contains(rec.Body.String(), `id="import-progress"`) {
		t.Error("run page should expose the shared reporter progress target")
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
	assertOperationStreamResult(t, rec, "remove", "alert-info", true)
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
	assertOperationStreamResult(t, rec, "remove", "alert-success", true)
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
	assertOperationStreamResult(t, rec, "remove", "alert-error", true)
	if !strings.Contains(rec.Body.String(), "login failed") {
		t.Errorf("body should contain error; got: %.200s", rec.Body.String())
	}
}

func TestRemoveUsers_RunPage_And_SubmitErrors(t *testing.T) {
	h := newRemoveHandler(t, nil)

	for _, id := range []string{"7", "42"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/remove-users-teams/"+id+"/run", nil))
		assertOperationRunPage(t, rec, "/remove-users-teams/"+id+"/stream", "remove")
	}

	recBadID := httptest.NewRecorder()
	h.ServeHTTP(recBadID, httptest.NewRequest(http.MethodGet, "/remove-users-teams/bad/run", nil))
	if recBadID.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", recBadID.Code)
	}

	formBad := url.Values{"profile_id": {"notanumber"}}
	reqBad := httptest.NewRequest(http.MethodPost, "/remove-users-teams", strings.NewReader(formBad.Encode()))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBad := httptest.NewRecorder()
	h.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", recBad.Code)
	}

	formMissing := url.Values{"profile_id": {"9999"}}
	reqMissing := httptest.NewRequest(http.MethodPost, "/remove-users-teams", strings.NewReader(formMissing.Encode()))
	reqMissing.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recMissing := httptest.NewRecorder()
	h.ServeHTTP(recMissing, reqMissing)
	if recMissing.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", recMissing.Code)
	}
}

func TestRemoveUsers_Stream_Errors(t *testing.T) {
	h := newRemoveHandler(t, nil)

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/remove-users-teams/bad/stream", nil))
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/remove-users-teams/9999/stream", nil))
	if rec2.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec2.Code)
	}
}

func TestRemoveUsers_Stream_CanceledRequestPersistsOutcome(t *testing.T) {
	tests := []struct {
		name       string
		result     handler.DeleteResult
		canceled   bool
		wantStatus string
		wantRows   int
	}{
		{
			name:       "canceled deletion",
			canceled:   true,
			wantStatus: oplog.StatusError,
		},
		{
			name:       "deletion completed before disconnect",
			result:     handler.DeleteResult{DeletedUsers: 2, DeletedTeams: 1},
			wantStatus: oplog.StatusDone,
			wantRows:   3,
		},
		{
			name: "dry run completed before disconnect",
			result: handler.DeleteResult{
				DryRun:       true,
				PlannedUsers: []string{"user1", "user2"},
				PlannedTeams: []string{"team-1"},
			},
			wantStatus: oplog.StatusDone,
			wantRows:   3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newHistoryEnv(t)
			mux := handler.New(env.pStore)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			handler.RegisterRemoveUsersTeams(mux, env.pStore, env.oStore,
				func(ctx context.Context, _ profile.ServerProfile, _ handler.DeleteOpts, _ progress.Reporter) (handler.DeleteResult, error) {
					cancel()
					if tc.canceled {
						return handler.DeleteResult{}, ctx.Err()
					}
					return tc.result, nil
				})
			submit := postRemoveForm(t, mux, "user1,user2", "", tc.result.DryRun)
			if submit.Code != http.StatusSeeOther {
				t.Fatalf("submit status = %d, want 303", submit.Code)
			}
			req := httptest.NewRequest(http.MethodGet, "/remove-users-teams/1/stream", nil).WithContext(ctx)
			mux.ServeHTTP(httptest.NewRecorder(), req)

			entry, err := env.oStore.Get(context.Background(), 1)
			if err != nil {
				t.Fatalf("get operation: %v", err)
			}
			if entry.Status != tc.wantStatus || entry.FinishedAt == nil {
				t.Fatalf("terminal outcome = status %q, finished %v; want %q with a finish time",
					entry.Status, entry.FinishedAt, tc.wantStatus)
			}
			if entry.RowCount != tc.wantRows {
				t.Errorf("row count = %d, want %d", entry.RowCount, tc.wantRows)
			}
			if tc.canceled && entry.ErrorMsg != context.Canceled.Error() {
				t.Errorf("error = %q, want cancellation", entry.ErrorMsg)
			}
		})
	}
}
