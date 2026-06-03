package scoreboard

import (
	"context"
	"fmt"
	"sort"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/model"
)

// Download fetches the scoreboard API JSON for cid, joins team and
// organization metadata, and returns the Phase 1 CSV header + rows.
//
// DOMjudge segments the scoreboard by team-category sortorder and only
// returns the lowest sortorder unless one is named, so Download enumerates
// the populated sortorders and fetches each with allteams=true (which also
// surfaces hidden categories). Rows are merged in ascending sortorder.
func Download(ctx context.Context, c *apiv4.Client, cid string, excludeHidden bool) (header []string, rows [][]string, err error) {
	if c == nil {
		return nil, nil, fmt.Errorf("scoreboard: nil api client")
	}
	if _, err := c.Contests.Get(ctx, cid); err != nil {
		return nil, nil, fmt.Errorf("scoreboard: get contest %s: %w", cid, err)
	}
	teams, err := c.Teams.List(ctx, cid)
	if err != nil {
		return nil, nil, fmt.Errorf("scoreboard: list teams: %w", err)
	}
	orgs, err := c.Organizations.List(ctx, cid)
	if err != nil {
		return nil, nil, fmt.Errorf("scoreboard: list organizations: %w", err)
	}
	groups, err := c.Groups.List(ctx, cid)
	if err != nil {
		return nil, nil, fmt.Errorf("scoreboard: list groups: %w", err)
	}

	groupByID := indexGroups(groups)
	teamByID := indexTeams(teams)
	sortorders := populatedSortorders(teams, groupByID)
	allTeams := true

	var base *model.ScoreboardSnapshot
	mergedRows := make([]model.ScoreboardRow, 0)
	categoryByRow := make([]string, 0)
	for _, sortorder := range sortorders {
		so := sortorder
		snap, err := c.Scoreboard.Get(ctx, cid, apiv4.ScoreboardListOpts{SortOrder: &so, AllTeams: &allTeams})
		if err != nil {
			return nil, nil, fmt.Errorf("scoreboard: download sortorder %d: %w", sortorder, err)
		}
		if base == nil && len(snap.Rows) > 0 {
			base = snap
		}
		// Resolve each row against the group that matches THIS sortorder, not
		// team.GroupIDs[0]: a team in several groups appears once per section,
		// and category labelling plus hidden filtering must follow the group
		// that produced the row.
		for _, row := range snap.Rows {
			group, ok := groupForSortorder(teamByID[row.TeamID], groupByID, so)
			if excludeHidden && ok && group.Hidden {
				continue
			}
			mergedRows = append(mergedRows, row)
			if ok {
				categoryByRow = append(categoryByRow, group.Name)
			} else {
				categoryByRow = append(categoryByRow, "")
			}
		}
	}

	merged := model.ScoreboardSnapshot{}
	if base != nil {
		merged = *base
	}
	merged.Rows = mergedRows
	return buildWithCategories(merged, teams, orgs, groups, categoryByRow)
}

// populatedSortorders returns, ascending, the distinct sortorder values of
// the groups that at least one team belongs to.
func populatedSortorders(teams []model.Team, groupByID map[string]model.Group) []int {
	seen := make(map[int]struct{})
	for _, team := range teams {
		for _, groupID := range team.GroupIDs {
			group, ok := groupByID[groupID]
			if !ok {
				continue
			}
			seen[group.SortOrder] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for sortorder := range seen {
		out = append(out, sortorder)
	}
	sort.Ints(out)
	return out
}

// groupForSortorder returns the team's group whose SortOrder matches so. When
// a team belongs to several groups, this picks the one that placed it in the
// current scoreboard section, keeping category labelling and hidden filtering
// correct across merged sortorder sections.
func groupForSortorder(team model.Team, groupByID map[string]model.Group, so int) (model.Group, bool) {
	for _, groupID := range team.GroupIDs {
		group, ok := groupByID[groupID]
		if ok && group.SortOrder == so {
			return group, true
		}
	}
	return model.Group{}, false
}
