package v4

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ntub/domjudge-tool/internal/model"
)

// UsersResource wraps the /users endpoints.
type UsersResource struct{ c *Client }

// UsersListOpts holds optional query parameters for List.
// Nil / empty fields are omitted from the query string.
type UsersListOpts struct {
	IDs    []string
	TeamID *string
}

// List returns all users matching opts from GET /api/v4/users.
func (r *UsersResource) List(ctx context.Context, opts UsersListOpts) ([]model.User, error) {
	q := url.Values{}
	for _, id := range opts.IDs {
		q.Add("ids[]", id)
	}
	if opts.TeamID != nil {
		q.Set("team_id", *opts.TeamID)
	}
	var out []model.User
	if err := r.c.getJSON(ctx, "/users"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single user by id from GET /api/v4/users/{id}.
func (r *UsersResource) Get(ctx context.Context, id string) (*model.User, error) {
	var out model.User
	if err := r.c.getJSON(ctx, fmt.Sprintf("/users/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
