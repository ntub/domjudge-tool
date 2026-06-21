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

// RegisterAPIFragment wires the /api/fragment/* routes onto mux.
func RegisterAPIFragment(mux *http.ServeMux, store *profile.Store, fetchContests ContestsFetchFunc) {
	mux.HandleFunc("GET /api/fragment/contests", fragmentContests(store, fetchContests))
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
