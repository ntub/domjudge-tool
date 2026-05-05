package httpx

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_BasicAuthHeaderOnEveryRequest(t *testing.T) {
	wantCreds := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))

	var seen atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if got != wantCreds {
			t.Errorf("Authorization header = %q, want %q", got, wantCreds)
		}
		seen.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := New(Options{Username: "admin", Password: "secret"})

	tests := []struct {
		name string
		call func(ctx context.Context, url string) (*http.Response, error)
	}{
		{name: "GET", call: c.GET},
		{name: "GETStream", call: c.GETStream},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := tc.call(context.Background(), srv.URL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want 200", resp.StatusCode)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if string(body) != "ok" {
				t.Errorf("body = %q, want %q", string(body), "ok")
			}
		})
	}

	if got := seen.Load(); got != 2 {
		t.Errorf("server saw %d requests, want 2", got)
	}
}

func TestClient_DisableSSLFlipsInsecureSkipVerify(t *testing.T) {
	tests := []struct {
		name       string
		disableSSL bool
		want       bool
	}{
		{name: "DisableSSL=true", disableSSL: true, want: true},
		{name: "DisableSSL=false", disableSSL: false, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := New(Options{DisableSSL: tc.disableSSL})
			tr := c.Transport()
			if tr == nil {
				t.Fatalf("Transport() returned nil")
			}
			if tr.TLSClientConfig == nil {
				t.Fatalf("TLSClientConfig is nil")
			}
			if got := tr.TLSClientConfig.InsecureSkipVerify; got != tc.want {
				t.Errorf("InsecureSkipVerify = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClient_ContextCancelled_AbortsRequest(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-block:
		}
	}))
	defer srv.Close()
	defer close(block)

	c := New(Options{Username: "admin", Password: "secret"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := c.GET(ctx, srv.URL)
	if err == nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "httpx: do request:") {
		t.Errorf("error %q does not contain %q", err.Error(), "httpx: do request:")
	}
}

func TestClient_TimeoutHonoured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{Username: "admin", Password: "secret", Timeout: 50 * time.Millisecond})

	resp, err := c.GET(context.Background(), srv.URL)
	if err == nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Fatalf("expected timeout error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "timeout") &&
		!strings.Contains(msg, "deadline exceeded") &&
		!errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not look like a timeout", msg)
	}
}
