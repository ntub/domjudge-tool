package scoreboard

import (
	"context"
	"fmt"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
)

// Download fetches the scoreboard API JSON for cid, joins team and
// organization metadata, and returns the Phase 1 CSV header + rows.
func Download(ctx context.Context, c *apiv4.Client, cid string) (header []string, rows [][]string, err error) {
	if c == nil {
		return nil, nil, fmt.Errorf("scoreboard: nil api client")
	}
	if _, err := c.Contests.Get(ctx, cid); err != nil {
		return nil, nil, fmt.Errorf("scoreboard: get contest %s: %w", cid, err)
	}
	snap, err := c.Scoreboard.Get(ctx, cid, apiv4.ScoreboardListOpts{})
	if err != nil {
		return nil, nil, fmt.Errorf("scoreboard: download: %w", err)
	}
	teams, err := c.Teams.List(ctx, cid)
	if err != nil {
		return nil, nil, fmt.Errorf("scoreboard: list teams: %w", err)
	}
	orgs, err := c.Organizations.List(ctx, cid)
	if err != nil {
		return nil, nil, fmt.Errorf("scoreboard: list organizations: %w", err)
	}
	return Build(*snap, teams, orgs)
}
