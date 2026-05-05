package v4

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
)

func TestImports_GroupsTSV_HappyPath(t *testing.T) {
	const body = "id\tname\n1\tParticipants\n"
	srv := importServer(t, "/api/v4/users/groups", func(t *testing.T, r *http.Request) {
		got := readMultipartFile(t, r, "tsv")
		if got != body {
			t.Fatalf("tsv body = %q, want %q", got, body)
		}
	}, `["1"]`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	ids, err := c.Imports.GroupsTSV(context.Background(), strings.NewReader(body))
	if err != nil {
		t.Fatalf("GroupsTSV: %v", err)
	}
	if !reflect.DeepEqual(ids, []string{"1"}) {
		t.Fatalf("ids = %v, want [1]", ids)
	}
}

func TestImports_GroupsTSV_RejectedReturnsErrImportRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/users/groups" {
			t.Fatalf("path = %q, want /api/v4/users/groups", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`["line 2: missing name"]`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	_, err := c.Imports.GroupsTSV(context.Background(), strings.NewReader("bad"))
	if !errors.Is(err, ErrImportRejected) {
		t.Fatalf("err = %v, want ErrImportRejected", err)
	}
	var rejected *ImportRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("err = %v, want ImportRejectedError", err)
	}
	if !reflect.DeepEqual(rejected.Details, []string{"line 2: missing name"}) {
		t.Fatalf("Details = %v, want validation message", rejected.Details)
	}
}

func TestImports_OrganizationsJSON_HappyPath(t *testing.T) {
	want := []model.Organization{{
		ID:        "1",
		ShortName: "NTUB",
		Name:      "Sample University",
		Country:   "TWN",
	}}
	srv := importServer(t, "/api/v4/users/organizations", func(t *testing.T, r *http.Request) {
		var got []model.Organization
		decodeMultipartJSON(t, r, "json", &got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("organizations = %+v, want %+v", got, want)
		}
	}, `["1"]`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	ids, err := c.Imports.Organizations(context.Background(), want)
	if err != nil {
		t.Fatalf("Organizations: %v", err)
	}
	if !reflect.DeepEqual(ids, []string{"1"}) {
		t.Fatalf("ids = %v, want [1]", ids)
	}
}

func TestImports_TeamsTSV_HappyPath(t *testing.T) {
	const body = "teams\t1\n1\tteam-1\t1\tTeam One\tTeam One\t1\n"
	srv := importServer(t, "/api/v4/users/teams", func(t *testing.T, r *http.Request) {
		got := readMultipartFile(t, r, "tsv")
		if got != body {
			t.Fatalf("tsv body = %q, want %q", got, body)
		}
	}, `["1"]`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	ids, err := c.Imports.TeamsTSV(context.Background(), strings.NewReader(body))
	if err != nil {
		t.Fatalf("TeamsTSV: %v", err)
	}
	if !reflect.DeepEqual(ids, []string{"1"}) {
		t.Fatalf("ids = %v, want [1]", ids)
	}
}

func TestImports_TeamsJSON_HappyPath(t *testing.T) {
	icpcID := "team-1"
	displayName := "Team One"
	orgID := "1"
	want := []model.TeamImport{{
		ID:             "1",
		ICPCID:         &icpcID,
		GroupIDs:       []string{"2"},
		Name:           "team-one",
		DisplayName:    &displayName,
		OrganizationID: &orgID,
	}}
	srv := importServer(t, "/api/v4/users/teams", func(t *testing.T, r *http.Request) {
		var got []model.TeamImport
		decodeMultipartJSON(t, r, "json", &got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("teams = %+v, want %+v", got, want)
		}
	}, `["1"]`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	ids, err := c.Imports.TeamsJSON(context.Background(), want)
	if err != nil {
		t.Fatalf("TeamsJSON: %v", err)
	}
	if !reflect.DeepEqual(ids, []string{"1"}) {
		t.Fatalf("ids = %v, want [1]", ids)
	}
}

func TestImports_Accounts_HappyPath(t *testing.T) {
	const body = "accounts\t1\nteam\tTeam One\tteam1\tpw\n"
	srv := importServer(t, "/api/v4/users/accounts", func(t *testing.T, r *http.Request) {
		got := readMultipartFile(t, r, "tsv")
		if got != body {
			t.Fatalf("tsv body = %q, want %q", got, body)
		}
	}, `["team1"]`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	ids, err := c.Imports.Accounts(context.Background(), strings.NewReader(body))
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	if !reflect.DeepEqual(ids, []string{"team1"}) {
		t.Fatalf("ids = %v, want [team1]", ids)
	}
}

func TestImports_Contest_ReturnsNewCID(t *testing.T) {
	const body = "name: Demo Contest\n"
	srv := importServer(t, "/api/v4/contests", func(t *testing.T, r *http.Request) {
		got := readMultipartFile(t, r, "yaml")
		if got != body {
			t.Fatalf("yaml body = %q, want %q", got, body)
		}
	}, `{"id":"7"}`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	cid, err := c.Imports.Contest(context.Background(), strings.NewReader(body))
	if err != nil {
		t.Fatalf("Contest: %v", err)
	}
	if cid != "7" {
		t.Fatalf("cid = %q, want 7", cid)
	}
}

func TestImports_Problem_PassesContestAndProblemFields(t *testing.T) {
	const zipBody = "problem zip"
	srv := importServer(t, "/api/v4/contests/2/problems", func(t *testing.T, r *http.Request) {
		if got := r.FormValue("problem"); got != "boolfind" {
			t.Fatalf("problem = %q, want boolfind", got)
		}
		got := readMultipartFile(t, r, "zip[]")
		if got != zipBody {
			t.Fatalf("zip body = %q, want %q", got, zipBody)
		}
	}, `["3"]`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	id, err := c.Imports.Problem(context.Background(), "2", "boolfind", strings.NewReader(zipBody))
	if err != nil {
		t.Fatalf("Problem: %v", err)
	}
	if id != "3" {
		t.Fatalf("id = %q, want 3", id)
	}
}

func importServer(t *testing.T, path string, inspect func(t *testing.T, r *http.Request), response string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != path {
			t.Fatalf("path = %q, want %q", r.URL.Path, path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		inspect(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
}

func readMultipartFile(t *testing.T, r *http.Request, field string) string {
	t.Helper()
	file, _, err := r.FormFile(field)
	if err != nil {
		t.Fatalf("FormFile(%s): %v", field, err)
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll(%s): %v", field, err)
	}
	return string(body)
}

func decodeMultipartJSON(t *testing.T, r *http.Request, field string, out interface{}) {
	t.Helper()
	file, _, err := r.FormFile(field)
	if err != nil {
		t.Fatalf("FormFile(%s): %v", field, err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(out); err != nil {
		t.Fatalf("Decode(%s): %v", field, err)
	}
}
