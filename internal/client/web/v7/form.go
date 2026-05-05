package v7

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// postForm performs the GET → parse → merge overrides → POST flow that
// every write op shares (CreateAffiliation, SetUserPassword,
// CreateTeamAndUser, UpdateTeam).
//
// Snapshot semantics: ParseFormFields returns a url.Values that mirrors
// the form's current state (every <input>, every <textarea>, every
// selected <option> on every <select> — multi-selects keep all
// selections; unselected selects are absent). Overrides replace whole
// keys. The merged map is the POST body. Callers no longer need an
// explicit "drop nil" hint — keys absent from the parser's snapshot
// stay absent unless an override puts them back.
//
// Redirect-to-login guard: SessionClient follows redirects, so an
// expired session would otherwise cause both the GET and the POST to
// silently land on /login with status 200. We check the post-redirect
// URL.Path on both legs and surface web.ErrLoginFailed instead.
//
// The returned *http.Response has had its body fully read and closed —
// callers must NOT read Body further. Inspect resp.Request.URL to see
// the post-redirect landing URL (used for redirect-as-success checks
// in the caller).
func (c *Client) postForm(
	ctx context.Context,
	path string,
	overrides url.Values,
) (*http.Response, error) {
	resp, err := c.opts.Session.GET(ctx, path)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read form body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, describeStatus(resp, body)
	}
	if resp.Request.URL.Path == pathLogin {
		return nil, fmt.Errorf("v7: form GET %s redirected to /login: %w",
			path, web.ErrLoginFailed)
	}

	fields, err := web.ParseFormFields(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}

	values := url.Values{}
	for k, vs := range fields {
		values[k] = append([]string(nil), vs...)
	}
	for k, vs := range overrides {
		values[k] = append([]string(nil), vs...)
	}

	postResp, err := c.opts.Session.PostForm(ctx, path, values)
	if err != nil {
		return nil, err
	}
	postBody, err := io.ReadAll(postResp.Body)
	_ = postResp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read post body: %w", err)
	}
	if postResp.StatusCode >= 400 {
		return nil, describeStatus(postResp, postBody)
	}
	if postResp.Request.URL.Path == pathLogin {
		return nil, fmt.Errorf("v7: form POST %s redirected to /login: %w",
			path, web.ErrLoginFailed)
	}
	return postResp, nil
}
