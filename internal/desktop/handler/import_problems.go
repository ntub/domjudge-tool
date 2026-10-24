package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/starfederation/datastar-go/datastar"

	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// ImportProblemsFile is one uploaded ZIP file recorded in the oplog.
type ImportProblemsFile struct {
	Path string `json:"path"` // temp file path; cleared after streaming finishes
	Name string `json:"name"` // original upload filename
}

// ImportProblemsOpts are the parameters for an import-problems operation.
type ImportProblemsOpts struct {
	ContestID string               `json:"contest_id"`
	Upsert    bool                 `json:"upsert"`
	Files     []ImportProblemsFile `json:"files"`
}

// ImportProblemsResult is the outcome of an import-problems operation.
type ImportProblemsResult struct {
	Imported int `json:"imported"`
}

// ImportProblemsFunc is the injectable import operation used by the SSE stream handler.
// Production wires desktopRunImportProblems; tests pass a mock.
type ImportProblemsFunc func(ctx context.Context, p profile.ServerProfile, opts ImportProblemsOpts, rep progress.Reporter) (ImportProblemsResult, error)

// RegisterImportProblems wires the /import-problems routes onto mux.
func RegisterImportProblems(
	mux *http.ServeMux,
	profileStore *profile.Store,
	oplogStore *oplog.Store,
	runImportProblems ImportProblemsFunc,
) {
	mux.HandleFunc("GET /import-problems", importProblemsForm(profileStore))
	mux.HandleFunc("POST /import-problems", importProblemsUpload(profileStore, oplogStore))
	mux.HandleFunc("GET /import-problems/{id}/run", importProblemsRunPage())
	mux.HandleFunc("GET /import-problems/{id}/stream", importProblemsStream(profileStore, oplogStore, runImportProblems))
}

func importProblemsForm(profileStore *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("import-problems form: list profiles", "err", err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ImportProblemsForm(profiles, "", "").Render(r.Context(), w); err != nil {
			slog.Error("render import-problems form", "err", err)
		}
	}
}

func importProblemsUpload(profileStore *profile.Store, oplogStore *oplog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(256 << 20); err != nil {
			http.Error(w, "bad multipart form", http.StatusBadRequest)
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

		// Contest ID: prefer form override, fall back to profile.
		contestID := strings.TrimSpace(r.FormValue("contest_id"))
		if contestID == "" && p.ContestID != nil {
			contestID = strings.TrimSpace(*p.ContestID)
		}
		if contestID == "" {
			http.Error(w, "contest_id is required (set in the form or on the server profile)", http.StatusBadRequest)
			return
		}

		upsert := r.FormValue("upsert") == "on"

		fileHeaders := r.MultipartForm.File["zip_files"]
		if len(fileHeaders) == 0 {
			http.Error(w, "at least one .zip file is required", http.StatusBadRequest)
			return
		}

		// Save each uploaded ZIP to a temp file.
		// All writes must complete before the oplog entry is created so that the
		// SSE stream can open every file safely.
		savedFiles := make([]ImportProblemsFile, 0, len(fileHeaders))
		for _, fh := range fileHeaders {
			src, err := fh.Open()
			if err != nil {
				removeSavedFiles(savedFiles)
				http.Error(w, "could not open uploaded file", http.StatusInternalServerError)
				slog.Error("import-problems upload: open file header", "filename", fh.Filename, "err", err)
				return
			}
			tmp, err := os.CreateTemp("", "domjudge-import-problem-*.zip")
			if err != nil {
				_ = src.Close()
				removeSavedFiles(savedFiles)
				http.Error(w, "could not create temp file", http.StatusInternalServerError)
				slog.Error("import-problems upload: create temp", "err", err)
				return
			}
			if _, err := io.Copy(tmp, src); err != nil {
				_ = src.Close()
				_ = tmp.Close()
				_ = os.Remove(tmp.Name())
				removeSavedFiles(savedFiles)
				http.Error(w, "could not save uploaded file", http.StatusInternalServerError)
				slog.Error("import-problems upload: copy zip", "filename", fh.Filename, "err", err)
				return
			}
			_ = src.Close()
			if err := tmp.Close(); err != nil {
				_ = os.Remove(tmp.Name())
				removeSavedFiles(savedFiles)
				http.Error(w, "could not flush temp file", http.StatusInternalServerError)
				slog.Error("import-problems upload: close temp", "err", err)
				return
			}
			savedFiles = append(savedFiles, ImportProblemsFile{Path: tmp.Name(), Name: fh.Filename})
		}

		opts := ImportProblemsOpts{
			ContestID: contestID,
			Upsert:    upsert,
			Files:     savedFiles,
		}
		paramsJSON, _ := json.Marshal(opts)

		entry, err := oplogStore.Create(r.Context(), oplog.OperationLog{
			Type:       oplog.TypeImportProblems,
			ProfileID:  uint(profileID),
			Status:     oplog.StatusRunning,
			ParamsJSON: string(paramsJSON),
		})
		if err != nil {
			removeSavedFiles(savedFiles)
			http.Error(w, "could not create log entry", http.StatusInternalServerError)
			slog.Error("import-problems upload: create oplog", "err", err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/import-problems/%d/run", entry.ID), http.StatusSeeOther)
	}
}

func importProblemsRunPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ImportProblemsRun(id).Render(r.Context(), w); err != nil {
			slog.Error("render import-problems run page", "err", err)
		}
	}
}

func importProblemsStream(
	profileStore *profile.Store,
	oplogStore *oplog.Store,
	runImportProblems ImportProblemsFunc,
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

		var opts ImportProblemsOpts
		if entry.ParamsJSON != "" {
			if err := json.Unmarshal([]byte(entry.ParamsJSON), &opts); err != nil {
				slog.Warn("import-problems stream: parse params", "err", err)
			}
		}

		// Always clean up every temp file, whether import succeeds or fails.
		defer removeSavedFiles(opts.Files)

		sse := datastar.NewSSE(w, r)
		rep := &sseReporter{sse: sse}

		result, importErr := runImportProblems(r.Context(), p, opts, rep)
		if importErr != nil {
			slog.Error("import-problems stream: import failed", "err", importErr)
			if patchErr := sse.PatchElementf(
				`<div id="import-result" role="status" class="alert alert-error">Import failed: %s</div>`,
				importErr.Error(),
			); patchErr != nil {
				slog.Error("sse patch import-problems error", "err", patchErr)
			}
			_ = oplogStore.FinishError(r.Context(), entryID, importErr.Error())
			return
		}

		summaryJSON, _ := json.Marshal(result)
		_ = oplogStore.FinishOK(r.Context(), entryID, result.Imported, string(summaryJSON))

		if patchErr := sse.PatchElementf(
			`<div id="import-result" role="status" class="alert alert-success">Done — %d problem(s) imported.</div>`,
			result.Imported,
		); patchErr != nil {
			slog.Error("sse patch import-problems result", "err", patchErr)
		}
	}
}

// removeSavedFiles removes every temp file recorded in files. Errors are
// logged but do not abort the loop so all files are attempted.
func removeSavedFiles(files []ImportProblemsFile) {
	for _, f := range files {
		if f.Path == "" {
			continue
		}
		if err := os.Remove(f.Path); err != nil && !os.IsNotExist(err) {
			slog.Warn("import-problems: remove temp zip", "path", f.Path, "err", err)
		}
	}
}
