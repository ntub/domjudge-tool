package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
)

// RegisterSettings wires the /settings routes onto mux.
func RegisterSettings(mux *http.ServeMux, settingsStore *settings.Store, profileStore *profile.Store) {
	mux.HandleFunc("GET /settings", settingsForm(settingsStore, profileStore))
	mux.HandleFunc("POST /settings", settingsSave(settingsStore, profileStore))
}

func settingsForm(settingsStore *settings.Store, profileStore *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cur, err := settingsStore.Get(r.Context())
		if err != nil {
			http.Error(w, "could not load settings", http.StatusInternalServerError)
			slog.Error("settings form: get", "err", err)
			return
		}
		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("settings form: list profiles", "err", err)
			return
		}
		flash := r.URL.Query().Get("flash")
		kind := r.URL.Query().Get("kind")
		if kind == "" {
			kind = "info"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.SettingsForm(cur, profiles, flash, kind).Render(r.Context(), w); err != nil {
			slog.Error("render settings form", "err", err)
		}
	}
}

func settingsSave(settingsStore *settings.Store, profileStore *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		theme := r.FormValue("theme")
		if theme != "dark" && theme != "light" && theme != "system" {
			theme = "dark"
		}

		var defaultProfileID uint
		if pidStr := r.FormValue("default_profile_id"); pidStr != "" {
			if pid, err := strconv.ParseUint(pidStr, 10, 64); err == nil {
				defaultProfileID = uint(pid)
			}
		}

		port := 25
		if portStr := r.FormValue("smtp_port"); portStr != "" {
			if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
				port = p
			}
		}

		row := settings.AppSettings{
			Theme:            theme,
			DefaultProfileID: defaultProfileID,
			SMTPHost:         strings.TrimSpace(r.FormValue("smtp_host")),
			SMTPPort:         port,
			SMTPFromEmail:    strings.TrimSpace(r.FormValue("smtp_from_email")),
			SMTPUseTLS:       r.FormValue("smtp_use_tls") == "on",
			SMTPUsername:     strings.TrimSpace(r.FormValue("smtp_username")),
			SMTPPassword:     r.FormValue("smtp_password"),
		}
		if err := settingsStore.Save(r.Context(), row); err != nil {
			slog.Error("settings save: save", "err", err)
			// Re-render form with error.
			cur, _ := settingsStore.Get(r.Context())
			profiles, _ := profileStore.List(r.Context())
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if tErr := templates.SettingsForm(cur, profiles, err.Error(), "error").Render(r.Context(), w); tErr != nil {
				slog.Error("render settings form error", "err", tErr)
			}
			return
		}
		http.Redirect(w, r, "/settings?flash=Settings+saved.&kind=success", http.StatusSeeOther)
	}
}
