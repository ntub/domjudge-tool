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
// lookup data into the Phase 1 CSV shape produced by Parse.
func Build(snap model.ScoreboardSnapshot, teams []model.Team, orgs []model.Organization) (header []string, rows [][]string, err error) {
	header = append([]string{}, fixedHeader...)
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
	solvedByProblem := make([]int, len(labels))
	totalSolved := 0

	rows = make([][]string, 0, len(snap.Rows)+1)
	for _, row := range snap.Rows {
		if err := validateProblemLabels(row, labels); err != nil {
			return header, nil, err
		}
		team := teamByID[row.TeamID]
		record := []string{
			strconv.Itoa(row.Rank),
			affiliationName(team, orgByID),
			phase1TeamName(displayName(team)),
			strconv.Itoa(row.Score.NumSolved),
			strconv.Itoa(row.Score.TotalTime),
		}
		totalSolved += row.Score.NumSolved
		for i, problem := range row.Problems {
			if problem.Solved {
				solvedByProblem[i]++
			}
			record = append(record, formatScoreCell(problem))
		}
		rows = append(rows, record)
	}

	rows = append(rows, buildSummaryRow(totalSolved, solvedByProblem, len(snap.Rows)))
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
	switch {
	case problem.Solved:
		judged := problem.NumJudged
		if judged == 0 {
			judged = 1
		}
		if problem.Time != nil {
			return fmt.Sprintf("%d %d", judged, *problem.Time)
		}
		return strconv.Itoa(judged)
	case problem.NumPending > 0:
		if problem.NumJudged > 0 {
			return fmt.Sprintf("%d/%d", problem.NumJudged, problem.NumPending)
		}
		return "?" + strconv.Itoa(problem.NumPending)
	case problem.NumJudged > 0:
		return strconv.Itoa(problem.NumJudged)
	default:
		return ""
	}
}

func buildSummaryRow(totalSolved int, solvedByProblem []int, teamCount int) []string {
	row := []string{"", "", "", strconv.Itoa(totalSolved), ""}
	for _, solved := range solvedByProblem {
		row = append(row, fmt.Sprintf("%d///%d", solved, teamCount))
	}
	return row
}
