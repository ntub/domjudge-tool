package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/starfederation/datastar-go/datastar"

	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
	"github.com/ntub/domjudge-tool/internal/model"
)

// ContestsFetchFunc fetches the available contests for a server profile.
// Production wires desktopFetchContests; tests pass a mock.
type ContestsFetchFunc func(ctx context.Context, p profile.ServerProfile) ([]model.Contest, error)

// UsersFetchFunc fetches the available users for a server profile.
// Production wires desktopFetchUsers; tests pass a mock.
type UsersFetchFunc func(ctx context.Context, p profile.ServerProfile) ([]model.User, error)

// ProblemsFetchFunc fetches the available problems for a server profile and contest.
// Production wires desktopFetchProblems; tests pass a mock.
type ProblemsFetchFunc func(ctx context.Context, p profile.ServerProfile, contestID string) ([]model.Problem, error)

// RegisterAPIFragment wires the /api/fragment/* routes onto mux.
func RegisterAPIFragment(
	mux *http.ServeMux,
	store *profile.Store,
	fetchContests ContestsFetchFunc,
	fetchUsers UsersFetchFunc,
	fetchProblems ProblemsFetchFunc,
) {
	mux.HandleFunc("GET /api/fragment/contests", fragmentContests(store, fetchContests))
	mux.HandleFunc("GET /api/fragment/users", fragmentUsers(store, fetchUsers))
	mux.HandleFunc("GET /api/fragment/problems", fragmentProblems(store, fetchProblems))
}

// contestsSignals holds the subset of Datastar signals we need.
type contestsSignals struct {
	ProfileId string `json:"profileId"` //nolint:revive // camelCase matches Datastar signal name
}

func fragmentContests(store *profile.Store, fetch ContestsFetchFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)
		required := r.URL.Query().Get("required") == "true"

		var sigs contestsSignals
		if err := datastar.ReadSignals(r, &sigs); err != nil {
			slog.Debug("fragment/contests: read signals", "err", err)
		}

		// Fall back to direct query param when signals are absent (e.g. direct browser hit).
		if sigs.ProfileId == "" {
			sigs.ProfileId = r.URL.Query().Get("profile_id")
		}

		profileID, err := strconv.ParseInt(sigs.ProfileId, 10, 64)
		if err != nil || profileID <= 0 {
			_ = sse.PatchElementTempl(
				templates.ContestInputFragment("", "Select a server profile first.", "", required),
			)
			return
		}

		p, err := store.Get(r.Context(), profileID)
		if err != nil {
			slog.Debug("fragment/contests: get profile", "id", profileID, "err", err)
			_ = sse.PatchElementTempl(
				templates.ContestInputFragment("e.g. 1", "Profile not found.", "", required),
			)
			return
		}

		contests, err := fetch(r.Context(), p)
		if err != nil {
			slog.Warn("fragment/contests: fetch contests", "profile", p.Name, "err", err)
			_ = sse.PatchElementTempl(
				templates.ContestInputFragment("e.g. 1", "",
					"Could not load contests from the server.", required),
			)
			return
		}

		_ = sse.PatchElementTempl(
			templates.ContestSelectFragment(contests, p.ContestID, required),
		)
	}
}

// usersSignals holds the subset of Datastar signals we need for users.
type usersSignals struct {
	ProfileId string `json:"profileId"` //nolint:revive // camelCase matches Datastar signal name
}

func fragmentUsers(store *profile.Store, fetch UsersFetchFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)
		required := r.URL.Query().Get("required") == "true"

		var sigs usersSignals
		if err := datastar.ReadSignals(r, &sigs); err != nil {
			slog.Debug("fragment/users: read signals", "err", err)
		}

		// Fall back to direct query param when signals are absent (e.g. direct browser hit).
		if sigs.ProfileId == "" {
			sigs.ProfileId = r.URL.Query().Get("profile_id")
		}

		profileID, err := strconv.ParseInt(sigs.ProfileId, 10, 64)
		if err != nil || profileID <= 0 {
			_ = sse.PatchElementTempl(
				templates.UsernameInputFragment("e.g. user1", "Select a server profile to load users, or enter the username directly.", "", required),
			)
			return
		}

		p, err := store.Get(r.Context(), profileID)
		if err != nil {
			slog.Debug("fragment/users: get profile", "id", profileID, "err", err)
			_ = sse.PatchElementTempl(
				templates.UsernameInputFragment("e.g. user1", "Profile not found.", "", required),
			)
			return
		}

		users, err := fetch(r.Context(), p)
		if err != nil {
			slog.Warn("fragment/users: fetch users", "profile", p.Name, "err", err)
			_ = sse.PatchElementTempl(
				templates.UsernameInputFragment("e.g. user1", "",
					"Could not load users from the server.", required),
			)
			return
		}

		_ = sse.PatchElementTempl(
			templates.UsernameSelectFragment(users, required),
		)
	}
}

// problemsSignals holds the subset of Datastar signals we need for problems.
type problemsSignals struct {
	ProfileId string `json:"profileId"` //nolint:revive // camelCase matches Datastar signal name
	ContestId string `json:"contestId"` //nolint:revive // camelCase matches Datastar signal name
}

func fragmentProblems(store *profile.Store, fetch ProblemsFetchFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)
		field := r.URL.Query().Get("field")

		var fieldName, label, placeholder, helpText, fragmentID string
		switch field {
		case "problem_ids":
			fieldName = "problem_ids"
			label = "Problem IDs (optional)"
			placeholder = "P1, P2 …"
			helpText = "Select a contest to load problems."
			fragmentID = "problems-problem_ids-fragment"
		case "only":
			fieldName = "only"
			label = "Only Problem IDs (optional)"
			placeholder = "A, B, C …"
			helpText = "Comma-separated problem IDs to include. Leave blank to download all (uses profile contest ID if set)."
			fragmentID = "problems-only-fragment"
		case "exclude":
			fieldName = "exclude"
			label = "Exclude Problem IDs (optional)"
			placeholder = "X, Y …"
			helpText = ""
			fragmentID = "problems-exclude-fragment"
		default:
			slog.Debug("fragment/problems: invalid field", "field", field)
			http.Error(w, "invalid field", http.StatusBadRequest)
			return
		}

		var sigs problemsSignals
		if err := datastar.ReadSignals(r, &sigs); err != nil {
			slog.Debug("fragment/problems: read signals", "err", err)
		}

		// Fall back to direct query param when signals are absent (e.g. direct browser hit).
		if sigs.ProfileId == "" {
			sigs.ProfileId = r.URL.Query().Get("profile_id")
		}
		if sigs.ContestId == "" {
			sigs.ContestId = r.URL.Query().Get("contest_id")
		}

		profileID, err := strconv.ParseInt(sigs.ProfileId, 10, 64)
		if err != nil || profileID <= 0 {
			_ = sse.PatchElementTempl(
				templates.ProblemsTextInputFragment(fieldName, label, placeholder, helpText),
			)
			return
		}

		p, err := store.Get(r.Context(), profileID)
		if err != nil {
			slog.Debug("fragment/problems: get profile", "id", profileID, "err", err)
			_ = sse.PatchElementTempl(
				templates.ProblemsTextInputFragment(fieldName, label, placeholder, "Profile not found."),
			)
			return
		}

		contestID := sigs.ContestId
		if contestID == "" && p.ContestID != nil {
			contestID = *p.ContestID
		}

		if contestID == "" {
			_ = sse.PatchElementTempl(
				templates.ProblemsTextInputFragment(fieldName, label, placeholder, helpText),
			)
			return
		}

		problems, err := fetch(r.Context(), p, contestID)
		if err != nil {
			slog.Warn("fragment/problems: fetch problems", "profile", p.Name, "contest", contestID, "err", err)
			warningMsg := "Could not load problems from the server: " + err.Error()
			_ = sse.PatchElementTempl(
				templates.ProblemsTextInputFragment(fieldName, label, placeholder, warningMsg),
			)
			return
		}

		options := make([]templates.SelectOption, len(problems))
		for i, prob := range problems {
			optLabel := prob.Label
			if optLabel == "" {
				optLabel = prob.ShortName
			}
			if optLabel != "" && prob.Name != "" {
				optLabel = optLabel + " - " + prob.Name
			} else if prob.Name != "" {
				optLabel = prob.Name
			} else {
				optLabel = prob.ID
			}
			options[i] = templates.SelectOption{
				Value: prob.ID,
				Label: optLabel,
			}
		}

		_ = sse.PatchElementTempl(
			templates.TagMultiSelect(fragmentID, fieldName, label, options),
		)
	}
}
