package model

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

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

// UnmarshalJSON accepts DOMjudge 7 integer time_limit values and DOMjudge 8
// integral floats such as 5.0 while keeping the domain model as an int.
func (p *Problem) UnmarshalJSON(data []byte) error {
	type problemAlias Problem
	var alias problemAlias
	aux := struct {
		TimeLimit json.RawMessage `json:"time_limit"`
		*problemAlias
	}{
		problemAlias: &alias,
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*p = Problem(alias)
	if aux.TimeLimit == nil {
		return nil
	}
	timeLimit, err := decodeIntegralJSONNumber(aux.TimeLimit, "time_limit")
	if err != nil {
		return err
	}
	p.TimeLimit = timeLimit
	return nil
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

// UnmarshalJSON applies the same time_limit tolerance as Problem. ProblemItem
// is scraper-facing today, but it carries the same field and should not
// silently truncate a future integral-float JSON value.
func (p *ProblemItem) UnmarshalJSON(data []byte) error {
	type problemItemAlias ProblemItem
	var alias problemItemAlias
	aux := struct {
		TimeLimit json.RawMessage `json:"time_limit"`
		*problemItemAlias
	}{
		problemItemAlias: &alias,
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*p = ProblemItem(alias)
	if aux.TimeLimit == nil {
		return nil
	}
	timeLimit, err := decodeIntegralJSONNumber(aux.TimeLimit, "time_limit")
	if err != nil {
		return err
	}
	p.TimeLimit = timeLimit
	return nil
}

func decodeIntegralJSONNumber(raw []byte, field string) (int, error) {
	s := strings.TrimSpace(string(raw))
	if s == "null" {
		return 0, nil
	}
	if n, err := strconv.ParseInt(s, 10, 0); err == nil {
		return int(n), nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a JSON number: %w", field, err)
	}
	if math.Trunc(f) != f {
		return 0, fmt.Errorf("%s must be integral, got %s", field, s)
	}
	if f < float64(math.MinInt) || f > float64(math.MaxInt) {
		return 0, fmt.Errorf("%s out of int range: %s", field, s)
	}
	return int(f), nil
}
