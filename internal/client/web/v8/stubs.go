package v8

import (
	"context"

	"github.com/ntub/domjudge-tool/internal/model"
)

// GetAffiliations is intentionally unsupported by the v8 scraper.
func (c *Client) GetAffiliations(ctx context.Context) ([]model.Affiliation, error) {
	return nil, unsupported("get affiliations")
}

// GetAffiliation is intentionally unsupported by the v8 scraper.
func (c *Client) GetAffiliation(ctx context.Context, name string) (*model.Affiliation, error) {
	return nil, unsupported("get affiliation")
}

// CreateAffiliation is intentionally unsupported by the v8 scraper.
func (c *Client) CreateAffiliation(
	ctx context.Context,
	shortname, name, country string,
) (*model.Affiliation, error) {
	return nil, unsupported("create affiliation")
}

// SetUserPassword is intentionally unsupported by the v8 scraper.
func (c *Client) SetUserPassword(
	ctx context.Context,
	userID, password string,
	roles []int,
	enabled bool,
) error {
	return unsupported("set user password")
}

// UpdateUserEmail is intentionally unsupported by the v8 scraper.
func (c *Client) UpdateUserEmail(ctx context.Context, userID, email string) error {
	return unsupported("update user email")
}

// CreateTeamAndUser is intentionally unsupported by the v8 scraper.
func (c *Client) CreateTeamAndUser(
	ctx context.Context,
	user model.CreateUser,
	categoryID, affiliationID int,
	enabled bool,
) (teamID, userID string, err error) {
	return "", "", unsupported("create team and user")
}

// UpdateTeam is intentionally unsupported by the v8 scraper.
func (c *Client) UpdateTeam(
	ctx context.Context,
	user model.User,
	categoryID, affiliationID int,
	enabled bool,
) (teamID, userID string, err error) {
	return "", "", unsupported("update team")
}
