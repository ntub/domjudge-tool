package scoreboard_test

import (
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/scoreboard"
)

func TestBuildProblemCellsMatchParseProblemCells(t *testing.T) {
	t91, t143 := 91, 143
	html := strings.NewReader(`
<table class="scoreboard">
  <thead>
    <tr class="scoreheader">
      <th></th><th></th><th></th><th></th><th></th>
      <th title="problem A"></th>
      <th title="problem B"></th>
      <th title="problem C"></th>
      <th title="problem D"></th>
      <th title="problem E"></th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td class="scorepl">1</td>
      <td class="scoretn"><span class="univ">NTUB</span><span>Team Alpha</span></td>
      <td class="scorenc">2</td>
      <td class="scorett">234</td>
      <td class="score_cell">91 1 try</td>
      <td class="score_cell">143 4 tries</td>
      <td class="score_cell">5 tries</td>
      <td class="score_cell"></td>
      <td class="score_cell">91 2 tries</td>
    </tr>
  </tbody>
</table>`)
	apiSnap := model.ScoreboardSnapshot{Rows: []model.ScoreboardRow{{
		Rank:   1,
		TeamID: "1",
		Score:  model.ScoreboardScore{NumSolved: 2, TotalTime: 234},
		Problems: []model.ScoreboardProblem{
			{Label: "A", ProblemID: "a", Solved: true, NumJudged: 1, Time: &t91},
			{Label: "B", ProblemID: "b", Solved: true, NumJudged: 4, Time: &t143},
			{Label: "C", ProblemID: "c", NumJudged: 4, NumPending: 1},
			{Label: "D", ProblemID: "d"},
			{Label: "E", ProblemID: "e", Solved: true, NumJudged: 1, NumPending: 1, Time: &t91},
		},
	}}}

	_, parsedRows, err := scoreboard.Parse(html)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, builtRows, err := scoreboard.Build(
		apiSnap,
		[]model.Team{{ID: "1", Name: "Team Alpha", GroupIDs: []string{"10"}}},
		nil,
		[]model.Group{{ID: "10", Name: "Main"}},
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(parsedRows) != 1 || len(builtRows) != 1 {
		t.Fatalf("row counts parsed=%d built=%d, want one each", len(parsedRows), len(builtRows))
	}

	parsedProblemCells := parsedRows[0][5:]
	builtProblemCells := builtRows[0][6:]
	assertStringSlice(t, builtProblemCells, parsedProblemCells)
}
