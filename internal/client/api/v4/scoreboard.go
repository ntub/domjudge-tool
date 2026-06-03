package v4

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/ntub/domjudge-tool/internal/model"
)

// ScoreboardResource wraps the /contests/{cid}/scoreboard endpoint.
type ScoreboardResource struct{ c *Client }

// ScoreboardListOpts holds optional query parameters for Get.
type ScoreboardListOpts struct {
	AllTeams  *bool
	Category  *string
	Public    *bool
	SortOrder *int
}

// Get returns the JSON scoreboard snapshot for contest cid.
func (r *ScoreboardResource) Get(ctx context.Context, cid string, opts ScoreboardListOpts) (*model.ScoreboardSnapshot, error) {
	q := url.Values{}
	if opts.AllTeams != nil {
		q.Set("allteams", strconv.FormatBool(*opts.AllTeams))
	}
	if opts.Category != nil {
		q.Set("category", *opts.Category)
	}
	if opts.Public != nil {
		q.Set("public", strconv.FormatBool(*opts.Public))
	}
	if opts.SortOrder != nil {
		q.Set("sortorder", strconv.Itoa(*opts.SortOrder))
	}
	var out model.ScoreboardSnapshot
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/scoreboard", cid)+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
