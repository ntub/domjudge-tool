package model

// Problem maps to `domjudge_tool_cli.models.problem.Problem` (the JSON
// representation returned by the v4 API problems list).
type Problem struct {
	Ordinal       int     `json:"ordinal"`
	ID            string  `json:"id"`
	ShortName     string  `json:"short_name"`
	Label         string  `json:"label"`
	TimeLimit     int     `json:"time_limit"`
	ExternalID    string  `json:"externalid"`
	Name          string  `json:"name"`
	RGB           *string `json:"rgb"`
	Color         *string `json:"color"`
	TestDataCount int     `json:"test_data_count"`
}

// ProblemItem maps to `domjudge_tool_cli.models.problem.ProblemItem`,
// the lighter-weight shape used when scraping the jury problems page.
type ProblemItem struct {
	ID             string  `json:"id"`
	TimeLimit      int     `json:"time_limit"`
	TestDataCount  int     `json:"test_data_count"`
	Name           string  `json:"name"`
	ExportFilePath *string `json:"export_file_path"`
}
