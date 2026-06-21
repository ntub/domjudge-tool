package handler

import (
	"context"
	"fmt"
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

// TeamsFetchFunc fetches the available teams for a server profile.
// Production wires desktopFetchTeams; tests pass a mock.
type TeamsFetchFunc func(ctx context.Context, p profile.ServerProfile) ([]model.Team, error)

// RegisterAPIFragment wires the /api/fragment/* routes onto mux.
func RegisterAPIFragment(mux *http.ServeMux, store *profile.Store, fetchContests ContestsFetchFunc, fetchUsers UsersFetchFunc, fetchTeams TeamsFetchFunc) {
	mux.HandleFunc("GET /api/fragment/contests", fragmentContests(store, fetchContests))
	mux.HandleFunc("GET /api/fragment/users", fragmentUsers(store, fetchUsers))
	mux.HandleFunc("GET /api/fragment/teams", fragmentTeams(store, fetchTeams))
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
		field := r.URL.Query().Get("field")
		isMulti := field == "include" || field == "exclude" || field == "ids"

		var sigs usersSignals
		if err := datastar.ReadSignals(r, &sigs); err != nil {
			slog.Debug("fragment/users: read signals", "err", err)
		}

		// Fall back to direct query param when signals are absent (e.g. direct browser hit).
		if sigs.ProfileId == "" {
			sigs.ProfileId = r.URL.Query().Get("profile_id")
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
			var items []templates.SelectOption
			var fragmentID string
			switch field {
			case "include":
				fragmentID = "users-include-fragment"
				for i := range users {
					u := &users[i]
					label := u.Username
					if u.Name != "" {
						label = fmt.Sprintf("%s (%s)", u.Username, u.Name)
					}
					items = append(items, templates.SelectOption{
						Label: label,
						Value: u.Username,
					})
				}
				_ = sse.PatchElementTempl(
					templates.TagMultiSelect(fragmentID, "include", "Include (usernames to delete)", items, "Select one or more users to delete. Hold Ctrl/Cmd to multi-select. Leave blank to delete all non-protected users."),
				)
			case "exclude":
				fragmentID = "users-exclude-fragment"
				for i := range users {
					u := &users[i]
					label := u.Username
					if u.Name != "" {
						label = fmt.Sprintf("%s (%s)", u.Username, u.Name)
					}
					items = append(items, templates.SelectOption{
						Label: label,
						Value: u.Username,
					})
				}
				_ = sse.PatchElementTempl(
					templates.TagMultiSelect(fragmentID, "exclude", "Exclude (usernames to keep)", items, "Protected users (admin, judgehost, server account) are always excluded."),
				)
			case "ids":
				fragmentID = "users-ids-fragment"
				for i := range users {
					u := &users[i]
					items = append(items, templates.SelectOption{
						Label: fmt.Sprintf("%s (%s)", u.Username, u.ID),
						Value: u.ID,
					})
				}
				_ = sse.PatchElementTempl(
					templates.TagMultiSelect(fragmentID, "ids", "Filter by User IDs (optional)", items, "Select user IDs. Hold Ctrl/Cmd to multi-select. Leave blank to export all."),
				)
			}
			return
		}

		_ = sse.PatchElementTempl(
			templates.UsernameSelectFragment(users, required),
		)
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
