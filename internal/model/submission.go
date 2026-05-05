package model

// Submission maps to `domjudge_tool_cli.models.submission.Submission`.
//
// `Files` is a list of free-form string→string maps because the v4 API
// returns objects whose keys vary by endpoint (`{"href", "mime"}` for
// metadata, etc.) — the Python reference uses `List[Dict[str, str]]`.
type Submission struct {
	LanguageID   *string             `json:"language_id"`
	Time         *string             `json:"time"`
	ContestTime  *string             `json:"contest_time"`
	ID           string              `json:"id"`
	ExternalID   *string             `json:"externalid"`
	TeamID       string              `json:"team_id"`
	ProblemID    string              `json:"problem_id"`
	EntryPoint   *string             `json:"entry_point"`
	Files        []map[string]string `json:"files"`
	SubmissionID *string             `json:"submission_id"`
	Filename     *string             `json:"filename"`
	Source       *string             `json:"source"`
}

// SubmissionFile maps to
// `domjudge_tool_cli.models.submission.SubmissionFile`.
type SubmissionFile struct {
	ID           string  `json:"id"`
	SubmissionID *string `json:"submission_id"`
	Filename     *string `json:"filename"`
	Source       *string `json:"source"`
}
