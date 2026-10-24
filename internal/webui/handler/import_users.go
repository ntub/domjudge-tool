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

	"github.com/starfederation/datastar-go/datastar"

	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/util/progress"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
	"github.com/ntub/domjudge-tool/internal/webui/templates"
)

// ImportFunc is the injectable import operation used by the SSE stream handler.
// Production wires importer.ImportUsersTeamsWithReport; tests pass a mock.
type ImportFunc func(ctx context.Context, p profile.ServerProfile, r io.Reader, rep progress.Reporter) (importer.ImportUsersTeamsSummary, error)

// RegisterImportUsersTeams wires the /import-users-teams routes onto mux.
func RegisterImportUsersTeams(
	mux *http.ServeMux,
	profileStore *profile.Store,
	oplogStore *oplog.Store,
	runImport ImportFunc,
) {
	mux.HandleFunc("GET /import-users-teams", importUsersForm(profileStore))
	mux.HandleFunc("POST /import-users-teams", importUsersUpload(profileStore, oplogStore))
	mux.HandleFunc("GET /import-users-teams/{id}/run", importUsersRunPage())
	mux.HandleFunc("GET /import-users-teams/{id}/stream", importUsersStream(profileStore, oplogStore, runImport))
}

func importUsersForm(profileStore *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("import form: list profiles", "err", err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ImportUsersForm(profiles, "", "").Render(r.Context(), w); err != nil {
			slog.Error("render import form", "err", err)
		}
	}
}

func importUsersUpload(profileStore *profile.Store, oplogStore *oplog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "bad multipart form", http.StatusBadRequest)
			return
		}

		profileIDStr := r.FormValue("profile_id")
		profileID, err := strconv.ParseInt(profileIDStr, 10, 64)
		if err != nil || profileID <= 0 {
			http.Error(w, "invalid profile_id", http.StatusBadRequest)
			return
		}

		// Validate profile exists
		if _, err := profileStore.Get(r.Context(), profileID); err != nil {
			http.Error(w, "profile not found", http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("csv_file")
		if err != nil {
			http.Error(w, "missing csv_file", http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()

		// Save CSV to a temp file so the SSE stream can open it independently.
		// Close tmp before creating the oplog entry so data is fully flushed.
		tmp, err := os.CreateTemp("", "domjudge-import-*.csv")
		if err != nil {
			http.Error(w, "could not create temp file", http.StatusInternalServerError)
			slog.Error("import upload: create temp", "err", err)
			return
		}
		if _, err := io.Copy(tmp, file); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			http.Error(w, "could not save file", http.StatusInternalServerError)
			slog.Error("import upload: copy csv", "err", err)
			return
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmp.Name())
			http.Error(w, "could not flush temp file", http.StatusInternalServerError)
			slog.Error("import upload: close temp", "err", err)
			return
		}

		entry, err := oplogStore.Create(r.Context(), oplog.OperationLog{
			Type:      oplog.TypeImportUsersTeams,
			ProfileID: uint(profileID),
			Status:    oplog.StatusRunning,
			CsvPath:   tmp.Name(),
		})
		if err != nil {
			_ = os.Remove(tmp.Name())
			http.Error(w, "could not create log entry", http.StatusInternalServerError)
			slog.Error("import upload: create oplog", "err", err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/import-users-teams/%d/run", entry.ID), http.StatusSeeOther)
	}
}

func importUsersRunPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ImportUsersRun(id).Render(r.Context(), w); err != nil {
			slog.Error("render import run page", "err", err)
		}
	}
}

func importUsersStream(profileStore *profile.Store, oplogStore *oplog.Store, runImport ImportFunc) http.HandlerFunc {
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

		sse := datastar.NewSSE(w, r)

		// Open CSV temp file
		csvFile, err := os.Open(entry.CsvPath)
		if err != nil {
			slog.Error("import stream: open csv", "path", entry.CsvPath, "err", err)
			if patchErr := sse.PatchElementf(`<div id="import-result" role="status" class="alert alert-error">Error: could not open CSV file: %s</div>`, err.Error()); patchErr != nil {
				slog.Error("sse patch error alert", "err", patchErr)
			}
			_ = oplogStore.FinishError(r.Context(), entryID, err.Error())
			return
		}
		defer func() {
			_ = csvFile.Close()
			// Clean up temp file after streaming
			if removeErr := os.Remove(entry.CsvPath); removeErr != nil {
				slog.Warn("import stream: remove temp csv", "path", entry.CsvPath, "err", removeErr)
			}
		}()

		rep := &sseReporter{sse: sse}
		summary, importErr := runImport(r.Context(), p, csvFile, rep)

		if importErr != nil {
			slog.Error("import stream: import failed", "err", importErr)
			if patchErr := sse.PatchElementf(`<div id="import-result" role="status" class="alert alert-error">Import failed: %s</div>`, importErr.Error()); patchErr != nil {
				slog.Error("sse patch import error", "err", patchErr)
			}
			_ = oplogStore.FinishError(r.Context(), entryID, importErr.Error())
			return
		}

		summaryJSON, _ := json.Marshal(summary)
		_ = oplogStore.FinishOK(r.Context(), entryID, summary.ProcessedRows, string(summaryJSON))

		if patchErr := sse.PatchElementf(`<div id="import-result" role="status" class="alert alert-success">
			Done — %d rows processed (%d created, %d updated).
		</div>`, summary.ProcessedRows, summary.CreatedUsers, summary.UpdatedUsers); patchErr != nil {
			slog.Error("sse patch result", "err", patchErr)
		}
	}
}

// sseReporter implements progress.Reporter by patching the DOM via Datastar SSE.
type sseReporter struct {
	sse   *datastar.ServerSentEventGenerator
	total int
	done  int
}

func (r *sseReporter) Start(total int, _ string) {
	r.total = total
	r.done = 0
	if err := r.sse.PatchElementf(
		`<progress id="import-progress" class="progress progress-primary w-full" value="0" max="%d"></progress>`+
			`<span id="import-count" aria-live="polite">0 / %d</span>`,
		total, total,
	); err != nil {
		slog.Error("sseReporter: Start patch", "err", err)
	}
}

func (r *sseReporter) Tick() {
	r.done++
	if err := r.sse.PatchElementf(
		`<progress id="import-progress" class="progress progress-primary w-full" value="%d" max="%d"></progress>`+
			`<span id="import-count" aria-live="polite">%d / %d</span>`,
		r.done, r.total, r.done, r.total,
	); err != nil {
		slog.Error("sseReporter: Tick patch", "err", err)
	}
}

func (r *sseReporter) Finish() {
	if err := r.sse.PatchElementf(
		`<progress id="import-progress" class="progress progress-success w-full" value="%d" max="%d"></progress>`+
			`<span id="import-count" aria-live="polite">%d / %d</span>`,
		r.total, r.total, r.total, r.total,
	); err != nil {
		slog.Error("sseReporter: Finish patch", "err", err)
	}
}
