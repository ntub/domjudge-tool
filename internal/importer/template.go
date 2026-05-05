// Package importer provides the import-users-teams use case.
package importer

import (
	"bytes"
	"io"

	"github.com/ntub/domjudge-tool/internal/util/csvio"
)

// csvHeaders is the canonical column order for both the import template
// and the import_users_teams CSV input. Column drift is locked by
// `TestCSVImport_HeadersByteIdentical` and the
// `template_python_golden.csv` fixture.
var csvHeaders = []string{"username", "name", "email", "affiliation", "password"}

// templateSampleRow is the single demonstration row written by
// [TemplateCSV]. The Python reference uses the same values; keep them
// in lock-step so the byte-identical assertion in
// `TestTemplateCSV_MatchesGolden` continues to hold.
var templateSampleRow = []string{"ntubuser", "test", "ntubuser@mail.com", "ntub", "testpassword"}

// TemplateCSV writes the import template CSV to w.
//
// The output must be byte-identical to the Python template at
// domjudge_tool_cli/templates/csv/import-users-teams.csv. That file
// terminates with the last column of the sample row (no trailing
// newline), which the Go csv writer would normally emit; the trailing
// newline is stripped before the bytes hit w.
//
// PHASE1.md DoD #5: ./bin/domjudge-tool import-users-teams-example
// writes a CSV byte-identical to Python templates/csv/import-users-teams.csv.
func TemplateCSV(w io.Writer) error {
	var buf bytes.Buffer
	if err := csvio.Write(&buf, csvHeaders, [][]string{templateSampleRow}); err != nil {
		return err
	}
	_, err := w.Write(bytes.TrimRight(buf.Bytes(), "\n"))
	return err
}
