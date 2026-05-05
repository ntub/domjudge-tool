package scoreboard

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ErrNoScoreboard is returned when the input HTML has no
// `table.scoreboard` element. Callers translate this into a
// user-facing error in the CLI layer.
var ErrNoScoreboard = errors.New("scoreboard: table.scoreboard not found")

// Parse extracts the scoreboard table from r and returns the CSV
// header (always 5 fixed columns + one per problem) and one row per
// `tbody tr`. Summary rows (`td.scoresummary`) are emitted with the
// Python-locked layout: three blank columns, then the total solved
// count, a blank, then per-problem `slash-joined-on-whitespace`
// strings.
//
// QUIRKS locked from Python `commands/scoreboard/__init__.py`:
//
//   - Team-name column = the last whitespace-separated token of the
//     first `<span>` descendant of the row. Python's
//     `element.find("span").text.split()[-1]` is preserved verbatim
//     because NTUB team names follow a `"<prefix> <id>"` convention
//     and downstream tools depend on the trailing-id projection.
//
//   - Score-cell formatting branches on token count: 0 → empty,
//     2 → "<a> <b>", 3 → "<a>/<b> <c>". Other token counts are
//     skipped, producing fewer columns than the header. Python has
//     the same cliff. Locked here without an exhaustive fallback.
//
//   - Summary `td` cells starting at index 3 are emitted as
//     `strings.Join(strings.Fields(text), "/")` — so "1 / 2"
//     becomes "1///2" because `Fields` keeps the literal `/` token.
//     This is a known Python quirk; the CSV consumers parse around it.
func Parse(r io.Reader) (header []string, rows [][]string, err error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("scoreboard: parse html: %w", err)
	}
	table := doc.Find("table.scoreboard").First()
	if table.Length() == 0 {
		return nil, nil, ErrNoScoreboard
	}

	header = append([]string{}, fixedHeader...)
	table.Find("tr.scoreheader th").Each(func(_ int, s *goquery.Selection) {
		title, ok := s.Attr("title")
		if !ok {
			return
		}
		if strings.HasPrefix(title, "problem ") {
			header = append(header, title[len("problem "):])
		}
	})

	table.Find("tbody tr").Each(func(_ int, row *goquery.Selection) {
		if row.Find("td.scoresummary").Length() > 0 {
			rows = append(rows, parseSummary(row))
			return
		}
		rows = append(rows, parseScores(row))
	})

	return header, rows, nil
}

// fixedHeader is the per-Python prefix applied to every scoreboard CSV.
var fixedHeader = []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score"}

func parseScores(row *goquery.Selection) []string {
	rank := strings.TrimSpace(row.Find("td.scorepl").First().Text())

	aff := ""
	if affEl := row.Find("td.scoretn span.univ").First(); affEl.Length() > 0 {
		aff = strings.TrimSpace(affEl.Text())
	}

	teamName := ""
	if firstSpan := row.Find("span").First(); firstSpan.Length() > 0 {
		toks := strings.Fields(firstSpan.Text())
		if len(toks) > 0 {
			teamName = toks[len(toks)-1]
		}
	}

	solved := strings.TrimSpace(row.Find("td.scorenc").First().Text())
	score := strings.TrimSpace(row.Find("td.scorett").First().Text())

	out := []string{rank, aff, teamName, solved, score}
	row.Find("td.score_cell").Each(func(_ int, cell *goquery.Selection) {
		toks := strings.Fields(strings.TrimSpace(cell.Text()))
		switch len(toks) {
		case 0:
			out = append(out, "")
		case 2:
			out = append(out, fmt.Sprintf("%s %s", toks[0], toks[1]))
		case 3:
			out = append(out, fmt.Sprintf("%s/%s %s", toks[0], toks[1], toks[2]))
		}
		// 1 / 4+ token cells are intentionally skipped — Python parity.
	})
	return out
}

func parseSummary(row *goquery.Selection) []string {
	out := []string{"", "", "", strings.TrimSpace(row.Find("td.scorenc").First().Text()), ""}

	row.Find("td").Each(func(i int, td *goquery.Selection) {
		if i < 3 {
			return
		}
		toks := strings.Fields(strings.TrimSpace(td.Text()))
		out = append(out, strings.Join(toks, "/"))
	})
	return out
}
