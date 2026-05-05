package model

// TeamImport is the compact JSON payload accepted by
// POST /api/v4/users/teams. It is separate from Team because the read model
// contains API response-only fields.
type TeamImport struct {
	ID             string   `json:"id"`
	ICPCID         *string  `json:"icpc_id"`
	GroupIDs       []string `json:"group_ids"`
	Name           string   `json:"name"`
	DisplayName    *string  `json:"display_name"`
	OrganizationID *string  `json:"organization_id"`
}
