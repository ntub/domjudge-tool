package gatewayfactory

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

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

func TestNew_V7DoesNotPassContestID(t *testing.T) {
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
			t.Fatalf("unexpected domjudge_cid cookie: %s=%s", cookie.Name, cookie.Value)
		}
	}
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
