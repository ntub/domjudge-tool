package v4

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGeneralResource_Version(t *testing.T) {
	fixturePath := "../../../../testdata/api/v4/version.json"

	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var parsed struct {
		APIVersion string `json:"api_version"`
	}
	if err := json.Unmarshal(fixture, &parsed); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	expectedVersion := parsed.APIVersion

	type wantFn func(t *testing.T, got string, err error)

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		preCancel  bool
		assertions wantFn
	}{
		{
			name: "happy path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/api/v4/version") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, got string, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != expectedVersion {
					t.Errorf("got version %q, want %q", got, expectedVersion)
				}
			},
		},
		{
			name: "401 unauthorized",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			assertions: func(t *testing.T, _ string, err error) {
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
			assertions: func(t *testing.T, _ string, err error) {
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
			assertions: func(t *testing.T, _ string, err error) {
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
			assertions: func(t *testing.T, _ string, err error) {
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
			assertions: func(t *testing.T, _ string, err error) {
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

			got, err := c.General.Version(ctx)
			tc.assertions(t, got, err)
		})
	}
}
