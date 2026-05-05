package v4

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
)

func TestUsersResource_List(t *testing.T) {
	listFixturePath := "../../../../testdata/api/v4/users_list.json"

	listFixture, err := os.ReadFile(listFixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	type wantFn func(t *testing.T, got []model.User, err error)

	tests := []struct {
		name       string
		opts       UsersListOpts
		handler    http.HandlerFunc
		preCancel  bool
		assertions wantFn
	}{
		{
			name: "happy path",
			opts: UsersListOpts{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/api/v4/users") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(listFixture)
			},
			assertions: func(t *testing.T, got []model.User, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(got) != 1 {
					t.Fatalf("got %d users, want 1", len(got))
				}
				if got[0].Username != "testuser" {
					t.Errorf("got username %q, want %q", got[0].Username, "testuser")
				}
			},
		},
		{
			name: "empty opts no query string",
			opts: UsersListOpts{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.RawQuery != "" {
					t.Errorf("got raw query %q, want empty", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
			},
			assertions: func(t *testing.T, _ []model.User, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "all opts query string",
			opts: UsersListOpts{IDs: []string{"1", "2"}, TeamID: strPtr("1")},
			handler: func(w http.ResponseWriter, r *http.Request) {
				vals, err := url.ParseQuery(r.URL.RawQuery)
				if err != nil {
					t.Fatalf("parse query: %v", err)
				}
				if got := vals["ids[]"]; !reflect.DeepEqual(got, []string{"1", "2"}) {
					t.Errorf("got ids[]=%v, want [1 2]", got)
				}
				if got := vals.Get("team_id"); got != "1" {
					t.Errorf("got team_id=%q, want %q", got, "1")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
			},
			assertions: func(t *testing.T, _ []model.User, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "401 unauthorized",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			assertions: func(t *testing.T, _ []model.User, err error) {
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
			assertions: func(t *testing.T, _ []model.User, err error) {
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
			assertions: func(t *testing.T, _ []model.User, err error) {
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
			assertions: func(t *testing.T, _ []model.User, err error) {
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
			assertions: func(t *testing.T, _ []model.User, err error) {
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

			got, err := c.Users.List(ctx, tc.opts)
			tc.assertions(t, got, err)
		})
	}
}

func TestUsersResource_Get(t *testing.T) {
	userFixturePath := "../../../../testdata/api/v4/user.json"

	userFixture, err := os.ReadFile(userFixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	type wantFn func(t *testing.T, got *model.User, err error)

	tests := []struct {
		name       string
		id         string
		handler    http.HandlerFunc
		preCancel  bool
		assertions wantFn
	}{
		{
			name: "happy path",
			id:   "1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/users/1") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(userFixture)
			},
			assertions: func(t *testing.T, got *model.User, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got == nil {
					t.Fatalf("got nil user, want non-nil")
				}
				if got.ID != "1" {
					t.Errorf("got id %q, want %q", got.ID, "1")
				}
				if got.Username != "testuser" {
					t.Errorf("got username %q, want %q", got.Username, "testuser")
				}
			},
		},
		{
			name: "401",
			id:   "1",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			assertions: func(t *testing.T, _ *model.User, err error) {
				if !errors.Is(err, ErrUnauthorized) {
					t.Errorf("got err=%v, want ErrUnauthorized", err)
				}
			},
		},
		{
			name: "404",
			id:   "1",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			assertions: func(t *testing.T, _ *model.User, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("got err=%v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "500",
			id:   "1",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			assertions: func(t *testing.T, _ *model.User, err error) {
				if !errors.Is(err, ErrServerError) {
					t.Errorf("got err=%v, want ErrServerError", err)
				}
			},
		},
		{
			name: "malformed json",
			id:   "1",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{bad json`))
			},
			assertions: func(t *testing.T, _ *model.User, err error) {
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
			id:   "1",
			handler: func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-make(chan struct{}):
				}
			},
			preCancel: true,
			assertions: func(t *testing.T, _ *model.User, err error) {
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

			got, err := c.Users.Get(ctx, tc.id)
			tc.assertions(t, got, err)
		})
	}
}
