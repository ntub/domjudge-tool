package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
)

// RegisterHistory wires the /history routes onto mux.
func RegisterHistory(mux *http.ServeMux, profileStore *profile.Store, oplogStore *oplog.Store) {
	mux.HandleFunc("GET /history", historyList(profileStore, oplogStore))
	mux.HandleFunc("GET /history/{id}", historyDetail(profileStore, oplogStore))
}

func historyList(profileStore *profile.Store, oplogStore *oplog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opType := r.URL.Query().Get("type")
		var profileID uint
		if pidStr := r.URL.Query().Get("profile_id"); pidStr != "" {
			if pid, err := strconv.ParseUint(pidStr, 10, 64); err == nil {
				profileID = uint(pid)
			}
		}

		entries, err := oplogStore.ListFiltered(r.Context(), opType, profileID)
		if err != nil {
			http.Error(w, "could not load history", http.StatusInternalServerError)
			slog.Error("history: list filtered", "err", err)
			return
		}

		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("history: list profiles", "err", err)
			return
		}
		profileNames := make(map[uint]string, len(profiles))
		for i := range profiles {
			profileNames[profiles[i].ID] = profiles[i].Name
		}

		filter := templates.HistoryFilter{Type: opType, ProfileID: profileID}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.HistoryList(entries, profileNames, filter).Render(r.Context(), w); err != nil {
			slog.Error("render history list", "err", err)
		}
	}
}

func historyDetail(profileStore *profile.Store, oplogStore *oplog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil || id <= 0 {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		entry, err := oplogStore.Get(r.Context(), uint(id))
		if errors.Is(err, oplog.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "could not load entry", http.StatusInternalServerError)
			slog.Error("history detail: get entry", "err", err)
			return
		}

		profileName := ""
		if p, err := profileStore.Get(r.Context(), int64(entry.ProfileID)); err == nil {
			profileName = p.Name
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.HistoryDetail(entry, profileName).Render(r.Context(), w); err != nil {
			slog.Error("render history detail", "err", err)
		}
	}
}
