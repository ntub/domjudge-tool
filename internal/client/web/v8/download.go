package v8

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// Download fetches path through the authenticated session and returns
// the raw response body.
func (c *Client) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	resp, err := c.opts.Session.GET(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("v8: download %s: %w", path, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, describeStatus(resp, body)
	}
	if resp.Request.URL.Path == pathLogin {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("v8: download %s redirected to /login: %w",
			path, web.ErrLoginFailed)
	}
	return resp.Body, nil
}
