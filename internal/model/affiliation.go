package model

// Affiliation maps to `domjudge_tool_cli.models.affiliation.Affiliation`.
type Affiliation struct {
	ID              *string `json:"id"`
	ShortName       string  `json:"shortname"`
	Name            string  `json:"name"`
	Country         string  `json:"country"`
	TeamAffiliation *string `json:"team_affiliation"`
}
