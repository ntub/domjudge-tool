package v4

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
)

func TestJudgementTypesResource_List(t *testing.T) {
	fixturePath := "../../../../testdata/api/v4/judgement_types_list.json"
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	type wantFn func(t *testing.T, got []model.JudgementType, err error)

	tests := []struct {
		name       string
		opts       JudgementTypesListOpts
		handler    http.HandlerFunc
		preCancel  bool
		assertions wantFn
	}{
		{
			name: "happy path",
			opts: JudgementTypesListOpts{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/api/v4/contests/1/judgement-types") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, got []model.JudgementType, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(got) != 2 {
					t.Fatalf("got len=%d, want 2", len(got))
				}
				if got[0].ID != "AC" {
					t.Errorf("got ID=%q, want %q", got[0].ID, "AC")
				}
			},
		},
		{
			name: "empty opts no query string",
			opts: JudgementTypesListOpts{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.RawQuery != "" {
					t.Errorf("expected empty query string, got %q", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, _ []model.JudgementType, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "all opts query string",
			opts: JudgementTypesListOpts{
				IDs:    []string{"AC", "WA"},
				Strict: boolPtr(true),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				vals, perr := url.ParseQuery(r.URL.RawQuery)
				if perr != nil {
					t.Fatalf("parse query: %v", perr)
				}
				ids := vals["ids[]"]
				if len(ids) != 2 || ids[0] != "AC" || ids[1] != "WA" {
					t.Errorf("got ids[]=%v, want [AC WA]", ids)
				}
				if got := vals.Get("strict"); got != "true" {
					t.Errorf("got strict=%q, want %q", got, "true")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, _ []model.JudgementType, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "401 unauthorized",
			opts: JudgementTypesListOpts{},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			assertions: func(t *testing.T, _ []model.JudgementType, err error) {
				if !errors.Is(err, ErrUnauthorized) {
					t.Errorf("got err=%v, want ErrUnauthorized", err)
				}
			},
		},
		{
			name: "404 not found",
			opts: JudgementTypesListOpts{},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			assertions: func(t *testing.T, _ []model.JudgementType, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("got err=%v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "500 server error",
			opts: JudgementTypesListOpts{},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			assertions: func(t *testing.T, _ []model.JudgementType, err error) {
				if !errors.Is(err, ErrServerError) {
					t.Errorf("got err=%v, want ErrServerError", err)
				}
			},
		},
		{
			name: "malformed json",
			opts: JudgementTypesListOpts{},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{bad json`))
			},
			assertions: func(t *testing.T, _ []model.JudgementType, err error) {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "decode") {
					t.Errorf("error %q does not contain %q", err.Error(), "decode")
				}
			},
		},
		{
			name: "context cancelled",
			opts: JudgementTypesListOpts{},
			handler: func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-make(chan struct{}):
				}
			},
			preCancel: true,
			assertions: func(t *testing.T, _ []model.JudgementType, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Errorf("got err=%v, want context.Canceled", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})

			ctx := context.Background()
			if tc.preCancel {
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cctx
			}

			got, err := c.JudgementTypes.List(ctx, "1", tc.opts)
			tc.assertions(t, got, err)
		})
	}
}

func TestJudgementTypesResource_Get(t *testing.T) {
	fixturePath := "../../../../testdata/api/v4/judgement_type.json"
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	type wantFn func(t *testing.T, got *model.JudgementType, err error)

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		preCancel  bool
		assertions wantFn
	}{
		{
			name: "happy path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/api/v4/contests/1/judgement-types/AC") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, got *model.JudgementType, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got == nil {
					t.Fatalf("got nil result")
				}
				if got.ID != "AC" {
					t.Errorf("got ID=%q, want %q", got.ID, "AC")
				}
				if !got.Solved {
					t.Errorf("got Solved=false, want true")
				}
			},
		},
		{
			name: "401 unauthorized",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			assertions: func(t *testing.T, _ *model.JudgementType, err error) {
				if !errors.Is(err, ErrUnauthorized) {
					t.Errorf("got err=%v, want ErrUnauthorized", err)
				}
			},
		},
		{
			name: "404 not found",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			assertions: func(t *testing.T, _ *model.JudgementType, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("got err=%v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "500 server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			assertions: func(t *testing.T, _ *model.JudgementType, err error) {
				if !errors.Is(err, ErrServerError) {
					t.Errorf("got err=%v, want ErrServerError", err)
				}
			},
		},
		{
			name: "malformed json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{bad json`))
			},
			assertions: func(t *testing.T, _ *model.JudgementType, err error) {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "decode") {
					t.Errorf("error %q does not contain %q", err.Error(), "decode")
				}
			},
		},
		{
			name: "context cancelled",
			handler: func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-make(chan struct{}):
				}
			},
			preCancel: true,
			assertions: func(t *testing.T, _ *model.JudgementType, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Errorf("got err=%v, want context.Canceled", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})

			ctx := context.Background()
			if tc.preCancel {
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cctx
			}

			got, err := c.JudgementTypes.Get(ctx, "1", "AC")
			tc.assertions(t, got, err)
		})
	}
}
