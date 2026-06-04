// Package importer provides the import-users-teams use case.
package importer

import (
	"bytes"
	"io"

	"github.com/ntub/domjudge-tool/internal/util/csvio"
)

// csvHeaders is the canonical column order for the import template.
// The first five columns keep the Phase 1 Python-compatible order;
// category_id is an optional per-row override appended for operators
// that need mixed team categories in one CSV.
var csvHeaders = []string{"username", "name", "email", "affiliation", "password", "category_id"}

// templateSampleRow is the single demonstration row written by
// [TemplateCSV]. Keep it in lock-step with
// `testdata/importer/template_python_golden.csv`.
var templateSampleRow = []string{"ntubuser", "test", "ntubuser@mail.com", "ntub", "testpassword", ""}

// TemplateCSV writes the import template CSV to w.
//
// The output terminates with the last column of the sample row (no
// trailing newline), preserving the Phase 1 CLI behaviour.
func TemplateCSV(w io.Writer) error {
	var buf bytes.Buffer
	if err := csvio.Write(&buf, csvHeaders, [][]string{templateSampleRow}); err != nil {
		return err
	}
	_, err := w.Write(bytes.TrimRight(buf.Bytes(), "\n"))
	return err
}
