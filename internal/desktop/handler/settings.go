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

// RegisterSettings wires Settings and its separately protected CLI actions.
// Host must match the exact loopback address bound by the caller.
func RegisterSettings(mux *http.ServeMux, settingsStore *settings.Store, profileStore *profile.Store, opts CLIInstallOptions) error {
	cli, err := newCLIInstallHandler(opts, settingsStore, profileStore)
	if err != nil {
		return err
	}
	mux.HandleFunc("GET /settings", cli.guard(settingsForm(settingsStore, profileStore, cli)))
	mux.HandleFunc("POST /settings", settingsSave(settingsStore, profileStore))
	registerCLIActions(mux, cli)
	return nil
}

func settingsForm(settingsStore *settings.Store, profileStore *profile.Store, cli *cliInstallHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, _ := cli.inspect(r.Context()) // Inspection failures are visible in the CLI card.
		flash, kind := cliFlash(r)
		renderSettings(w, r, settingsStore, profileStore, cli.view(status), flash, kind, http.StatusOK)
	}
}

func renderSettings(w http.ResponseWriter, r *http.Request, settingsStore *settings.Store, profileStore *profile.Store, cli templates.CLIInstallView, flash, kind string, statusCode int) {
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
	if err := templates.SettingsForm(cur, profiles, cli, flash, kind).Render(r.Context(), w); err != nil {
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
			// This legacy POST is outside the CLI guard, so never render its token.
			cliResponseHeaders(w)
			renderSettings(w, r, settingsStore, profileStore, cliUnavailableView(), err.Error(), "error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/settings?flash=Settings+saved.&kind=success", http.StatusSeeOther)
	}
}
