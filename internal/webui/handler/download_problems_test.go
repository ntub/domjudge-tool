package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/util/progress"
	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
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

	assertOperationStreamResult(t, w, "import", "alert-success", true)
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

	assertOperationStreamResult(t, w, "import", "alert-error", false)
	body := w.Body.String()
	if !strings.Contains(body, "network timeout") {
		t.Errorf("response missing error message; body=%q", body)
	}
}

func TestDownloadProblemsRunPage(t *testing.T) {
	h := newDownloadHandler(t, nil, "/tmp/out")

	for _, id := range []string{"7", "42"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download-problems/"+id+"/run", nil))
		assertOperationRunPage(t, rec, "/download-problems/"+id+"/stream", "import")
	}

	recBad := httptest.NewRecorder()
	h.ServeHTTP(recBad, httptest.NewRequest(http.MethodGet, "/download-problems/bad/run", nil))
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", recBad.Code)
	}
}

func TestDownloadProblemsSubmit_ValidationErrors(t *testing.T) {
	env := newDownloadEnv(t)
	mux := handler.New(env.profileStore)
	handler.RegisterDownloadProblems(mux, env.profileStore, env.oplogStore, nil, "")

	form1 := url.Values{"profile_id": {"bad"}}
	req1 := httptest.NewRequest(http.MethodPost, "/download-problems", strings.NewReader(form1.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec1.Code)
	}

	form2 := url.Values{"profile_id": {"9999"}}
	req2 := httptest.NewRequest(http.MethodPost, "/download-problems", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec2.Code)
	}

	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "test-dl-prof", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})
	form3 := url.Values{"profile_id": {fmt.Sprintf("%d", pid)}, "out_dir": {""}}
	req3 := httptest.NewRequest(http.MethodPost, "/download-problems", strings.NewReader(form3.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec3.Code)
	}
}

func TestDownloadProblemsSubmit_ContestSelectionAndFilters_Integration(t *testing.T) {
	for _, tc := range []struct {
		name          string
		contest       string
		wantContest   string
		wantSelection string
	}{
		{name: "profile contest fallback", wantContest: "profile-contest"},
		{name: "selected contest overrides profile", contest: "  selected-contest  ", wantContest: "selected-contest", wantSelection: "selected-contest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newDownloadEnv(t)
			ctx := context.Background()
			profileContest := "profile-contest"
			pid, err := env.profileStore.Create(ctx, profile.ServerProfile{
				Name: "contest-server", Host: "https://test-station.example.org", ContestID: &profileContest,
			})
			if err != nil {
				t.Fatalf("create profile: %v", err)
			}
			outDir := t.TempDir()
			wantOpts := handler.DownloadOpts{
				OutDir: outDir, ContestID: tc.wantSelection, Only: []string{"p1", "p2"}, Exclude: []string{"p3"},
			}
			calls := 0
			run := func(_ context.Context, p profile.ServerProfile, opts handler.DownloadOpts, rep progress.Reporter) (handler.DownloadResult, error) {
				calls++
				if p.ID != uint(pid) || p.ContestID == nil || *p.ContestID != tc.wantContest {
					t.Errorf("effective download profile = %+v, want profile %d contest %s", p, pid, tc.wantContest)
				}
				if !reflect.DeepEqual(opts, wantOpts) {
					t.Errorf("download options = %+v, want %+v", opts, wantOpts)
				}
				rep.Start(2, "downloading")
				rep.Tick()
				rep.Tick()
				rep.Finish()
				return handler.DownloadResult{Downloaded: 2, OutDir: opts.OutDir}, nil
			}
			mux := handler.New(env.profileStore)
			handler.RegisterDownloadProblems(mux, env.profileStore, env.oplogStore, run, "")
			form := url.Values{
				"profile_id": {fmt.Sprintf("%d", pid)}, "out_dir": {"  " + outDir + "  "},
				"contest_id": {tc.contest}, "only": {", p1, , p2, "}, "exclude": {" , p3, , "},
			}
			req := httptest.NewRequest(http.MethodPost, "/download-problems", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("download submit status = %d, want 303", rec.Code)
			}
			entries, err := env.oplogStore.List(ctx)
			if err != nil || len(entries) != 1 {
				t.Fatalf("download operations = %+v, err = %v; want one", entries, err)
			}
			entry := entries[0]
			if entry.Type != oplog.TypeDownloadProblems || entry.ProfileID != uint(pid) || entry.Status != oplog.StatusRunning {
				t.Errorf("submitted download operation = %+v", entry)
			}
			var savedOpts handler.DownloadOpts
			if err := json.Unmarshal([]byte(entry.ParamsJSON), &savedOpts); err != nil || !reflect.DeepEqual(savedOpts, wantOpts) {
				t.Fatalf("persisted download options = %+v, err = %v; want %+v", savedOpts, err, wantOpts)
			}
			if got := rec.Header().Get("Location"); got != fmt.Sprintf("/download-problems/%d/run", entry.ID) {
				t.Errorf("download redirect = %q, want operation-specific run page", got)
			}
			if calls != 0 {
				t.Errorf("submit ran download before streaming: calls=%d", calls)
			}
			stream := httptest.NewRecorder()
			mux.ServeHTTP(stream, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/download-problems/%d/stream", entry.ID), nil))
			assertOperationStreamResult(t, stream, "import", "alert-success", true)
			if calls != 1 {
				t.Errorf("stream download calls = %d, want 1", calls)
			}
			savedProfile, err := env.profileStore.Get(ctx, pid)
			if err != nil || savedProfile.ContestID == nil || *savedProfile.ContestID != profileContest {
				t.Errorf("per-operation contest selection mutated saved profile: %+v, err = %v", savedProfile, err)
			}
		})
	}
}
