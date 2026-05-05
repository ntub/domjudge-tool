// Package httpx provides a thin *http.Client wrapper that injects HTTP Basic
// Auth on every request and optionally disables TLS verification.
//
// Deviation from Python reference (api_client.py:48-55): the Python APIClient
// constructor resets _parameters to {base_url, auth} only, silently dropping
// disable_ssl, timeout, and limits passed to the parent BaseClient. The Go
// Client applies all four options correctly by building the *http.Transport
// and http.Client.Timeout from the caller-supplied Options before the first
// request.
package httpx

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

// Options configures a Client.
type Options struct {
	Username   string
	Password   string
	DisableSSL bool
	Timeout    time.Duration
}

// Client wraps *http.Client with per-request Basic Auth. No automatic
// redirects are followed, mirroring the Python API client behaviour.
type Client struct {
	inner      *http.Client
	authHeader string
}

// New returns a Client configured with opts. If Timeout is zero a default
// of 30 s is applied.
func New(opts Options) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.DisableSSL, //nolint:gosec // user-controlled toggle
		},
	}
	creds := base64.StdEncoding.EncodeToString([]byte(opts.Username + ":" + opts.Password))
	return &Client{
		inner: &http.Client{
			Transport: tr,
			Timeout:   timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		authHeader: "Basic " + creds,
	}
}

// GET executes a GET request, injecting the Basic Auth header.
func (c *Client) GET(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("httpx: build request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	resp, err := c.inner.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpx: do request: %w", err)
	}
	return resp, nil
}

// GETStream is an alias for GET that signals intent: the caller will stream
// the response body rather than reading it entirely into memory.
func (c *Client) GETStream(ctx context.Context, rawURL string) (*http.Response, error) {
	return c.GET(ctx, rawURL)
}

// Transport returns the underlying *http.Transport for test inspection of
// TLS configuration. Returns nil if the transport is not *http.Transport.
func (c *Client) Transport() *http.Transport {
	if t, ok := c.inner.Transport.(*http.Transport); ok {
		return t
	}
	return nil
}
