package v8

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// Login establishes an authenticated DOMjudge 8.3.1 jury session.
func (c *Client) Login(ctx context.Context) error {
	resp, err := c.opts.Session.GET(ctx, pathLogin)
	if err != nil {
		return fmt.Errorf("v8: login: GET /login: %w", err)
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("v8: login: read /login body: %w", readErr)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("v8: login: %w", describeStatus(resp, body))
	}

	fields, err := web.ParseFormFields(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("v8: login: parse form: %w", err)
	}

	values := url.Values{}
	for k, vs := range fields {
		values[k] = append([]string(nil), vs...)
	}
	values.Set("_username", c.opts.Username)
	values.Set("_password", c.opts.Password)

	resp, err = c.opts.Session.PostForm(ctx, pathLogin, values)
	if err != nil {
		return fmt.Errorf("v8: login: POST /login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.Request.URL.Path != pathJury {
		return fmt.Errorf("v8: login: landed on %q, want %q: %w",
			resp.Request.URL.Path, pathJury, web.ErrLoginFailed)
	}
	return nil
}
