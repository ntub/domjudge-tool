package importer_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/importer"
)

// TestTemplateCSV_MatchesGolden locks the canonical template emitted by
// import-users-teams-example. The first five columns preserve the Phase
// 1 Python-compatible order; category_id is an optional appended field.
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
