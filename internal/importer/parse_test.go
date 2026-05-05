package importer_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/util/csvio"
)

func TestParseUsersCSV_GoldenRoundTrip(t *testing.T) {
	b, err := os.ReadFile("../../testdata/importer/template_python_golden.csv")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	users, err := importer.ParseUsersCSV(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("ParseUsersCSV: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}
	u := users[0]
	if u.Username != "ntubuser" {
		t.Errorf("Username = %q, want ntubuser", u.Username)
	}
	if u.Name != "test" {
		t.Errorf("Name = %q, want test", u.Name)
	}
	if u.Email == nil || *u.Email != "ntubuser@mail.com" {
		t.Errorf("Email = %v, want ntubuser@mail.com", u.Email)
	}
	if u.Affiliation == nil || *u.Affiliation != "ntub" {
		t.Errorf("Affiliation = %v, want ntub", u.Affiliation)
	}
	if u.Password == nil || *u.Password != "testpassword" {
		t.Errorf("Password = %v, want testpassword", u.Password)
	}
}

func TestParseUsersCSV_SourceCSVShape(t *testing.T) {
	in := strings.NewReader("username,email,affiliation,name\nfoo,foo@example.org,NTUB,Foo Bar\n")
	users, err := importer.ParseUsersCSV(in)
	if err != nil {
		t.Fatalf("ParseUsersCSV: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}
	u := users[0]
	if u.Username != "foo" || u.Name != "Foo Bar" {
		t.Fatalf("parsed user = %+v", u)
	}
	if u.Email == nil || *u.Email != "foo@example.org" {
		t.Fatalf("Email = %v, want foo@example.org", u.Email)
	}
	if u.Affiliation == nil || *u.Affiliation != "NTUB" {
		t.Fatalf("Affiliation = %v, want NTUB", u.Affiliation)
	}
	if u.Password != nil {
		t.Fatalf("Password = %v, want nil", u.Password)
	}
}

func TestParseUsersCSV_OutputCSVShapeWithIsExist(t *testing.T) {
	in := strings.NewReader("username,name,email,affiliation,password,is_exist\nfoo,Foo,foo@example.org,NTUB,pw,true\n")
	users, err := importer.ParseUsersCSV(in)
	if err != nil {
		t.Fatalf("ParseUsersCSV: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}
	if users[0].IsExist == nil || !*users[0].IsExist {
		t.Fatalf("IsExist = %v, want true", users[0].IsExist)
	}
}

func TestParseUsersCSV_StripsBOM(t *testing.T) {
	b, err := os.ReadFile("../../testdata/importer/with_bom.csv")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	users, err := importer.ParseUsersCSV(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("ParseUsersCSV: %v", err)
	}
	if len(users) != 1 || users[0].Username != "ntubuser" {
		t.Fatalf("BOM not stripped: got %+v", users)
	}
}

func TestParseUsersCSV_RejectsHeaderMismatch(t *testing.T) {
	in := strings.NewReader("user,name,email,affiliation,password\nfoo,bar,,,\n")
	_, err := importer.ParseUsersCSV(in)
	if err == nil {
		t.Fatal("expected error on header mismatch, got nil")
	}
	if !errors.Is(err, csvio.ErrHeaderMismatch) {
		t.Errorf("errors.Is(err, csvio.ErrHeaderMismatch) = false; err = %v", err)
	}
}

func TestParseUsersCSV_PreservesNilOptionals(t *testing.T) {
	// Empty email/affiliation/password cells map to nil pointers, not
	// to *string("") — the rest of the importer pipeline assumes
	// nil-vs-non-nil semantics for optional fields.
	in := strings.NewReader("username,name,email,affiliation,password\nfoo,bar,,,\n")
	users, err := importer.ParseUsersCSV(in)
	if err != nil {
		t.Fatalf("ParseUsersCSV: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("want 1 user, got %d", len(users))
	}
	u := users[0]
	if u.Email != nil {
		t.Errorf("Email = %v, want nil", *u.Email)
	}
	if u.Affiliation != nil {
		t.Errorf("Affiliation = %v, want nil", *u.Affiliation)
	}
	if u.Password != nil {
		t.Errorf("Password = %v, want nil", *u.Password)
	}
}

func TestParseUsersCSV_RejectsMalformedRow(t *testing.T) {
	// validateRows enforces non-empty username + name; the CLI for
	// send-user-accounts depends on this so it can crash early on
	// operator typos rather than half-way through SMTP traffic.
	in := strings.NewReader("username,name,email,affiliation,password\n,bar,,,\n")
	_, err := importer.ParseUsersCSV(in)
	if err == nil {
		t.Fatal("expected error on empty username, got nil")
	}
}
