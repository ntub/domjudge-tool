package gatewayfactory

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ntub/domjudge-tool/internal/client/httpx"
	"github.com/ntub/domjudge-tool/internal/config"
)

func TestNew_SelectsGatewayByVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		wantType string
	}{
		{name: "v7 default", version: "7.3.2", wantType: "*v7.Client"},
		{name: "v7 patch", version: "7.3.4", wantType: "*v7.Client"},
		{name: "v8", version: "8.3.1", wantType: "*v8.Client"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gw, err := New(context.Background(), cfgWithVersion(tc.version))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if got := fmt.Sprintf("%T", gw); got != tc.wantType {
				t.Fatalf("gateway type = %q, want %q", got, tc.wantType)
			}
		})
	}
}

func TestNew_UnsupportedVersionReturnsSentinel(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{name: "unknown major", version: "9.0.0"},
		{name: "v7 typo", version: "7.30.0"},
		{name: "v8 typo", version: "8.31"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(context.Background(), cfgWithVersion(tc.version))
			if !errors.Is(err, ErrUnsupportedVersion) {
				t.Fatalf("errors.Is(err, ErrUnsupportedVersion) = false; err = %v", err)
			}
			if !strings.Contains(err.Error(), tc.version) {
				t.Fatalf("err = %q, want it to name the unsupported version %q", err, tc.version)
			}
		})
	}
}

func TestNew_V7PassesContestID(t *testing.T) {
	gw, err := New(context.Background(), cfgWithVersion("7.3.2"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sessioned, ok := gw.(interface {
		Session() *httpx.SessionClient
	})
	if !ok {
		t.Fatalf("gateway %T does not expose Session", gw)
	}
	root, err := url.Parse(sessioned.Session().BaseURL())
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}
	root.Path = "/"
	for _, cookie := range sessioned.Session().Jar().Cookies(root) {
		if cookie.Name == "domjudge_cid" {
			if cookie.Value != "2" {
				t.Fatalf("domjudge_cid = %q, want 2", cookie.Value)
			}
			return
		}
	}
	t.Fatal("domjudge_cid cookie missing")
}

func cfgWithVersion(version string) *config.DomServerClient {
	contestID := "2"
	return &config.DomServerClient{
		Host:      "http://example.org",
		Username:  "jury",
		Password:  "secret",
		Version:   version,
		ContestID: &contestID,
	}
}

func TestNew_NilConfig(t *testing.T) {
	_, err := New(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
}

func TestNew_InvalidHost(t *testing.T) {
	cfg := cfgWithVersion("8.3.1")
	cfg.Host = "://invalid-url"
	_, err := New(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for invalid host, got nil")
	}
}

func TestNew_ConfiguredTimeoutAbortsRequests_Integration(t *testing.T) {
	for _, version := range []string{"7.3.2", "8.3.1"} {
		t.Run(version, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/slow" {
					t.Errorf("path = %q, want /slow", r.URL.Path)
				}
				<-r.Context().Done()
			}))
			t.Cleanup(srv.Close)

			timeout := 0.05
			cfg := &config.DomServerClient{
				Host:     srv.URL,
				Username: "jury",
				Password: "secret",
				Version:  version,
				Timeout:  &timeout,
			}
			gw, err := New(context.Background(), cfg)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			sessioned, ok := gw.(interface {
				Session() *httpx.SessionClient
			})
			if !ok {
				t.Fatalf("gateway %T does not expose Session", gw)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			t.Cleanup(cancel)
			resp, err := sessioned.Session().GET(ctx, "/slow")
			if resp != nil {
				_ = resp.Body.Close()
			}
			var timeoutErr net.Error
			if !errors.As(err, &timeoutErr) || !timeoutErr.Timeout() {
				t.Fatalf("request error = %v, want HTTP client timeout", err)
			}
			if ctx.Err() != nil {
				t.Fatalf("request lasted until the fallback context deadline; configured timeout was not applied: %v", err)
			}
		})
	}
}
