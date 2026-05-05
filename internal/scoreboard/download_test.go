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
	var contests, snapshots, teams, orgs atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/contests/2":
			contests.Add(1)
			_, _ = w.Write([]byte(`{"id":"2","shortname":"demo","name":"Demo","state":{}}`))
		case "/api/v4/contests/2/scoreboard":
			snapshots.Add(1)
			_, _ = w.Write([]byte(`{"event_id":"e1","time":"2026-01-01T00:00:00+00:00","state":{},"rows":[{"rank":1,"team_id":"1","score":{"num_solved":1,"total_time":30},"problems":[{"label":"A","problem_id":"a","num_judged":1,"num_pending":0,"solved":true,"first_to_solve":false,"time":30}]}]}`))
		case "/api/v4/contests/2/teams":
			teams.Add(1)
			_, _ = w.Write([]byte(`[{"group_ids":["1"],"affiliation":"NTUB","nationality":"TWN","id":"1","icpc_id":null,"name":"Team Alpha","display_name":null,"organization_id":"1","members":null}]`))
		case "/api/v4/contests/2/organizations":
			orgs.Add(1)
			_, _ = w.Write([]byte(`[{"id":"1","icpc_id":null,"shortname":"NTUB","name":"NTUB","formal_name":null,"country":"TWN","url":null,"twitter_hashtag":null}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})
	header, rows, err := scoreboard.Download(context.Background(), c, "2")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	assertStringSlice(t, header, []string{"Rank", "TeamAffiliation", "TeamName", "SolvedCount", "Score", "A"})
	assertStringSlice(t, rows[0], []string{"1", "NTUB", "Alpha", "1", "30", "1 30"})

	if contests.Load() != 1 || snapshots.Load() != 1 || teams.Load() != 1 || orgs.Load() != 1 {
		t.Fatalf("endpoint hits contests=%d scoreboard=%d teams=%d orgs=%d, want 1 each",
			contests.Load(), snapshots.Load(), teams.Load(), orgs.Load())
	}
}

func TestDownload_NonOKErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	if _, _, err := scoreboard.Download(context.Background(), c, "2"); err == nil {
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

	_, _, err := scoreboard.Download(ctx, c, "2")
	if err == nil {
		t.Fatal("expected ctx cancelled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}
