package v8

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

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
	if len(roles) > 0 {
		rolesAsStr := make([]string, len(roles))
		for i, r := range roles {
			rolesAsStr[i] = strconv.Itoa(r)
		}
		overrides["user[user_roles][]"] = rolesAsStr
	}
	resp, err := c.postForm(ctx, editPath, overrides)
	if err != nil {
		return fmt.Errorf("v8: set user password: %w", err)
	}
	if resp.Request.URL.Path == editPath {
		return fmt.Errorf("v8: set user password: stayed on %s: %w",
			editPath, web.ErrUpdateFailed)
	}
	return nil
}

func (c *Client) UpdateUserEmail(ctx context.Context, userID, email string) error {
	editPath := fmt.Sprintf(pathUsersEditFmt, userID)

	overrides := url.Values{
		"user[email]": {email},
	}
	resp, err := c.postForm(ctx, editPath, overrides)
	if err != nil {
		return fmt.Errorf("v8: update user email: %w", err)
	}
	if resp.Request.URL.Path == editPath {
		return fmt.Errorf("v8: update user email: stayed on %s: %w",
			editPath, web.ErrUpdateFailed)
	}
	return nil
}
