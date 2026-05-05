package v7

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// ProblemFilter is an alias for web.ProblemFilter so callers that
// imported the v7 package keep working.
type ProblemFilter = web.ProblemFilter

// GetProblems parses /jury/problems and returns one model.ProblemItem
// per row. Mirrors v7.py:259-291. Locks the __init__.py:21 panic by
// treating nil filters as empty sets.
func (c *Client) GetProblems(ctx context.Context, f web.ProblemFilter) ([]model.ProblemItem, error) {
	doc, err := c.fetchDoc(ctx, pathProblemsList)
	if err != nil {
		return nil, fmt.Errorf("v7: get problems: %w", err)
	}

	only := stringSet(f.Only)
	exclude := stringSet(f.Exclude)

	out := make([]model.ProblemItem, 0)
	var rowErr error
	doc.Find("table tbody tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		anchors := row.Find("td a")
		if anchors.Length() < 8 {
			rowErr = fmt.Errorf("v7: problem row has %d anchors, want ≥8: %w",
				anchors.Length(), web.ErrMalformedRow)
			return false
		}

		id := strings.TrimSpace(anchors.Eq(0).Text())
		// Apply filters. Empty 'only' means "include all"; non-empty
		// requires id be present.
		if len(only) > 0 {
			if _, in := only[id]; !in {
				return true
			}
		}
		if _, dropped := exclude[id]; dropped {
			return true
		}

		name := strings.TrimSpace(anchors.Eq(1).Text())
		timeLimit, _ := strconv.Atoi(strings.TrimSpace(anchors.Eq(3).Text()))
		testDataCount, _ := strconv.Atoi(strings.TrimSpace(anchors.Eq(6).Text()))
		exportPath, _ := anchors.Eq(7).Attr("href")
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
