package v8

import (
	"errors"
	"net/http"

	"github.com/ntub/domjudge-tool/internal/client/httpx"
	"github.com/ntub/domjudge-tool/internal/client/web"
)

// Options configures a v8.Client.
type Options struct {
	// Session is the cookie-aware HTTP transport. Required.
	Session *httpx.SessionClient
	// Username is the jury username posted to /login.
	Username string
	// Password is the jury password posted to /login.
	Password string
	// ContestID is reserved for future v8 contest scoping.
	ContestID string
}

// ErrMissingSession is returned by New when Options.Session is nil.
var ErrMissingSession = errors.New("v8: Options.Session is required")

// Client implements web.Gateway against DOMjudge 8.3.1.
type Client struct {
	opts Options
}

var _ web.Gateway = (*Client)(nil)

// New constructs a v8.Client.
func New(opts Options) (*Client, error) {
	if opts.Session == nil {
		return nil, ErrMissingSession
	}
	return &Client{opts: opts}, nil
}

// Session returns the underlying HTTP transport. Exposed for tests and
// advanced callers; normal callers go through web.Gateway.
func (c *Client) Session() *httpx.SessionClient {
	return c.opts.Session
}

func describeStatus(resp *http.Response, body []byte) error {
	return &HTTPError{
		Status: resp.StatusCode,
		URL:    resp.Request.URL.String(),
		Body:   body,
	}
}
