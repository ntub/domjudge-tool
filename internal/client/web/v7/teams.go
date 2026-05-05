package v7

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// CreateTeamAndUser submits /jury/teams/add with the seven team[*]
// fields merged into the hidden inputs, then GETs the resulting team
// view page to extract the auto-created user's ID from the first
// .container-fluid anchor. Mirrors v7.py:49-82.
//
// team[contests][] is dropped from the POST body if the form yielded a
// nil value (no selected option). This locks Python v7.py:69-70.
func (c *Client) CreateTeamAndUser(
	ctx context.Context,
	user model.CreateUser,
	categoryID, affiliationID int,
	enabled bool,
) (teamID, userID string, err error) {
	overrides := teamFormOverrides(user, categoryID, affiliationID, enabled)
	overrides.Set("team[addUserForTeam]", "1")
	overrides.Set("team[users][0][username]", user.Username)

	resp, err := c.postForm(ctx, pathTeamsAdd, overrides)
	if err != nil {
		return "", "", fmt.Errorf("v7: create team and user: %w", err)
	}
	if resp.Request.URL.Path == pathTeamsAdd {
		return "", "", fmt.Errorf("v7: create team and user: stayed on %s: %w",
			pathTeamsAdd, web.ErrCreateFailed)
	}
	teamID = lastPathSegment(resp.Request.URL.Path)

	userID, err = c.fetchTeamViewUserID(ctx, resp.Request.URL.Path)
	if err != nil {
		return "", "", err
	}
	return teamID, userID, nil
}

// UpdateTeam submits /jury/teams/{user.TeamID}/edit, re-using the
// teamFormOverrides helper. team[addUserForTeam] / team[users][0][username]
// are create-only; UpdateTeam omits them. Mirrors v7.py:84-117.
func (c *Client) UpdateTeam(
	ctx context.Context,
	user model.User,
	categoryID, affiliationID int,
	enabled bool,
) (teamID, userID string, err error) {
	if user.TeamID == nil || *user.TeamID == "" {
		return "", "", fmt.Errorf("v7: update team: TeamID is required")
	}
	editPath := fmt.Sprintf(pathTeamsEditFmt, *user.TeamID)

	overrides := teamFormOverrides(model.CreateUser{
		Username: user.Username,
		Name:     user.Name,
	}, categoryID, affiliationID, enabled)

	resp, err := c.postForm(ctx, editPath, overrides)
	if err != nil {
		return "", "", fmt.Errorf("v7: update team: %w", err)
	}
	if resp.Request.URL.Path == editPath {
		return "", "", fmt.Errorf("v7: update team: stayed on %s: %w",
			editPath, web.ErrUpdateFailed)
	}
	teamID = lastPathSegment(resp.Request.URL.Path)

	userID, err = c.fetchTeamViewUserID(ctx, resp.Request.URL.Path)
	if err != nil {
		return "", "", err
	}
	return teamID, userID, nil
}

// DeleteTeams iterates the /jury/teams table and POSTs the delete
// href of each row whose team-id text matches `include` and is not in
// `exclude`, case-insensitively. The Python QUIRK at v7.py:188 is the
// same as DeleteUsers — see DeleteUsers' docstring; nil/empty include
// causes zero POSTs.
//
// Anchor offset: Python `row.select("a")[-2]["href"]` at v7.py:192 —
// the second-to-last anchor (the last is the clarification link).
//
// Mirrors v7.py:171-196.
func (c *Client) DeleteTeams(ctx context.Context, include, exclude []string) error {
	includeSet := lowerSet(include)
	excludeSet := lowerSet(exclude)
	return c.deleteRowsByFirstAnchor(ctx, pathTeamsList, "team", -2, includeSet, excludeSet)
}

// fetchTeamViewUserID GETs the team-view page at viewPath and returns
// the trailing path segment of the first anchor inside .container-fluid
// — that's the auto-created user's ID. Mirrors v7.py:78-80.
func (c *Client) fetchTeamViewUserID(ctx context.Context, viewPath string) (string, error) {
	doc, err := c.fetchDoc(ctx, viewPath)
	if err != nil {
		return "", fmt.Errorf("v7: fetch team view: %w", err)
	}
	link := doc.Find(".container-fluid a").First()
	if link.Length() == 0 {
		return "", fmt.Errorf("v7: team view: user link missing in %s", viewPath)
	}
	href, ok := link.Attr("href")
	if !ok {
		return "", fmt.Errorf("v7: team view: user link missing href in %s", viewPath)
	}
	return lastPathSegment(href), nil
}

// teamFormOverrides builds the common team[*] form overrides shared by
// CreateTeamAndUser and UpdateTeam.
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
		"team[affiliation]": {strconv.Itoa(affiliationID)},
		"team[enabled]":     {enabledStr},
		"team[category]":    {strconv.Itoa(categoryID)},
	}
}
