package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ntub/domjudge-tool/internal/webui/profile"
	"github.com/ntub/domjudge-tool/internal/webui/templates"
)

// ExportOpts are the parameters for an export-users operation.
type ExportOpts struct {
	Format string // "csv" | "json"
	IDs    []string
	TeamID string
}

// ExportFunc is the injectable export operation used by the POST handler.
// Production wires server.runExport; tests pass a mock.
type ExportFunc func(ctx context.Context, p profile.ServerProfile, opts ExportOpts, w io.Writer) error

// RegisterExportUsers wires the /export-users routes onto mux.
func RegisterExportUsers(mux *http.ServeMux, profileStore *profile.Store, runExport ExportFunc) {
	mux.HandleFunc("GET /export-users", exportUsersForm(profileStore))
	mux.HandleFunc("POST /export-users", exportUsersDownload(profileStore, runExport))
}

func exportUsersForm(profileStore *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("export form: list profiles", "err", err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExportUsersForm(profiles).Render(r.Context(), w); err != nil {
			slog.Error("render export form", "err", err)
		}
	}
}

func exportUsersDownload(profileStore *profile.Store, runExport ExportFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		profileID, err := strconv.ParseInt(r.FormValue("profile_id"), 10, 64)
		if err != nil || profileID <= 0 {
			http.Error(w, "invalid profile_id", http.StatusBadRequest)
			return
		}
		p, err := profileStore.Get(r.Context(), profileID)
		if err != nil {
			http.Error(w, "profile not found", http.StatusBadRequest)
			return
		}

		format := r.FormValue("format")
		if format != "csv" && format != "json" {
			format = "csv"
		}

		opts := ExportOpts{
			Format: format,
			IDs:    getFormValues(r, "ids"),
			TeamID: strings.TrimSpace(r.FormValue("team_id")),
		}

		// Buffer the export so we can serve a clean error page if it fails.
		var buf bytes.Buffer
		if err := runExport(r.Context(), p, opts, &buf); err != nil {
			slog.Error("export users: run export", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		stamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("users-%s.%s", stamp, format)
		contentType := "text/csv; charset=utf-8"
		if format == "json" {
			contentType = "application/json; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		if _, err := io.Copy(w, &buf); err != nil {
			slog.Error("export users: write response", "err", err)
		}
	}
}
