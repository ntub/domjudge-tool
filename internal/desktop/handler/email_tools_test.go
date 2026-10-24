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

func TestEmailTools_Scaffold_Post_EmptyDir(t *testing.T) {
	calls := 0
	env := buildEmailEnv(t, func(_ context.Context, _ string, _ bool) error {
		calls++
		return nil
	}, nopValidate, nopRender, nopSend)
	for _, dir := range []string{"", " \t "} {
		form := url.Values{"template_dir": {dir}}
		req := httptest.NewRequest(http.MethodPost, "/email-tools/scaffold", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		env.mux.ServeHTTP(rec, req)
		assertEmailRedirect(t, rec, "/email-tools/scaffold", true)
	}
	if calls != 0 {
		t.Errorf("invalid scaffold requests invoked writer %d times, want 0", calls)
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

func TestEmailTools_Forms(t *testing.T) {
	for _, tc := range []struct {
		path   string
		fields []string
	}{
		{path: "/email-tools/validate", fields: []string{"template_dir"}},
		{path: "/email-tools/render", fields: []string{"csv_path", "template_dir", "output_dir"}},
		{path: "/email-tools/send", fields: []string{"csv_path", "template_dir"}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender, nopSend)
			for _, query := range []string{"", "?flash=review+input&kind=error"} {
				rec := httptest.NewRecorder()
				env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path+query, nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("form status = %d, want 200", rec.Code)
				}
				body := rec.Body.String()
				form := handlerTag(t, body, "form", "action", tc.path)
				if !strings.EqualFold(form["method"], "post") {
					t.Errorf("form method = %q, want POST", form["method"])
				}
				for _, name := range tc.fields {
					input := handlerTag(t, body, "input", "name", name)
					if _, ok := input["required"]; !ok {
						t.Errorf("field %q is not required", name)
					}
				}
				if query != "" {
					alert := handlerTag(t, body, "div", "role", "alert")
					if !strings.Contains(" "+alert["class"]+" ", " alert-error ") {
						t.Errorf("form error classification = %q", alert["class"])
					}
					if !strings.Contains(body, "review input") {
						t.Error("form lost redirected feedback")
					}
				}
			}
		})
	}
}

func TestEmailTools_Validate_SubmitClassification(t *testing.T) {
	failure := errors.New("template parse rejected at line 4")
	for _, tc := range []struct {
		name      string
		body      string
		runErr    error
		wantCalls int
		wantCode  int
	}{
		{name: "missing directory", body: "", wantCode: http.StatusSeeOther},
		{name: "whitespace directory", body: "template_dir=+++", wantCode: http.StatusSeeOther},
		{name: "malformed form", body: "template_dir=%zz", wantCode: http.StatusBadRequest},
		{name: "validation rejected", body: "template_dir=++%2Ftemplates++", runErr: failure, wantCalls: 1, wantCode: http.StatusSeeOther},
		{name: "valid template", body: "template_dir=++%2Ftemplates++", wantCalls: 1, wantCode: http.StatusSeeOther},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			env := buildEmailEnv(t, nopScaffold, func(_ context.Context, dir string) error {
				calls++
				if dir != "/templates" {
					t.Errorf("validation directory = %q, want trimmed path", dir)
				}
				return tc.runErr
			}, nopRender, nopSend)
			req := httptest.NewRequest(http.MethodPost, "/email-tools/validate", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			env.mux.ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("validation status = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.wantCode == http.StatusSeeOther {
				assertEmailRedirect(t, rec, "/email-tools/validate", tc.wantCalls == 0 || tc.runErr != nil)
				loc, err := rec.Result().Location()
				if err != nil {
					t.Fatalf("validation location: %v", err)
				}
				if tc.runErr != nil && loc.Query().Get("flash") != failure.Error() {
					t.Errorf("validation error feedback = %q, want %q", loc.Query().Get("flash"), failure)
				}
			}
			if calls != tc.wantCalls {
				t.Errorf("validator calls = %d, want %d", calls, tc.wantCalls)
			}
			entries, err := env.oStore.List(context.Background())
			if err != nil || len(entries) != 0 {
				t.Errorf("validate created operations = %+v, err = %v; want none", entries, err)
			}
		})
	}
}

func TestEmailTools_Render_SubmitClassification(t *testing.T) {
	failure := errors.New("recipient template rejected")
	for _, tc := range []struct {
		name      string
		omit      string
		runErr    error
		wantCalls int
	}{
		{name: "missing CSV", omit: "csv_path"},
		{name: "missing template", omit: "template_dir"},
		{name: "missing output", omit: "output_dir"},
		{name: "render failed", runErr: failure, wantCalls: 1},
		{name: "render succeeded", wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			wantOpts := handler.EmailRenderOpts{
				CSVPath: "/inputs/accounts.csv", TemplateDir: "/templates", OutputDir: "/rendered", Force: true,
			}
			env := buildEmailEnv(t, nopScaffold, nopValidate, func(_ context.Context, opts handler.EmailRenderOpts) (int, error) {
				calls++
				if opts != wantOpts {
					t.Errorf("render options = %+v, want %+v", opts, wantOpts)
				}
				return 7, tc.runErr
			}, nopSend)
			form := url.Values{
				"csv_path": {"  /inputs/accounts.csv  "}, "template_dir": {"  /templates  "},
				"output_dir": {"  /rendered  "}, "force": {"on"},
			}
			if tc.omit != "" {
				form.Set(tc.omit, " \t ")
			}
			req := httptest.NewRequest(http.MethodPost, "/email-tools/render", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			env.mux.ServeHTTP(rec, req)
			assertEmailRedirect(t, rec, "/email-tools/render", tc.omit != "" || tc.runErr != nil)
			loc, err := rec.Result().Location()
			if err != nil {
				t.Fatalf("render location: %v", err)
			}
			feedback := loc.Query().Get("flash")
			if tc.runErr != nil && feedback != failure.Error() {
				t.Errorf("render failure feedback = %q, want %q", feedback, failure)
			}
			if tc.wantCalls == 1 && tc.runErr == nil && (!strings.Contains(feedback, "7") || !strings.Contains(feedback, wantOpts.OutputDir)) {
				t.Errorf("render feedback lost count or output directory: %q", feedback)
			}
			if calls != tc.wantCalls {
				t.Errorf("renderer calls = %d, want %d", calls, tc.wantCalls)
			}
			entries, err := env.oStore.List(context.Background())
			if err != nil || len(entries) != 0 {
				t.Errorf("render created operations = %+v, err = %v; want none", entries, err)
			}
		})
	}
}

func TestEmailTools_Send_InvalidInputCreatesNoOperation(t *testing.T) {
	for _, form := range []url.Values{
		{},
		{"csv_path": {"/accounts.csv"}},
		{"template_dir": {"/templates"}},
		{"csv_path": {" \t "}, "template_dir": {"/templates"}},
		{"csv_path": {"/accounts.csv"}, "template_dir": {" \t "}},
	} {
		t.Run(form.Encode(), func(t *testing.T) {
			calls := 0
			env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender,
				func(_ context.Context, _ handler.EmailSendOpts, _ settings.AppSettings, _ progress.Reporter) (int, error) {
					calls++
					return 0, nil
				})
			req := httptest.NewRequest(http.MethodPost, "/email-tools/send", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			env.mux.ServeHTTP(rec, req)
			assertEmailRedirect(t, rec, "/email-tools/send", true)
			entries, err := env.oStore.List(context.Background())
			if err != nil || len(entries) != 0 {
				t.Errorf("invalid send created operations = %+v, err = %v; want none", entries, err)
			}
			if calls != 0 {
				t.Errorf("invalid request sent email %d times, want 0", calls)
			}
		})
	}
}

func TestEmailTools_Send_RunPage(t *testing.T) {
	env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender, nopSend)
	for _, id := range []string{"7", "42"} {
		rec := httptest.NewRecorder()
		env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/email-tools/send/"+id+"/run", nil))
		assertOperationRunPage(t, rec, "/email-tools/send/"+id+"/stream", "import")
	}
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/email-tools/send/bad/run", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid run id status = %d, want 400", rec.Code)
	}
}

func TestEmailTools_Send_Stream_SuccessAndError(t *testing.T) {
	failure := errors.New("SMTP rejected recipient row 2")
	for _, tc := range []struct {
		name     string
		sendErr  error
		wantRows int
		status   string
	}{
		{name: "sent all rows", wantRows: 2, status: oplog.StatusDone},
		{name: "recipient failure", sendErr: failure, status: oplog.StatusError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			wantOpts := handler.EmailSendOpts{CSVPath: "/inputs/accounts.csv", TemplateDir: "/templates"}
			calls := 0
			sendFn := func(_ context.Context, opts handler.EmailSendOpts, smtp settings.AppSettings, rep progress.Reporter) (int, error) {
				calls++
				if opts != wantOpts {
					t.Errorf("send options = %+v, want %+v", opts, wantOpts)
				}
				if smtp.SMTPHost != "smtp.example.org" || smtp.SMTPPort != 2525 || smtp.SMTPUsername != "operator" || smtp.SMTPPassword != "smtp-test-password" {
					t.Errorf("send did not load saved SMTP settings: %+v", smtp)
				}
				rep.Start(2, "sending")
				rep.Tick()
				if tc.sendErr != nil {
					return 1, tc.sendErr
				}
				rep.Tick()
				rep.Finish()
				return 2, nil
			}
			env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender, sendFn)
			smtp, err := env.sStore.Get(ctx)
			if err != nil {
				t.Fatalf("load SMTP settings: %v", err)
			}
			smtp.SMTPHost, smtp.SMTPPort = "smtp.example.org", 2525
			smtp.SMTPUsername, smtp.SMTPPassword = "operator", "smtp-test-password"
			if err := env.sStore.Save(ctx, smtp); err != nil {
				t.Fatalf("save SMTP settings: %v", err)
			}
			form := url.Values{"csv_path": {"  " + wantOpts.CSVPath + "  "}, "template_dir": {"  " + wantOpts.TemplateDir + "  "}}
			req := httptest.NewRequest(http.MethodPost, "/email-tools/send", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recPost := httptest.NewRecorder()
			env.mux.ServeHTTP(recPost, req)
			if recPost.Code != http.StatusSeeOther {
				t.Fatalf("send submit status = %d, want 303", recPost.Code)
			}
			entries, err := env.oStore.List(ctx)
			if err != nil || len(entries) != 1 {
				t.Fatalf("send operations = %+v, err = %v; want one", entries, err)
			}
			entry := entries[0]
			if entry.Type != oplog.TypeEmailSend || entry.Status != oplog.StatusRunning || entry.FinishedAt != nil {
				t.Errorf("submitted operation = %+v, want unfinished email-send", entry)
			}
			if got := recPost.Header().Get("Location"); got != fmt.Sprintf("/email-tools/send/%d/run", entry.ID) {
				t.Errorf("send redirect = %q, want submitted operation run page", got)
			}
			var savedOpts handler.EmailSendOpts
			if err := json.Unmarshal([]byte(entry.ParamsJSON), &savedOpts); err != nil || savedOpts != wantOpts {
				t.Fatalf("persisted send options = %+v, err = %v; want %+v", savedOpts, err, wantOpts)
			}
			if calls != 0 {
				t.Errorf("POST sent email before stream: calls=%d", calls)
			}

			rec := httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/email-tools/send/%d/stream", entry.ID), nil)
			req.Header.Set("Accept", "text/event-stream")
			env.mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
				t.Fatalf("stream status/content type = %d/%q", rec.Code, rec.Header().Get("Content-Type"))
			}
			patches := operationElementPatches(t, rec.Body.String())
			wantValues := []string{"0", "1", "2", "2"}
			if tc.sendErr != nil {
				wantValues = []string{"0", "1"}
			}
			if len(patches) != len(wantValues)+1 {
				t.Fatalf("stream patches = %d, want %d; body=%s", len(patches), len(wantValues)+1, rec.Body.String())
			}
			for i, wantValue := range wantValues {
				bar := handlerTag(t, patches[i], "progress", "id", "import-progress")
				if bar["value"] != wantValue || bar["max"] != "2" {
					t.Errorf("progress patch %d = value %q max %q, want %s/2", i, bar["value"], bar["max"], wantValue)
				}
				count := handlerTag(t, patches[i], "span", "id", "import-count")
				if count["aria-live"] != "polite" || !strings.Contains(patches[i], fmt.Sprintf(`>%s / 2</span>`, wantValue)) {
					t.Errorf("progress patch %d lost announced count: %s", i, patches[i])
				}
			}
			result := patches[len(patches)-1]
			alert := handlerTag(t, result, "div", "id", "import-result")
			if alert["role"] != "status" {
				t.Errorf("stream result role = %q, want status", alert["role"])
			}
			wantClass := "alert-success"
			if tc.sendErr != nil {
				wantClass = "alert-error"
				if !strings.Contains(result, failure.Error()) {
					t.Errorf("error result lost recipient failure: %s", result)
				}
			} else if !strings.Contains(result, "2") {
				t.Errorf("success result lost sent count: %s", result)
			}
			if !strings.Contains(" "+alert["class"]+" ", " "+wantClass+" ") {
				t.Errorf("result class = %q, want %s", alert["class"], wantClass)
			}
			finished, err := env.oStore.Get(ctx, entry.ID)
			if err != nil {
				t.Fatalf("read finished send: %v", err)
			}
			if finished.Status != tc.status || finished.RowCount != tc.wantRows {
				t.Errorf("finished send status/rows = %s/%d, want %s/%d", finished.Status, finished.RowCount, tc.status, tc.wantRows)
			}
			if finished.FinishedAt == nil || finished.FinishedAt.Before(entry.StartedAt) {
				t.Errorf("send completion time = %v, want at or after start %v", finished.FinishedAt, entry.StartedAt)
			}
			if tc.sendErr != nil {
				if finished.ErrorMsg != failure.Error() || finished.SummaryJSON != "" {
					t.Errorf("failed send error/summary = %q/%q", finished.ErrorMsg, finished.SummaryJSON)
				}
			} else {
				var summary map[string]int
				if err := json.Unmarshal([]byte(finished.SummaryJSON), &summary); err != nil || !reflect.DeepEqual(summary, map[string]int{"sent": 2}) {
					t.Errorf("send summary = %+v, err = %v; want sent=2", summary, err)
				}
				if finished.ErrorMsg != "" {
					t.Errorf("successful send recorded error = %q", finished.ErrorMsg)
				}
			}
			if finished.ParamsJSON != entry.ParamsJSON || calls != 1 {
				t.Errorf("stream changed input or execution count: params=%q calls=%d", finished.ParamsJSON, calls)
			}
		})
	}
}

func TestEmailTools_Send_Stream_InvalidID(t *testing.T) {
	calls := 0
	env := buildEmailEnv(t, nopScaffold, nopValidate, nopRender,
		func(_ context.Context, _ handler.EmailSendOpts, _ settings.AppSettings, _ progress.Reporter) (int, error) {
			calls++
			return 0, nil
		})
	for _, tc := range []struct {
		id   string
		code int
	}{
		{id: "bad", code: http.StatusBadRequest},
		{id: "9999", code: http.StatusNotFound},
	} {
		t.Run(tc.id, func(t *testing.T) {
			rec := httptest.NewRecorder()
			env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/email-tools/send/"+tc.id+"/stream", nil))
			if rec.Code != tc.code {
				t.Errorf("stream status = %d, want %d", rec.Code, tc.code)
			}
		})
	}
	if calls != 0 {
		t.Errorf("invalid operation id sent email %d times, want 0", calls)
	}
}
