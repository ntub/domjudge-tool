package v8

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

func (c *Client) GetAffiliations(ctx context.Context) ([]model.Affiliation, error) {
	doc, err := c.fetchDoc(ctx, pathAffiliationsList)
	if err != nil {
		return nil, fmt.Errorf("v8: get affiliations: %w", err)
	}

	out := make([]model.Affiliation, 0)
	var rowErr error
	doc.Find("table tbody tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		anchors := row.Find("td a")
		if anchors.Length() < 5 {
			rowErr = fmt.Errorf("v8: affiliation row has %d anchors, want >=5: %w",
				anchors.Length(), web.ErrMalformedRow)
			return false
		}

		id := strings.TrimSpace(anchors.Eq(0).Text())
		shortname := strings.TrimSpace(anchors.Eq(2).Text())
		name := strings.TrimSpace(anchors.Eq(3).Text())
		country := readCountry(anchors.Eq(4))

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

func (c *Client) CreateAffiliation(
	ctx context.Context,
	shortname, name, country string,
) (*model.Affiliation, error) {
	overrides := url.Values{
		"team_affiliation[shortname]": {shortname},
		"team_affiliation[name]":      {name},
		"team_affiliation[country]":   {country},
	}
	resp, err := c.postForm(ctx, pathAffiliationsAdd, overrides)
	if err != nil {
		return nil, fmt.Errorf("v8: create affiliation: %w", err)
	}
	if resp.Request.URL.Path == pathAffiliationsAdd {
		return nil, fmt.Errorf("v8: create affiliation: stayed on %s: %w",
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

func readCountry(cell *goquery.Selection) string {
	if img := cell.Find("img").First(); img.Length() > 0 {
		alt, _ := img.Attr("alt")
		return strings.TrimSpace(alt)
	}
	return strings.TrimSpace(cell.Text())
}
