package v4

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ntub/domjudge-tool/internal/model"
)

// JudgementsResource wraps the /contests/{cid}/judgements endpoints.
type JudgementsResource struct{ c *Client }

// JudgementsListOpts holds optional query parameters for List.
// Nil / empty fields are omitted from the query string.
type JudgementsListOpts struct {
	IDs          []string
	SubmissionID *string
	Result       *string
	Strict       *bool
}

// List returns all judgements in contest cid matching opts from
// GET /api/v4/contests/{cid}/judgements.
func (r *JudgementsResource) List(ctx context.Context, cid string, opts JudgementsListOpts) ([]model.Judgement, error) {
	q := url.Values{}
	for _, id := range opts.IDs {
		q.Add("ids[]", id)
	}
	if opts.SubmissionID != nil {
		q.Set("submission_id", *opts.SubmissionID)
	}
	if opts.Result != nil {
		q.Set("result", *opts.Result)
	}
	if opts.Strict != nil {
		if *opts.Strict {
			q.Set("strict", "true")
		} else {
			q.Set("strict", "false")
		}
	}
	var out []model.Judgement
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/judgements", cid)+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single judgement from
// GET /api/v4/contests/{cid}/judgements/{id}.
func (r *JudgementsResource) Get(ctx context.Context, cid, id string) (*model.Judgement, error) {
	var out model.Judgement
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/judgements/%s", cid, id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
