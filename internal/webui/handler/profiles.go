package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ntub/domjudge-tool/internal/webui/profile"
	"github.com/ntub/domjudge-tool/internal/webui/templates"
)

func registerProfiles(mux *http.ServeMux, store *profile.Store) {
	mux.HandleFunc("GET /profiles", profileList(store))
	mux.HandleFunc("GET /profiles/new", profileNew())
	mux.HandleFunc("POST /profiles", profileCreate(store))
	mux.HandleFunc("GET /profiles/{id}/edit", profileEditForm(store))
	mux.HandleFunc("POST /profiles/{id}", profileUpdate(store))
	mux.HandleFunc("POST /profiles/{id}/delete", profileDelete(store))
	mux.HandleFunc("POST /profiles/{id}/default", profileSetDefault(store))
}

func profileList(store *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := store.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("profile list", "err", err)
			return
		}
		flash, flashKind := flashFromQuery(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ProfileList(profiles, flash, flashKind).Render(r.Context(), w); err != nil {
			slog.Error("render profile list", "err", err)
		}
	}
}

func profileNew() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ProfileForm(
			profile.ServerProfile{Version: "7.3.2"},
			"/profiles", "Add", "", "",
		).Render(r.Context(), w); err != nil {
			slog.Error("render profile form", "err", err)
		}
	}
}

func profileCreate(store *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		p := profileFromForm(r)
		if _, err := store.Create(r.Context(), p); err != nil {
			slog.Error("profile create", "err", err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// Render errors are best-effort: the writer may be partial; log above covers diagnosis.
			if rerr := templates.ProfileForm(p, "/profiles", "Add", err.Error(), "error").Render(r.Context(), w); rerr != nil {
				slog.Error("render create error form", "err", rerr)
			}
			return
		}
		http.Redirect(w, r, "/profiles?flash=created&kind=success", http.StatusSeeOther)
	}
}

func profileEditForm(store *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		p, err := store.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		action := fmt.Sprintf("/profiles/%d", id)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ProfileForm(p, action, "Save", "", "").Render(r.Context(), w); err != nil {
			slog.Error("render edit form", "err", err)
		}
	}
}

func profileUpdate(store *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		p := profileFromForm(r)
		p.ID = uint(id)
		if err := store.Update(r.Context(), p); err != nil {
			slog.Error("profile update", "err", err)
			action := fmt.Sprintf("/profiles/%d", id)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// Render errors are best-effort: the writer may be partial; log above covers diagnosis.
			if rerr := templates.ProfileForm(p, action, "Save", err.Error(), "error").Render(r.Context(), w); rerr != nil {
				slog.Error("render update error form", "err", rerr)
			}
			return
		}
		http.Redirect(w, r, "/profiles?flash=saved&kind=success", http.StatusSeeOther)
	}
}

func profileDelete(store *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if err := store.Delete(r.Context(), id); err != nil {
			slog.Error("profile delete", "id", id, "err", err)
		}
		http.Redirect(w, r, "/profiles?flash=deleted&kind=info", http.StatusSeeOther)
	}
}

func profileSetDefault(store *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if err := store.SetDefault(r.Context(), id); err != nil {
			slog.Error("profile set-default", "id", id, "err", err)
		}
		http.Redirect(w, r, "/profiles?flash=default-set&kind=success", http.StatusSeeOther)
	}
}

// profileFromForm builds a ServerProfile from parsed form values.
func profileFromForm(r *http.Request) profile.ServerProfile {
	p := profile.ServerProfile{
		Name:       strings.TrimSpace(r.FormValue("name")),
		Host:       strings.TrimSpace(r.FormValue("host")),
		Username:   strings.TrimSpace(r.FormValue("username")),
		Password:   r.FormValue("password"),
		Version:    strings.TrimSpace(r.FormValue("version")),
		DisableSSL: r.FormValue("disable_ssl") == "on",
	}
	if cid := strings.TrimSpace(r.FormValue("contest_id")); cid != "" {
		p.ContestID = &cid
	}
	return p
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func flashFromQuery(r *http.Request) (msg, kind string) {
	flash := r.URL.Query().Get("flash")
	kind = r.URL.Query().Get("kind")
	msgs := map[string]string{
		"created":     "Server profile created.",
		"saved":       "Server profile saved.",
		"deleted":     "Server profile deleted.",
		"default-set": "Default server updated.",
	}
	if kind == "" {
		kind = "info"
	}
	return msgs[flash], kind
}
