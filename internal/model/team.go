package model

// Team maps to `domjudge_tool_cli.models.team.Team`.
type Team struct {
	GroupIDs       []string `json:"group_ids"`
	Affiliation    *string  `json:"affiliation"`
	Nationality    *string  `json:"nationality"`
	ID             string   `json:"id"`
	ICPCID         *string  `json:"icpc_id"`
	Name           string   `json:"name"`
	DisplayName    *string  `json:"display_name"`
	OrganizationID *string  `json:"organization_id"`
	Members        *string  `json:"members"`
}
