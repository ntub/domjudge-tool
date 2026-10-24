package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ntub/domjudge-tool/internal/webui/profile"
	"github.com/ntub/domjudge-tool/internal/webui/settings"
	"github.com/ntub/domjudge-tool/internal/webui/templates"
)

// RegisterSettings wires the Settings page.
func RegisterSettings(mux *http.ServeMux, settingsStore *settings.Store, profileStore *profile.Store) {
	mux.HandleFunc("GET /settings", settingsForm(settingsStore, profileStore))
	mux.HandleFunc("POST /settings", settingsSave(settingsStore, profileStore))
}

func settingsForm(settingsStore *settings.Store, profileStore *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.URL.Query().Get("kind")
		if strings.TrimSpace(kind) == "" {
			kind = "info"
		}
		renderSettings(w, r, settingsStore, profileStore, r.URL.Query().Get("flash"), kind, http.StatusOK)
	}
}

func renderSettings(w http.ResponseWriter, r *http.Request, settingsStore *settings.Store, profileStore *profile.Store, flash, kind string, statusCode int) {
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := templates.SettingsForm(cur, profiles, flash, kind).Render(r.Context(), w); err != nil {
		slog.Error("render settings form", "err", err)
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
			renderSettings(w, r, settingsStore, profileStore, err.Error(), "error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/settings?flash=Settings+saved.&kind=success", http.StatusSeeOther)
	}
}
