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

// RegisterAPIFragment wires the /api/fragment/* routes onto mux.
func RegisterAPIFragment(mux *http.ServeMux, store *profile.Store, fetchContests ContestsFetchFunc, fetchUsers UsersFetchFunc) {
	mux.HandleFunc("GET /api/fragment/contests", fragmentContests(store, fetchContests))
	mux.HandleFunc("GET /api/fragment/users", fragmentUsers(store, fetchUsers))
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
