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

	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
	"github.com/ntub/domjudge-tool/internal/problems"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// DownloadOpts are the parameters for a download-problems operation.
type DownloadOpts struct {
	OutDir  string   `json:"out_dir"`
	Only    []string `json:"only,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// DownloadResult is the outcome of a download-problems operation.
type DownloadResult struct {
	Downloaded int    `json:"downloaded"`
	Skipped    int    `json:"skipped"`
	OutDir     string `json:"out_dir"`
}

// DownloadFunc is the injectable download operation used by the SSE stream handler.
// Production wires desktopRunDownload; tests pass a mock.
type DownloadFunc func(ctx context.Context, p profile.ServerProfile, opts DownloadOpts, rep progress.Reporter) (DownloadResult, error)

// RegisterDownloadProblems wires the /download-problems routes onto mux.
func RegisterDownloadProblems(
	mux *http.ServeMux,
	profileStore *profile.Store,
	oplogStore *oplog.Store,
	runDownload DownloadFunc,
	defaultOutDir string,
) {
	mux.HandleFunc("GET /download-problems", downloadProblemsForm(profileStore, defaultOutDir))
	mux.HandleFunc("POST /download-problems", downloadProblemsSubmit(profileStore, oplogStore))
	mux.HandleFunc("GET /download-problems/{id}/run", downloadProblemsRunPage())
	mux.HandleFunc("GET /download-problems/{id}/stream", downloadProblemsStream(profileStore, oplogStore, runDownload))
}

func downloadProblemsForm(profileStore *profile.Store, defaultOutDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("download-problems form: list profiles", "err", err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.DownloadProblemsForm(profiles, defaultOutDir, "", "").Render(r.Context(), w); err != nil {
			slog.Error("render download-problems form", "err", err)
		}
	}
}

func downloadProblemsSubmit(profileStore *profile.Store, oplogStore *oplog.Store) http.HandlerFunc {
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

		outDir := strings.TrimSpace(r.FormValue("out_dir"))
		if outDir == "" {
			http.Error(w, "out_dir is required", http.StatusBadRequest)
			return
		}

		opts := DownloadOpts{
			OutDir:  outDir,
			Only:    splitTrimmed(r.FormValue("only")),
			Exclude: splitTrimmed(r.FormValue("exclude")),
		}
		paramsJSON, _ := json.Marshal(opts)

		entry, err := oplogStore.Create(r.Context(), oplog.OperationLog{
			Type:       oplog.TypeDownloadProblems,
			ProfileID:  uint(profileID),
			Status:     oplog.StatusRunning,
			ParamsJSON: string(paramsJSON),
		})
		if err != nil {
			http.Error(w, "could not create log entry", http.StatusInternalServerError)
			slog.Error("download-problems submit: create oplog", "err", err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/download-problems/%d/run", entry.ID), http.StatusSeeOther)
	}
}

func downloadProblemsRunPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.DownloadProblemsRun(id).Render(r.Context(), w); err != nil {
			slog.Error("render download-problems run page", "err", err)
		}
	}
}

func downloadProblemsStream(
	profileStore *profile.Store,
	oplogStore *oplog.Store,
	runDownload DownloadFunc,
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

		var opts DownloadOpts
		if entry.ParamsJSON != "" {
			if err := json.Unmarshal([]byte(entry.ParamsJSON), &opts); err != nil {
				slog.Warn("download-problems stream: parse params", "err", err)
			}
		}

		sse := datastar.NewSSE(w, r)
		rep := &sseReporter{sse: sse}

		result, dlErr := runDownload(r.Context(), p, opts, rep)
		if dlErr != nil {
			slog.Error("download-problems stream: download failed", "err", dlErr)
			if patchErr := sse.PatchElementf(
				`<div id="import-result" class="alert alert-error">Download failed: %s</div>`,
				dlErr.Error(),
			); patchErr != nil {
				slog.Error("sse patch download error", "err", patchErr)
			}
			_ = oplogStore.FinishError(r.Context(), entryID, dlErr.Error())
			return
		}

		summaryJSON, _ := json.Marshal(problems.Result{
			Downloaded: result.Downloaded,
			Skipped:    result.Skipped,
			OutDir:     result.OutDir,
		})
		_ = oplogStore.FinishOK(r.Context(), entryID, result.Downloaded, string(summaryJSON))

		if patchErr := sse.PatchElementf(
			`<div id="import-result" class="alert alert-success">
				Done — %d problem(s) downloaded (%d skipped).<br>
				Files saved to: <code>%s</code>
			</div>`,
			result.Downloaded, result.Skipped, result.OutDir,
		); patchErr != nil {
			slog.Error("sse patch download result", "err", patchErr)
		}
	}
}
