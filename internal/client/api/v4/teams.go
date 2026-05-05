package v4

import (
	"context"
	"fmt"

	"github.com/ntub/domjudge-tool/internal/model"
)

// TeamsResource wraps the /contests/{cid}/teams endpoints.
type TeamsResource struct{ c *Client }

// List returns all teams in contest cid from GET /api/v4/contests/{cid}/teams.
func (r *TeamsResource) List(ctx context.Context, cid string) ([]model.Team, error) {
	var out []model.Team
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/teams", cid), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single team by id from GET /api/v4/contests/{cid}/teams/{id}.
func (r *TeamsResource) Get(ctx context.Context, cid, id string) (*model.Team, error) {
	var out model.Team
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/teams/%s", cid, id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
