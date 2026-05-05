package v4

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
)

func TestTeamsResource_List(t *testing.T) {
	teamsListJSON, err := os.ReadFile("../../../../testdata/api/v4/teams_list.json")
	if err != nil {
		t.Fatalf("read teams_list.json fixture: %v", err)
	}

	cases := []struct {
		name    string
		handler http.HandlerFunc
		ctxFn   func() (context.Context, context.CancelFunc)
		check   func(t *testing.T, teams []model.Team, err error)
	}{
		{
			name: "happy path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(teamsListJSON)
			},
			check: func(t *testing.T, teams []model.Team, err error) {
				if err != nil {
					t.Fatalf("List returned error: %v", err)
				}
				if got, want := len(teams), 1; got != want {
					t.Fatalf("len(teams) = %d, want %d", got, want)
				}
				if got, want := teams[0].Name, "Team Sample University"; got != want {
					t.Fatalf("teams[0].Name = %q, want %q", got, want)
				}
			},
		},
		{
			name: "path contains cid",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, "/contests/1/teams") {
					http.Error(w, "unexpected path: "+r.URL.Path, http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(teamsListJSON)
			},
			check: func(t *testing.T, teams []model.Team, err error) {
				if err != nil {
					t.Fatalf("List returned error: %v", err)
				}
				if len(teams) != 1 {
					t.Fatalf("len(teams) = %d, want 1", len(teams))
				}
			},
		},
		{
			name: "401",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			},
			check: func(t *testing.T, _ []model.Team, err error) {
				if !errors.Is(err, ErrUnauthorized) {
					t.Fatalf("err = %v, want ErrUnauthorized", err)
				}
			},
		},
		{
			name: "404",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			},
			check: func(t *testing.T, _ []model.Team, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("err = %v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			},
			check: func(t *testing.T, _ []model.Team, err error) {
				if !errors.Is(err, ErrServerError) {
					t.Fatalf("err = %v, want ErrServerError", err)
				}
			},
		},
		{
			name: "malformed json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("{not json"))
			},
			check: func(t *testing.T, _ []model.Team, err error) {
				if err == nil {
					t.Fatal("expected error for malformed json, got nil")
				}
				if !strings.Contains(err.Error(), "decode") {
					t.Fatalf("err = %v, want error containing %q", err, "decode")
				}
			},
		},
		{
			name: "context cancelled",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(teamsListJSON)
			},
			ctxFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			check: func(t *testing.T, _ []model.Team, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("err = %v, want context.Canceled", err)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})

			var (
				ctx    context.Context
				cancel context.CancelFunc
			)
			if tc.ctxFn != nil {
				ctx, cancel = tc.ctxFn()
			} else {
				ctx, cancel = context.Background(), func() {}
			}
			defer cancel()

			teams, err := c.Teams.List(ctx, "1")
			tc.check(t, teams, err)
		})
	}
}

func TestTeamsResource_Get(t *testing.T) {
	teamJSON, err := os.ReadFile("../../../../testdata/api/v4/team.json")
	if err != nil {
		t.Fatalf("read team.json fixture: %v", err)
	}

	cases := []struct {
		name    string
		handler http.HandlerFunc
		ctxFn   func() (context.Context, context.CancelFunc)
		check   func(t *testing.T, team *model.Team, err error)
	}{
		{
			name: "happy path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(teamJSON)
			},
			check: func(t *testing.T, team *model.Team, err error) {
				if err != nil {
					t.Fatalf("Get returned error: %v", err)
				}
				if team == nil {
					t.Fatal("Get returned nil team")
				}
				if got, want := team.ID, "1"; got != want {
					t.Fatalf("team.ID = %q, want %q", got, want)
				}
				if got, want := team.Name, "Team Sample University"; got != want {
					t.Fatalf("team.Name = %q, want %q", got, want)
				}
			},
		},
		{
			name: "path contains cid and id",
			// Path assertion is performed via a flag set inside the handler so
			// that a wrong path is reported as a test failure rather than a
			// confusing 4xx response body.
			handler: nil,
			check: func(t *testing.T, team *model.Team, err error) {
				if err != nil {
					t.Fatalf("Get returned error: %v", err)
				}
				if team == nil || team.ID != "1" {
					t.Fatalf("team = %+v, want ID=1", team)
				}
			},
		},
		{
			name: "401",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			},
			check: func(t *testing.T, _ *model.Team, err error) {
				if !errors.Is(err, ErrUnauthorized) {
					t.Fatalf("err = %v, want ErrUnauthorized", err)
				}
			},
		},
		{
			name: "404",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			},
			check: func(t *testing.T, _ *model.Team, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("err = %v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			},
			check: func(t *testing.T, _ *model.Team, err error) {
				if !errors.Is(err, ErrServerError) {
					t.Fatalf("err = %v, want ErrServerError", err)
				}
			},
		},
		{
			name: "malformed json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("{not json"))
			},
			check: func(t *testing.T, _ *model.Team, err error) {
				if err == nil {
					t.Fatal("expected error for malformed json, got nil")
				}
				if !strings.Contains(err.Error(), "decode") {
					t.Fatalf("err = %v, want error containing %q", err, "decode")
				}
			},
		},
		{
			name: "context cancelled",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(teamJSON)
			},
			ctxFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			check: func(t *testing.T, _ *model.Team, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("err = %v, want context.Canceled", err)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var srv *httptest.Server
			if tc.name == "path contains cid and id" {
				var pathSeen bool
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "/contests/1/teams/1") {
						pathSeen = true
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(teamJSON)
				}))
				defer srv.Close()
				defer func() {
					if !pathSeen {
						t.Fatalf("expected request path to contain %q", "/contests/1/teams/1")
					}
				}()
			} else {
				srv = httptest.NewServer(tc.handler)
				defer srv.Close()
			}

			c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})

			var (
				ctx    context.Context
				cancel context.CancelFunc
			)
			if tc.ctxFn != nil {
				ctx, cancel = tc.ctxFn()
			} else {
				ctx, cancel = context.Background(), func() {}
			}
			defer cancel()

			team, err := c.Teams.Get(ctx, "1", "1")
			tc.check(t, team, err)
		})
	}
}
