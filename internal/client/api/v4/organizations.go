package v4

import (
	"context"
	"fmt"

	"github.com/ntub/domjudge-tool/internal/model"
)

// OrganizationsResource wraps the /contests/{cid}/organizations endpoints.
type OrganizationsResource struct{ c *Client }

// List returns all organizations visible in contest cid.
func (r *OrganizationsResource) List(ctx context.Context, cid string) ([]model.Organization, error) {
	var out []model.Organization
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/organizations", cid), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single organization by id from contest cid.
func (r *OrganizationsResource) Get(ctx context.Context, cid, id string) (*model.Organization, error) {
	var out model.Organization
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/organizations/%s", cid, id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FindByName returns the first organization with an exact Name match, or nil
// when no organization matches.
func (r *OrganizationsResource) FindByName(ctx context.Context, cid, name string) (*model.Organization, error) {
	orgs, err := r.List(ctx, cid)
	if err != nil {
		return nil, err
	}
	for i := range orgs {
		if orgs[i].Name == name {
			return &orgs[i], nil
		}
	}
	return nil, nil
}
