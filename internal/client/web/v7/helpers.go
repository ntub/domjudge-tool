package v7

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// fetchDoc issues a GET against urlPath, surfaces non-2xx as
// *HTTPError, and returns the parsed *goquery.Document on success.
//
// Redirect-to-login guard: SessionClient follows redirects, so an
// expired session would otherwise see /jury/* requests land at /login
// with status 200; goquery would parse the login HTML as if it were
// the requested page, and list parsers would return empty results
// (zero affiliations / problems / users), with no error to alert the
// caller. We check resp.Request.URL.Path post-redirect and surface
// web.ErrLoginFailed instead.
func (c *Client) fetchDoc(ctx context.Context, urlPath string) (*goquery.Document, error) {
	resp, err := c.opts.Session.GET(ctx, urlPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, describeStatus(resp, body)
	}
	if resp.Request.URL.Path == pathLogin {
		return nil, fmt.Errorf("v7: fetch %s redirected to /login: %w",
			urlPath, web.ErrLoginFailed)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}

// deleteRowsByFirstAnchor walks /jury/<list>, identifies each row's
// primary anchor text (a[0]) as the row's "name", applies the
// Python-quirk filter, and POSTs the delete-link href.
//
// linkIdx selects which anchor to POST: -1 = last (Python a[-1] for
// users), -2 = second-to-last (Python a[-2] for teams). The QUIRK
// (empty include short-circuits to "skip every row") lives in the
// !in include guard below; see DeleteUsers / DeleteTeams docstrings.
func (c *Client) deleteRowsByFirstAnchor(
	ctx context.Context,
	listPath, what string,
	linkIdx int,
	include, exclude map[string]struct{},
) error {
	doc, err := c.fetchDoc(ctx, listPath)
	if err != nil {
		return fmt.Errorf("v7: delete %ss: %w", what, err)
	}

	var deleteHrefs []string
	var rowErr error
	doc.Find("table tbody tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		anchors := row.Find("a")
		if anchors.Length() == 0 {
			rowErr = fmt.Errorf("v7: %s row has 0 anchors: %w", what, web.ErrMalformedRow)
			return false
		}
		name := strings.TrimSpace(anchors.First().Text())
		lower := strings.ToLower(name)

		// QUIRK: see DeleteUsers / DeleteTeams docstring.
		if _, in := include[lower]; !in {
			return true
		}
		if _, ex := exclude[lower]; ex {
			return true
		}

		idx := linkIdx
		if idx < 0 {
			idx = anchors.Length() + idx
		}
		if idx < 0 || idx >= anchors.Length() {
			rowErr = fmt.Errorf("v7: %s row anchor index %d out of range: %w",
				what, linkIdx, web.ErrMalformedRow)
			return false
		}
		href, ok := anchors.Eq(idx).Attr("href")
		if !ok {
			rowErr = fmt.Errorf("v7: %s delete link missing href: %w", what, web.ErrMalformedRow)
			return false
		}
		deleteHrefs = append(deleteHrefs, href)
		return true
	})
	if rowErr != nil {
		return rowErr
	}

	for _, href := range deleteHrefs {
		dResp, err := c.opts.Session.PostForm(ctx, href, url.Values{})
		if err != nil {
			return fmt.Errorf("v7: delete %s POST %s: %w", what, href, err)
		}
		dBody, _ := io.ReadAll(dResp.Body)
		_ = dResp.Body.Close()
		if dResp.StatusCode >= 400 {
			return fmt.Errorf("v7: delete %s POST %s: %w", what, href, describeStatus(dResp, dBody))
		}
		if dResp.Request.URL.Path == pathLogin {
			return fmt.Errorf("v7: delete %s POST %s redirected to /login: %w",
				what, href, web.ErrLoginFailed)
		}
	}
	return nil
}

// lowerSet returns a lowercase string set, mirroring Python
// set(map(str.lower, include or [])).
func lowerSet(xs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		out[strings.ToLower(x)] = struct{}{}
	}
	return out
}

// stringSet returns a set view of xs (no case folding).
func stringSet(xs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		out[x] = struct{}{}
	}
	return out
}

// lastPathSegment returns the substring after the final '/' of p.
// Equivalent to path.Base except that path.Base on an empty input
// returns ".", and on a trailing-slash input returns the segment
// before — neither matters for our /jury/...-style URLs.
func lastPathSegment(p string) string {
	return path.Base(p)
}
