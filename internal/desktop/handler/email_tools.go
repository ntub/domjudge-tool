package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/starfederation/datastar-go/datastar"

	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// EmailScaffoldFunc writes a template scaffold to dir.
type EmailScaffoldFunc func(ctx context.Context, dir string, force bool) error

// EmailValidateFunc validates a template directory.
type EmailValidateFunc func(ctx context.Context, dir string) error

// EmailRenderOpts are the parameters for a render-user-accounts operation.
type EmailRenderOpts struct {
	CSVPath     string `json:"csv_path"`
	TemplateDir string `json:"template_dir"`
	OutputDir   string `json:"out_dir"`
	Force       bool   `json:"force"`
}

// EmailRenderFunc renders .eml files from CSV + template dir.
type EmailRenderFunc func(ctx context.Context, opts EmailRenderOpts) (int, error)

// EmailSendOpts are the parameters for a send-user-accounts SSE operation.
type EmailSendOpts struct {
	CSVPath     string `json:"csv_path"`
	TemplateDir string `json:"template_dir"`
}

// EmailSendFunc sends credential emails via SMTP.
// Production reads CSV, loads template, dials SMTP, and streams progress.
type EmailSendFunc func(ctx context.Context, opts EmailSendOpts, smtp settings.AppSettings, rep progress.Reporter) (int, error)

// RegisterEmailTools wires the /email-tools routes onto mux.
func RegisterEmailTools(
	mux *http.ServeMux,
	settingsStore *settings.Store,
	oplogStore *oplog.Store,
	scaffoldFn EmailScaffoldFunc,
	validateFn EmailValidateFunc,
	renderFn EmailRenderFunc,
	sendFn EmailSendFunc,
) {
	mux.HandleFunc("GET /email-tools", emailToolsIndex())
	mux.HandleFunc("GET /email-tools/scaffold", emailScaffoldForm())
	mux.HandleFunc("POST /email-tools/scaffold", emailScaffoldSubmit(scaffoldFn))
	mux.HandleFunc("GET /email-tools/validate", emailValidateForm())
	mux.HandleFunc("POST /email-tools/validate", emailValidateSubmit(validateFn))
	mux.HandleFunc("GET /email-tools/render", emailRenderForm())
	mux.HandleFunc("POST /email-tools/render", emailRenderSubmit(renderFn))
	mux.HandleFunc("GET /email-tools/send", emailSendForm())
	mux.HandleFunc("POST /email-tools/send", emailSendSubmit(oplogStore))
	mux.HandleFunc("GET /email-tools/send/{id}/run", emailSendRunPage())
	mux.HandleFunc("GET /email-tools/send/{id}/stream", emailSendStream(settingsStore, oplogStore, sendFn))
}

func emailToolsIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flash := r.URL.Query().Get("flash")
		kind := r.URL.Query().Get("kind")
		if kind == "" {
			kind = "info"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.EmailToolsIndex(flash, kind).Render(r.Context(), w); err != nil {
			slog.Error("render email-tools index", "err", err)
		}
	}
}

// --- scaffold ---

func emailScaffoldForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flash := r.URL.Query().Get("flash")
		kind := r.URL.Query().Get("kind")
		if kind == "" {
			kind = "info"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.EmailScaffoldForm(flash, kind).Render(r.Context(), w); err != nil {
			slog.Error("render email-scaffold form", "err", err)
		}
	}
}

func emailScaffoldSubmit(scaffoldFn EmailScaffoldFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		dir := strings.TrimSpace(r.FormValue("template_dir"))
		if dir == "" {
			http.Redirect(w, r, "/email-tools/scaffold?flash=template_dir+is+required&kind=error", http.StatusSeeOther)
			return
		}
		force := r.FormValue("force") == "on"
		if err := scaffoldFn(r.Context(), dir, force); err != nil {
			slog.Error("email scaffold: run", "err", err)
			http.Redirect(w, r,
				"/email-tools/scaffold?flash="+url.QueryEscape(err.Error())+"&kind=error",
				http.StatusSeeOther)
			return
		}
		http.Redirect(w, r,
			"/email-tools/scaffold?flash="+url.QueryEscape("Template scaffold written to "+dir+"."),
			http.StatusSeeOther)
	}
}

// --- validate ---

func emailValidateForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flash := r.URL.Query().Get("flash")
		kind := r.URL.Query().Get("kind")
		if kind == "" {
			kind = "info"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.EmailValidateForm(flash, kind).Render(r.Context(), w); err != nil {
			slog.Error("render email-validate form", "err", err)
		}
	}
}

func emailValidateSubmit(validateFn EmailValidateFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		dir := strings.TrimSpace(r.FormValue("template_dir"))
		if dir == "" {
			http.Redirect(w, r, "/email-tools/validate?flash=template_dir+is+required&kind=error", http.StatusSeeOther)
			return
		}
		if err := validateFn(r.Context(), dir); err != nil {
			slog.Error("email validate: run", "err", err)
			http.Redirect(w, r,
				"/email-tools/validate?flash="+url.QueryEscape(err.Error())+"&kind=error",
				http.StatusSeeOther)
			return
		}
		http.Redirect(w, r,
			"/email-tools/validate?flash="+url.QueryEscape("Template in "+dir+" is valid."),
			http.StatusSeeOther)
	}
}

// --- render ---

func emailRenderForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flash := r.URL.Query().Get("flash")
		kind := r.URL.Query().Get("kind")
		if kind == "" {
			kind = "info"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.EmailRenderForm(flash, kind).Render(r.Context(), w); err != nil {
			slog.Error("render email-render form", "err", err)
		}
	}
}

func emailRenderSubmit(renderFn EmailRenderFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		opts := EmailRenderOpts{
			CSVPath:     strings.TrimSpace(r.FormValue("csv_path")),
			TemplateDir: strings.TrimSpace(r.FormValue("template_dir")),
			OutputDir:   strings.TrimSpace(r.FormValue("output_dir")),
			Force:       r.FormValue("force") == "on",
		}
		if opts.CSVPath == "" || opts.TemplateDir == "" || opts.OutputDir == "" {
			http.Redirect(w, r, "/email-tools/render?flash=All+fields+are+required&kind=error", http.StatusSeeOther)
			return
		}
		n, err := renderFn(r.Context(), opts)
		if err != nil {
			slog.Error("email render: run", "err", err)
			http.Redirect(w, r,
				"/email-tools/render?flash="+url.QueryEscape(err.Error())+"&kind=error",
				http.StatusSeeOther)
			return
		}
		http.Redirect(w, r,
			"/email-tools/render?flash="+url.QueryEscape(fmt.Sprintf("Rendered %d email(s) to %s.", n, opts.OutputDir)),
			http.StatusSeeOther)
	}
}

// --- send (SSE) ---

func emailSendForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flash := r.URL.Query().Get("flash")
		kind := r.URL.Query().Get("kind")
		if kind == "" {
			kind = "info"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.EmailSendForm(flash, kind).Render(r.Context(), w); err != nil {
			slog.Error("render email-send form", "err", err)
		}
	}
}

func emailSendSubmit(oplogStore *oplog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		opts := EmailSendOpts{
			CSVPath:     strings.TrimSpace(r.FormValue("csv_path")),
			TemplateDir: strings.TrimSpace(r.FormValue("template_dir")),
		}
		if opts.CSVPath == "" || opts.TemplateDir == "" {
			http.Redirect(w, r, "/email-tools/send?flash=CSV+path+and+template+dir+are+required&kind=error", http.StatusSeeOther)
			return
		}
		paramsJSON, _ := json.Marshal(opts)
		entry, err := oplogStore.Create(r.Context(), oplog.OperationLog{
			Type:       oplog.TypeEmailSend,
			Status:     oplog.StatusRunning,
			ParamsJSON: string(paramsJSON),
		})
		if err != nil {
			http.Error(w, "could not create log entry", http.StatusInternalServerError)
			slog.Error("email-send submit: create oplog", "err", err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/email-tools/send/%d/run", entry.ID), http.StatusSeeOther)
	}
}

func emailSendRunPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.EmailSendRun(id).Render(r.Context(), w); err != nil {
			slog.Error("render email-send run page", "err", err)
		}
	}
}

func emailSendStream(
	settingsStore *settings.Store,
	oplogStore *oplog.Store,
	sendFn EmailSendFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idRaw, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		entryID := uint(idRaw)

		entry, err := oplogStore.Get(r.Context(), entryID)
		if err != nil {
			http.Error(w, "log entry not found", http.StatusNotFound)
			return
		}

		var opts EmailSendOpts
		if entry.ParamsJSON != "" {
			if err := json.Unmarshal([]byte(entry.ParamsJSON), &opts); err != nil {
				slog.Warn("email-send stream: parse params", "err", err)
			}
		}

		smtpCfg, err := settingsStore.Get(r.Context())
		if err != nil {
			slog.Error("email-send stream: load settings", "err", err)
		}

		sse := datastar.NewSSE(w, r)
		rep := &sseReporter{sse: sse}

		n, sendErr := sendFn(r.Context(), opts, smtpCfg, rep)
		if sendErr != nil {
			slog.Error("email-send stream: send failed", "err", sendErr)
			if patchErr := sse.PatchElementf(
				`<div id="import-result" class="alert alert-error">Send failed: %s</div>`,
				sendErr.Error(),
			); patchErr != nil {
				slog.Error("sse patch email-send error", "err", patchErr)
			}
			_ = oplogStore.FinishError(r.Context(), entryID, sendErr.Error())
			return
		}

		summaryJSON, _ := json.Marshal(map[string]int{"sent": n})
		_ = oplogStore.FinishOK(r.Context(), entryID, n, string(summaryJSON))

		if patchErr := sse.PatchElementf(
			`<div id="import-result" class="alert alert-success">Done — %d email(s) sent.</div>`,
			n,
		); patchErr != nil {
			slog.Error("sse patch email-send result", "err", patchErr)
		}
	}
}
