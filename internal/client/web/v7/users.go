package v7

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// SetUserPassword merges the /jury/users/{id}/edit form's hidden
// inputs with the supplied password / enabled flag / role IDs and
// posts the merged body. Mirrors v7.py:119-142.
func (c *Client) SetUserPassword(
	ctx context.Context,
	userID, password string,
	roles []int,
	enabled bool,
) error {
	editPath := fmt.Sprintf(pathUsersEditFmt, userID)

	enabledStr := "0"
	if enabled {
		enabledStr = "1"
	}

	overrides := url.Values{
		"user[plainPassword]": {password},
		"user[enabled]":       {enabledStr},
	}
	if roles != nil {
		rolesAsStr := make([]string, len(roles))
		for i, r := range roles {
			rolesAsStr[i] = strconv.Itoa(r)
		}
		overrides["user[user_roles][]"] = rolesAsStr
	}
	resp, err := c.postForm(ctx, editPath, overrides)
	if err != nil {
		return fmt.Errorf("v7: set user password: %w", err)
	}
	if resp.Request.URL.Path == editPath {
		return fmt.Errorf("v7: set user password: stayed on %s: %w",
			editPath, web.ErrUpdateFailed)
	}
	return nil
}

// UpdateUserEmail merges the /jury/users/{id}/edit form's hidden
// inputs with the supplied email address and posts the merged body.
// The rest of the form state is preserved from the current server-side
// values, matching the behavior of SetUserPassword.
func (c *Client) UpdateUserEmail(ctx context.Context, userID, email string) error {
	editPath := fmt.Sprintf(pathUsersEditFmt, userID)

	overrides := url.Values{
		"user[email]": {email},
	}
	resp, err := c.postForm(ctx, editPath, overrides)
	if err != nil {
		return fmt.Errorf("v7: update user email: %w", err)
	}
	if resp.Request.URL.Path == editPath {
		return fmt.Errorf("v7: update user email: stayed on %s: %w",
			editPath, web.ErrUpdateFailed)
	}
	return nil
}

// DeleteUsers iterates the /jury/users table and POSTs the delete link
// of each row whose username matches `include` and is not in `exclude`,
// case-insensitively.
//
// QUIRK: Python services/web/v7.py:161 short-circuits the filter as
//
//	if lower_name not in include or lower_name in exclude: continue
//
// which means an empty `include` set causes EVERY row to be skipped —
// DeleteUsers with no include argument issues zero POSTs. The Python
// user base now relies on this accidental safety net; the regression
// test TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk locks it.
// Do NOT "fix" this without an ADR; see docs/ROADMAP.md Phase 1 quirks.
//
// Mirrors v7.py:144-169.
func (c *Client) DeleteUsers(ctx context.Context, include, exclude []string) error {
	includeSet := lowerSet(include)
	excludeSet := lowerSet(exclude)
	return c.deleteRowsByFirstAnchor(ctx, pathUsersList, "user", -1, includeSet, excludeSet)
}
