package scoreboard

import (
	"encoding/csv"
	"fmt"
	"io"
)

// WriteCSV encodes header and rows as a CRLF-delimited CSV. The Python
// default dialect uses \r\n; callers must not re-implement this encoder.
func WriteCSV(header []string, rows [][]string, w io.Writer) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("scoreboard: write csv header: %w", err)
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("scoreboard: write csv row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}
