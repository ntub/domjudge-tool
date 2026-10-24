package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/starfederation/datastar-go/datastar"

	"github.com/ntub/domjudge-tool/internal/util/progress"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
	"github.com/ntub/domjudge-tool/internal/webui/templates"
)

// DownloadSubmissionsOpts are the parameters for a download-submissions operation.
type DownloadSubmissionsOpts struct {
	ContestID  string   `json:"contest_id"`
	OutDir     string   `json:"out_dir"`
	ProblemIDs []string `json:"problem_ids,omitempty"`
	Mode       int      `json:"mode"`
	Extract    bool     `json:"extract"`
}

// DownloadSubmissionsResult is the outcome of a download-submissions operation.
type DownloadSubmissionsResult struct {
	Downloaded int `json:"downloaded"`
}

// DownloadSubmissionsFunc is the injectable download operation used by the SSE stream handler.
// Production wires server.runDownloadSubmissions; tests pass a mock.
type DownloadSubmissionsFunc func(ctx context.Context, p profile.ServerProfile, opts DownloadSubmissionsOpts, rep progress.Reporter) (DownloadSubmissionsResult, error)

// RegisterDownloadSubmissions wires the /download-submissions routes onto mux.
func RegisterDownloadSubmissions(
	mux *http.ServeMux,
	profileStore *profile.Store,
	oplogStore *oplog.Store,
	runDownload DownloadSubmissionsFunc,
	defaultOutDir string,
) {
	mux.HandleFunc("GET /download-submissions", downloadSubmissionsForm(profileStore, defaultOutDir))
	mux.HandleFunc("POST /download-submissions", downloadSubmissionsSubmit(profileStore, oplogStore))
	mux.HandleFunc("GET /download-submissions/{id}/run", downloadSubmissionsRunPage())
	mux.HandleFunc("GET /download-submissions/{id}/stream", downloadSubmissionsStream(profileStore, oplogStore, runDownload))
}

func downloadSubmissionsForm(profileStore *profile.Store, defaultOutDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("download-submissions form: list profiles", "err", err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.DownloadSubmissionsForm(profiles, defaultOutDir, "", "").Render(r.Context(), w); err != nil {
			slog.Error("render download-submissions form", "err", err)
		}
	}
}

func downloadSubmissionsSubmit(profileStore *profile.Store, oplogStore *oplog.Store) http.HandlerFunc {
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
		if _, err := profileStore.Get(r.Context(), profileID); err != nil {
			http.Error(w, "profile not found", http.StatusBadRequest)
			return
		}

		contestID := strings.TrimSpace(r.FormValue("contest_id"))
		if contestID == "" {
			http.Error(w, "contest_id is required", http.StatusBadRequest)
			return
		}

		outDir := strings.TrimSpace(r.FormValue("out_dir"))
		if outDir == "" {
			http.Error(w, "out_dir is required", http.StatusBadRequest)
			return
		}

		mode := 2
		if m, err := strconv.Atoi(r.FormValue("mode")); err == nil && m >= 0 && m <= 2 {
			mode = m
		}

		opts := DownloadSubmissionsOpts{
			ContestID:  contestID,
			OutDir:     outDir,
			ProblemIDs: splitTrimmed(r.FormValue("problem_ids")),
			Mode:       mode,
			Extract:    r.FormValue("extract") == "on",
		}
		paramsJSON, _ := json.Marshal(opts)

		entry, err := oplogStore.Create(r.Context(), oplog.OperationLog{
			Type:       oplog.TypeDownloadSubmissions,
			ProfileID:  uint(profileID),
			Status:     oplog.StatusRunning,
			ParamsJSON: string(paramsJSON),
		})
		if err != nil {
			http.Error(w, "could not create log entry", http.StatusInternalServerError)
			slog.Error("download-submissions submit: create oplog", "err", err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/download-submissions/%d/run", entry.ID), http.StatusSeeOther)
	}
}

func downloadSubmissionsRunPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.DownloadSubmissionsRun(id).Render(r.Context(), w); err != nil {
			slog.Error("render download-submissions run page", "err", err)
		}
	}
}

func downloadSubmissionsStream(
	profileStore *profile.Store,
	oplogStore *oplog.Store,
	runDownload DownloadSubmissionsFunc,
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
		p, err := profileStore.Get(r.Context(), int64(entry.ProfileID))
		if err != nil {
			http.Error(w, "profile not found", http.StatusInternalServerError)
			return
		}

		var opts DownloadSubmissionsOpts
		if entry.ParamsJSON != "" {
			if err := json.Unmarshal([]byte(entry.ParamsJSON), &opts); err != nil {
				slog.Warn("download-submissions stream: parse params", "err", err)
			}
		}

		sse := datastar.NewSSE(w, r)
		rep := &sseReporter{sse: sse}

		result, dlErr := runDownload(r.Context(), p, opts, rep)
		if dlErr != nil {
			slog.Error("download-submissions stream: download failed", "err", dlErr)
			if patchErr := sse.PatchElementf(
				`<div id="import-result" role="status" class="alert alert-error">Download failed: %s</div>`,
				dlErr.Error(),
			); patchErr != nil {
				slog.Error("sse patch download-submissions error", "err", patchErr)
			}
			_ = oplogStore.FinishError(r.Context(), entryID, dlErr.Error())
			return
		}

		_ = oplogStore.FinishOK(r.Context(), entryID, result.Downloaded, "")

		if patchErr := sse.PatchElementf(
			`<div id="import-result" role="status" class="alert alert-success">
				Done — %d submission(s) downloaded.<br>
				Files saved to: <code>%s</code>
			</div>`,
			result.Downloaded, opts.OutDir,
		); patchErr != nil {
			slog.Error("sse patch download-submissions result", "err", patchErr)
		}
	}
}
