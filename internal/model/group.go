package model

// Group maps the DOMjudge v4 team category/group JSON shape.
type Group struct {
	ID        string  `json:"id"`
	ICPCID    *string `json:"icpc_id"`
	Name      string  `json:"name"`
	SortOrder int     `json:"sortorder"`
	Color     *string `json:"color"`
	Hidden    bool    `json:"hidden"`
}
