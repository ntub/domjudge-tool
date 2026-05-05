package v7

import (
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// GetAffiliations parses /jury/affiliations and returns one
// model.Affiliation per row. Mirrors v7.py:225-248. The country column
// reads the country flag's <img alt="..."> when present, otherwise
// falls back to the cell's text.
func (c *Client) GetAffiliations(ctx context.Context) ([]model.Affiliation, error) {
	doc, err := c.fetchDoc(ctx, pathAffiliationsList)
	if err != nil {
		return nil, fmt.Errorf("v7: get affiliations: %w", err)
	}

	out := make([]model.Affiliation, 0)
	var rowErr error
	doc.Find("table tbody tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		anchors := row.Find("td a")
		if anchors.Length() < 4 {
			rowErr = fmt.Errorf("v7: affiliation row has %d anchors, want ≥4: %w",
				anchors.Length(), web.ErrMalformedRow)
			return false
		}

		id := strings.TrimSpace(anchors.Eq(0).Text())
		shortname := strings.TrimSpace(anchors.Eq(1).Text())
		name := strings.TrimSpace(anchors.Eq(2).Text())
		country := readCountry(anchors.Eq(3))

		idCopy := id
		out = append(out, model.Affiliation{
			ID:        &idCopy,
			ShortName: shortname,
			Name:      name,
			Country:   country,
		})
		return true
	})
	if rowErr != nil {
		return nil, rowErr
	}
	return out, nil
}

// CreateAffiliation submits the /jury/affiliations/add form. The four
// caller-supplied fields (shortname, name, country, comments="") are
// merged with the form's hidden inputs via postForm and POSTed.
// Success is decided by the post-redirect URL: a final path other
// than /jury/affiliations/add is treated as "created" and the trailing
// segment becomes the new affiliation's ID. Mirrors v7.py:198-223.
func (c *Client) CreateAffiliation(
	ctx context.Context,
	shortname, name, country string,
) (*model.Affiliation, error) {
	overrides := map[string][]string{
		"team_affiliation[shortname]": {shortname},
		"team_affiliation[name]":      {name},
		"team_affiliation[country]":   {country},
		"team_affiliation[comments]":  {""},
	}
	resp, err := c.postForm(ctx, pathAffiliationsAdd, overrides)
	if err != nil {
		return nil, fmt.Errorf("v7: create affiliation: %w", err)
	}
	if resp.Request.URL.Path == pathAffiliationsAdd {
		return nil, fmt.Errorf("v7: create affiliation: stayed on %s: %w",
			pathAffiliationsAdd, web.ErrCreateFailed)
	}
	idStr := lastPathSegment(resp.Request.URL.Path)
	idCopy := idStr
	return &model.Affiliation{
		ID:        &idCopy,
		ShortName: shortname,
		Name:      name,
		Country:   country,
	}, nil
}

// GetAffiliation returns the first affiliation whose Name or ShortName
// equals name. Returns (nil, nil) when no row matches — mirrors Python
// Optional[Affiliation] at v7.py:250-257. The underlying list-fetch
// error propagates unchanged.
func (c *Client) GetAffiliation(ctx context.Context, name string) (*model.Affiliation, error) {
	all, err := c.GetAffiliations(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Name == name || all[i].ShortName == name {
			return &all[i], nil
		}
	}
	return nil, nil //nolint:nilnil // matches Python Optional[Affiliation]
}

// readCountry returns the <img alt="..."> of the cell when an <img>
// child is present (alt may be empty), or the cell's text otherwise.
// Locks the try/except behaviour at v7.py:236-238: Python's
// `.img.get("alt", "")` returns "" when alt is missing — it does NOT
// fall through to the text branch — so the alt check must NOT be
// gated on alt being present.
func readCountry(cell *goquery.Selection) string {
	if img := cell.Find("img").First(); img.Length() > 0 {
		alt, _ := img.Attr("alt")
		return strings.TrimSpace(alt)
	}
	return strings.TrimSpace(cell.Text())
}
