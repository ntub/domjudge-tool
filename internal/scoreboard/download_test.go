package scoreboard_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/scoreboard"
)

func TestDownload_HitsAPIEndpoints(t *testing.T) {
	var contests, snapshots, teams, orgs, groups atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/contests/2":
			contests.Add(1)
			_, _ = w.Write([]byte(`{"id":"2","shortname":"demo","name":"Demo","state":{}}`))
		case "/api/v4/contests/2/scoreboard":
			snapshots.Add(1)
			if r.URL.Query().Get("sortorder") != "0" || r.URL.Query().Get("allteams") != "true" {
				t.Fatalf("scoreboard query = %q, want allteams=true&sortorder=0", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"event_id":"e1","time":"2026-01-01T00:00:00+00:00","state":{},"rows":[{"rank":1,"team_id":"1","score":{"num_solved":1,"total_time":30},"problems":[{"label":"A","problem_id":"a","num_judged":1,"num_pending":0,"solved":true,"first_to_solve":false,"time":30}]}]}`))
		case "/api/v4/contests/2/teams":
			teams.Add(1)
			_, _ = w.Write([]byte(`[{"group_ids":["1"],"affiliation":"NTUB","nationality":"TWN","id":"1","icpc_id":null,"name":"Team Alpha","display_name":null,"organization_id":"1","members":null}]`))
		case "/api/v4/contests/2/organizations":
			orgs.Add(1)
			_, _ = w.Write([]byte(`[{"id":"1","icpc_id":null,"shortname":"NTUB","name":"NTUB","formal_name":null,"country":"TWN","url":null,"twitter_hashtag":null}]`))
		case "/api/v4/contests/2/groups":
			groups.Add(1)
			_, _ = w.Write([]byte(`[{"id":"1","icpc_id":null,"name":"Main","sortorder":0,"color":null,"hidden":false}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})
	header, rows, err := scoreboard.Download(context.Background(), c, "2", false)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	assertStringSlice(t, header, []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score", "Category", "A"})
	assertStringSlice(t, rows[0], []string{"1", "NTUB", "Alpha", "1", "30", "Main", "30/1 try"})

	if contests.Load() != 1 || snapshots.Load() != 1 || teams.Load() != 1 || orgs.Load() != 1 || groups.Load() != 1 {
		t.Fatalf("endpoint hits contests=%d scoreboard=%d teams=%d orgs=%d groups=%d, want 1 each",
			contests.Load(), snapshots.Load(), teams.Load(), orgs.Load(), groups.Load())
	}
}

func TestDownload_FetchesPopulatedSortordersAndCanExcludeHidden(t *testing.T) {
	tests := []struct {
		name          string
		excludeHidden bool
		wantTeams     []string
	}{
		{name: "includes hidden by default", wantTeams: []string{"Alpha", "System"}},
		{name: "excludes hidden", excludeHidden: true, wantTeams: []string{"Alpha"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotSortorders []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v4/contests/2":
					_, _ = w.Write([]byte(`{"id":"2","shortname":"demo","name":"Demo","state":{}}`))
				case "/api/v4/contests/2/teams":
					_, _ = w.Write([]byte(`[
						{"group_ids":["main"],"affiliation":"NTUB","nationality":"TWN","id":"1","icpc_id":null,"name":"Team Alpha","display_name":null,"organization_id":"1","members":null},
						{"group_ids":["system"],"affiliation":null,"nationality":null,"id":"2","icpc_id":null,"name":"DOMjudge System","display_name":null,"organization_id":null,"members":null}
					]`))
				case "/api/v4/contests/2/organizations":
					_, _ = w.Write([]byte(`[{"id":"1","icpc_id":null,"shortname":"NTUB","name":"NTUB","formal_name":null,"country":"TWN","url":null,"twitter_hashtag":null}]`))
				case "/api/v4/contests/2/groups":
					_, _ = w.Write([]byte(`[
						{"id":"main","icpc_id":null,"name":"Main","sortorder":0,"color":null,"hidden":false},
						{"id":"system","icpc_id":null,"name":"System","sortorder":9,"color":null,"hidden":true},
						{"id":"unused","icpc_id":null,"name":"Unused","sortorder":20,"color":null,"hidden":false}
					]`))
				case "/api/v4/contests/2/scoreboard":
					if r.URL.Query().Get("allteams") != "true" {
						t.Fatalf("allteams = %q, want true", r.URL.Query().Get("allteams"))
					}
					sortorder := r.URL.Query().Get("sortorder")
					gotSortorders = append(gotSortorders, sortorder)
					switch sortorder {
					case "0":
						_, _ = w.Write([]byte(`{"event_id":"e1","time":"2026-01-01T00:00:00+00:00","state":{},"rows":[{"rank":1,"team_id":"1","score":{"num_solved":1,"total_time":30},"problems":[{"label":"A","problem_id":"a","num_judged":1,"num_pending":0,"solved":true,"first_to_solve":false,"time":30}]}]}`))
					case "9":
						_, _ = w.Write([]byte(`{"event_id":"e2","time":"2026-01-01T00:00:00+00:00","state":{},"rows":[{"rank":1,"team_id":"2","score":{"num_solved":0,"total_time":0},"problems":[{"label":"A","problem_id":"a","num_judged":0,"num_pending":0,"solved":false,"first_to_solve":false,"time":null}]}]}`))
					default:
						t.Fatalf("unexpected sortorder %q", sortorder)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})
			_, rows, err := scoreboard.Download(context.Background(), c, "2", tc.excludeHidden)
			if err != nil {
				t.Fatalf("Download: %v", err)
			}
			assertStringSlice(t, gotSortorders, []string{"0", "9"})
			if len(rows) != len(tc.wantTeams) {
				t.Fatalf("rows = %d, want %d: %v", len(rows), len(tc.wantTeams), rows)
			}
			for i, wantTeam := range tc.wantTeams {
				if rows[i][2] != wantTeam {
					t.Fatalf("row %d team = %q, want %q; rows=%v", i, rows[i][2], wantTeam, rows)
				}
			}
		})
	}
}

func TestDownload_MultiGroupTeam_LabelsAndFiltersByMatchedSortorder(t *testing.T) {
	// Team 1 belongs to BOTH a visible group (sortorder 0) and a hidden group
	// (sortorder 9); its FIRST group is the visible one. It is returned in
	// both sortorder sections, so the category and --exclude-hidden decision
	// must follow the section's group, not team.GroupIDs[0].
	newServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			row := `{"rank":1,"team_id":"1","score":{"num_solved":1,"total_time":30},"problems":[{"label":"A","problem_id":"a","num_judged":1,"num_pending":0,"solved":true,"first_to_solve":false,"time":30}]}`
			switch r.URL.Path {
			case "/api/v4/contests/2":
				_, _ = w.Write([]byte(`{"id":"2","shortname":"demo","name":"Demo","state":{}}`))
			case "/api/v4/contests/2/teams":
				_, _ = w.Write([]byte(`[{"group_ids":["main","system"],"affiliation":"NTUB","nationality":"TWN","id":"1","icpc_id":null,"name":"MultiTeam","display_name":null,"organization_id":"1","members":null}]`))
			case "/api/v4/contests/2/organizations":
				_, _ = w.Write([]byte(`[{"id":"1","icpc_id":null,"shortname":"NTUB","name":"NTUB","formal_name":null,"country":"TWN","url":null,"twitter_hashtag":null}]`))
			case "/api/v4/contests/2/groups":
				_, _ = w.Write([]byte(`[{"id":"main","icpc_id":null,"name":"Main","sortorder":0,"color":null,"hidden":false},{"id":"system","icpc_id":null,"name":"System","sortorder":9,"color":null,"hidden":true}]`))
			case "/api/v4/contests/2/scoreboard":
				switch r.URL.Query().Get("sortorder") {
				case "0", "9":
					_, _ = w.Write([]byte(`{"event_id":"e","time":"2026-01-01T00:00:00+00:00","state":{},"rows":[` + row + `]}`))
				default:
					t.Fatalf("unexpected sortorder %q", r.URL.Query().Get("sortorder"))
				}
			default:
				http.NotFound(w, r)
			}
		}))
	}

	t.Run("include hidden labels each section by its own group", func(t *testing.T) {
		srv := newServer()
		defer srv.Close()
		c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})
		_, rows, err := scoreboard.Download(context.Background(), c, "2", false)
		if err != nil {
			t.Fatalf("Download: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2: %v", len(rows), rows)
		}
		// Category is column index 5; sortorders are visited ascending.
		if rows[0][5] != "Main" {
			t.Errorf("row0 category = %q, want Main", rows[0][5])
		}
		if rows[1][5] != "System" {
			t.Errorf("row1 category = %q, want System", rows[1][5])
		}
	})

	t.Run("exclude hidden drops the hidden-section copy even if first group is visible", func(t *testing.T) {
		srv := newServer()
		defer srv.Close()
		c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})
		_, rows, err := scoreboard.Download(context.Background(), c, "2", true)
		if err != nil {
			t.Fatalf("Download: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1: %v", len(rows), rows)
		}
		if rows[0][5] != "Main" {
			t.Errorf("row0 category = %q, want Main", rows[0][5])
		}
	})
}

func TestDownload_NonOKErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	if _, _, err := scoreboard.Download(context.Background(), c, "2", false); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

func TestDownload_CtxCancelled(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-block:
		}
	}))
	defer srv.Close()
	defer close(block)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := scoreboard.Download(ctx, c, "2", false)
	if err == nil {
		t.Fatal("expected ctx cancelled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}
