package scoreboard

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ntub/domjudge-tool/internal/model"
)

// ErrMalformedSnapshot is returned when scoreboard rows disagree on the
// problem columns emitted by the API.
var ErrMalformedSnapshot = errors.New("scoreboard: malformed API snapshot")

// Build converts a DOMjudge API scoreboard snapshot plus team/organization
// lookup data into the Phase 1 CSV shape produced by Parse. The Category
// column is resolved from each team's first group.
func Build(snap model.ScoreboardSnapshot, teams []model.Team, orgs []model.Organization, groups []model.Group) (header []string, rows [][]string, err error) {
	return buildWithCategories(snap, teams, orgs, groups, nil)
}

// buildWithCategories is Build with an optional per-row Category override.
// When categoryByRow is non-nil it must be index-aligned with snap.Rows and
// supplies the Category cell for each row. Download uses this because it knows
// the exact group that produced each row when merging multiple sortorder
// sections — a team in several groups appears once per section and must be
// labelled by the matching group, not blindly by team.GroupIDs[0]. When
// categoryByRow is nil the category falls back to the team's first group.
func buildWithCategories(snap model.ScoreboardSnapshot, teams []model.Team, orgs []model.Organization, groups []model.Group, categoryByRow []string) (header []string, rows [][]string, err error) {
	header = append([]string{}, fixedHeader...)
	header = append(header, "Category")
	if len(snap.Rows) == 0 {
		return header, nil, nil
	}

	labels := make([]string, len(snap.Rows[0].Problems))
	for i, problem := range snap.Rows[0].Problems {
		labels[i] = problem.Label
		header = append(header, problem.Label)
	}

	teamByID := indexTeams(teams)
	orgByID := indexOrganizations(orgs)
	groupByID := indexGroups(groups)

	rows = make([][]string, 0, len(snap.Rows))
	for i, row := range snap.Rows {
		if err := validateProblemLabels(row, labels); err != nil {
			return header, nil, err
		}
		team := teamByID[row.TeamID]
		category := categoryName(team, groupByID)
		if categoryByRow != nil && i < len(categoryByRow) {
			category = categoryByRow[i]
		}
		record := []string{
			strconv.Itoa(row.Rank),
			affiliationName(team, orgByID),
			phase1TeamName(displayName(team)),
			strconv.Itoa(row.Score.NumSolved),
			strconv.Itoa(row.Score.TotalTime),
			category,
		}
		for _, problem := range row.Problems {
			record = append(record, formatScoreCell(problem))
		}
		rows = append(rows, record)
	}

	return header, rows, nil
}

func validateProblemLabels(row model.ScoreboardRow, labels []string) error {
	if len(row.Problems) != len(labels) {
		return fmt.Errorf("%w: row %s has %d problems, want %d", ErrMalformedSnapshot, row.TeamID, len(row.Problems), len(labels))
	}
	for i, problem := range row.Problems {
		if problem.Label != labels[i] {
			return fmt.Errorf("%w: row %s problem %d label %q, want %q", ErrMalformedSnapshot, row.TeamID, i, problem.Label, labels[i])
		}
	}
	return nil
}

func indexTeams(teams []model.Team) map[string]model.Team {
	out := make(map[string]model.Team, len(teams))
	for i := range teams {
		out[teams[i].ID] = teams[i]
	}
	return out
}

func indexOrganizations(orgs []model.Organization) map[string]model.Organization {
	out := make(map[string]model.Organization, len(orgs))
	for i := range orgs {
		out[orgs[i].ID] = orgs[i]
	}
	return out
}

func indexGroups(groups []model.Group) map[string]model.Group {
	out := make(map[string]model.Group, len(groups))
	for i := range groups {
		out[groups[i].ID] = groups[i]
	}
	return out
}

func affiliationName(team model.Team, orgByID map[string]model.Organization) string {
	if team.OrganizationID != nil {
		if org, ok := orgByID[*team.OrganizationID]; ok {
			return org.Name
		}
	}
	if team.Affiliation != nil {
		return *team.Affiliation
	}
	return ""
}

func displayName(team model.Team) string {
	if team.DisplayName != nil && *team.DisplayName != "" {
		return *team.DisplayName
	}
	return team.Name
}

func categoryName(team model.Team, groupByID map[string]model.Group) string {
	if len(team.GroupIDs) == 0 {
		return ""
	}
	group, ok := groupByID[team.GroupIDs[0]]
	if !ok {
		return ""
	}
	return group.Name
}

// phase1TeamName preserves Parse's locked Python quirk: emit only the last
// whitespace-separated token of the rendered team name.
func phase1TeamName(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func formatScoreCell(problem model.ScoreboardProblem) string {
	// tries counts every submission for the problem — judged plus still
	// pending — matching DOMjudge's scoreboard cell count for both solved
	// and unsolved problems. Keeping the count identical across branches
	// avoids under-reporting attempts on a solved problem that still has a
	// pending submission (e.g. during freeze).
	tries := problem.NumJudged + problem.NumPending
	switch {
	case problem.Solved:
		if tries == 0 {
			tries = 1
		}
		if problem.Time != nil {
			return fmt.Sprintf("%d/%s", *problem.Time, triesPhrase(tries))
		}
		return triesPhrase(tries)
	case tries > 0:
		return triesPhrase(tries)
	default:
		return ""
	}
}

func triesPhrase(n int) string {
	if n == 1 {
		return "1 try"
	}
	return fmt.Sprintf("%d tries", n)
}
