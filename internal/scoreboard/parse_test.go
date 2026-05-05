package scoreboard_test

import (
	"bytes"
	"encoding/csv"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/scoreboard"
)

func TestParse_FixtureGolden(t *testing.T) {
	b, err := os.ReadFile("../../testdata/scoreboard/sample.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	header, rows, err := scoreboard.Parse(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	wantHeader := []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score", "A", "B"}
	if !equalSlice(header, wantHeader) {
		t.Errorf("header = %v, want %v", header, wantHeader)
	}
	wantRows := [][]string{
		{"1", "NTUB", "Alpha", "2", "100", "1 30", "1 70"},
		{"2", "SampleU", "One", "1", "42", "1 42", ""},
		// Summary row: Python locks the "1///2" formatting (split-on-
		// whitespace + slash-join) bit-for-bit. Lock it here too.
		{"", "", "", "3", "", "2///2", "1///2"},
	}
	if len(rows) != len(wantRows) {
		t.Fatalf("got %d rows, want %d", len(rows), len(wantRows))
	}
	for i := range rows {
		if !equalSlice(rows[i], wantRows[i]) {
			t.Errorf("row %d:\n got %v\nwant %v", i, rows[i], wantRows[i])
		}
	}
}

// TestScoreboard_GoldenCSV is the anchor test for PR-7's scoreboard
// CSV output. It locks the byte-identical CSV the Python tool produces
// for the sample fixture (header + 2 team rows + 1 summary row).
func TestScoreboard_GoldenCSV(t *testing.T) {
	b, err := os.ReadFile("../../testdata/scoreboard/sample.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	header, rows, err := scoreboard.Parse(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	cw.UseCRLF = true // Python csv default dialect is "excel" → \r\n.
	if err := cw.Write(header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	want := "Rank,TeamAffiliation,TeamName,SolvedCount,Score,A,B\r\n" +
		"1,NTUB,Alpha,2,100,1 30,1 70\r\n" +
		"2,SampleU,One,1,42,1 42,\r\n" +
		",,,3,,2///2,1///2\r\n"
	if got := buf.String(); got != want {
		t.Errorf("CSV mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestParse_RejectsMissingScoreboardTable(t *testing.T) {
	_, _, err := scoreboard.Parse(strings.NewReader("<html><body><p>nothing here</p></body></html>"))
	if err == nil {
		t.Fatal("expected error for HTML without table.scoreboard, got nil")
	}
	if !errors.Is(err, scoreboard.ErrNoScoreboard) {
		t.Errorf("errors.Is(err, ErrNoScoreboard) = false; err = %v", err)
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
