// Package csvio provides BOM-aware CSV helpers.
//
// Excel and many other spreadsheet exporters prepend a UTF-8 byte-order
// mark (the three bytes 0xEF 0xBB 0xBF) to CSV files. The Go
// `encoding/csv` package treats those three bytes as part of the first
// column header, which then fails to match the expected name. The
// helpers here strip a leading BOM, parse against an explicit header
// list, and round-trip records through pluggable column orders.
package csvio

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
)

// utf8BOM is the three-byte UTF-8 encoding of the byte order mark.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// ErrHeaderMismatch is returned when the parsed header row does not
// match the expected column names exactly (order-sensitive).
var ErrHeaderMismatch = errors.New("csvio: header row does not match expected columns")

// StripBOM returns b with a leading UTF-8 BOM removed if present.
// Inputs that don't start with a BOM are returned unchanged so callers
// can use it unconditionally.
func StripBOM(b []byte) []byte {
	if bytes.HasPrefix(b, utf8BOM) {
		return b[len(utf8BOM):]
	}
	return b
}

// NewReader wraps r in a bufio.Reader that transparently consumes a
// leading UTF-8 BOM if present, then returns a csv.Reader configured
// for typical DOMjudge CSV input (comma-delimited, no comment prefix,
// variable field count handled at the row level).
func NewReader(r io.Reader) *csv.Reader {
	br := bufio.NewReader(r)
	if peek, err := br.Peek(len(utf8BOM)); err == nil && bytes.Equal(peek, utf8BOM) {
		_, _ = br.Discard(len(utf8BOM))
	}
	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1
	return cr
}

// ReadAll parses r as CSV and returns the data rows. The first row
// must match wantHeader exactly (column count, names, and order); any
// drift returns [ErrHeaderMismatch] wrapped with the offending column.
func ReadAll(r io.Reader, wantHeader []string) ([][]string, error) {
	cr := NewReader(r)
	header, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("csvio: empty input")
		}
		return nil, fmt.Errorf("csvio: read header: %w", err)
	}
	if len(header) != len(wantHeader) {
		return nil, fmt.Errorf("%w: got %d columns, want %d", ErrHeaderMismatch, len(header), len(wantHeader))
	}
	for i := range header {
		if header[i] != wantHeader[i] {
			return nil, fmt.Errorf("%w: column %d is %q, want %q", ErrHeaderMismatch, i, header[i], wantHeader[i])
		}
	}
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csvio: read rows: %w", err)
	}
	return rows, nil
}

// Write emits the header followed by rows to w using an RFC 4180 CSV
// encoder. The writer is flushed before return; any encoding error is
// returned wrapped.
func Write(w io.Writer, header []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csvio: write header: %w", err)
	}
	for i, row := range rows {
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("csvio: write row %d: %w", i, err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csvio: flush: %w", err)
	}
	return nil
}
