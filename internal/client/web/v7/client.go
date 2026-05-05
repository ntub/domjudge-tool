package v7

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/ntub/domjudge-tool/internal/client/httpx"
	"github.com/ntub/domjudge-tool/internal/client/web"
)

// Options configures a v7.Client.
type Options struct {
	// Session is the cookie-aware HTTP transport. Required.
	Session *httpx.SessionClient
	// Username is the jury username posted to /login.
	Username string
	// Password is the jury password posted to /login.
	Password string
	// ContestID, if non-empty, is translated into a domjudge_cid cookie
	// set on Session's jar at construction time. v8 implementations may
	// translate the same field into path scoping instead.
	ContestID string
}

// ErrMissingSession is returned by New when Options.Session is nil.
var ErrMissingSession = errors.New("v7: Options.Session is required")

// Client implements web.Gateway against DOMjudge 7.3.x.
type Client struct {
	opts Options
}

// Compile-time guarantee that *Client satisfies web.Gateway. Phase 2's
// v8.Client will satisfy the same interface, so callers (PR-4 importer,
// PR-6 CLI) program against web.Gateway and the implementation swaps
// without code changes.
var _ web.Gateway = (*Client)(nil)

// New constructs a v7.Client. If Options.ContestID is non-empty, the
// translation to a domjudge_cid cookie happens here and is invisible to
// callers — they only ever interact with web.Gateway methods.
func New(opts Options) (*Client, error) {
	if opts.Session == nil {
		return nil, ErrMissingSession
	}
	if opts.ContestID != "" {
		if err := setContestCookie(opts.Session, opts.ContestID); err != nil {
			return nil, err
		}
	}
	return &Client{opts: opts}, nil
}

// Session returns the underlying HTTP transport. Exposed for testing
// and for advanced callers that need to set additional cookies; normal
// callers go through web.Gateway.
func (c *Client) Session() *httpx.SessionClient {
	return c.opts.Session
}

// setContestCookie installs domjudge_cid on the session jar at the
// session's BaseURL host, scoped to "/".
func setContestCookie(sc *httpx.SessionClient, cid string) error {
	root, err := url.Parse(sc.BaseURL())
	if err != nil {
		return err
	}
	root.Path = "/"
	sc.Jar().SetCookies(root, []*http.Cookie{
		{Name: "domjudge_cid", Value: cid, Path: "/"},
	})
	return nil
}

// describeStatus wraps a non-2xx response in an *HTTPError so callers
// can match against web.ErrUnauthorized / web.ErrServerError.
func describeStatus(resp *http.Response, body []byte) error {
	return &HTTPError{
		Status: resp.StatusCode,
		URL:    resp.Request.URL.String(),
		Body:   body,
	}
}
