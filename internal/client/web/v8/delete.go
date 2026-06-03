package v8

import "context"

// DeleteUsers iterates /jury/users and deletes rows selected by the
// locked Python inverted-filter quirk.
func (c *Client) DeleteUsers(ctx context.Context, include, exclude []string) error {
	includeSet := lowerSet(include)
	excludeSet := lowerSet(exclude)
	return c.deleteRowsByFirstAnchor(ctx, pathUsersList, "user", -1, includeSet, excludeSet)
}

// DeleteTeams iterates /jury/teams and deletes rows selected by the
// locked Python inverted-filter quirk.
func (c *Client) DeleteTeams(ctx context.Context, include, exclude []string) error {
	includeSet := lowerSet(include)
	excludeSet := lowerSet(exclude)
	return c.deleteRowsByFirstAnchor(ctx, pathTeamsList, "team", -2, includeSet, excludeSet)
}
