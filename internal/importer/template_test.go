package importer_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/importer"
)

// TestTemplateCSV_MatchesGolden locks PHASE1.md DoD #5: the bytes
// written by [importer.TemplateCSV] must round-trip byte-identical to
// the Python reference template at
// `domjudge_tool_cli/templates/csv/import-users-teams.csv`, captured
// here as `testdata/importer/template_python_golden.csv`.
func TestTemplateCSV_MatchesGolden(t *testing.T) {
	goldenPath := filepath.Join("..", "..", "testdata", "importer", "template_python_golden.csv")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	var got bytes.Buffer
	if err := importer.TemplateCSV(&got); err != nil {
		t.Fatalf("TemplateCSV: %v", err)
	}

	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("TemplateCSV output drift\n got: %q\nwant: %q", got.Bytes(), want)
	}
}
