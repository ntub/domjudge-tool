package v8

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

func (c *Client) CreateTeamAndUser(
	ctx context.Context,
	user model.CreateUser,
	categoryID, affiliationID int,
	enabled bool,
) (teamID, userID string, err error) {
	overrides := teamFormOverrides(user, categoryID, affiliationID, enabled)
	overrides.Set("team[penalty]", "0")
	overrides.Set("team[addUserForTeam]", "create-new-user")
	overrides.Set("team[newUsername]", user.Username)

	resp, err := c.postForm(ctx, pathTeamsAdd, overrides)
	if err != nil {
		return "", "", fmt.Errorf("v8: create team and user: %w", err)
	}
	if resp.Request.URL.Path == pathTeamsAdd {
		return "", "", fmt.Errorf("v8: create team and user: stayed on %s: %w",
			pathTeamsAdd, web.ErrCreateFailed)
	}
	teamID = lastPathSegment(resp.Request.URL.Path)

	userID, err = c.fetchTeamViewUserID(ctx, resp.Request.URL.Path)
	if err != nil {
		return "", "", err
	}
	return teamID, userID, nil
}

func (c *Client) UpdateTeam(
	ctx context.Context,
	user model.User,
	categoryID, affiliationID int,
	enabled bool,
) (teamID, userID string, err error) {
	if user.TeamID == nil || *user.TeamID == "" {
		return "", "", fmt.Errorf("v8: update team: TeamID is required")
	}
	editPath := fmt.Sprintf(pathTeamsEditFmt, *user.TeamID)

	overrides := teamFormOverrides(model.CreateUser{
		Username: user.Username,
		Name:     user.Name,
	}, categoryID, affiliationID, enabled)

	resp, err := c.postForm(ctx, editPath, overrides)
	if err != nil {
		return "", "", fmt.Errorf("v8: update team: %w", err)
	}
	if resp.Request.URL.Path == editPath {
		return "", "", fmt.Errorf("v8: update team: stayed on %s: %w",
			editPath, web.ErrUpdateFailed)
	}
	teamID = lastPathSegment(resp.Request.URL.Path)

	userID, err = c.fetchTeamViewUserID(ctx, resp.Request.URL.Path)
	if err != nil {
		return "", "", err
	}
	return teamID, userID, nil
}

func (c *Client) fetchTeamViewUserID(ctx context.Context, viewPath string) (string, error) {
	doc, err := c.fetchDoc(ctx, viewPath)
	if err != nil {
		return "", fmt.Errorf("v8: fetch team view: %w", err)
	}
	link := doc.Find(`a[href^="/jury/users/"]`).First()
	if link.Length() == 0 {
		return "", fmt.Errorf("v8: team view: user link missing in %s", viewPath)
	}
	href, ok := link.Attr("href")
	if !ok {
		return "", fmt.Errorf("v8: team view: user link missing href in %s", viewPath)
	}
	return lastPathSegment(href), nil
}

func teamFormOverrides(
	user model.CreateUser,
	categoryID, affiliationID int,
	enabled bool,
) url.Values {
	enabledStr := "0"
	if enabled {
		enabledStr = "1"
	}
	return url.Values{
		"team[name]":        {user.Username},
		"team[displayName]": {user.Name},
		"team[category]":    {strconv.Itoa(categoryID)},
		"team[affiliation]": {strconv.Itoa(affiliationID)},
		"team[enabled]":     {enabledStr},
	}
}
