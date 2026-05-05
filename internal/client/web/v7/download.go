package v7

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// Download fetches path through the authenticated session and returns
// the raw response body. Non-2xx responses are translated into
// *HTTPError so callers can errors.Is against web.ErrUnauthorized /
// web.ErrServerError.
//
// The body is returned untouched (no buffering); callers MUST close
// the io.ReadCloser. Used by the PR-7 download-problems CLI to stream
// export ZIPs to disk one problem at a time.
func (c *Client) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	resp, err := c.opts.Session.GET(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("v7: download %s: %w", path, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, describeStatus(resp, body)
	}
	if resp.Request.URL.Path == pathLogin {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("v7: download %s redirected to /login: %w",
			path, web.ErrLoginFailed)
	}
	return resp.Body, nil
}
