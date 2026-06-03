package v8

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// ProblemFilter is an alias for web.ProblemFilter.
type ProblemFilter = web.ProblemFilter

// GetProblems parses the DOMjudge 8.3.1 /jury/problems table.
func (c *Client) GetProblems(ctx context.Context, f web.ProblemFilter) ([]model.ProblemItem, error) {
	doc, err := c.fetchDoc(ctx, pathProblemsList)
	if err != nil {
		return nil, fmt.Errorf("v8: get problems: %w", err)
	}

	only := stringSet(f.Only)
	exclude := stringSet(f.Exclude)

	out := make([]model.ProblemItem, 0)
	var rowErr error
	doc.Find("table tbody tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		cells := row.Find("td")
		if cells.Length() < 8 {
			rowErr = fmt.Errorf("v8: problem row has %d cells, want ≥8: %w",
				cells.Length(), web.ErrMalformedRow)
			return false
		}

		id := strings.TrimSpace(cells.Eq(0).Find("a").First().Text())
		if id == "" {
			rowErr = fmt.Errorf("v8: problem row id missing: %w", web.ErrMalformedRow)
			return false
		}
		if len(only) > 0 {
			if _, in := only[id]; !in {
				return true
			}
		}
		if _, dropped := exclude[id]; dropped {
			return true
		}

		name := strings.TrimSpace(cells.Eq(1).Find("a").First().Text())
		timeLimit, _ := parseSeconds(cells.Eq(4).Find("a").First().Text())
		testDataCount, _ := strconv.Atoi(strings.TrimSpace(cells.Eq(7).Find("a").First().Text()))

		exportLink := row.Find(`a[title="export problem as zip-file"]`).First()
		if exportLink.Length() == 0 {
			exportLink = row.Find(`a[href$="/export"]`).First()
		}
		exportPath, ok := exportLink.Attr("href")
		if !ok || strings.TrimSpace(exportPath) == "" {
			rowErr = fmt.Errorf("v8: problem row export link missing: %w", web.ErrMalformedRow)
			return false
		}
		exportPath = strings.TrimSpace(exportPath)
		exportPathCopy := exportPath

		out = append(out, model.ProblemItem{
			ID:             id,
			Name:           name,
			TimeLimit:      timeLimit,
			TestDataCount:  testDataCount,
			ExportFilePath: &exportPathCopy,
		})
		return true
	})
	if rowErr != nil {
		return nil, rowErr
	}
	return out, nil
}

func parseSeconds(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "s")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}
