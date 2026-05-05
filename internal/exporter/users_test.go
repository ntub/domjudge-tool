package exporter_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/exporter"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/csvio"
)

func strPtr(s string) *string { return &s }

// usersCSVHeader locks the CSV column order for User exports. It must
// match the model.User field declaration order verbatim so the Python
// 0.3.0 CLI's outputs round-trip byte-identically when re-imported.
var usersCSVHeader = []string{
	"last_login_time", "first_login_time", "team", "roles",
	"id", "username", "name", "email", "last_ip", "ip",
	"enabled", "team_id", "affiliation", "password",
}

func TestUsers_CSV_RoundTrips(t *testing.T) {
	users := []model.User{
		{
			LastLoginTime:  strPtr("2024-01-02T03:04:05+00:00"),
			FirstLoginTime: nil,
			Team:           strPtr("teamA"),
			Roles:          []string{"team"},
			ID:             "1",
			Username:       "ntubuser",
			Name:           "NTUB User",
			Email:          strPtr("ntubuser@mail.com"),
			LastIP:         strPtr("10.0.0.1"),
			IP:             nil,
			Enabled:        true,
			TeamID:         strPtr("42"),
			Affiliation:    strPtr("ntub"),
			Password:       strPtr("secret"),
		},
		{
			// All optional fields nil, empty roles slice — verifies pointer
			// fields render as empty strings and an empty roles slice
			// renders as `[]`, not `null`.
			Roles:    []string{},
			ID:       "2",
			Username: "minimal",
			Name:     "",
			Enabled:  false,
		},
	}

	var buf bytes.Buffer
	if err := exporter.Users(users, exporter.FormatCSV, &buf); err != nil {
		t.Fatalf("Users CSV: %v", err)
	}

	rows, err := csvio.ReadAll(&buf, usersCSVHeader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}

	row0 := rows[0]
	wantRow0 := []string{
		"2024-01-02T03:04:05+00:00", "", "teamA", `["team"]`,
		"1", "ntubuser", "NTUB User", "ntubuser@mail.com", "10.0.0.1", "",
		"true", "42", "ntub", "secret",
	}
	if !reflect.DeepEqual(row0, wantRow0) {
		t.Fatalf("row0 mismatch\n got %q\nwant %q", row0, wantRow0)
	}

	row1 := rows[1]
	wantRow1 := []string{
		"", "", "", `[]`,
		"2", "minimal", "", "", "", "",
		"false", "", "", "",
	}
	if !reflect.DeepEqual(row1, wantRow1) {
		t.Fatalf("row1 mismatch\n got %q\nwant %q", row1, wantRow1)
	}
}

func TestUsers_JSON_RoundTrips(t *testing.T) {
	users := []model.User{
		{
			Roles:    []string{"admin", "team"},
			ID:       "1",
			Username: "ntubuser",
			Name:     "NTUB User",
			Email:    strPtr("ntubuser@mail.com"),
			Enabled:  true,
		},
	}

	var buf bytes.Buffer
	if err := exporter.Users(users, exporter.FormatJSON, &buf); err != nil {
		t.Fatalf("Users JSON: %v", err)
	}

	var got []model.User
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, buf.String())
	}
	if !reflect.DeepEqual(got, users) {
		t.Fatalf("round-trip mismatch\n got %#v\nwant %#v", got, users)
	}
}

func TestUsers_CSV_EmptySlice_WritesHeaderOnly(t *testing.T) {
	var buf bytes.Buffer
	if err := exporter.Users(nil, exporter.FormatCSV, &buf); err != nil {
		t.Fatalf("Users CSV nil: %v", err)
	}
	got := buf.String()
	want := strings.Join(usersCSVHeader, ",") + "\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUsers_JSON_EmptySlice_WritesEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := exporter.Users(nil, exporter.FormatJSON, &buf); err != nil {
		t.Fatalf("Users JSON nil: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "[]" {
		t.Fatalf("got %q, want %q", got, "[]")
	}
}

func TestUsers_UnknownFormat_ReturnsErr(t *testing.T) {
	var buf bytes.Buffer
	err := exporter.Users(nil, exporter.Format("xlsx"), &buf)
	if err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
}

// createUsersCSVHeader pins the CSV column order expected by
// CreateUsers exports. Note the order is username,name,email,
// affiliation,password,is_exist — affiliation comes BEFORE password,
// which is NOT the model.CreateUser field declaration order. The
// header instead matches the importer template (see csvio_test.go's
// importHeader) plus an is_exist column for round-tripping.
var createUsersCSVHeader = []string{
	"username", "name", "email", "affiliation", "password", "is_exist",
}

func boolPtr(b bool) *bool { return &b }

func TestCreateUsers_CSV_RoundTrips(t *testing.T) {
	in := []model.CreateUser{
		{
			Username:    "ntubuser",
			Name:        "NTUB User",
			Email:       strPtr("ntubuser@mail.com"),
			Password:    strPtr("secret"),
			Affiliation: strPtr("ntub"),
			IsExist:     boolPtr(true),
		},
		{
			// All optional fields nil — verifies pointer rendering.
			Username: "minimal",
			Name:     "min",
		},
		{
			// IsExist false → "false".
			Username: "absent",
			Name:     "absent user",
			IsExist:  boolPtr(false),
		},
	}

	var buf bytes.Buffer
	if err := exporter.CreateUsers(in, exporter.FormatCSV, &buf); err != nil {
		t.Fatalf("CreateUsers CSV: %v", err)
	}

	rows, err := csvio.ReadAll(&buf, createUsersCSVHeader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}

	wantRow0 := []string{"ntubuser", "NTUB User", "ntubuser@mail.com", "ntub", "secret", "true"}
	if !reflect.DeepEqual(rows[0], wantRow0) {
		t.Fatalf("row0 mismatch\n got %q\nwant %q", rows[0], wantRow0)
	}
	wantRow1 := []string{"minimal", "min", "", "", "", ""}
	if !reflect.DeepEqual(rows[1], wantRow1) {
		t.Fatalf("row1 mismatch\n got %q\nwant %q", rows[1], wantRow1)
	}
	wantRow2 := []string{"absent", "absent user", "", "", "", "false"}
	if !reflect.DeepEqual(rows[2], wantRow2) {
		t.Fatalf("row2 mismatch\n got %q\nwant %q", rows[2], wantRow2)
	}
}

func TestCreateUsers_JSON_RoundTrips(t *testing.T) {
	in := []model.CreateUser{
		{
			Username:    "ntubuser",
			Name:        "NTUB User",
			Email:       strPtr("ntubuser@mail.com"),
			Password:    strPtr("secret"),
			Affiliation: strPtr("ntub"),
			IsExist:     boolPtr(true),
		},
	}

	var buf bytes.Buffer
	if err := exporter.CreateUsers(in, exporter.FormatJSON, &buf); err != nil {
		t.Fatalf("CreateUsers JSON: %v", err)
	}

	var got []model.CreateUser
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, buf.String())
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round-trip mismatch\n got %#v\nwant %#v", got, in)
	}
}

func TestCreateUsers_CSV_EmptySlice_WritesHeaderOnly(t *testing.T) {
	var buf bytes.Buffer
	if err := exporter.CreateUsers(nil, exporter.FormatCSV, &buf); err != nil {
		t.Fatalf("CreateUsers CSV nil: %v", err)
	}
	got := buf.String()
	want := strings.Join(createUsersCSVHeader, ",") + "\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCreateUsers_JSON_EmptySlice_WritesEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := exporter.CreateUsers(nil, exporter.FormatJSON, &buf); err != nil {
		t.Fatalf("CreateUsers JSON nil: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "[]" {
		t.Fatalf("got %q, want %q", got, "[]")
	}
}
