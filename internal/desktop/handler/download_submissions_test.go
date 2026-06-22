package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

func newDownloadSubmissionsHandler(t *testing.T, runFn handler.DownloadSubmissionsFunc) http.Handler {
	t.Helper()
	env := newDownloadEnv(t)
	mux := handler.New(env.profileStore)
	handler.RegisterDownloadSubmissions(mux, env.profileStore, env.oplogStore, runFn, "/tmp/submissions")
	return mux
}

func TestDownloadSubmissionsForm_Renders(t *testing.T) {
	h := newDownloadSubmissionsHandler(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/download-submissions", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Download Submissions") {
		t.Error("response missing 'Download Submissions' heading")
	}
	for _, field := range []string{"contest_id", "out_dir", "mode", "extract"} {
		if !strings.Contains(body, field) {
			t.Errorf("response missing field %q", field)
		}
	}
}

func TestDownloadSubmissionsSubmit_RedirectsToRun(t *testing.T) {
	env := newDownloadEnv(t)
	pid, err := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "sub-test", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadSubmissions(mux, env.profileStore, env.oplogStore, nil, "")

	form := url.Values{
		"profile_id": {fmt.Sprintf("%d", pid)},
		"contest_id": {"1"},
		"out_dir":    {"/tmp/subs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/download-submissions",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status=%d, want 303", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/download-submissions/") || !strings.HasSuffix(loc, "/run") {
		t.Errorf("Location=%q, want /download-submissions/{id}/run", loc)
	}
}

func TestDownloadSubmissionsSubmit_ModeZero(t *testing.T) {
	env := newDownloadEnv(t)
	pid, err := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "sub-mode", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadSubmissions(mux, env.profileStore, env.oplogStore, nil, "")

	form := url.Values{
		"profile_id": {fmt.Sprintf("%d", pid)},
		"contest_id": {"1"},
		"out_dir":    {"/tmp/subs"},
		"mode":       {"0"},
	}
	req := httptest.NewRequest(http.MethodPost, "/download-submissions",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status=%d, want 303", w.Code)
	}

	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/download-submissions/") || !strings.HasSuffix(loc, "/run") {
		t.Fatalf("Location=%q, want /download-submissions/{id}/run", loc)
	}
	idRaw := strings.TrimSuffix(strings.TrimPrefix(loc, "/download-submissions/"), "/run")
	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil {
		t.Fatalf("location id parse: %v", err)
	}

	entry, err := env.oplogStore.Get(context.Background(), uint(id))
	if err != nil {
		t.Fatalf("get oplog entry: %v", err)
	}

	var opts handler.DownloadSubmissionsOpts
	if err := json.Unmarshal([]byte(entry.ParamsJSON), &opts); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if opts.Mode != 0 {
		t.Errorf("mode=%d, want 0", opts.Mode)
	}
}

func TestDownloadSubmissionsSubmit_MissingContestID(t *testing.T) {
	env := newDownloadEnv(t)
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "sub-t2", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadSubmissions(mux, env.profileStore, env.oplogStore, nil, "")

	form := url.Values{
		"profile_id": {fmt.Sprintf("%d", pid)},
		"out_dir":    {"/tmp/subs"},
		// contest_id omitted
	}
	req := httptest.NewRequest(http.MethodPost, "/download-submissions",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

func TestDownloadSubmissionsSubmit_MissingOutDir(t *testing.T) {
	env := newDownloadEnv(t)
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "sub-t3", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadSubmissions(mux, env.profileStore, env.oplogStore, nil, "")

	form := url.Values{
		"profile_id": {fmt.Sprintf("%d", pid)},
		"contest_id": {"1"},
		// out_dir omitted
	}
	req := httptest.NewRequest(http.MethodPost, "/download-submissions",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

func TestDownloadSubmissionsStream_Success(t *testing.T) {
	env := newDownloadEnv(t)
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "sub-t4", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})
	dir := t.TempDir()

	stub := func(_ context.Context, _ profile.ServerProfile, _ handler.DownloadSubmissionsOpts, rep progress.Reporter) (handler.DownloadSubmissionsResult, error) {
		rep.Start(3, "downloading")
		rep.Tick()
		rep.Tick()
		rep.Tick()
		rep.Finish()
		return handler.DownloadSubmissionsResult{Downloaded: 3}, nil
	}

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadSubmissions(mux, env.profileStore, env.oplogStore, stub, dir)

	entry, _ := env.oplogStore.Create(context.Background(), oplog.OperationLog{
		Type:       oplog.TypeDownloadSubmissions,
		ProfileID:  uint(pid),
		Status:     oplog.StatusRunning,
		ParamsJSON: fmt.Sprintf(`{"contest_id":"1","out_dir":"%s","mode":2,"extract":true}`, dir),
	})

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/download-submissions/%d/stream", entry.ID), nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "3 submission(s) downloaded") {
		t.Errorf("response missing success message; body=%q", body)
	}
}

func TestDownloadSubmissionsStream_Error(t *testing.T) {
	env := newDownloadEnv(t)
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "sub-t5", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})

	stub := func(_ context.Context, _ profile.ServerProfile, _ handler.DownloadSubmissionsOpts, _ progress.Reporter) (handler.DownloadSubmissionsResult, error) {
		return handler.DownloadSubmissionsResult{}, errors.New("api unreachable")
	}

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadSubmissions(mux, env.profileStore, env.oplogStore, stub, "")

	entry, _ := env.oplogStore.Create(context.Background(), oplog.OperationLog{
		Type:       oplog.TypeDownloadSubmissions,
		ProfileID:  uint(pid),
		Status:     oplog.StatusRunning,
		ParamsJSON: `{"contest_id":"1","out_dir":"/tmp/x","mode":2,"extract":false}`,
	})

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/download-submissions/%d/stream", entry.ID), nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "api unreachable") {
		t.Errorf("response missing error message; body=%q", body)
	}
}
