package scoreboard_test

import (
	"errors"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/scoreboard"
)

func TestBuild_HappyPathMatchesPhase1Shape(t *testing.T) {
	t30, t70, t42 := 30, 70, 42
	snap := model.ScoreboardSnapshot{Rows: []model.ScoreboardRow{
		{
			Rank:   1,
			TeamID: "1",
			Score:  model.ScoreboardScore{NumSolved: 2, TotalTime: 100},
			Problems: []model.ScoreboardProblem{
				{Label: "A", ProblemID: "a", NumJudged: 1, Solved: true, Time: &t30},
				{Label: "B", ProblemID: "b", NumJudged: 1, Solved: true, Time: &t70},
			},
		},
		{
			Rank:   2,
			TeamID: "2",
			Score:  model.ScoreboardScore{NumSolved: 1, TotalTime: 42},
			Problems: []model.ScoreboardProblem{
				{Label: "A", ProblemID: "a", NumJudged: 1, Solved: true, Time: &t42},
				{Label: "B", ProblemID: "b"},
			},
		},
	}}
	teams := []model.Team{
		{ID: "1", Name: "Team Alpha", OrganizationID: strPtr("1")},
		{ID: "2", Name: "Team Beta One", OrganizationID: strPtr("2")},
	}
	orgs := []model.Organization{
		{ID: "1", Name: "NTUB"},
		{ID: "2", Name: "SampleU"},
	}

	header, rows, err := scoreboard.Build(snap, teams, orgs)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	wantHeader := []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score", "A", "B"}
	assertStringSlice(t, header, wantHeader)
	wantRows := [][]string{
		{"1", "NTUB", "Alpha", "2", "100", "1 30", "1 70"},
		{"2", "SampleU", "One", "1", "42", "1 42", ""},
		{"", "", "", "3", "", "2///2", "1///2"},
	}
	if len(rows) != len(wantRows) {
		t.Fatalf("rows = %d, want %d: %v", len(rows), len(wantRows), rows)
	}
	for i := range rows {
		assertStringSlice(t, rows[i], wantRows[i])
	}
}

func TestBuild_EmptyScoreboard_HeaderOnly(t *testing.T) {
	header, rows, err := scoreboard.Build(model.ScoreboardSnapshot{}, nil, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertStringSlice(t, header, []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score"})
	if rows != nil {
		t.Fatalf("rows = %v, want nil", rows)
	}
}

func TestBuild_TeamMissingFromMap_FallsBackToEmptyName(t *testing.T) {
	snap := model.ScoreboardSnapshot{Rows: []model.ScoreboardRow{{
		Rank: 1, TeamID: "missing", Score: model.ScoreboardScore{}, Problems: []model.ScoreboardProblem{{Label: "A"}},
	}}}
	_, rows, err := scoreboard.Build(snap, nil, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertStringSlice(t, rows[0], []string{"1", "", "", "0", "0", ""})
}

func TestBuild_OrgMissing_FallsBackToTeamAffiliation(t *testing.T) {
	snap := model.ScoreboardSnapshot{Rows: []model.ScoreboardRow{{
		Rank: 1, TeamID: "1", Score: model.ScoreboardScore{}, Problems: []model.ScoreboardProblem{{Label: "A"}},
	}}}
	teams := []model.Team{{ID: "1", Name: "Team Alpha", OrganizationID: strPtr("404"), Affiliation: strPtr("FallbackU")}}
	_, rows, err := scoreboard.Build(snap, teams, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if rows[0][1] != "FallbackU" {
		t.Fatalf("affiliation = %q, want FallbackU", rows[0][1])
	}
}

func TestBuild_MalformedRow_ReturnsErr(t *testing.T) {
	snap := model.ScoreboardSnapshot{Rows: []model.ScoreboardRow{
		{TeamID: "1", Problems: []model.ScoreboardProblem{{Label: "A"}}},
		{TeamID: "2", Problems: []model.ScoreboardProblem{{Label: "B"}}},
	}}
	_, _, err := scoreboard.Build(snap, nil, nil)
	if !errors.Is(err, scoreboard.ErrMalformedSnapshot) {
		t.Fatalf("err = %v, want ErrMalformedSnapshot", err)
	}
}

func strPtr(s string) *string { return &s }

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice length = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q\ngot:  %v\nwant: %v", i, got[i], want[i], got, want)
		}
	}
}
