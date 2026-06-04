package v8

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

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
		return nil, fmt.Errorf("v8: form GET %s redirected to /login: %w",
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
		return nil, fmt.Errorf("v8: form POST %s redirected to /login: %w",
			path, web.ErrLoginFailed)
	}
	return postResp, nil
}
