package handler_test

import (
	"context"
	"errors"
	"fmt"
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

type downloadTestEnv struct {
	profileStore *profile.Store
	oplogStore   *oplog.Store
}

func newDownloadEnv(t *testing.T) downloadTestEnv {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB, _ := gdb.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })

	pStore, err := profile.NewStore(gdb)
	if err != nil {
		t.Fatalf("profile.NewStore: %v", err)
	}
	oStore, err := oplog.NewStore(gdb)
	if err != nil {
		t.Fatalf("oplog.NewStore: %v", err)
	}
	return downloadTestEnv{profileStore: pStore, oplogStore: oStore}
}

func newDownloadHandler(t *testing.T, runDownload handler.DownloadFunc, defaultDir string) http.Handler {
	t.Helper()
	env := newDownloadEnv(t)
	mux := handler.New(env.profileStore)
	handler.RegisterDownloadProblems(mux, env.profileStore, env.oplogStore, runDownload, defaultDir)
	return mux
}

func TestDownloadProblemsForm_Renders(t *testing.T) {
	h := newDownloadHandler(t, nil, "/tmp/test-out")

	req := httptest.NewRequest(http.MethodGet, "/download-problems", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Download Problems") {
		t.Error("response missing 'Download Problems' heading")
	}
	if !strings.Contains(w.Body.String(), "/tmp/test-out") {
		t.Error("response missing default out_dir value")
	}
}

func TestDownloadProblemsSubmit_RedirectsToRun(t *testing.T) {
	env := newDownloadEnv(t)
	pid, err := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "test", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadProblems(mux, env.profileStore, env.oplogStore, nil, "")

	form := url.Values{
		"profile_id": {fmt.Sprintf("%d", pid)},
		"out_dir":    {"/tmp/problems"},
	}
	req := httptest.NewRequest(http.MethodPost, "/download-problems",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status=%d, want 303", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/download-problems/") || !strings.HasSuffix(loc, "/run") {
		t.Errorf("Location=%q, want /download-problems/{id}/run", loc)
	}
}

func TestDownloadProblemsSubmit_MissingOutDir(t *testing.T) {
	env := newDownloadEnv(t)
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "test2", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadProblems(mux, env.profileStore, env.oplogStore, nil, "")

	form := url.Values{"profile_id": {fmt.Sprintf("%d", pid)}} // out_dir omitted
	req := httptest.NewRequest(http.MethodPost, "/download-problems",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

func TestDownloadProblemsStream_Success(t *testing.T) {
	env := newDownloadEnv(t)
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "test3", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})
	dir := t.TempDir()

	stub := func(_ context.Context, _ profile.ServerProfile, opts handler.DownloadOpts, rep progress.Reporter) (handler.DownloadResult, error) {
		rep.Start(2, "downloading")
		rep.Tick()
		rep.Tick()
		rep.Finish()
		return handler.DownloadResult{Downloaded: 2, Skipped: 0, OutDir: opts.OutDir}, nil
	}

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadProblems(mux, env.profileStore, env.oplogStore, stub, dir)

	entry, _ := env.oplogStore.Create(context.Background(), oplog.OperationLog{
		Type:       oplog.TypeDownloadProblems,
		ProfileID:  uint(pid),
		Status:     oplog.StatusRunning,
		ParamsJSON: fmt.Sprintf(`{"out_dir":"%s"}`, dir),
	})

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/download-problems/%d/stream", entry.ID), nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "2 problem(s) downloaded") {
		t.Errorf("response missing success message; body=%q", body)
	}
}

func TestDownloadProblemsStream_Error(t *testing.T) {
	env := newDownloadEnv(t)
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "test4", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})

	stub := func(_ context.Context, _ profile.ServerProfile, _ handler.DownloadOpts, _ progress.Reporter) (handler.DownloadResult, error) {
		return handler.DownloadResult{}, errors.New("network timeout")
	}

	mux := handler.New(env.profileStore)
	handler.RegisterDownloadProblems(mux, env.profileStore, env.oplogStore, stub, "")

	entry, _ := env.oplogStore.Create(context.Background(), oplog.OperationLog{
		Type:       oplog.TypeDownloadProblems,
		ProfileID:  uint(pid),
		Status:     oplog.StatusRunning,
		ParamsJSON: `{"out_dir":"/tmp/x"}`,
	})

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/download-problems/%d/stream", entry.ID), nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "network timeout") {
		t.Errorf("response missing error message; body=%q", body)
	}
}
