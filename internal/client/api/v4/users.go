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

// UserWrite is the JSON body accepted by DOMjudge 8.3.1's single-user
// POST /users and PUT /users/{id} endpoints.
type UserWrite struct {
	ID       *string  `json:"id,omitempty"`
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Email    *string  `json:"email,omitempty"`
	IP       *string  `json:"ip,omitempty"`
	Password *string  `json:"password,omitempty"`
	Enabled  *bool    `json:"enabled,omitempty"`
	TeamID   *string  `json:"team_id,omitempty"`
	Roles    []string `json:"roles"`
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

// Create adds a user via POST /api/v4/users.
func (r *UsersResource) Create(ctx context.Context, body UserWrite) (*model.User, error) {
	var out model.User
	if err := r.c.postJSON(ctx, "/users", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update updates or creates a user via PUT /api/v4/users/{id}.
func (r *UsersResource) Update(ctx context.Context, id string, body UserWrite) (*model.User, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: user id must not be empty", ErrInvalidArgument)
	}
	var out model.User
	if err := r.c.putJSON(ctx, fmt.Sprintf("/users/%s", id), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
