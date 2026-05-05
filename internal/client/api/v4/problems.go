package v4

import (
	"context"
	"fmt"

	"github.com/ntub/domjudge-tool/internal/model"
)

// ProblemsResource wraps the /contests/{cid}/problems endpoints.
type ProblemsResource struct{ c *Client }

// List returns all problems in contest cid from
// GET /api/v4/contests/{cid}/problems.
func (r *ProblemsResource) List(ctx context.Context, cid string) ([]model.Problem, error) {
	var out []model.Problem
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/problems", cid), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single problem by id from
// GET /api/v4/contests/{cid}/problems/{id}.
func (r *ProblemsResource) Get(ctx context.Context, cid, id string) (*model.Problem, error) {
	var out model.Problem
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/problems/%s", cid, id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
