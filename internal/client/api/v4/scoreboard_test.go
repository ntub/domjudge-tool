package v4

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const scoreboardFixture = `{
  "event_id":"scoreboard-1",
  "time":"2026-01-02T00:10:00+00:00",
  "state":{"started":"2026-01-02T00:00:00+00:00","frozen":null,"thawed":null},
  "rows":[{
    "rank":1,
    "team_id":"2",
    "score":{"num_solved":1,"total_time":42},
    "problems":[{
      "label":"boolfind",
      "problem_id":"3",
      "num_judged":1,
      "num_pending":0,
      "solved":true,
      "first_to_solve":true,
      "time":42
    }]
  }]
}`

const scoreboardWithFreezeFixture = `{
  "event_id":"scoreboard-2",
  "time":"2026-01-02T01:00:00+00:00",
  "state":{"started":"2026-01-02T00:00:00+00:00","frozen":"2026-01-02T00:45:00+00:00","thawed":null},
  "rows":[]
}`

func TestScoreboardResource_Get_DecodesContractFixture_NoFreeze(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/contests/2/scoreboard" {
			t.Errorf("path = %q, want /api/v4/contests/2/scoreboard", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("RawQuery = %q, want empty", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(scoreboardFixture))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	snap, err := c.Scoreboard.Get(context.Background(), "2", ScoreboardListOpts{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if snap.EventID != "scoreboard-1" {
		t.Fatalf("EventID = %q, want scoreboard-1", snap.EventID)
	}
	if len(snap.Rows) != 1 || len(snap.Rows[0].Problems) != 1 {
		t.Fatalf("Rows = %+v, want one row and one problem", snap.Rows)
	}
	if snap.Rows[0].Problems[0].Time == nil || *snap.Rows[0].Problems[0].Time != 42 {
		t.Fatalf("problem time = %v, want 42", snap.Rows[0].Problems[0].Time)
	}
}

func TestScoreboardResource_Get_DecodesContractFixture_WithFreeze(t *testing.T) {
	allTeams := true
	public := false
	category := "2"
	sortOrder := 9
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vals, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			t.Fatalf("ParseQuery: %v", err)
		}
		if vals.Get("allteams") != "true" || vals.Get("public") != "false" || vals.Get("category") != "2" || vals.Get("sortorder") != "9" {
			t.Fatalf("query = %q, want allteams=true&category=2&public=false&sortorder=9", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(scoreboardWithFreezeFixture))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	snap, err := c.Scoreboard.Get(context.Background(), "2", ScoreboardListOpts{
		AllTeams:  &allTeams,
		Category:  &category,
		Public:    &public,
		SortOrder: &sortOrder,
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if snap.State.Frozen == nil || *snap.State.Frozen != "2026-01-02T00:45:00+00:00" {
		t.Fatalf("Frozen = %v, want freeze timestamp", snap.State.Frozen)
	}
}

func TestScoreboardResource_Get_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	_, err := c.Scoreboard.Get(context.Background(), "2", ScoreboardListOpts{})
	if !errors.Is(err, ErrServerError) {
		t.Fatalf("err = %v, want ErrServerError", err)
	}
}
