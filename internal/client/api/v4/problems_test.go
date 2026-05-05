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

func TestProblemsResource_List(t *testing.T) {
	problemsListJSON, err := os.ReadFile("../../../../testdata/api/v4/problems_list.json")
	if err != nil {
		t.Fatalf("read problems_list.json fixture: %v", err)
	}

	cases := []struct {
		name    string
		handler http.HandlerFunc
		ctxFn   func() (context.Context, context.CancelFunc)
		check   func(t *testing.T, problems []model.Problem, err error)
	}{
		{
			name: "happy path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(problemsListJSON)
			},
			check: func(t *testing.T, problems []model.Problem, err error) {
				if err != nil {
					t.Fatalf("List returned error: %v", err)
				}
				if got, want := len(problems), 1; got != want {
					t.Fatalf("len(problems) = %d, want %d", got, want)
				}
				if got, want := problems[0].Name, "Problem A"; got != want {
					t.Fatalf("problems[0].Name = %q, want %q", got, want)
				}
			},
		},
		{
			name: "path contains cid",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, "/contests/1/problems") {
					http.Error(w, "unexpected path: "+r.URL.Path, http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(problemsListJSON)
			},
			check: func(t *testing.T, problems []model.Problem, err error) {
				if err != nil {
					t.Fatalf("List returned error: %v", err)
				}
				if len(problems) != 1 {
					t.Fatalf("len(problems) = %d, want 1", len(problems))
				}
			},
		},
		{
			name: "401",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			},
			check: func(t *testing.T, _ []model.Problem, err error) {
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
			check: func(t *testing.T, _ []model.Problem, err error) {
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
			check: func(t *testing.T, _ []model.Problem, err error) {
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
			check: func(t *testing.T, _ []model.Problem, err error) {
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
				_, _ = w.Write(problemsListJSON)
			},
			ctxFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			check: func(t *testing.T, _ []model.Problem, err error) {
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

			problems, err := c.Problems.List(ctx, "1")
			tc.check(t, problems, err)
		})
	}
}

func TestProblemsResource_Get(t *testing.T) {
	problemJSON, err := os.ReadFile("../../../../testdata/api/v4/problem.json")
	if err != nil {
		t.Fatalf("read problem.json fixture: %v", err)
	}

	cases := []struct {
		name    string
		handler http.HandlerFunc
		ctxFn   func() (context.Context, context.CancelFunc)
		check   func(t *testing.T, problem *model.Problem, err error)
	}{
		{
			name: "happy path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(problemJSON)
			},
			check: func(t *testing.T, problem *model.Problem, err error) {
				if err != nil {
					t.Fatalf("Get returned error: %v", err)
				}
				if problem == nil {
					t.Fatal("Get returned nil problem")
				}
				if got, want := problem.ID, "1"; got != want {
					t.Fatalf("problem.ID = %q, want %q", got, want)
				}
				if got, want := problem.ShortName, "A"; got != want {
					t.Fatalf("problem.ShortName = %q, want %q", got, want)
				}
			},
		},
		{
			name:    "path contains cid and id",
			handler: nil,
			check: func(t *testing.T, problem *model.Problem, err error) {
				if err != nil {
					t.Fatalf("Get returned error: %v", err)
				}
				if problem == nil || problem.ID != "1" {
					t.Fatalf("problem = %+v, want ID=1", problem)
				}
			},
		},
		{
			name: "401",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			},
			check: func(t *testing.T, _ *model.Problem, err error) {
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
			check: func(t *testing.T, _ *model.Problem, err error) {
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
			check: func(t *testing.T, _ *model.Problem, err error) {
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
			check: func(t *testing.T, _ *model.Problem, err error) {
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
				_, _ = w.Write(problemJSON)
			},
			ctxFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			check: func(t *testing.T, _ *model.Problem, err error) {
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
					if strings.Contains(r.URL.Path, "/contests/1/problems/1") {
						pathSeen = true
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(problemJSON)
				}))
				defer srv.Close()
				defer func() {
					if !pathSeen {
						t.Fatalf("expected request path to contain %q", "/contests/1/problems/1")
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

			problem, err := c.Problems.Get(ctx, "1", "1")
			tc.check(t, problem, err)
		})
	}
}
