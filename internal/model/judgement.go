package model

// Judgement maps to `domjudge_tool_cli.models.judgements.Judgement`.
type Judgement struct {
	JudgementTypeID  *string  `json:"judgement_type_id"`
	Judgehost        *string  `json:"judgehost"`
	Valid            bool     `json:"valid"`
	SubmissionID     string   `json:"submission_id"`
	ID               string   `json:"id"`
	EndContestTime   *string  `json:"end_contest_time"`
	EndTime          *string  `json:"end_time"`
	StartContestTime *string  `json:"start_contest_time"`
	StartTime        *string  `json:"start_time"`
	MaxRunTime       *float64 `json:"max_run_time"`
}
