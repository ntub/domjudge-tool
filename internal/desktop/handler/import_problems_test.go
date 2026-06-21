package handler_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

type importProblemsTestEnv struct {
	profileStore *profile.Store
	oplogStore   *oplog.Store
}

func newImportProblemsEnv(t *testing.T) importProblemsTestEnv {
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
	return importProblemsTestEnv{profileStore: pStore, oplogStore: oStore}
}

func newImportProblemsHandler(t *testing.T, run handler.ImportProblemsFunc) http.Handler {
	t.Helper()
	env := newImportProblemsEnv(t)
	mux := handler.New(env.profileStore)
	handler.RegisterImportProblems(mux, env.profileStore, env.oplogStore, run)
	return mux
}

// buildZipUpload constructs a multipart/form-data request body with one named
// field "zip_files" carrying a fake ZIP file.
func buildZipUpload(t *testing.T, extraFields url.Values, files []struct{ name, content string }) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, vals := range extraFields {
		for _, v := range vals {
			if err := mw.WriteField(k, v); err != nil {
				t.Fatalf("write field %s: %v", k, err)
			}
		}
	}
	for _, f := range files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="zip_files"; filename="%s"`, f.name))
		h.Set("Content-Type", "application/zip")
		pw, err := mw.CreatePart(h)
		if err != nil {
			t.Fatalf("create part %s: %v", f.name, err)
		}
		if _, err := io.WriteString(pw, f.content); err != nil {
			t.Fatalf("write part %s: %v", f.name, err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, mw.FormDataContentType()
}

func TestImportProblemsForm_Renders(t *testing.T) {
	h := newImportProblemsHandler(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/import-problems", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Import Problems") {
		t.Error("response missing 'Import Problems' heading")
	}
}

func TestImportProblemsSubmit_RedirectsToRun(t *testing.T) {
	env := newImportProblemsEnv(t)
	cid := "42"
	pid, err := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name:      "test",
		Host:      "https://test-station.example.org",
		Username:  "u",
		Password:  "p",
		ContestID: &cid,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	mux := handler.New(env.profileStore)
	handler.RegisterImportProblems(mux, env.profileStore, env.oplogStore, nil)

	body, ct := buildZipUpload(t,
		url.Values{"profile_id": {fmt.Sprintf("%d", pid)}},
		[]struct{ name, content string }{{"A.zip", "fake-zip"}},
	)
	req := httptest.NewRequest(http.MethodPost, "/import-problems", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status=%d, want 303; body=%s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/import-problems/") || !strings.HasSuffix(loc, "/run") {
		t.Errorf("Location=%q, want /import-problems/{id}/run", loc)
	}
}

func TestImportProblemsSubmit_MissingContestID_Returns400(t *testing.T) {
	env := newImportProblemsEnv(t)
	// Profile has no ContestID set.
	pid, err := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "no-cid", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	mux := handler.New(env.profileStore)
	handler.RegisterImportProblems(mux, env.profileStore, env.oplogStore, nil)

	body, ct := buildZipUpload(t,
		url.Values{"profile_id": {fmt.Sprintf("%d", pid)}},
		[]struct{ name, content string }{{"A.zip", "fake-zip"}},
	)
	req := httptest.NewRequest(http.MethodPost, "/import-problems", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

func TestImportProblemsSubmit_MissingFiles_Returns400(t *testing.T) {
	env := newImportProblemsEnv(t)
	cid := "10"
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name:      "x",
		Host:      "https://test-station.example.org",
		Username:  "u",
		Password:  "p",
		ContestID: &cid,
	})

	mux := handler.New(env.profileStore)
	handler.RegisterImportProblems(mux, env.profileStore, env.oplogStore, nil)

	// No zip_files in the form.
	form := url.Values{"profile_id": {fmt.Sprintf("%d", pid)}}
	req := httptest.NewRequest(http.MethodPost, "/import-problems",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

func TestImportProblemsStream_Success(t *testing.T) {
	env := newImportProblemsEnv(t)
	cid := "5"
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name:      "s",
		Host:      "https://test-station.example.org",
		Username:  "u",
		Password:  "p",
		ContestID: &cid,
	})

	stub := func(_ context.Context, _ profile.ServerProfile, opts handler.ImportProblemsOpts, rep progress.Reporter) (handler.ImportProblemsResult, error) {
		rep.Start(1, "importing")
		rep.Tick()
		rep.Finish()
		return handler.ImportProblemsResult{Imported: 1}, nil
	}

	mux := handler.New(env.profileStore)
	handler.RegisterImportProblems(mux, env.profileStore, env.oplogStore, stub)

	entry, _ := env.oplogStore.Create(context.Background(), oplog.OperationLog{
		Type:       oplog.TypeImportProblems,
		ProfileID:  uint(pid),
		Status:     oplog.StatusRunning,
		ParamsJSON: `{"contest_id":"5","upsert":false,"files":[]}`,
	})

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/import-problems/%d/stream", entry.ID), nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "1 problem(s) imported") {
		t.Errorf("response missing success message; body=%q", body)
	}
}

func TestImportProblemsStream_Error(t *testing.T) {
	env := newImportProblemsEnv(t)
	cid := "7"
	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name:      "e",
		Host:      "https://test-station.example.org",
		Username:  "u",
		Password:  "p",
		ContestID: &cid,
	})

	stub := func(_ context.Context, _ profile.ServerProfile, _ handler.ImportProblemsOpts, _ progress.Reporter) (handler.ImportProblemsResult, error) {
		return handler.ImportProblemsResult{}, errors.New("server rejected zip")
	}

	mux := handler.New(env.profileStore)
	handler.RegisterImportProblems(mux, env.profileStore, env.oplogStore, stub)

	entry, _ := env.oplogStore.Create(context.Background(), oplog.OperationLog{
		Type:       oplog.TypeImportProblems,
		ProfileID:  uint(pid),
		Status:     oplog.StatusRunning,
		ParamsJSON: `{"contest_id":"7","upsert":false,"files":[]}`,
	})

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/import-problems/%d/stream", entry.ID), nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "server rejected zip") {
		t.Errorf("response missing error message; body=%q", body)
	}
}
