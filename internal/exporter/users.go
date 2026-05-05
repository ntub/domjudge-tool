// Package exporter serialises user data to CSV or JSON.
package exporter

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/csvio"
)

// Format selects the on-disk shape of an export.
type Format string

const (
	FormatCSV  Format = "csv"
	FormatJSON Format = "json"
)

// usersCSVHeader matches model.User field declaration order so that CSV
// exports round-trip through the importer without column reshuffling.
var usersCSVHeader = []string{
	"last_login_time", "first_login_time", "team", "roles",
	"id", "username", "name", "email", "last_ip", "ip",
	"enabled", "team_id", "affiliation", "password",
}

// createUsersCSVHeader pins the column order for CreateUser exports.
// Note this differs from model.CreateUser field declaration order:
// affiliation precedes password so the leading five columns match the
// importer template (templates/csv/import-users-teams.csv from the
// Python 0.3.0 CLI). The trailing is_exist column carries the
// idempotency flag for re-imports.
var createUsersCSVHeader = []string{
	"username", "name", "email", "affiliation", "password", "is_exist",
}

// Users marshals users to w in the given format.
//
// CSV: header row matches model.User field declaration order
// (last_login_time, first_login_time, team, roles, id, username, name,
// email, last_ip, ip, enabled, team_id, affiliation, password).
// Pointer fields render as the pointed-to value or empty string for nil.
// Slice fields (roles) render as a JSON array string (e.g. `["team"]`)
// so that the output is machine-parseable.
// JSON: valid JSON array using json.Marshal field order; empty slice → "[]" not "null".
// Unknown format → non-nil error.
func Users(users []model.User, format Format, w io.Writer) error {
	return exportTable(users, format, w, usersCSVHeader, userToCSVRow)
}

// CreateUsers marshals createUsers to w in the given format.
//
// CSV: header row is username, name, email, affiliation, password, is_exist.
// Note this is NOT model.CreateUser field declaration order — affiliation
// precedes password so the leading five columns match the importer
// template; is_exist carries the idempotency flag.
// JSON: valid JSON array; empty slice → "[]".
func CreateUsers(createUsers []model.CreateUser, format Format, w io.Writer) error {
	return exportTable(createUsers, format, w, createUsersCSVHeader, createUserToCSVRowOK)
}

// exportTable is the shared dispatch for the two table-shaped exports.
// CSV path renders each item via toRow and writes the configured header;
// JSON path delegates to writeJSONArray so a nil slice surfaces as `[]`.
// Unknown formats return a wrapped error naming the offending value.
func exportTable[T any](
	items []T,
	format Format,
	w io.Writer,
	header []string,
	toRow func(T) ([]string, error),
) error {
	switch format {
	case FormatCSV:
		rows := make([][]string, 0, len(items))
		for _, it := range items {
			row, err := toRow(it)
			if err != nil {
				return err
			}
			rows = append(rows, row)
		}
		return csvio.Write(w, header, rows)
	case FormatJSON:
		return writeJSONArray(w, items)
	default:
		return fmt.Errorf("exporter: unknown format %q", format)
	}
}

// userToCSVRow renders a single User into the column order declared by
// usersCSVHeader. The roles slice is JSON-encoded so an empty slice
// surfaces as `[]` rather than the JSON `null` that `[]string(nil)`
// would produce.
func userToCSVRow(u model.User) ([]string, error) {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	rolesJSON, err := json.Marshal(roles)
	if err != nil {
		return nil, fmt.Errorf("exporter: marshal roles: %w", err)
	}
	return []string{
		strOrEmpty(u.LastLoginTime),
		strOrEmpty(u.FirstLoginTime),
		strOrEmpty(u.Team),
		string(rolesJSON),
		u.ID,
		u.Username,
		u.Name,
		strOrEmpty(u.Email),
		strOrEmpty(u.LastIP),
		strOrEmpty(u.IP),
		strconv.FormatBool(u.Enabled),
		strOrEmpty(u.TeamID),
		strOrEmpty(u.Affiliation),
		strOrEmpty(u.Password),
	}, nil
}

// createUserToCSVRowOK adapts createUserToCSVRow to the toRow signature
// expected by exportTable. CreateUser rendering can't fail (no JSON
// marshalling), so the returned error is always nil.
func createUserToCSVRowOK(c model.CreateUser) ([]string, error) {
	return createUserToCSVRow(c), nil
}

// createUserToCSVRow renders a single CreateUser into the column order
// declared by createUsersCSVHeader. Pointer fields render as the
// pointed-to value or empty string for nil; the *bool IsExist field
// renders as "true"/"false" or empty when nil.
func createUserToCSVRow(c model.CreateUser) []string {
	return []string{
		c.Username,
		c.Name,
		strOrEmpty(c.Email),
		strOrEmpty(c.Affiliation),
		strOrEmpty(c.Password),
		boolPtrOrEmpty(c.IsExist),
	}
}

// writeJSONArray marshals slice as a JSON array to w. A nil slice is
// rendered as the empty array `[]` rather than the JSON `null` that
// json.Marshal would otherwise emit for `[]T(nil)`.
func writeJSONArray[T any](w io.Writer, slice []T) error {
	if slice == nil {
		slice = []T{}
	}
	b, err := json.Marshal(slice)
	if err != nil {
		return fmt.Errorf("exporter: marshal: %w", err)
	}
	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("exporter: write: %w", err)
	}
	return nil
}

// strOrEmpty returns the pointed-to value, or empty string for nil.
func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// boolPtrOrEmpty returns "true"/"false" for the pointed-to value, or
// empty string for nil.
func boolPtrOrEmpty(p *bool) string {
	if p == nil {
		return ""
	}
	return strconv.FormatBool(*p)
}
