package importer

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/csvio"
)

// ParseUsersCSV reads one of the supported import CSV layouts from r
// and returns one model.CreateUser per data row.
//
// Supported headers are:
// - username,name,email,affiliation,password
// - username,name,email,affiliation,password,category_id
// - username,email,affiliation,name
// - username,name,email,affiliation,password,is_exist
//
// The parser is BOM-tolerant and field-name driven, so the column
// order can vary as long as the names are present. Empty optional
// cells (email / affiliation / password) become nil pointers. The
// optional is_exist column, when present, is parsed into the returned
// model.CreateUser values. The optional category_id column is consumed
// by ImportUsersTeams as per-row metadata and is not represented in the
// returned model.CreateUser values.
func ParseUsersCSV(r io.Reader) ([]model.CreateUser, error) {
	rows, err := readImportRows(r)
	if err != nil {
		return nil, err
	}
	out := make([]model.CreateUser, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.user)
	}
	return out, nil
}

type importRow struct {
	user       model.CreateUser
	categoryID *int
}

func readImportRows(r io.Reader) ([]importRow, error) {
	cr := csvio.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, wrapCSVReadHeaderErr(err)
	}
	index, err := usersCSVColumns(header)
	if err != nil {
		return nil, err
	}
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("importer: parse csv: %w", err)
	}
	out := make([]importRow, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for i, row := range records {
		ir, err := buildImportRowFromCSV(row, index, i+1)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[ir.user.Username]; dup {
			return nil, fmt.Errorf("importer: validate csv: row %d: duplicate username %q", i+1, ir.user.Username)
		}
		seen[ir.user.Username] = struct{}{}
		out = append(out, ir)
	}
	return out, nil
}

func wrapCSVReadHeaderErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return fmt.Errorf("importer: parse csv: csvio: empty input")
	}
	return fmt.Errorf("importer: parse csv: %w", err)
}

type usersCSVIndex struct {
	username    int
	name        int
	email       int
	affiliation int
	password    int
	isExist     int
	categoryID  int
}

func usersCSVColumns(header []string) (usersCSVIndex, error) {
	const (
		colUsername    = "username"
		colName        = "name"
		colEmail       = "email"
		colAffiliation = "affiliation"
		colPassword    = "password"
		colIsExist     = "is_exist"
		colCategoryID  = "category_id"
	)

	allowed := map[string]struct{}{
		colUsername: {}, colName: {}, colEmail: {}, colAffiliation: {}, colPassword: {}, colIsExist: {}, colCategoryID: {},
	}
	index := usersCSVIndex{
		username:    -1,
		name:        -1,
		email:       -1,
		affiliation: -1,
		password:    -1,
		isExist:     -1,
		categoryID:  -1,
	}
	seen := make(map[string]struct{}, len(header))
	for i, raw := range header {
		name := strings.TrimSpace(raw)
		if _, ok := allowed[name]; !ok {
			return usersCSVIndex{}, fmt.Errorf("%w: unknown column %q", csvio.ErrHeaderMismatch, name)
		}
		if _, dup := seen[name]; dup {
			return usersCSVIndex{}, fmt.Errorf("%w: duplicate column %q", csvio.ErrHeaderMismatch, name)
		}
		seen[name] = struct{}{}
		switch name {
		case colUsername:
			index.username = i
		case colName:
			index.name = i
		case colEmail:
			index.email = i
		case colAffiliation:
			index.affiliation = i
		case colPassword:
			index.password = i
		case colIsExist:
			index.isExist = i
		case colCategoryID:
			index.categoryID = i
		}
	}
	if index.username < 0 || index.name < 0 {
		return usersCSVIndex{}, fmt.Errorf("%w: username/name required", csvio.ErrHeaderMismatch)
	}
	return index, nil
}

func buildImportRowFromCSV(row []string, index usersCSVIndex, rowNum int) (importRow, error) {
	if len(row) != 0 {
		// Accept rows whose length matches the header or omits only the
		// trailing optional fields. A shorter row that drops a middle
		// column is still an error because the field mapping would be
		// ambiguous.
		want := maxIndex(index) + 1
		if len(row) != want {
			return importRow{}, fmt.Errorf("importer: validate csv: row %d has %d columns, want %d", rowNum, len(row), want)
		}
	}

	cu := model.CreateUser{
		Username: valueAt(row, index.username),
		Name:     valueAt(row, index.name),
	}
	if cu.Username == "" {
		return importRow{}, fmt.Errorf("importer: validate csv: row %d: username is required", rowNum)
	}
	if cu.Name == "" {
		return importRow{}, fmt.Errorf("importer: validate csv: row %d: name is required", rowNum)
	}
	if v := valueAt(row, index.email); v != "" {
		cu.Email = stringPtr(v)
	}
	if v := valueAt(row, index.affiliation); v != "" {
		cu.Affiliation = stringPtr(v)
	}
	if v := valueAt(row, index.password); v != "" {
		cu.Password = stringPtr(v)
	}
	if index.isExist >= 0 {
		v := valueAt(row, index.isExist)
		if v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return importRow{}, fmt.Errorf("importer: validate csv: row %d: invalid is_exist %q: %w", rowNum, v, err)
			}
			cu.IsExist = &b
		}
	}
	out := importRow{user: cu}
	if index.categoryID >= 0 {
		v := strings.TrimSpace(valueAt(row, index.categoryID))
		if v != "" {
			categoryID, err := strconv.Atoi(v)
			if err != nil {
				return importRow{}, fmt.Errorf("importer: validate csv: row %d: invalid category_id %q: %w", rowNum, v, err)
			}
			out.categoryID = &categoryID
		}
	}
	return out, nil
}

func maxIndex(index usersCSVIndex) int {
	highest := index.username
	if index.name > highest {
		highest = index.name
	}
	if index.email > highest {
		highest = index.email
	}
	if index.affiliation > highest {
		highest = index.affiliation
	}
	if index.password > highest {
		highest = index.password
	}
	if index.isExist > highest {
		highest = index.isExist
	}
	if index.categoryID > highest {
		highest = index.categoryID
	}
	return highest
}

func valueAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}

func stringPtr(s string) *string { return &s }
