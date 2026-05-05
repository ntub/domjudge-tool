package v4

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ntub/domjudge-tool/internal/model"
)

// JudgementTypesResource wraps the /contests/{cid}/judgement-types endpoints.
type JudgementTypesResource struct{ c *Client }

// JudgementTypesListOpts holds optional query parameters for List.
// Nil / empty fields are omitted from the query string.
type JudgementTypesListOpts struct {
	IDs    []string
	Strict *bool
}

// List returns all judgement types in contest cid matching opts from
// GET /api/v4/contests/{cid}/judgement-types.
func (r *JudgementTypesResource) List(ctx context.Context, cid string, opts JudgementTypesListOpts) ([]model.JudgementType, error) {
	q := url.Values{}
	for _, id := range opts.IDs {
		q.Add("ids[]", id)
	}
	if opts.Strict != nil {
		if *opts.Strict {
			q.Set("strict", "true")
		} else {
			q.Set("strict", "false")
		}
	}
	var out []model.JudgementType
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/judgement-types", cid)+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single judgement type from
// GET /api/v4/contests/{cid}/judgement-types/{id}.
func (r *JudgementTypesResource) Get(ctx context.Context, cid, id string) (*model.JudgementType, error) {
	var out model.JudgementType
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/judgement-types/%s", cid, id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
