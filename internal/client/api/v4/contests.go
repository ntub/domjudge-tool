package v4

import (
	"context"
	"fmt"

	"github.com/ntub/domjudge-tool/internal/model"
)

// ContestsResource wraps the /contests endpoints.
type ContestsResource struct{ c *Client }

// List returns all contests.
func (r *ContestsResource) List(ctx context.Context) ([]model.Contest, error) {
	var out []model.Contest
	if err := r.c.getJSON(ctx, "/contests", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns one contest by DOMjudge internal id.
func (r *ContestsResource) Get(ctx context.Context, cid string) (*model.Contest, error) {
	var out model.Contest
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s", cid), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Resolve converts a contest hint into a DOMjudge internal numeric id. Numeric
// hints pass through without a request; non-numeric hints are matched against
// contest shortnames.
func (r *ContestsResource) Resolve(ctx context.Context, hint string) (string, error) {
	if hint == "" {
		return "", fmt.Errorf("%w: contest hint must not be empty", ErrInvalidArgument)
	}
	if isDigits(hint) {
		return hint, nil
	}
	contests, err := r.List(ctx)
	if err != nil {
		return "", err
	}
	for i := range contests {
		if contests[i].ShortName == hint {
			return contests[i].ID, nil
		}
	}
	return "", ErrNotFound
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
