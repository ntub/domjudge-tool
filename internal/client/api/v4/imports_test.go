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
	"sync/atomic"
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

func TestImports_GroupsTSV_RejectedReturnsErrImportRejected_Integration(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   []string
	}{
		{
			name:   "string array",
			status: http.StatusBadRequest,
			body:   `["line 2: missing name"]`,
			want:   []string{"line 2: missing name"},
		},
		{
			name:   "object array has no empty string prefix",
			status: http.StatusUnprocessableEntity,
			body:   `[{"message":"line 2: missing name"},{"error":"line 3: invalid ID"},{"detail":"line 4: duplicate group"},{"other":"unrecognized error"}]`,
			want:   []string{"line 2: missing name", "line 3: invalid ID", "line 4: duplicate group", `{"other":"unrecognized error"}`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/v4/users/groups" {
					t.Errorf("request = %s %s, want POST /api/v4/users/groups", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Errorf("parse multipart: %v", err)
					http.Error(w, "invalid multipart", http.StatusBadRequest)
					return
				}
				defer r.MultipartForm.RemoveAll()
				if got := readMultipartFile(t, r, "tsv"); got != "bad" {
					t.Errorf("TSV payload = %q, want bad", got)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(srv.Close)

			c := New(Options{BaseURL: srv.URL})
			_, err := c.Imports.GroupsTSV(context.Background(), strings.NewReader("bad"))
			if !errors.Is(err, ErrImportRejected) {
				t.Fatalf("err = %v, want ErrImportRejected", err)
			}
			var rejected *ImportRejectedError
			if !errors.As(err, &rejected) {
				t.Fatalf("err = %v, want ImportRejectedError", err)
			}
			if !reflect.DeepEqual(rejected.Details, tc.want) {
				t.Errorf("Details = %#v, want exactly %#v", rejected.Details, tc.want)
			}
			if rejected.StatusCode != tc.status || string(rejected.Body) != tc.body || rejected.URL != srv.URL+"/api/v4/users/groups" {
				t.Errorf("rejection = %+v, want status %d, original body and endpoint URL", rejected, tc.status)
			}
		})
	}
}

func TestImports_GroupsJSON_HappyPath(t *testing.T) {
	want := []model.Group{{
		ID:   "1",
		Name: "Participants",
	}}
	srv := importServer(t, "/api/v4/users/groups", func(t *testing.T, r *http.Request) {
		got := readMultipartFile(t, r, "json")
		var decoded []model.Group
		if err := json.Unmarshal([]byte(got), &decoded); err != nil {
			t.Fatalf("unmarshal json: %v", err)
		}
		if !reflect.DeepEqual(decoded, want) {
			t.Fatalf("groups = %+v, want %+v", decoded, want)
		}
	}, `["1"]`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	ids, err := c.Imports.GroupsJSON(context.Background(), want)
	if err != nil {
		t.Fatalf("GroupsJSON: %v", err)
	}
	if !reflect.DeepEqual(ids, []string{"1"}) {
		t.Fatalf("ids = %v, want [1]", ids)
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

func TestImports_Problem_AcceptsProblemIDsResponse(t *testing.T) {
	const zipBody = "problem zip"
	srv := importServer(t, "/api/v4/contests/2/problems", func(t *testing.T, r *http.Request) {
		if got := r.FormValue("problem"); got != "10" {
			t.Fatalf("problem = %q, want 10", got)
		}
		got := readMultipartFile(t, r, "zip[]")
		if got != zipBody {
			t.Fatalf("zip body = %q, want %q", got, zipBody)
		}
	}, `{"problem_ids":[10],"messages":["Saved problem 10"]}`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	id, err := c.Imports.Problem(context.Background(), "2", "10", strings.NewReader(zipBody))
	if err != nil {
		t.Fatalf("Problem: %v", err)
	}
	if id != "10" {
		t.Fatalf("id = %q, want 10", id)
	}
}

func TestImports_ProblemPackageUpdate_UsesZipField(t *testing.T) {
	const zipBody = "problem zip"
	srv := importServer(t, "/api/v4/contests/2/problems", func(t *testing.T, r *http.Request) {
		if got := r.FormValue("problem"); got != "boolfind" {
			t.Fatalf("problem = %q, want boolfind", got)
		}
		file, header, err := r.FormFile("zip")
		if err != nil {
			t.Fatalf("FormFile zip: %v", err)
		}
		defer func() { _ = file.Close() }()
		if got := header.Filename; got != "boolfind-package.zip" {
			t.Fatalf("filename = %q, want boolfind-package.zip", got)
		}
		raw, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read zip: %v", err)
		}
		got := string(raw)
		if got != zipBody {
			t.Fatalf("zip body = %q, want %q", got, zipBody)
		}
	}, `{"problem_id":3,"messages":[]}`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	id, err := c.Imports.ProblemPackageUpdate(context.Background(), "2", "boolfind", "boolfind-package.zip", strings.NewReader(zipBody))
	if err != nil {
		t.Fatalf("ProblemPackageUpdate: %v", err)
	}
	if id != "3" {
		t.Fatalf("id = %q, want 3", id)
	}
}

func TestImports_ProblemPackage_OmitsProblemField(t *testing.T) {
	const zipBody = "problem zip"
	srv := importServer(t, "/api/v4/contests/2/problems", func(t *testing.T, r *http.Request) {
		if got := r.FormValue("problem"); got != "" {
			t.Fatalf("problem = %q, want empty for create", got)
		}
		got := readMultipartFile(t, r, "zip")
		if got != zipBody {
			t.Fatalf("zip body = %q, want %q", got, zipBody)
		}
	}, `{"problem_id":3,"messages":[]}`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	if err := c.Imports.ProblemPackage(context.Background(), "2", "boolfind.zip", strings.NewReader(zipBody)); err != nil {
		t.Fatalf("ProblemPackage: %v", err)
	}
}

func TestImports_ProblemPackageZipArray_UsesLegacyZipArrayField(t *testing.T) {
	const zipBody = "problem zip"
	srv := importServer(t, "/api/v4/contests/2/problems", func(t *testing.T, r *http.Request) {
		if got := r.FormValue("problem"); got != "" {
			t.Fatalf("problem = %q, want empty for create", got)
		}
		got := readMultipartFile(t, r, "zip[]")
		if got != zipBody {
			t.Fatalf("zip[] body = %q, want %q", got, zipBody)
		}
	}, `{"problem_ids":[3],"messages":[]}`)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	if err := c.Imports.ProblemPackageZipArray(context.Background(), "2", "boolfind.zip", strings.NewReader(zipBody)); err != nil {
		t.Fatalf("ProblemPackageZipArray: %v", err)
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

func TestImportIDs_UnmarshalJSON_AllVariants(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    []string
		wantErr bool
	}{
		{"empty", "", nil, true},
		{"single string", `"created-id"`, []string{"created-id"}, false},
		{"array of strings", `["id1", "id2"]`, []string{"id1", "id2"}, false},
		{"array of numbers", `[1, 2, 3]`, []string{"1", "2", "3"}, false},
		{"array of objects with id", `[{"id":"obj1"},{"id":"obj2"}]`, []string{"obj1", "obj2"}, false},
		{"array of objects with externalid", `[{"externalid":"ext1"}]`, []string{"ext1"}, false},
		{"array of objects with problem_id", `[{"problem_id":"prob1"}]`, []string{"prob1"}, false},
		{"array of objects missing id", `[{"name":"foo"}]`, nil, true},
		{"object with ids array", `{"ids":["a","b"]}`, []string{"a", "b"}, false},
		{"object with problem_ids array", `{"problem_ids":[10,20]}`, []string{"10", "20"}, false},
		{"object with cid", `{"cid":"42"}`, []string{"42"}, false},
		{"object with new_id", `{"new_id":"new1"}`, []string{"new1"}, false},
		{"unsupported format", `12345`, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ids importIDs
			err := ids.UnmarshalJSON([]byte(tc.json))
			if (err != nil) != tc.wantErr {
				t.Fatalf("UnmarshalJSON(%s) err = %v, wantErr %v", tc.json, err, tc.wantErr)
			}
			if !tc.wantErr && !reflect.DeepEqual([]string(ids), tc.want) {
				t.Fatalf("got %v, want %v", []string(ids), tc.want)
			}
		})
	}

	if _, err := singleImportID(nil); !errors.Is(err, ErrEmptyResponse) {
		t.Errorf("singleImportID(nil) = %v, want ErrEmptyResponse", err)
	}
}

func TestImportRejectionDetails_Variants(t *testing.T) {
	if _, ok := importRejectionDetails([]byte("plain text")); ok {
		t.Error("expected false for plain text")
	}

	jsonObj := `[{"message":"first err"},{"error":"second err"},{"detail":"third err"},{"other":"raw json"}]`
	msgs, ok := importRejectionDetails([]byte(jsonObj))
	if !ok || len(msgs) != 4 {
		t.Fatalf("importRejectionDetails got len=%d (%v, %v), want 4 messages", len(msgs), msgs, ok)
	}
	if msgs[0] != "first err" || msgs[1] != "second err" || msgs[2] != "third err" {
		t.Errorf("unexpected messages: %v", msgs)
	}

	if _, ok := importRejectionDetails([]byte("[invalid json")); ok {
		t.Error("expected false for invalid json")
	}
}

func TestImports_InvalidProblemArguments_Integration(t *testing.T) {
	type invalidInput struct {
		name     string
		cid      string
		id       string
		noReader bool
	}
	methods := []struct {
		name              string
		requiresProblemID bool
		call              func(*ImportsResource, string, string, io.Reader) error
	}{
		{
			name:              "legacy update",
			requiresProblemID: true,
			call: func(r *ImportsResource, cid, id string, body io.Reader) error {
				_, err := r.Problem(context.Background(), cid, id, body)
				return err
			},
		},
		{
			name:              "modern update",
			requiresProblemID: true,
			call: func(r *ImportsResource, cid, id string, body io.Reader) error {
				_, err := r.ProblemPackageUpdate(context.Background(), cid, id, "", body)
				return err
			},
		},
		{
			name: "modern create",
			call: func(r *ImportsResource, cid, _ string, body io.Reader) error {
				return r.ProblemPackage(context.Background(), cid, "", body)
			},
		},
		{
			name: "legacy create",
			call: func(r *ImportsResource, cid, _ string, body io.Reader) error {
				return r.ProblemPackageZipArray(context.Background(), cid, "", body)
			},
		},
	}
	for _, method := range methods {
		t.Run(method.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				_, _ = io.WriteString(w, `["unexpected-import"]`)
			}))
			t.Cleanup(srv.Close)
			c := New(Options{BaseURL: srv.URL})
			cases := []invalidInput{
				{name: "missing contest", id: "p"},
				{name: "missing archive", cid: "2", id: "p", noReader: true},
			}
			if method.requiresProblemID {
				cases = append(cases, invalidInput{name: "missing update target", cid: "2"})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					var body io.Reader = strings.NewReader("zip bytes")
					if tc.noReader {
						body = nil
					}
					if err := method.call(c.Imports, tc.cid, tc.id, body); !errors.Is(err, ErrInvalidArgument) {
						t.Fatalf("import error = %v, want ErrInvalidArgument", err)
					}
					if got := requests.Load(); got != 0 {
						t.Fatalf("requests = %d, want invalid input rejected before upload", got)
					}
				})
			}
		})
	}
}

func TestImports_MissingAssignedIDIsNotSuccess_Integration(t *testing.T) {
	methods := []struct {
		name      string
		path      string
		field     string
		filename  string
		payload   string
		problemID string
		call      func(*ImportsResource) (string, error)
	}{
		{
			name:     "contest",
			path:     "/api/v4/contests",
			field:    "yaml",
			filename: "contest.yaml",
			payload:  "name: Demo\n",
			call: func(r *ImportsResource) (string, error) {
				return r.Contest(context.Background(), strings.NewReader("name: Demo\n"))
			},
		},
		{
			name:      "legacy problem update",
			path:      "/api/v4/contests/2/problems",
			field:     "zip[]",
			filename:  "p.zip",
			payload:   "zip bytes",
			problemID: "p",
			call: func(r *ImportsResource) (string, error) {
				return r.Problem(context.Background(), "2", "p", strings.NewReader("zip bytes"))
			},
		},
		{
			name:      "modern problem update",
			path:      "/api/v4/contests/2/problems",
			field:     "zip",
			filename:  "p.zip",
			payload:   "zip bytes",
			problemID: "p",
			call: func(r *ImportsResource) (string, error) {
				return r.ProblemPackageUpdate(context.Background(), "2", "p", "", strings.NewReader("zip bytes"))
			},
		},
	}
	for _, method := range methods {
		t.Run(method.name, func(t *testing.T) {
			for _, response := range []string{`[]`, `[""]`} {
				t.Run(response, func(t *testing.T) {
					srv := importServer(t, method.path, func(t *testing.T, r *http.Request) {
						if got := r.FormValue("problem"); got != method.problemID {
							t.Errorf("problem = %q, want %q", got, method.problemID)
						}
						if got := readMultipartFile(t, r, method.field); got != method.payload {
							t.Errorf("uploaded payload = %q, want %q", got, method.payload)
						}
						files := r.MultipartForm.File[method.field]
						if len(files) != 1 || files[0].Filename != method.filename {
							t.Errorf("upload file headers = %+v, want one file named %s", files, method.filename)
						}
					}, response)
					t.Cleanup(srv.Close)
					id, err := method.call(New(Options{BaseURL: srv.URL}).Imports)
					if !errors.Is(err, ErrEmptyResponse) || id != "" {
						t.Errorf("import result = (%q, %v), want empty ID and ErrEmptyResponse", id, err)
					}
				})
			}
		})
	}
}
