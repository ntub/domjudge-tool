package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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

// TeamsFetchFunc fetches the available teams for a server profile.
// Production wires desktopFetchTeams; tests pass a mock.
type TeamsFetchFunc func(ctx context.Context, p profile.ServerProfile) ([]model.Team, error)

// RegisterAPIFragment wires the /api/fragment/* routes onto mux.
func RegisterAPIFragment(
	mux *http.ServeMux,
	store *profile.Store,
	fetchContests ContestsFetchFunc,
	fetchUsers UsersFetchFunc,
	fetchProblems ProblemsFetchFunc,
	fetchTeams TeamsFetchFunc,
) {
	mux.HandleFunc("GET /api/fragment/contests", fragmentContests(store, fetchContests))
	mux.HandleFunc("GET /api/fragment/users", fragmentUsers(store, fetchUsers))
	mux.HandleFunc("GET /api/fragment/problems", fragmentProblems(store, fetchProblems))
	mux.HandleFunc("GET /api/fragment/teams", fragmentTeams(store, fetchTeams))
	mux.HandleFunc("GET /api/fragment/export-users-init", fragmentExportUsersInit(store, fetchUsers, fetchTeams))
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

// userNameItems builds SelectOption slices for username-keyed fields (include/exclude).
func userNameItems(users []model.User) []templates.SelectOption {
	items := make([]templates.SelectOption, 0, len(users))
	for i := range users {
		u := &users[i]
		label := u.Username
		if u.Name != "" {
			label = fmt.Sprintf("%s (%s)", u.Username, u.Name)
		}
		items = append(items, templates.SelectOption{Label: label, Value: u.Username})
	}
	return items
}

// userIDItems builds SelectOption slices for ID-keyed fields (ids).
func userIDItems(users []model.User) []templates.SelectOption {
	items := make([]templates.SelectOption, 0, len(users))
	for i := range users {
		u := &users[i]
		items = append(items, templates.SelectOption{
			Label: fmt.Sprintf("%s (%s)", u.Username, u.ID),
			Value: u.ID,
		})
	}
	return items
}

func fragmentUsers(store *profile.Store, fetch UsersFetchFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)
		required := r.URL.Query().Get("required") == "true"
		field := r.URL.Query().Get("field")
		isMulti := field == "include" || field == "exclude" || field == "ids" || field == "include,exclude"

		var sigs usersSignals
		if err := datastar.ReadSignals(r, &sigs); err != nil {
			slog.Debug("fragment/users: read signals", "err", err)
		}

		// Fall back to direct query param when signals are absent (e.g. direct browser hit).
		if sigs.ProfileId == "" {
			sigs.ProfileId = r.URL.Query().Get("profile_id")
		}

		// patchIncludeExcludeFallback patches both include and exclude with text inputs.
		patchIncludeExcludeFallback := func(errMsg string) {
			includeHelp := "Comma-separated. Leave blank to delete all non-protected users."
			excludeHelp := "Protected users (admin, judgehost, server account) are always excluded."
			if errMsg != "" {
				includeHelp = errMsg + " Enter usernames manually."
				excludeHelp = errMsg + " Enter usernames manually."
			}
			_ = sse.PatchElementTempl(
				templates.UsersTextInputFragment("include", "Include (usernames to delete)", "user1, user2, … (blank = all non-protected)", includeHelp),
			)
			_ = sse.PatchElementTempl(
				templates.UsersTextInputFragment("exclude", "Exclude (usernames to keep)", "user3, user4, …", excludeHelp),
			)
		}

		fallback := func(errMsg string) {
			if !isMulti {
				var msg string
				if errMsg != "" {
					msg = errMsg + " Enter the username manually."
				} else {
					msg = "Select a server profile to load users, or enter the username directly."
				}
				_ = sse.PatchElementTempl(
					templates.UsernameInputFragment("e.g. user1", msg, "", required),
				)
				return
			}

			if field == "include,exclude" {
				patchIncludeExcludeFallback(errMsg)
				return
			}

			switch field {
			case "include":
				help := "Comma-separated. Leave blank to delete all non-protected users."
				if errMsg != "" {
					help = errMsg + " Enter usernames manually."
				}
				_ = sse.PatchElementTempl(
					templates.UsersTextInputFragment("include", "Include (usernames to delete)", "user1, user2, … (blank = all non-protected)", help),
				)
			case "exclude":
				help := "Protected users (admin, judgehost, server account) are always excluded."
				if errMsg != "" {
					help = errMsg + " Enter usernames manually."
				}
				_ = sse.PatchElementTempl(
					templates.UsersTextInputFragment("exclude", "Exclude (usernames to keep)", "user3, user4, …", help),
				)
			case "ids":
				help := "Comma-separated user IDs. Leave blank to export all."
				if errMsg != "" {
					help = errMsg + " Enter user IDs manually."
				}
				_ = sse.PatchElementTempl(
					templates.UsersTextInputFragment("ids", "Filter by User IDs (optional)", "1, 2, 3 …", help),
				)
			}
		}

		profileID, err := strconv.ParseInt(sigs.ProfileId, 10, 64)
		if err != nil || profileID <= 0 {
			fallback("")
			return
		}

		p, err := store.Get(r.Context(), profileID)
		if err != nil {
			slog.Debug("fragment/users: get profile", "id", profileID, "err", err)
			fallback("Profile not found.")
			return
		}

		users, err := fetch(r.Context(), p)
		if err != nil {
			slog.Warn("fragment/users: fetch users", "profile", p.Name, "err", err)
			fallback("Could not load users from the server.")
			return
		}

		if isMulti {
			if field == "include,exclude" {
				nameItems := userNameItems(users)
				_ = sse.PatchElementTempl(
					templates.TagMultiSelect("users-include-fragment", "include", "Include (usernames to delete)", nameItems, "Leave blank to delete all non-protected users."),
				)
				_ = sse.PatchElementTempl(
					templates.TagMultiSelect("users-exclude-fragment", "exclude", "Exclude (usernames to keep)", nameItems, "Protected users (admin, judgehost, server account) are always excluded."),
				)
				return
			}

			switch field {
			case "include":
				_ = sse.PatchElementTempl(
					templates.TagMultiSelect("users-include-fragment", "include", "Include (usernames to delete)", userNameItems(users), "Leave blank to delete all non-protected users."),
				)
			case "exclude":
				_ = sse.PatchElementTempl(
					templates.TagMultiSelect("users-exclude-fragment", "exclude", "Exclude (usernames to keep)", userNameItems(users), "Protected users (admin, judgehost, server account) are always excluded."),
				)
			case "ids":
				_ = sse.PatchElementTempl(
					templates.TagMultiSelect("users-ids-fragment", "ids", "Filter by User IDs (optional)", userIDItems(users), "Leave blank to export all."),
				)
			}
			return
		}

		_ = sse.PatchElementTempl(
			templates.UsernameSelectFragment(users, required),
		)
	}
}

// fragmentExportUsersInit handles GET /api/fragment/export-users-init.
// It fetches users and teams in a single SSE response, patching both
// #users-ids-fragment and #team-id-fragment so the export users page
// only needs one @get() action (avoiding Datastar's per-element abort).
func fragmentExportUsersInit(store *profile.Store, fetchUsers UsersFetchFunc, fetchTeams TeamsFetchFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		var sigs usersSignals
		if err := datastar.ReadSignals(r, &sigs); err != nil {
			slog.Debug("fragment/export-users-init: read signals", "err", err)
		}
		if sigs.ProfileId == "" {
			sigs.ProfileId = r.URL.Query().Get("profile_id")
		}

		usersFallback := func(errMsg string) {
			help := "Comma-separated user IDs. Leave blank to export all."
			if errMsg != "" {
				help = errMsg + " Enter user IDs manually."
			}
			_ = sse.PatchElementTempl(
				templates.UsersTextInputFragment("ids", "Filter by User IDs (optional)", "1, 2, 3 …", help),
			)
		}
		teamsFallback := func() {
			_ = sse.PatchElementTempl(templates.TeamInputFragment(false))
		}

		profileID, err := strconv.ParseInt(sigs.ProfileId, 10, 64)
		if err != nil || profileID <= 0 {
			usersFallback("")
			teamsFallback()
			return
		}

		p, err := store.Get(r.Context(), profileID)
		if err != nil {
			slog.Debug("fragment/export-users-init: get profile", "id", profileID, "err", err)
			usersFallback("Profile not found.")
			teamsFallback()
			return
		}

		// Patch users fragment.
		if users, err := fetchUsers(r.Context(), p); err != nil {
			slog.Warn("fragment/export-users-init: fetch users", "profile", p.Name, "err", err)
			usersFallback("Could not load users from the server.")
		} else {
			_ = sse.PatchElementTempl(
				templates.TagMultiSelect("users-ids-fragment", "ids", "Filter by User IDs (optional)", userIDItems(users), "Leave blank to export all."),
			)
		}

		// Patch teams fragment.
		if teams, err := fetchTeams(r.Context(), p); err != nil {
			slog.Warn("fragment/export-users-init: fetch teams", "profile", p.Name, "err", err)
			teamsFallback()
		} else {
			_ = sse.PatchElementTempl(templates.TeamSelectFragment(teams, false))
		}
	}
}

// problemsSignals holds the subset of Datastar signals we need for problems.
type problemsSignals struct {
	ProfileId string `json:"profileId"` //nolint:revive // camelCase matches Datastar signal name
	ContestId string `json:"contestId"` //nolint:revive // camelCase matches Datastar signal name
}

type problemFieldMeta struct {
	name, label, placeholder, helpText, fragmentID string
}

// problemFieldsByName returns the metadata for the given problem field name.
// Returns (meta, true) on success, (zero, false) for unknown field names.
func problemFieldByName(name string) (problemFieldMeta, bool) {
	switch name {
	case "problem_ids":
		return problemFieldMeta{"problem_ids", "Problem IDs (optional)", "P1, P2 …", "Select a contest to load problems.", "problems-problem_ids-fragment"}, true
	case "only":
		return problemFieldMeta{"only", "Only Problem IDs (optional)", "A, B, C …", "Comma-separated problem IDs to include. Leave blank to download all (uses profile contest ID if set).", "problems-only-fragment"}, true
	case "exclude":
		return problemFieldMeta{"exclude", "Exclude Problem IDs (optional)", "X, Y …", "", "problems-exclude-fragment"}, true
	}
	return problemFieldMeta{}, false
}

// parseProblemFields parses a comma-separated list of field names, returning
// metadata for each. Returns (nil, false) if any name is unknown.
func parseProblemFields(raw string) ([]problemFieldMeta, bool) {
	parts := strings.Split(raw, ",")
	out := make([]problemFieldMeta, 0, len(parts))
	for _, p := range parts {
		m, ok := problemFieldByName(strings.TrimSpace(p))
		if !ok {
			return nil, false
		}
		out = append(out, m)
	}
	return out, true
}

func fragmentProblems(store *profile.Store, fetch ProblemsFetchFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate field(s) BEFORE starting the SSE stream so http.Error still works.
		fields, ok := parseProblemFields(r.URL.Query().Get("field"))
		if !ok {
			slog.Debug("fragment/problems: invalid field", "field", r.URL.Query().Get("field"))
			http.Error(w, "invalid field", http.StatusBadRequest)
			return
		}

		sse := datastar.NewSSE(w, r)

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

		// fallback patches all configured fields with text input fragments.
		fallback := func(errMsg string) {
			for _, f := range fields {
				help := f.helpText
				if errMsg != "" {
					help = errMsg
				}
				_ = sse.PatchElementTempl(
					templates.ProblemsTextInputFragment(f.name, f.label, f.placeholder, help),
				)
			}
		}

		profileID, err := strconv.ParseInt(sigs.ProfileId, 10, 64)
		if err != nil || profileID <= 0 {
			fallback("")
			return
		}

		p, err := store.Get(r.Context(), profileID)
		if err != nil {
			slog.Debug("fragment/problems: get profile", "id", profileID, "err", err)
			fallback("Profile not found.")
			return
		}

		contestID := sigs.ContestId
		if contestID == "" && p.ContestID != nil {
			contestID = *p.ContestID
		}

		if contestID == "" {
			fallback("")
			return
		}

		problems, err := fetch(r.Context(), p, contestID)
		if err != nil {
			slog.Warn("fragment/problems: fetch problems", "profile", p.Name, "contest", contestID, "err", err)
			fallback("Could not load problems from the server: " + err.Error())
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

		for _, f := range fields {
			_ = sse.PatchElementTempl(
				templates.TagMultiSelect(f.fragmentID, f.name, f.label, options, ""),
			)
		}
	}
}

// teamsSignals holds the subset of Datastar signals we need for teams.
type teamsSignals struct {
	ProfileId string `json:"profileId"` //nolint:revive // camelCase matches Datastar signal name
}

func fragmentTeams(store *profile.Store, fetch TeamsFetchFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)
		required := r.URL.Query().Get("required") == "true"

		var sigs teamsSignals
		if err := datastar.ReadSignals(r, &sigs); err != nil {
			slog.Debug("fragment/teams: read signals", "err", err)
		}

		// Fall back to direct query param when signals are absent (e.g. direct browser hit).
		if sigs.ProfileId == "" {
			sigs.ProfileId = r.URL.Query().Get("profile_id")
		}

		profileID, err := strconv.ParseInt(sigs.ProfileId, 10, 64)
		if err != nil || profileID <= 0 {
			_ = sse.PatchElementTempl(
				templates.TeamInputFragment(required),
			)
			return
		}

		p, err := store.Get(r.Context(), profileID)
		if err != nil {
			slog.Debug("fragment/teams: get profile", "id", profileID, "err", err)
			_ = sse.PatchElementTempl(
				templates.TeamInputFragment(required),
			)
			return
		}

		teams, err := fetch(r.Context(), p)
		if err != nil {
			slog.Warn("fragment/teams: fetch teams", "profile", p.Name, "err", err)
			_ = sse.PatchElementTempl(
				templates.TeamInputFragment(required),
			)
			return
		}

		_ = sse.PatchElementTempl(
			templates.TeamSelectFragment(teams, required),
		)
	}
}
