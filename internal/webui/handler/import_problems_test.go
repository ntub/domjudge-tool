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

	"github.com/ntub/domjudge-tool/internal/util/progress"
	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
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

	assertOperationStreamResult(t, w, "import", "alert-success", true)
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

	assertOperationStreamResult(t, w, "import", "alert-error", false)
	body := w.Body.String()
	if !strings.Contains(body, "server rejected zip") {
		t.Errorf("response missing error message; body=%q", body)
	}
}

func TestImportProblemsRunPage(t *testing.T) {
	env := newImportProblemsEnv(t)
	mux := handler.New(env.profileStore)
	handler.RegisterImportProblems(mux, env.profileStore, env.oplogStore, nil)

	for _, id := range []string{"7", "42"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/import-problems/"+id+"/run", nil))
		assertOperationRunPage(t, rec, "/import-problems/"+id+"/stream", "import")
	}

	recBad := httptest.NewRecorder()
	mux.ServeHTTP(recBad, httptest.NewRequest(http.MethodGet, "/import-problems/bad/run", nil))
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", recBad.Code)
	}
}

func TestImportProblemsUpload_ValidationErrors(t *testing.T) {
	env := newImportProblemsEnv(t)
	mux := handler.New(env.profileStore)
	handler.RegisterImportProblems(mux, env.profileStore, env.oplogStore, nil)

	req1 := httptest.NewRequest(http.MethodPost, "/import-problems", strings.NewReader("bad"))
	req1.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec1.Code)
	}

	var buf2 bytes.Buffer
	w2 := multipart.NewWriter(&buf2)
	_ = w2.WriteField("profile_id", "bad")
	_ = w2.Close()
	req2 := httptest.NewRequest(http.MethodPost, "/import-problems", &buf2)
	req2.Header.Set("Content-Type", w2.FormDataContentType())
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec2.Code)
	}

	var buf3 bytes.Buffer
	w3 := multipart.NewWriter(&buf3)
	_ = w3.WriteField("profile_id", "9999")
	_ = w3.Close()
	req3 := httptest.NewRequest(http.MethodPost, "/import-problems", &buf3)
	req3.Header.Set("Content-Type", w3.FormDataContentType())
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec3.Code)
	}

	pid, _ := env.profileStore.Create(context.Background(), profile.ServerProfile{
		Name: "prob-no-cid", Host: "https://test-station.example.org", Username: "u", Password: "p",
	})
	var buf4 bytes.Buffer
	w4 := multipart.NewWriter(&buf4)
	_ = w4.WriteField("profile_id", fmt.Sprintf("%d", pid))
	_ = w4.Close()
	req4 := httptest.NewRequest(http.MethodPost, "/import-problems", &buf4)
	req4.Header.Set("Content-Type", w4.FormDataContentType())
	rec4 := httptest.NewRecorder()
	mux.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec4.Code)
	}

	var buf5 bytes.Buffer
	w5 := multipart.NewWriter(&buf5)
	_ = w5.WriteField("profile_id", fmt.Sprintf("%d", pid))
	_ = w5.WriteField("contest_id", "42")
	_ = w5.Close()
	req5 := httptest.NewRequest(http.MethodPost, "/import-problems", &buf5)
	req5.Header.Set("Content-Type", w5.FormDataContentType())
	rec5 := httptest.NewRecorder()
	mux.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec5.Code)
	}
}

func TestImportProblemsStream_Errors(t *testing.T) {
	env := newImportProblemsEnv(t)
	mux := handler.New(env.profileStore)
	handler.RegisterImportProblems(mux, env.profileStore, env.oplogStore, nil)

	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/import-problems/bad/stream", nil))
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/import-problems/9999/stream", nil))
	if rec2.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", rec2.Code)
	}

	entry, _ := env.oplogStore.Create(context.Background(), oplog.OperationLog{
		Type:      oplog.TypeImportProblems,
		ProfileID: 9999,
		Status:    oplog.StatusRunning,
	})
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/import-problems/%d/stream", entry.ID), nil))
	if rec3.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", rec3.Code)
	}
}
