package v8

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

func TestDownload_StreamsProblemExportBody(t *testing.T) {
	want := bytes.Repeat([]byte{0x50, 0x4b, 0x03, 0x04}, 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jury/problems/1/export" {
			t.Errorf("path = %q, want /jury/problems/1/export", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rc, err := c.Download(context.Background(), "/jury/problems/1/export")
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

func TestDownload_NonOKWrapsHTTPSentinels(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   error
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: web.ErrUnauthorized},
		{name: "server-error", status: http.StatusInternalServerError, want: web.ErrServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			c := newTestClient(t, srv)
			rc, err := c.Download(context.Background(), "/jury/problems/1/export")
			if rc != nil {
				_ = rc.Close()
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("errors.Is(err, %v) = false; err = %v", tc.want, err)
			}
		})
	}
}
