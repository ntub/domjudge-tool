package v8

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

const (
	teamAddFormFixture  = "../../../../testdata/web/v8/team_add_form.html"
	teamEditFormFixture = "../../../../testdata/web/v8/team_edit_form.html"
	teamViewFixture     = "../../../../testdata/web/v8/team_view.html"
)

func strPtr(s string) *string { return &s }

func TestCreateTeamAndUser_Integration(t *testing.T) {
	addForm := mustReadFixture(t, teamAddFormFixture)
	teamView := mustReadFixture(t, teamViewFixture)

	tests := []struct {
		name        string
		stayOnForm  bool
		wantErr     error
		wantTeamID  string
		wantUserID  string
		wantEnabled string
	}{
		{
			name:        "success posts v8 create-user fields",
			wantTeamID:  "99",
			wantUserID:  "7",
			wantEnabled: "1",
		},
		{
			name:        "stayed on add returns create failed",
			stayOnForm:  true,
			wantErr:     web.ErrCreateFailed,
			wantEnabled: "1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var posted url.Values
			mux := http.NewServeMux()
			mux.HandleFunc("/jury/teams/add", func(w http.ResponseWriter, r *http.Request) {
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
					http.Redirect(w, r, "/jury/teams/99", http.StatusFound)
				default:
					t.Errorf("method = %s, want GET or POST", r.Method)
				}
			})
			mux.HandleFunc("/jury/teams/99", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(teamView)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			c := newTestClient(t, srv)
			gotTeamID, gotUserID, err := c.CreateTeamAndUser(
				context.Background(),
				model.CreateUser{Username: "team-alpha", Name: "Team Alpha"},
				3,
				8,
				true,
			)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("errors.Is(err, %v) = false; err = %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateTeamAndUser: %v", err)
			}
			if gotTeamID != tc.wantTeamID {
				t.Errorf("teamID = %q, want %s", gotTeamID, tc.wantTeamID)
			}
			if gotUserID != tc.wantUserID {
				t.Errorf("userID = %q, want %s", gotUserID, tc.wantUserID)
			}
			assertPostedValue(t, posted, "team[name]", "team-alpha")
			assertPostedValue(t, posted, "team[displayName]", "Team Alpha")
			assertPostedValue(t, posted, "team[category]", "3")
			assertPostedValue(t, posted, "team[affiliation]", "8")
			assertPostedValue(t, posted, "team[penalty]", "0")
			assertPostedValue(t, posted, "team[enabled]", tc.wantEnabled)
			assertPostedValue(t, posted, "team[addUserForTeam]", "create-new-user")
			assertPostedValue(t, posted, "team[newUsername]", "team-alpha")
			if _, seen := posted["team[users][0][username]"]; seen {
				t.Errorf("team[users][0][username] should not be posted on v8")
			}
		})
	}
}

func TestUpdateTeam_Integration(t *testing.T) {
	editForm := mustReadFixture(t, teamEditFormFixture)
	teamView := mustReadFixture(t, teamViewFixture)

	tests := []struct {
		name       string
		user       model.User
		stayOnForm bool
		wantErr    error
	}{
		{
			name: "success posts team fields only",
			user: model.User{
				Username: "team-alpha",
				Name:     "Team Alpha",
				TeamID:   strPtr("99"),
			},
		},
		{
			name: "stayed on edit returns update failed",
			user: model.User{
				Username: "team-alpha",
				Name:     "Team Alpha",
				TeamID:   strPtr("99"),
			},
			stayOnForm: true,
			wantErr:    web.ErrUpdateFailed,
		},
		{
			name: "missing team id returns error",
			user: model.User{
				Username: "team-alpha",
				Name:     "Team Alpha",
			},
			wantErr: errAny,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var posted url.Values
			mux := http.NewServeMux()
			mux.HandleFunc("/jury/teams/99/edit", func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					_, _ = w.Write(editForm)
				case http.MethodPost:
					_ = r.ParseForm()
					posted = cloneValues(r.PostForm)
					if tc.stayOnForm {
						_, _ = w.Write(editForm)
						return
					}
					http.Redirect(w, r, "/jury/teams/99", http.StatusFound)
				default:
					t.Errorf("method = %s, want GET or POST", r.Method)
				}
			})
			mux.HandleFunc("/jury/teams/99", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(teamView)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			c := newTestClient(t, srv)
			gotTeamID, gotUserID, err := c.UpdateTeam(context.Background(), tc.user, 4, 8, false)
			if tc.wantErr != nil {
				if tc.wantErr == errAny {
					if err == nil {
						t.Fatalf("expected error, got nil")
					}
					if !strings.Contains(err.Error(), "TeamID") {
						t.Fatalf("error = %v, want TeamID mention", err)
					}
					return
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("errors.Is(err, %v) = false; err = %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateTeam: %v", err)
			}
			if gotTeamID != "99" {
				t.Errorf("teamID = %q, want 99", gotTeamID)
			}
			if gotUserID != "7" {
				t.Errorf("userID = %q, want 7", gotUserID)
			}
			assertPostedValue(t, posted, "team[name]", "team-alpha")
			assertPostedValue(t, posted, "team[displayName]", "Team Alpha")
			assertPostedValue(t, posted, "team[category]", "4")
			assertPostedValue(t, posted, "team[affiliation]", "8")
			assertPostedValue(t, posted, "team[penalty]", "20")
			assertPostedValue(t, posted, "team[enabled]", "0")
			if _, seen := posted["team[addUserForTeam]"]; seen {
				t.Errorf("team[addUserForTeam] should not be posted on update")
			}
			if _, seen := posted["team[newUsername]"]; seen {
				t.Errorf("team[newUsername] should not be posted on update")
			}
		})
	}
}

func TestFetchTeamViewUserID_MissingUserLink(t *testing.T) {
	tests := []struct {
		name string
		html string
	}{
		{name: "only breadcrumb", html: `<a href="/jury">Jury</a>`},
		{name: "no href", html: `<tr><th>User</th><td><a>user1</a></td></tr>`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.html))
			}))
			defer srv.Close()

			c := newTestClient(t, srv)
			_, err := c.fetchTeamViewUserID(context.Background(), "/jury/teams/99")
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "user link") {
				t.Errorf("error = %v, want user link mention", err)
			}
		})
	}
}

var errAny = errors.New("any error")

func cloneValues(in url.Values) url.Values {
	out := url.Values{}
	for k, vs := range in {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

func assertPostedValue(t *testing.T, posted url.Values, key, want string) {
	t.Helper()
	if got := posted.Get(key); got != want {
		t.Errorf("%s = %q, want %q", key, got, want)
	}
}
