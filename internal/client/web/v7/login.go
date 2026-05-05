package v7

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// Login establishes an authenticated session by GETing /login (to read
// the CSRF token from the form), then POSTing the token together with
// the configured credentials. Success is determined by the URL of the
// post-redirect landing page: anything other than /jury yields
// web.ErrLoginFailed, never a panic.
//
// Mirrors Python services/web/v7.py:login (38-47).
func (c *Client) Login(ctx context.Context) error {
	resp, err := c.opts.Session.GET(ctx, pathLogin)
	if err != nil {
		return fmt.Errorf("v7: login: GET /login: %w", err)
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("v7: login: read /login body: %w", readErr)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("v7: login: %w", describeStatus(resp, body))
	}

	fields, err := web.ParseFormFields(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("v7: login: parse form: %w", err)
	}

	values := url.Values{}
	for k, vs := range fields {
		values[k] = append([]string(nil), vs...)
	}
	values.Set("_username", c.opts.Username)
	values.Set("_password", c.opts.Password)

	resp, err = c.opts.Session.PostForm(ctx, pathLogin, values)
	if err != nil {
		return fmt.Errorf("v7: login: POST /login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck // drain so the conn is reusable

	if resp.Request.URL.Path != pathJury {
		return fmt.Errorf("v7: login: landed on %q, want %q: %w",
			resp.Request.URL.Path, pathJury, web.ErrLoginFailed)
	}
	return nil
}
