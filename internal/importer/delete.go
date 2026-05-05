package importer

import (
	"context"
	"fmt"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/model"
)

// DeleteOptions configures a single DeleteTeamsAndUsers call.
type DeleteOptions struct {
	Include []string
	Exclude []string
}

// DeleteTeamsAndUsers deletes users and their teams from DOMjudge.
//
// The default-protect list (admin, judgehost, cfg.Username) is applied
// BEFORE Include/Exclude filtering; protected users are never passed to
// gw.DeleteUsers or gw.DeleteTeams regardless of other options.
//
// Internally this function:
//  1. Calls users.List to obtain the current user list.
//  2. Derives team-id lists from the API result.
//  3. Calls gw.Login(ctx).
//  4. Calls gw.DeleteUsers(ctx, include, exclude).
//  5. Calls gw.DeleteTeams(ctx, includeTeams, excludeTeams).
func DeleteTeamsAndUsers(
	ctx context.Context,
	gw web.Gateway,
	users *apiv4.UsersResource,
	cfg *config.DomServerClient,
	opts DeleteOptions,
) error {
	snapshot, err := users.List(ctx, apiv4.UsersListOpts{})
	if err != nil {
		return fmt.Errorf("importer: list users: %w", err)
	}

	protected := protectedSet(cfg)
	teamIDs := teamIDByUsername(snapshot)

	includeUsers, excludeUsers := resolveDeleteFilter(snapshot, protected, opts)
	includeTeams := projectTeamIDs(includeUsers, teamIDs)
	excludeTeams := projectTeamIDs(excludeUsers, teamIDs)

	if err := gw.Login(ctx); err != nil {
		return fmt.Errorf("importer: login: %w", err)
	}
	if err := gw.DeleteUsers(ctx, includeUsers, excludeUsers); err != nil {
		return fmt.Errorf("importer: delete users: %w", err)
	}
	if err := gw.DeleteTeams(ctx, includeTeams, excludeTeams); err != nil {
		return fmt.Errorf("importer: delete teams: %w", err)
	}
	return nil
}

// resolveDeleteFilter returns the (include, exclude) username lists
// passed to gw.DeleteUsers. Both lists exclude protected names; the
// returned include list also excludes any name in opts.Exclude. A nil
// opts.Include means "all non-protected snapshot users".
func resolveDeleteFilter(
	snapshot []model.User,
	protected map[string]bool,
	opts DeleteOptions,
) (include, exclude []string) {
	excludeSet := make(map[string]struct{}, len(opts.Exclude))
	for _, name := range opts.Exclude {
		if protected[name] {
			continue
		}
		if _, dup := excludeSet[name]; dup {
			continue
		}
		excludeSet[name] = struct{}{}
		exclude = append(exclude, name)
	}

	candidate := opts.Include
	if candidate == nil {
		candidate = make([]string, 0, len(snapshot))
		for i := range snapshot {
			candidate = append(candidate, snapshot[i].Username)
		}
	}
	include = make([]string, 0, len(candidate))
	for _, name := range candidate {
		if protected[name] {
			continue
		}
		if _, drop := excludeSet[name]; drop {
			continue
		}
		include = append(include, name)
	}
	return include, exclude
}

// projectTeamIDs maps usernames to team-ids using the precomputed
// index, preserving order and skipping names that have no TeamID in
// the snapshot.
func projectTeamIDs(usernames []string, idx map[string]string) []string {
	out := make([]string, 0, len(usernames))
	for _, name := range usernames {
		if id, ok := idx[name]; ok {
			out = append(out, id)
		}
	}
	return out
}

// protectedSet builds the default-protect set: admin, judgehost, and
// cfg.Username (if non-empty). Returning a map keeps the membership
// test O(1) for any include/exclude size.
func protectedSet(cfg *config.DomServerClient) map[string]bool {
	out := map[string]bool{"admin": true, "judgehost": true}
	if cfg != nil && cfg.Username != "" {
		out[cfg.Username] = true
	}
	return out
}

// teamIDByUsername indexes the snapshot by username so derive-step
// lookups are constant-time. Users without a TeamID are skipped (no
// team to delete).
func teamIDByUsername(users []model.User) map[string]string {
	out := make(map[string]string, len(users))
	for i := range users {
		if users[i].TeamID != nil {
			out[users[i].Username] = *users[i].TeamID
		}
	}
	return out
}
