package v4

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const contestsFixture = `[
  {"id":"1","shortname":"practice","name":"Practice Contest","state":{"started":"2026-01-01T00:00:00+00:00"}},
  {"id":"2","shortname":"main","name":"Main Contest","state":{"started":"2026-01-02T00:00:00+00:00"}}
]`

func TestContestsResource_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/contests" {
			t.Errorf("path = %q, want /api/v4/contests", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(contestsFixture))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	contests, err := c.Contests.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(contests) != 2 {
		t.Fatalf("len(contests) = %d, want 2", len(contests))
	}
	if contests[1].ShortName != "main" {
		t.Fatalf("ShortName = %q, want main", contests[1].ShortName)
	}
}

func TestContestsResource_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/contests/2" {
			t.Errorf("path = %q, want /api/v4/contests/2", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"2","shortname":"main","name":"Main Contest","state":{"started":"2026-01-02T00:00:00+00:00"}}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	contest, err := c.Contests.Get(context.Background(), "2")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if contest == nil || contest.ID != "2" {
		t.Fatalf("contest = %+v, want ID=2", contest)
	}
}

func TestContestsResource_Resolve_ByNumericID_PassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request for numeric resolve: %s", r.URL.Path)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	got, err := c.Contests.Resolve(context.Background(), "42")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "42" {
		t.Fatalf("Resolve = %q, want 42", got)
	}
}

func TestContestsResource_Resolve_ByShortname_ResolvesToID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(contestsFixture))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	got, err := c.Contests.Resolve(context.Background(), "main")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "2" {
		t.Fatalf("Resolve = %q, want 2", got)
	}
}

func TestContestsResource_Resolve_Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(contestsFixture))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	if _, err := c.Contests.Resolve(context.Background(), ""); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("empty hint err = %v, want ErrInvalidArgument", err)
	}
	if _, err := c.Contests.Resolve(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing hint err = %v, want ErrNotFound", err)
	}
}
