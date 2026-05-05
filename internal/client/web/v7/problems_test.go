package v7

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

const problemsListFixture = "../../../../testdata/web/v7/jury_problems_list.html"

func TestGetProblems_HappyPath(t *testing.T) {
	srv := serveFixture(t, problemsListFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.GetProblems(context.Background(), ProblemFilter{})
	if err != nil {
		t.Fatalf("GetProblems: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// Row 1 in the fixture: id=1, name=Hello World, time_limit=5, test_data_count=1.
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

func TestGetProblems_OnlyFilter(t *testing.T) {
	srv := serveFixture(t, problemsListFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.GetProblems(context.Background(), ProblemFilter{Only: []string{"2"}})
	if err != nil {
		t.Fatalf("GetProblems: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != "2" {
		t.Errorf("got[0].ID = %q, want 2", got[0].ID)
	}
}

func TestGetProblems_ExcludeFilter(t *testing.T) {
	srv := serveFixture(t, problemsListFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.GetProblems(context.Background(), ProblemFilter{Exclude: []string{"2"}})
	if err != nil {
		t.Fatalf("GetProblems: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, p := range got {
		if p.ID == "2" {
			t.Errorf("id 2 should be excluded, got %+v", p)
		}
	}
}

// TestGetProblems_NilOnlyAndNilExclude_DoesNotPanic locks the Python
// regression at __init__.py:21 (no flags -> Python crashes on None membership
// test). The Go port must treat nil filters as empty.
func TestGetProblems_NilOnlyAndNilExclude_DoesNotPanic(t *testing.T) {
	srv := serveFixture(t, problemsListFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GetProblems panicked: %v", r)
		}
	}()

	got, err := c.GetProblems(context.Background(), ProblemFilter{Only: nil, Exclude: nil})
	if err != nil {
		t.Fatalf("GetProblems: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len = %d, want 3 (nil filters return all rows)", len(got))
	}
}

func TestGetProblems_500_WrapsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	_, err := c.GetProblems(context.Background(), ProblemFilter{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrServerError) {
		t.Errorf("errors.Is(err, web.ErrServerError) = false; err = %v", err)
	}
}

func TestGetProblems_ContextCancelled(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-block:
		}
	}))
	defer srv.Close()
	defer close(block)

	c := newTestClient(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetProblems(ctx, ProblemFilter{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}

func TestGetProblems_EmptyTable_EmptySlice(t *testing.T) {
	srv := serveFixture(t, tbodyEmptyFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.GetProblems(context.Background(), ProblemFilter{})
	if err != nil {
		t.Fatalf("GetProblems: %v", err)
	}
	if got == nil {
		t.Fatalf("got nil, want empty slice")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}
