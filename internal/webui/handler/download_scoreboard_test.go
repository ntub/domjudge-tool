package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

func newDownloadScoreboardHandler(t *testing.T, runDownload handler.DownloadScoreboardFunc) http.Handler {
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
	oStore, err := oplog.NewStore(gdb)
	if err != nil {
		t.Fatalf("oplog.NewStore: %v", err)
	}

	mux := handler.New(pStore)
	handler.RegisterDownloadScoreboard(mux, pStore, oStore, runDownload)
	return mux
}

func postScoreboardForm(t *testing.T, h http.Handler, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/download-scoreboard", strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestDownloadScoreboardForm_Renders(t *testing.T) {
	h := newDownloadScoreboardHandler(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download-scoreboard", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Download Scoreboard", "contest_id", "exclude_hidden"} {
		if !strings.Contains(body, want) {
			t.Errorf("body should contain %q", want)
		}
	}
}

func TestDownloadScoreboardSubmit_Success(t *testing.T) {
	csvContent := "rank,team,score\r\n1,Team A,100\r\n"
	mock := func(_ context.Context, _ profile.ServerProfile, opts handler.DownloadScoreboardOpts, w io.Writer) error {
		if opts.ContestID != "c1" {
			t.Errorf("ContestID: got %q, want %q", opts.ContestID, "c1")
		}
		if opts.ExcludeHidden {
			t.Error("ExcludeHidden should be false when checkbox absent")
		}
		_, _ = io.WriteString(w, csvContent)
		return nil
	}

	rec := postScoreboardForm(t, newDownloadScoreboardHandler(t, mock), url.Values{
		"profile_id": {"1"},
		"contest_id": {"c1"},
	})

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

func TestDownloadScoreboardSubmit_ExcludeHidden(t *testing.T) {
	mock := func(_ context.Context, _ profile.ServerProfile, opts handler.DownloadScoreboardOpts, w io.Writer) error {
		if !opts.ExcludeHidden {
			t.Error("ExcludeHidden should be true when checkbox present")
		}
		_, _ = io.WriteString(w, "h\r\n")
		return nil
	}

	rec := postScoreboardForm(t, newDownloadScoreboardHandler(t, mock), url.Values{
		"profile_id":     {"1"},
		"contest_id":     {"c2"},
		"exclude_hidden": {"on"},
	})

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestDownloadScoreboardSubmit_MissingContestID_Returns400(t *testing.T) {
	rec := postScoreboardForm(t, newDownloadScoreboardHandler(t, nil), url.Values{
		"profile_id": {"1"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "contest_id") {
		t.Error("body should mention contest_id")
	}
}

func TestDownloadScoreboardSubmit_Error(t *testing.T) {
	mock := func(_ context.Context, _ profile.ServerProfile, _ handler.DownloadScoreboardOpts, _ io.Writer) error {
		return errors.New("scoreboard fetch failed")
	}

	rec := postScoreboardForm(t, newDownloadScoreboardHandler(t, mock), url.Values{
		"profile_id": {"1"},
		"contest_id": {"c1"},
	})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "scoreboard fetch failed") {
		t.Errorf("body should contain error message; got: %s", rec.Body.String())
	}
}

func TestDownloadScoreboardSubmit_MoreValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "invalid profile id", body: "profile_id=bad&contest_id=c1"},
		{name: "unknown profile", body: "profile_id=9999&contest_id=c1"},
		{name: "missing contest", body: "profile_id=1"},
		{name: "malformed form", body: "profile_id=%zz"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newDownloadEnv(t)
			if _, err := env.profileStore.Create(context.Background(), profile.ServerProfile{
				Name: "scoreboard-server", Host: "https://test-station.example.org",
			}); err != nil {
				t.Fatalf("seed scoreboard profile: %v", err)
			}
			calls := 0
			mux := handler.New(env.profileStore)
			handler.RegisterDownloadScoreboard(mux, env.profileStore, env.oplogStore,
				func(_ context.Context, _ profile.ServerProfile, _ handler.DownloadScoreboardOpts, _ io.Writer) error {
					calls++
					return nil
				})
			req := httptest.NewRequest(http.MethodPost, "/download-scoreboard", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("invalid scoreboard status = %d, want 400", rec.Code)
			}
			entries, err := env.oplogStore.List(context.Background())
			if err != nil || len(entries) != 0 || calls != 0 {
				t.Errorf("invalid request created operations or ran download: entries=%+v err=%v calls=%d", entries, err, calls)
			}
		})
	}
}

func TestDownloadScoreboardSubmit_FallbackToProfileContestID(t *testing.T) {
	env := newDownloadEnv(t)
	cid := "profile-contest"
	pid, err := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "scoreboard-fallback", Host: "https://test-station.example.org", ContestID: &cid,
	})
	if err != nil {
		t.Fatalf("seed scoreboard profile: %v", err)
	}
	csvContent := "rank,team\n1,Alpha\n"
	calls := 0
	mock := func(_ context.Context, p profile.ServerProfile, opts handler.DownloadScoreboardOpts, w io.Writer) error {
		calls++
		if p.ID != uint(pid) || opts.ContestID != cid {
			t.Errorf("effective scoreboard profile/options = %+v/%+v, want profile %d contest %s", p, opts, pid, cid)
		}
		_, err := io.WriteString(w, csvContent)
		return err
	}
	mux := handler.New(env.profileStore)
	handler.RegisterDownloadScoreboard(mux, env.profileStore, env.oplogStore, mock)
	rec := postScoreboardForm(t, mux, url.Values{"profile_id": {"1"}})
	if rec.Code != http.StatusOK || rec.Body.String() != csvContent || calls != 1 {
		t.Errorf("fallback download = status %d body %q calls %d, want returned CSV once", rec.Code, rec.Body.String(), calls)
	}
}

func TestDownloadScoreboardSubmit_PersistsOutcome(t *testing.T) {
	failure := errors.New("scoreboard service unavailable")
	csvContent := "rank,team,score\r\n1,Alpha,100\r\n2,Beta,75\r\n"
	for _, tc := range []struct {
		name   string
		runErr error
		status string
		code   int
	}{
		{name: "completed download", status: oplog.StatusDone, code: http.StatusOK},
		{name: "failed download", runErr: failure, status: oplog.StatusError, code: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newDownloadEnv(t)
			ctx := context.Background()
			profileContest := "profile-contest"
			pid, err := env.profileStore.Create(ctx, profile.ServerProfile{
				Name: "scoreboard-audit", Host: "https://test-station.example.org", ContestID: &profileContest,
			})
			if err != nil {
				t.Fatalf("seed scoreboard profile: %v", err)
			}
			wantOpts := handler.DownloadScoreboardOpts{ContestID: "selected-contest", ExcludeHidden: true}
			calls := 0
			run := func(ctx context.Context, p profile.ServerProfile, opts handler.DownloadScoreboardOpts, w io.Writer) error {
				calls++
				if p.ID != uint(pid) || opts != wantOpts {
					t.Errorf("scoreboard profile/options = %+v/%+v, want profile %d and %+v", p, opts, pid, wantOpts)
				}
				entries, err := env.oplogStore.List(ctx)
				if err != nil || len(entries) != 1 {
					t.Fatalf("running scoreboard operations = %+v, err = %v; want one", entries, err)
				}
				if entries[0].Status != oplog.StatusRunning || entries[0].FinishedAt != nil {
					t.Errorf("download did not start with an unfinished operation: %+v", entries[0])
				}
				if _, err := io.WriteString(w, csvContent); err != nil {
					t.Fatalf("write scoreboard output: %v", err)
				}
				return tc.runErr
			}
			mux := handler.New(env.profileStore)
			handler.RegisterDownloadScoreboard(mux, env.profileStore, env.oplogStore, run)
			rec := postScoreboardForm(t, mux, url.Values{
				"profile_id": {"1"}, "contest_id": {wantOpts.ContestID}, "exclude_hidden": {"on"},
			})
			if rec.Code != tc.code || calls != 1 {
				t.Fatalf("scoreboard status/calls = %d/%d, want %d/1", rec.Code, calls, tc.code)
			}
			entries, err := env.oplogStore.List(ctx)
			if err != nil || len(entries) != 1 {
				t.Fatalf("finished scoreboard operations = %+v, err = %v; want one", entries, err)
			}
			entry := entries[0]
			if entry.Type != oplog.TypeDownloadScoreboard || entry.ProfileID != uint(pid) || entry.Status != tc.status {
				t.Errorf("finished scoreboard operation = %+v, want download-scoreboard for profile %d with status %s", entry, pid, tc.status)
			}
			if entry.FinishedAt == nil || entry.FinishedAt.Before(entry.StartedAt) {
				t.Errorf("scoreboard completion = %v, want at or after start %v", entry.FinishedAt, entry.StartedAt)
			}
			var params handler.DownloadScoreboardOpts
			if err := json.Unmarshal([]byte(entry.ParamsJSON), &params); err != nil || params != wantOpts {
				t.Errorf("scoreboard persisted options = %+v, err = %v; want %+v", params, err, wantOpts)
			}
			if tc.runErr != nil {
				if entry.ErrorMsg != failure.Error() || entry.SummaryJSON != "" {
					t.Errorf("failed scoreboard error/summary = %q/%q", entry.ErrorMsg, entry.SummaryJSON)
				}
				if rec.Header().Get("Content-Disposition") != "" || strings.Contains(rec.Body.String(), csvContent) || !strings.Contains(rec.Body.String(), failure.Error()) {
					t.Errorf("failed download leaked a CSV attachment or lost its error: headers=%v body=%s", rec.Header(), rec.Body.String())
				}
			} else {
				var summary map[string]int
				if err := json.Unmarshal([]byte(entry.SummaryJSON), &summary); err != nil || !reflect.DeepEqual(summary, map[string]int{"bytes": len(csvContent)}) {
					t.Errorf("scoreboard summary = %+v, err = %v; want exported byte count %d", summary, err, len(csvContent))
				}
				if entry.ErrorMsg != "" || rec.Body.String() != csvContent || !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
					t.Errorf("completed download error/output = %q/%q; disposition=%q", entry.ErrorMsg, rec.Body.String(), rec.Header().Get("Content-Disposition"))
				}
			}
			savedProfile, err := env.profileStore.Get(ctx, pid)
			if err != nil || savedProfile.ContestID == nil || *savedProfile.ContestID != profileContest {
				t.Errorf("scoreboard contest override mutated profile: %+v, err = %v", savedProfile, err)
			}
		})
	}
}
