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

	mux := handler.New(pStore)
	handler.RegisterDownloadScoreboard(mux, pStore, runDownload)
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
