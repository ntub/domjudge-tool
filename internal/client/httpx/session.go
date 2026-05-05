package httpx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// SessionOptions configures a SessionClient.
type SessionOptions struct {
	// BaseURL is required. All path-only arguments to GET / PostForm are
	// resolved against it.
	BaseURL string
	// DisableSSL flips http.Transport.TLSClientConfig.InsecureSkipVerify.
	DisableSSL bool
	// Timeout is applied to the underlying http.Client. Zero means use the
	// default (30 s, matching Client).
	Timeout time.Duration
}

// SessionClient is the cookie-aware HTTP transport used by the web/v7
// scraper. Unlike Client (which targets the JSON API and uses Basic Auth
// with no redirects), SessionClient holds a cookie jar and follows
// redirects, so a successful POST /login establishes a session that
// subsequent GETs reuse automatically.
//
// Path-only arguments (e.g. "/jury/users/3/delete") are resolved against
// BaseURL inside the client; callers do not concatenate URLs themselves.
type SessionClient struct {
	inner   *http.Client
	jar     *cookiejar.Jar
	base    *url.URL
	timeout time.Duration
}

// ErrInvalidBaseURL is returned by NewSession when BaseURL is empty or
// cannot be parsed.
var ErrInvalidBaseURL = errors.New("httpx: invalid BaseURL")

// NewSession returns a SessionClient configured per opts. BaseURL is
// required; an empty or malformed value returns ErrInvalidBaseURL.
func NewSession(opts SessionOptions) (*SessionClient, error) {
	if strings.TrimSpace(opts.BaseURL) == "" {
		return nil, fmt.Errorf("%w: empty", ErrInvalidBaseURL)
	}
	base, err := url.Parse(opts.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidBaseURL, err)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidBaseURL, opts.BaseURL)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("httpx: new cookie jar: %w", err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.DisableSSL, //nolint:gosec // user-controlled toggle
		},
	}
	return &SessionClient{
		inner: &http.Client{
			Transport: tr,
			Jar:       jar,
			Timeout:   timeout,
			// CheckRedirect left at default → up to 10 redirects followed.
		},
		jar:     jar,
		base:    base,
		timeout: timeout,
	}, nil
}

// GET issues a GET to path resolved against BaseURL.
func (s *SessionClient) GET(ctx context.Context, path string) (*http.Response, error) {
	u, err := s.resolve(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("httpx: build request: %w", err)
	}
	resp, err := s.inner.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpx: do request: %w", err)
	}
	return resp, nil
}

// PostForm issues a POST with Content-Type application/x-www-form-urlencoded.
// Repeated values for the same key (url.Values multi-value semantics) are
// preserved verbatim, matching how DOMjudge encodes user[user_roles][] etc.
func (s *SessionClient) PostForm(ctx context.Context, path string, values url.Values) (*http.Response, error) {
	u, err := s.resolve(path)
	if err != nil {
		return nil, err
	}
	body := strings.NewReader(values.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("httpx: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.inner.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpx: do request: %w", err)
	}
	return resp, nil
}

// Jar returns the cookie jar so callers can pre-seed cookies (e.g.
// domjudge_cid) that the v7.Client.New translates from Options.ContestID.
func (s *SessionClient) Jar() http.CookieJar {
	return s.jar
}

// BaseURL returns the configured base URL as a string. Callers that
// need to seed jar cookies use this to construct a URL whose host
// matches the cookie scope.
func (s *SessionClient) BaseURL() string {
	return s.base.String()
}

// Transport returns the underlying *http.Transport for test inspection of
// TLS configuration. Returns nil if the transport is not *http.Transport.
func (s *SessionClient) Transport() *http.Transport {
	if t, ok := s.inner.Transport.(*http.Transport); ok {
		return t
	}
	return nil
}

// Timeout reports the configured request timeout.
func (s *SessionClient) Timeout() time.Duration {
	return s.timeout
}

// resolve produces an absolute URL by joining path against BaseURL when
// path is relative, or returning path verbatim when it's already absolute.
func (s *SessionClient) resolve(path string) (*url.URL, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("httpx: parse path: %w", err)
	}
	return s.base.ResolveReference(ref), nil
}
