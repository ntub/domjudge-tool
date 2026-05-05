package v7

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// TestClient_Download_StreamsBody verifies the binary body returned by
// Download is byte-identical to what the server wrote.
func TestClient_Download_StreamsBody(t *testing.T) {
	want := bytes.Repeat([]byte{0xDE, 0xAD, 0xBE, 0xEF}, 256)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jury/problems/42/export" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rc, err := c.Download(context.Background(), "/jury/problems/42/export")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("body mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

// TestClient_Download_NonOKWrapsHTTPError checks that 4xx/5xx responses
// surface as *HTTPError so callers can errors.Is against the package
// sentinels.
func TestClient_Download_NonOKWrapsHTTPError(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{"not-found", http.StatusNotFound},
		{"internal-error", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			c := newTestClient(t, srv)
			rc, err := c.Download(context.Background(), "/missing")
			if rc != nil {
				rc.Close()
			}
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("error is not *HTTPError: %v", err)
			}
			if httpErr.Status != tc.status {
				t.Errorf("Status = %d, want %d", httpErr.Status, tc.status)
			}
		})
	}
}

func TestClient_Download_RedirectsToLogin_ReturnsErrLoginFailed(t *testing.T) {
	srv := loginRedirectMux(t)
	defer srv.Close()
	c := newTestClient(t, srv)

	rc, err := c.Download(context.Background(), "/jury/problems/42/export")
	if rc != nil {
		rc.Close()
	}
	if !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
}

// TestClient_Download_CtxCancelled exercises the cancellation path.
func TestClient_Download_CtxCancelled(t *testing.T) {
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

	rc, err := c.Download(ctx, "/anything")
	if rc != nil {
		rc.Close()
	}
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}
