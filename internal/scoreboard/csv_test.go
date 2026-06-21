package scoreboard

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteCSV_UsesCRLF(t *testing.T) {
	header := []string{"rank", "team", "score"}
	rows := [][]string{
		{"1", "Team A", "100"},
		{"2", "Team B", "80"},
	}
	var buf bytes.Buffer
	if err := WriteCSV(header, rows, &buf); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "\r\n") {
		t.Error("output should use CRLF line endings")
	}
	// Split on CRLF and verify 3 lines (header + 2 rows).
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\r\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (header+2 rows), got %d: %q", len(lines), out)
	}
}

func TestWriteCSV_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV([]string{"a", "b"}, nil, &buf); err != nil {
		t.Fatalf("WriteCSV empty rows: %v", err)
	}
	out := buf.String()
	if out != "a,b\r\n" {
		t.Errorf("expected %q, got %q", "a,b\r\n", out)
	}
}
