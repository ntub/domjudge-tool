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
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// DeleteOpts are the parameters for a remove-users-teams operation.
type DeleteOpts struct {
	Include []string `json:"include"` // nil means all non-protected users
	Exclude []string `json:"exclude"`
	DryRun  bool     `json:"dry_run"`
}

// DeleteResult is the outcome of a remove-users-teams operation.
type DeleteResult struct {
	PlannedUsers []string `json:"planned_users,omitempty"` // dry-run: users that would be deleted
	PlannedTeams []string `json:"planned_teams,omitempty"` // dry-run: teams that would be deleted
	DeletedUsers int      `json:"deleted_users,omitempty"`
	DeletedTeams int      `json:"deleted_teams,omitempty"`
	DryRun       bool     `json:"dry_run,omitempty"`
}

// DeleteFunc is the injectable remove operation used by the SSE stream handler.
// Production wires desktopRunDelete; tests pass a mock.
type DeleteFunc func(ctx context.Context, p profile.ServerProfile, opts DeleteOpts, rep progress.Reporter) (DeleteResult, error)

// RegisterRemoveUsersTeams wires the /remove-users-teams routes onto mux.
func RegisterRemoveUsersTeams(
	mux *http.ServeMux,
	profileStore *profile.Store,
	oplogStore *oplog.Store,
	runDelete DeleteFunc,
) {
	mux.HandleFunc("GET /remove-users-teams", removeUsersForm(profileStore))
	mux.HandleFunc("POST /remove-users-teams", removeUsersSubmit(profileStore, oplogStore))
	mux.HandleFunc("GET /remove-users-teams/{id}/run", removeUsersRunPage())
	mux.HandleFunc("GET /remove-users-teams/{id}/stream", removeUsersStream(profileStore, oplogStore, runDelete))
}

func removeUsersForm(profileStore *profile.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := profileStore.List(r.Context())
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			slog.Error("remove form: list profiles", "err", err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.RemoveUsersForm(profiles, "", "").Render(r.Context(), w); err != nil {
			slog.Error("render remove form", "err", err)
		}
	}
}

func removeUsersSubmit(profileStore *profile.Store, oplogStore *oplog.Store) http.HandlerFunc {
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

		opts := DeleteOpts{
			Include: splitTrimmed(r.FormValue("include")),
			Exclude: splitTrimmed(r.FormValue("exclude")),
			DryRun:  r.FormValue("dry_run") == "on",
		}
		paramsJSON, _ := json.Marshal(opts)

		entry, err := oplogStore.Create(r.Context(), oplog.OperationLog{
			Type:       oplog.TypeRemoveUsersTeams,
			ProfileID:  uint(profileID),
			Status:     oplog.StatusRunning,
			ParamsJSON: string(paramsJSON),
		})
		if err != nil {
			http.Error(w, "could not create log entry", http.StatusInternalServerError)
			slog.Error("remove submit: create oplog", "err", err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/remove-users-teams/%d/run", entry.ID), http.StatusSeeOther)
	}
}

func removeUsersRunPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.RemoveUsersRun(id).Render(r.Context(), w); err != nil {
			slog.Error("render remove run page", "err", err)
		}
	}
}

func removeUsersStream(profileStore *profile.Store, oplogStore *oplog.Store, runDelete DeleteFunc) http.HandlerFunc {
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

		var opts DeleteOpts
		if entry.ParamsJSON != "" {
			if err := json.Unmarshal([]byte(entry.ParamsJSON), &opts); err != nil {
				slog.Warn("remove stream: parse params", "err", err)
			}
		}

		sse := datastar.NewSSE(w, r)
		rep := &sseReporter{sse: sse}

		result, deleteErr := runDelete(r.Context(), p, opts, rep)
		if deleteErr != nil {
			slog.Error("remove stream: delete failed", "err", deleteErr)
			if patchErr := sse.PatchElementf(
				`<div id="remove-result" class="alert alert-error">Remove failed: %s</div>`,
				deleteErr.Error(),
			); patchErr != nil {
				slog.Error("sse patch remove error", "err", patchErr)
			}
			_ = oplogStore.FinishError(r.Context(), entryID, deleteErr.Error())
			return
		}

		summaryJSON, _ := json.Marshal(result)
		rowCount := result.DeletedUsers + result.DeletedTeams
		if result.DryRun {
			rowCount = len(result.PlannedUsers) + len(result.PlannedTeams)
		}
		_ = oplogStore.FinishOK(r.Context(), entryID, rowCount, string(summaryJSON))

		if result.DryRun {
			if patchErr := sse.PatchElementf(
				`<div id="remove-result" class="alert alert-info">
					Dry-run: would delete %d user(s) and %d team(s).
					<ul class="list-disc ml-4 mt-1">%s</ul>
				</div>`,
				len(result.PlannedUsers), len(result.PlannedTeams),
				userListItems(result.PlannedUsers),
			); patchErr != nil {
				slog.Error("sse patch dry-run result", "err", patchErr)
			}
		} else {
			if patchErr := sse.PatchElementf(
				`<div id="remove-result" class="alert alert-success">Done — deleted %d user(s) and %d team(s).</div>`,
				result.DeletedUsers, result.DeletedTeams,
			); patchErr != nil {
				slog.Error("sse patch delete result", "err", patchErr)
			}
		}
	}
}

// splitTrimmed splits a comma-separated string, trims whitespace, and drops blanks.
// Returns nil (not empty slice) when the input is blank so callers can distinguish
// "no filter" from "empty filter".
func splitTrimmed(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// userListItems renders PlannedUsers as HTML <li> elements for the dry-run summary.
func userListItems(users []string) string {
	if len(users) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, u := range users {
		sb.WriteString("<li>")
		sb.WriteString(u)
		sb.WriteString("</li>")
	}
	return sb.String()
}
