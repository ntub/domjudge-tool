package v4

import (
	"context"
	"fmt"

	"github.com/ntub/domjudge-tool/internal/model"
)

// GroupsResource wraps the /contests/{cid}/groups endpoints.
type GroupsResource struct{ c *Client }

// List returns all groups visible in contest cid.
func (r *GroupsResource) List(ctx context.Context, cid string) ([]model.Group, error) {
	var out []model.Group
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/groups", cid), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single group by id from contest cid.
func (r *GroupsResource) Get(ctx context.Context, cid, id string) (*model.Group, error) {
	var out model.Group
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/groups/%s", cid, id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
