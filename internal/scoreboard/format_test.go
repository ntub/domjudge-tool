package scoreboard

import (
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
)

func TestFormatScoreCell(t *testing.T) {
	t91, t143 := 91, 143
	tests := []struct {
		name    string
		problem model.ScoreboardProblem
		want    string
	}{
		{
			name:    "solved single try",
			problem: model.ScoreboardProblem{Solved: true, NumJudged: 1, Time: &t91},
			want:    "91/1 try",
		},
		{
			name:    "solved multiple tries",
			problem: model.ScoreboardProblem{Solved: true, NumJudged: 4, Time: &t143},
			want:    "143/4 tries",
		},
		{
			name:    "solved with pending counts pending as a try",
			problem: model.ScoreboardProblem{Solved: true, NumJudged: 1, NumPending: 1, Time: &t91},
			want:    "91/2 tries",
		},
		{
			name:    "tried unsolved single judged",
			problem: model.ScoreboardProblem{NumJudged: 1},
			want:    "1 try",
		},
		{
			name:    "tried unsolved judged and pending",
			problem: model.ScoreboardProblem{NumJudged: 4, NumPending: 1},
			want:    "5 tries",
		},
		{
			name:    "untried",
			problem: model.ScoreboardProblem{},
			want:    "",
		},
		{
			name:    "solved nil time",
			problem: model.ScoreboardProblem{Solved: true, NumJudged: 2},
			want:    "2 tries",
		},
		{
			name:    "solved zero judged uses one try",
			problem: model.ScoreboardProblem{Solved: true, Time: &t91},
			want:    "91/1 try",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatScoreCell(tc.problem); got != tc.want {
				t.Fatalf("formatScoreCell() = %q, want %q", got, tc.want)
			}
		})
	}
}
