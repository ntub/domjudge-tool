package model

// JudgementType maps to
// `domjudge_tool_cli.models.judgement_types.JudgementType`.
type JudgementType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Penalty bool   `json:"penalty"`
	Solved  bool   `json:"solved"`
}
