package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
)

// SetPasswordOpts are the parameters for a set-password operation.
type SetPasswordOpts struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // empty = generate
}

// SetPasswordResult is the outcome of a set-password operation.
type SetPasswordResult struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Generated bool   `json:"generated"`
}

// SetPasswordFunc is the injectable set-password operation.
// Production wires desktopRunSetPassword; tests pass a mock.
type SetPasswordFunc func(ctx context.Context, p profile.ServerProfile, opts SetPasswordOpts) (SetPasswordResult, error)

// RegisterSetPassword wires the /set-password routes onto mux.
func RegisterSetPassword(mux *http.ServeMux, profileStore *profile.Store, runFn SetPasswordFunc) {
	mux.HandleFunc("GET /set-password", setPasswordForm(profileStore))
	mux.HandleFunc("POST /set-password", setPasswordSubmit(profileStore, runFn))
}

func setPasswordForm(profileStore *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("set-password form: list profiles", "err", err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.SetPasswordForm(profiles, "", "").Render(r.Context(), w); err != nil {
			slog.Error("render set-password form", "err", err)
		}
	}
}

func setPasswordSubmit(profileStore *profile.Store, runFn SetPasswordFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		profileID, pErr := strconv.ParseInt(r.FormValue("profile_id"), 10, 64)
		if pErr != nil || profileID <= 0 {
			http.Error(w, "invalid profile_id", http.StatusBadRequest)
			return
		}
		p, err := profileStore.Get(r.Context(), profileID)
		if err != nil {
			http.Error(w, "profile not found", http.StatusBadRequest)
			return
		}

		username := strings.TrimSpace(r.FormValue("username"))
		if username == "" {
			http.Error(w, "username is required", http.StatusBadRequest)
			return
		}

		opts := SetPasswordOpts{
			Username: username,
			Password: strings.TrimSpace(r.FormValue("password")),
		}

		result, err := runFn(r.Context(), p, opts)
		if err != nil {
			slog.Error("set-password: run", "err", err)
			profiles, _ := profileStore.List(r.Context())
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if tErr := templates.SetPasswordForm(profiles, err.Error(), "error").Render(r.Context(), w); tErr != nil {
				slog.Error("render set-password form error", "err", tErr)
			}
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.SetPasswordDone(result.Username, result.Password, result.Generated).Render(r.Context(), w); err != nil {
			slog.Error("render set-password done", "err", err)
		}
	}
}
