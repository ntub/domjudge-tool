package v8

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

func TestGetProblems_HappyPath_ParsesV8TableAndExportPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jury/problems" {
			t.Errorf("path = %q, want /jury/problems", r.URL.Path)
		}
		_, _ = w.Write(mustReadFixture(t, problemsFixture))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.GetProblems(context.Background(), web.ProblemFilter{})
	if err != nil {
		t.Fatalf("GetProblems: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].ID != "1" {
		t.Errorf("got[0].ID = %q, want 1", got[0].ID)
	}
	if got[0].Name != "Hello World" {
		t.Errorf("got[0].Name = %q, want Hello World", got[0].Name)
	}
	if got[0].TimeLimit != 5 {
		t.Errorf("got[0].TimeLimit = %d, want 5", got[0].TimeLimit)
	}
	if got[0].TestDataCount != 1 {
		t.Errorf("got[0].TestDataCount = %d, want 1", got[0].TestDataCount)
	}
	if got[0].ExportFilePath == nil || *got[0].ExportFilePath != "/jury/problems/1/export" {
		t.Errorf("got[0].ExportFilePath = %v, want /jury/problems/1/export", got[0].ExportFilePath)
	}
}

func TestGetProblems_FiltersAndNilSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(mustReadFixture(t, problemsFixture))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	only, err := c.GetProblems(context.Background(), web.ProblemFilter{Only: []string{"2"}})
	if err != nil {
		t.Fatalf("GetProblems only: %v", err)
	}
	if len(only) != 1 || only[0].ID != "2" {
		t.Fatalf("only result = %+v, want only ID 2", only)
	}

	excluded, err := c.GetProblems(context.Background(), web.ProblemFilter{Exclude: []string{"2"}})
	if err != nil {
		t.Fatalf("GetProblems exclude: %v", err)
	}
	if len(excluded) != 2 {
		t.Fatalf("exclude len = %d, want 2", len(excluded))
	}
	for _, p := range excluded {
		if p.ID == "2" {
			t.Fatalf("ID 2 should be excluded, got %+v", excluded)
		}
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GetProblems panicked with nil filters: %v", r)
		}
	}()
	nilSafe, err := c.GetProblems(context.Background(), web.ProblemFilter{Only: nil, Exclude: nil})
	if err != nil {
		t.Fatalf("GetProblems nil-safe: %v", err)
	}
	if len(nilSafe) != 3 {
		t.Errorf("nil-safe len = %d, want 3", len(nilSafe))
	}
}

func TestGetProblems_500_WrapsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetProblems(context.Background(), web.ProblemFilter{})
	if !errors.Is(err, web.ErrServerError) {
		t.Errorf("errors.Is(err, web.ErrServerError) = false; err = %v", err)
	}
}
