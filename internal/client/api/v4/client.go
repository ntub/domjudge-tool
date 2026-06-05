package v4

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ntub/domjudge-tool/internal/client/httpx"
)

// Client is the top-level DOMjudge API v4 client. Sub-resource fields mirror
// the Python service class structure so callers write c.Users.List(ctx, opts).
// The /api/v4 path prefix is appended to BaseURL once on construction and
// prepended in every resource method.
type Client struct {
	base       string
	http       *httpx.Client
	postHTTP   *http.Client
	authHeader string

	General        *GeneralResource
	Users          *UsersResource
	Teams          *TeamsResource
	Problems       *ProblemsResource
	Submissions    *SubmissionsResource
	Judgements     *JudgementsResource
	JudgementTypes *JudgementTypesResource
	Imports        *ImportsResource
	Organizations  *OrganizationsResource
	Groups         *GroupsResource
	Contests       *ContestsResource
	Scoreboard     *ScoreboardResource
}

// Options configures a Client.
type Options struct {
	// BaseURL is the DOMjudge host root, e.g. "http://domjudge.example.com".
	// The /api/v4 prefix is appended automatically.
	BaseURL    string
	Username   string
	Password   string
	DisableSSL bool
	Timeout    time.Duration
}

// New creates a Client. All options are applied to the underlying
// httpx.Client; the Python APIClient constructor bug (api_client.py:48-55)
// that drops disable_ssl, timeout, and limits is not reproduced here.
func New(opts Options) *Client {
	hc := httpx.New(httpx.Options{
		Username:   opts.Username,
		Password:   opts.Password,
		DisableSSL: opts.DisableSSL,
		Timeout:    opts.Timeout,
	})
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	postHTTP := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: opts.DisableSSL, //nolint:gosec // user-controlled compatibility toggle
			},
		},
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(opts.Username+":"+opts.Password))
	base := strings.TrimRight(opts.BaseURL, "/") + "/api/v4"
	c := &Client{base: base, http: hc, postHTTP: postHTTP, authHeader: authHeader}
	c.General = &GeneralResource{c: c}
	c.Users = &UsersResource{c: c}
	c.Teams = &TeamsResource{c: c}
	c.Problems = &ProblemsResource{c: c}
	c.Submissions = &SubmissionsResource{c: c}
	c.Judgements = &JudgementsResource{c: c}
	c.JudgementTypes = &JudgementTypesResource{c: c}
	c.Imports = &ImportsResource{c: c}
	c.Organizations = &OrganizationsResource{c: c}
	c.Groups = &GroupsResource{c: c}
	c.Contests = &ContestsResource{c: c}
	c.Scoreboard = &ScoreboardResource{c: c}
	return c
}

// getJSON performs a GET to base+path, reads the body, checks the status,
// and JSON-decodes into out. Non-2xx responses are returned as *HTTPError.
func (c *Client) getJSON(ctx context.Context, path string, out interface{}) error {
	resp, err := c.http.GET(ctx, c.base+path)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode, Body: body, URL: c.base + path}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// getStream performs a GET to base+path and returns the raw *http.Response.
// The caller must close resp.Body. Non-2xx responses are consumed and
// returned as *HTTPError with the body captured.
func (c *Client) getStream(ctx context.Context, path string) (*http.Response, error) {
	resp, err := c.http.GETStream(ctx, c.base+path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: body, URL: c.base + path}
	}
	return resp, nil
}

func (c *Client) postJSON(ctx context.Context, path string, in, out interface{}) error {
	return c.writeJSON(ctx, http.MethodPost, path, in, out)
}

func (c *Client) putJSON(ctx context.Context, path string, in, out interface{}) error {
	return c.writeJSON(ctx, http.MethodPut, path, in, out)
}

func (c *Client) writeJSON(ctx context.Context, method, path string, in, out interface{}) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build JSON request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.postHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode, Body: respBody, URL: c.base + path}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
