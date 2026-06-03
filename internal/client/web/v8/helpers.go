package v8

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

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
		return nil, fmt.Errorf("v8: fetch %s redirected to /login: %w",
			urlPath, web.ErrLoginFailed)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}

func (c *Client) deleteRowsByFirstAnchor(
	ctx context.Context,
	listPath, what string,
	linkIdx int,
	include, exclude map[string]struct{},
) error {
	doc, err := c.fetchDoc(ctx, listPath)
	if err != nil {
		return fmt.Errorf("v8: delete %ss: %w", what, err)
	}

	var deleteHrefs []string
	var rowErr error
	doc.Find("table tbody tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		anchors := row.Find("a")
		if anchors.Length() == 0 {
			rowErr = fmt.Errorf("v8: %s row has 0 anchors: %w", what, web.ErrMalformedRow)
			return false
		}
		name := strings.TrimSpace(anchors.First().Text())
		lower := strings.ToLower(name)

		// QUIRK: match Python services/web/v7.py inverted filter:
		// empty include skips every row rather than deleting all rows.
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
			rowErr = fmt.Errorf("v8: %s row anchor index %d out of range: %w",
				what, linkIdx, web.ErrMalformedRow)
			return false
		}
		href, ok := anchors.Eq(idx).Attr("href")
		if !ok {
			rowErr = fmt.Errorf("v8: %s delete link missing href: %w", what, web.ErrMalformedRow)
			return false
		}
		deleteHrefs = append(deleteHrefs, href)
		return true
	})
	if rowErr != nil {
		return rowErr
	}

	for _, href := range deleteHrefs {
		if err := c.confirmAndPostDelete(ctx, what, href); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) confirmAndPostDelete(ctx context.Context, what, href string) error {
	values, err := c.fetchPostFormValues(ctx, href)
	if err != nil {
		return fmt.Errorf("v8: delete %s GET %s: %w", what, href, err)
	}

	resp, err := c.opts.Session.PostForm(ctx, href, values)
	if err != nil {
		return fmt.Errorf("v8: delete %s POST %s: %w", what, href, err)
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("v8: delete %s POST %s: read body: %w", what, href, readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("v8: delete %s POST %s: %w", what, href, describeStatus(resp, body))
	}
	if resp.Request.URL.Path == pathLogin {
		return fmt.Errorf("v8: delete %s POST %s redirected to /login: %w",
			what, href, web.ErrLoginFailed)
	}
	return nil
}

func (c *Client) fetchPostFormValues(ctx context.Context, formPath string) (url.Values, error) {
	resp, err := c.opts.Session.GET(ctx, formPath)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read form body: %w", readErr)
	}
	if resp.StatusCode >= 400 {
		return nil, describeStatus(resp, body)
	}
	if resp.Request.URL.Path == pathLogin {
		return nil, fmt.Errorf("v8: form GET %s redirected to /login: %w",
			formPath, web.ErrLoginFailed)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	hasPostForm := false
	doc.Find("form").EachWithBreak(func(_ int, form *goquery.Selection) bool {
		if strings.EqualFold(strings.TrimSpace(form.AttrOr("method", "get")), "post") {
			hasPostForm = true
			return false
		}
		return true
	})
	if !hasPostForm {
		return nil, fmt.Errorf("v8: form GET %s missing method=post form", formPath)
	}

	values, err := web.ParseFormFields(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}
	return values, nil
}

func lowerSet(xs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		out[strings.ToLower(x)] = struct{}{}
	}
	return out
}

func stringSet(xs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		out[x] = struct{}{}
	}
	return out
}
