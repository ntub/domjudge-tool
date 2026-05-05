package model

// ScoreboardSnapshot maps the JSON response from
// GET /api/v4/contests/{cid}/scoreboard.
type ScoreboardSnapshot struct {
	EventID string          `json:"event_id"`
	Time    string          `json:"time"`
	State   ContestState    `json:"state"`
	Rows    []ScoreboardRow `json:"rows"`
}

// ScoreboardRow maps one team row in a v4 scoreboard snapshot.
type ScoreboardRow struct {
	Rank     int                 `json:"rank"`
	TeamID   string              `json:"team_id"`
	Score    ScoreboardScore     `json:"score"`
	Problems []ScoreboardProblem `json:"problems"`
}

// ScoreboardScore maps the aggregate score object inside a scoreboard row.
type ScoreboardScore struct {
	NumSolved int `json:"num_solved"`
	TotalTime int `json:"total_time"`
}

// ScoreboardProblem maps one problem cell in a v4 scoreboard row.
type ScoreboardProblem struct {
	Label        string `json:"label"`
	ProblemID    string `json:"problem_id"`
	NumJudged    int    `json:"num_judged"`
	NumPending   int    `json:"num_pending"`
	Solved       bool   `json:"solved"`
	FirstToSolve bool   `json:"first_to_solve"`
	Time         *int   `json:"time"`
}
