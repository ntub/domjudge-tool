package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
	"github.com/ntub/domjudge-tool/internal/webui/templates"
)

// DownloadScoreboardOpts are the parameters for a download-scoreboard operation.
type DownloadScoreboardOpts struct {
	ContestID     string `json:"contest_id"`
	ExcludeHidden bool   `json:"exclude_hidden"`
}

// DownloadScoreboardFunc is the injectable scoreboard download operation.
// Production wires server.runDownloadScoreboard; tests pass a mock.
type DownloadScoreboardFunc func(ctx context.Context, p profile.ServerProfile, opts DownloadScoreboardOpts, w io.Writer) error

// RegisterDownloadScoreboard wires the /download-scoreboard routes onto mux.
func RegisterDownloadScoreboard(mux *http.ServeMux, profileStore *profile.Store, oplogStore *oplog.Store, runDownload DownloadScoreboardFunc) {
	mux.HandleFunc("GET /download-scoreboard", downloadScoreboardForm(profileStore))
	mux.HandleFunc("POST /download-scoreboard", downloadScoreboardPost(profileStore, oplogStore, runDownload))
}

func downloadScoreboardForm(profileStore *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("download scoreboard form: list profiles", "err", err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.DownloadScoreboardForm(profiles).Render(r.Context(), w); err != nil {
			slog.Error("render download scoreboard form", "err", err)
		}
	}
}

func downloadScoreboardPost(profileStore *profile.Store, oplogStore *oplog.Store, runDownload DownloadScoreboardFunc) http.HandlerFunc {
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

		// Contest ID: form field overrides profile.ContestID; missing → 400.
		cid := r.FormValue("contest_id")
		if cid == "" && p.ContestID != nil {
			cid = *p.ContestID
		}
		if cid == "" {
			http.Error(w, "contest_id is required", http.StatusBadRequest)
			return
		}

		opts := DownloadScoreboardOpts{
			ContestID:     cid,
			ExcludeHidden: r.FormValue("exclude_hidden") == "on",
		}
		params, _ := json.Marshal(opts)
		entry, err := oplogStore.Create(r.Context(), oplog.OperationLog{
			Type:       oplog.TypeDownloadScoreboard,
			ProfileID:  uint(profileID),
			Status:     oplog.StatusRunning,
			ParamsJSON: string(params),
		})
		if err != nil {
			slog.Error("download scoreboard: create history", "err", err)
			http.Error(w, "could not create history entry", http.StatusInternalServerError)
			return
		}

		var buf bytes.Buffer
		if err := runDownload(r.Context(), p, opts, &buf); err != nil {
			slog.Error("download scoreboard: run", "err", err)
			if finishErr := oplogStore.FinishError(r.Context(), entry.ID, err.Error()); finishErr != nil {
				slog.Error("download scoreboard: record failure", "err", finishErr)
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := oplogStore.FinishOK(r.Context(), entry.ID, 0, fmt.Sprintf(`{"bytes":%d}`, buf.Len())); err != nil {
			slog.Error("download scoreboard: finish history", "err", err)
			http.Error(w, "could not complete history entry", http.StatusInternalServerError)
			return
		}

		stamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("scoreboard-%s.csv", stamp)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		if _, err := io.Copy(w, &buf); err != nil {
			slog.Error("download scoreboard: write response", "err", err)
		}
	}
}
