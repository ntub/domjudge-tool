package importer

import (
	"context"
	"fmt"
	"log/slog"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// resolveTeamID returns the numeric DOMjudge team ID for a user whose
// API snapshot has team_id == nil. Two-step strategy:
//
//  1. GET /api/v4/users/{user.ID}. Some v7 builds populate team_id on
//     the singular endpoint even when /users (list) omits it.
//  2. If still nil and snapshot.Team (display name) is non-empty, list
//     /api/v4/contests/{contestID}/teams and match by team.Name OR
//     team.DisplayName. First match wins; logs a slog warning if
//     multiple matches found.
//
// Returns "" wrapped over web.ErrTeamIDMissing when neither succeeds,
// or when the second-step preconditions (teams resource / contestID /
// snapshot team display name) are missing.
//
// The caller is responsible for adding row context (username) to the
// returned error so the operator sees which row triggered the recovery
// failure.
//
// Network errors from either endpoint are propagated wrapped with %w;
// only the "both endpoints succeeded but neither populated team_id"
// terminal returns ErrTeamIDMissing. This keeps transient
// connectivity failures distinguishable from genuine missing-team
// state.
func resolveTeamID(
	ctx context.Context,
	users *apiv4.UsersResource,
	teams *apiv4.TeamsResource,
	contestID string,
	snapshot model.User,
) (string, error) {
	// Step 1: singular /api/v4/users/{id}. Some v7 builds populate
	// team_id here even when the list endpoint omits it.
	u, err := users.Get(ctx, snapshot.ID)
	if err != nil {
		return "", fmt.Errorf("resolve team_id: get user %s: %w", snapshot.ID, err)
	}
	if u != nil && u.TeamID != nil && *u.TeamID != "" {
		return *u.TeamID, nil
	}

	// Step 2 preconditions: need a teams resource, a contest hint, and
	// the snapshot's team display name to match against.
	if teams == nil || contestID == "" || snapshot.Team == nil || *snapshot.Team == "" {
		return "", fmt.Errorf("resolve team_id for user %s: %w", snapshot.Username, web.ErrTeamIDMissing)
	}

	// Step 2: list contest teams and match by name OR display_name. v7
	// servers populate either field depending on configuration; matching
	// both is a cheap defensive check.
	list, err := teams.List(ctx, contestID)
	if err != nil {
		return "", fmt.Errorf("resolve team_id: list teams for contest %s: %w", contestID, err)
	}

	target := *snapshot.Team
	matches := make([]string, 0, 2)
	for _, t := range list {
		if t.Name == target || (t.DisplayName != nil && *t.DisplayName == target) {
			matches = append(matches, t.ID)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("resolve team_id for user %s: %w", snapshot.Username, web.ErrTeamIDMissing)
	}
	if len(matches) > 1 {
		slog.Default().Warn(
			"team_id resolved via team-name fallback returned >1 match",
			"username", snapshot.Username,
			"team_name", target,
			"first_id", matches[0],
		)
	}
	return matches[0], nil
}
