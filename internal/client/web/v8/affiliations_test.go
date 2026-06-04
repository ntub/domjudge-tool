package v8

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

const (
	affiliationsListFixture = "../../../../testdata/web/v8/affiliations_list.html"
	affiliationAddFixture   = "../../../../testdata/web/v8/affiliation_add_form.html"
)

func TestGetAffiliations_Integration(t *testing.T) {
	listHTML := mustReadFixture(t, affiliationsListFixture)

	tests := []struct {
		name     string
		handler  http.HandlerFunc
		wantErr  error
		wantRows int
	}{
		{
			name: "success parses shifted v8 columns",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(listHTML)
			},
			wantRows: 1,
		},
		{
			name: "malformed row returns malformed row",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`<table><tbody><tr><td><a>8</a></td></tr></tbody></table>`))
			},
			wantErr: web.ErrMalformedRow,
		},
		{
			name: "redirect to login returns login failed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/login", http.StatusFound)
			},
			wantErr: web.ErrLoginFailed,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/jury/affiliations", tc.handler)
			mux.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			c := newTestClient(t, srv)
			got, err := c.GetAffiliations(context.Background())
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("errors.Is(err, %v) = false; err = %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetAffiliations: %v", err)
			}
			if len(got) != tc.wantRows {
				t.Fatalf("len = %d, want %d", len(got), tc.wantRows)
			}
			if got[0].ID == nil || *got[0].ID != "8" {
				t.Errorf("ID = %v, want 8", got[0].ID)
			}
			if got[0].ShortName != "course001" {
				t.Errorf("ShortName = %q, want course001", got[0].ShortName)
			}
			if got[0].Name != "Example University" {
				t.Errorf("Name = %q, want Example University", got[0].Name)
			}
			if got[0].Country != "TWN" {
				t.Errorf("Country = %q, want TWN", got[0].Country)
			}
		})
	}
}

func TestGetAffiliation_Integration(t *testing.T) {
	listHTML := mustReadFixture(t, affiliationsListFixture)

	tests := []struct {
		name      string
		query     string
		status    int
		wantNil   bool
		wantErr   error
		wantShort string
	}{
		{name: "found by name", query: "Example University", wantShort: "course001"},
		{name: "found by shortname", query: "course001", wantShort: "course001"},
		{name: "not found returns nil nil", query: "missing", wantNil: true},
		{name: "list error propagates", query: "course001", status: http.StatusInternalServerError, wantErr: web.ErrServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.status != 0 {
					w.WriteHeader(tc.status)
					return
				}
				_, _ = w.Write(listHTML)
			}))
			defer srv.Close()

			c := newTestClient(t, srv)
			got, err := c.GetAffiliation(context.Background(), tc.query)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("errors.Is(err, %v) = false; err = %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetAffiliation: %v", err)
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("got nil, want affiliation")
			}
			if got.ShortName != tc.wantShort {
				t.Errorf("ShortName = %q, want %s", got.ShortName, tc.wantShort)
			}
		})
	}
}

func TestCreateAffiliation_Integration(t *testing.T) {
	addForm := mustReadFixture(t, affiliationAddFixture)

	tests := []struct {
		name       string
		stayOnForm bool
		wantErr    error
	}{
		{name: "success posts v8 field names"},
		{name: "stayed on add returns create failed", stayOnForm: true, wantErr: web.ErrCreateFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var posted url.Values
			mux := http.NewServeMux()
			mux.HandleFunc("/jury/affiliations/add", func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					_, _ = w.Write(addForm)
				case http.MethodPost:
					_ = r.ParseForm()
					posted = cloneValues(r.PostForm)
					if tc.stayOnForm {
						_, _ = w.Write(addForm)
						return
					}
					http.Redirect(w, r, "/jury/affiliations/8", http.StatusFound)
				default:
					t.Errorf("method = %s, want GET or POST", r.Method)
				}
			})
			mux.HandleFunc("/jury/affiliations/8", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			c := newTestClient(t, srv)
			got, err := c.CreateAffiliation(context.Background(), "course001", "Example University", "TWN")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("errors.Is(err, %v) = false; err = %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateAffiliation: %v", err)
			}
			assertPostedValue(t, posted, "team_affiliation[shortname]", "course001")
			assertPostedValue(t, posted, "team_affiliation[name]", "Example University")
			assertPostedValue(t, posted, "team_affiliation[country]", "TWN")
			if _, seen := posted["team_affiliation[comments]"]; seen {
				t.Errorf("team_affiliation[comments] should not be posted on v8")
			}
			if _, seen := posted["team_affiliation[internalcomments]"]; !seen {
				t.Errorf("team_affiliation[internalcomments] should be preserved from form snapshot")
			}
			if got == nil {
				t.Fatalf("got nil affiliation")
			}
			if got.ID == nil || *got.ID != "8" {
				t.Errorf("ID = %v, want 8", got.ID)
			}
			if got.ShortName != "course001" {
				t.Errorf("ShortName = %q, want course001", got.ShortName)
			}
			if got.Name != "Example University" {
				t.Errorf("Name = %q, want Example University", got.Name)
			}
			if got.Country != "TWN" {
				t.Errorf("Country = %q, want TWN", got.Country)
			}
		})
	}
}
