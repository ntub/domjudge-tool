// Package gatewayfactory builds version-aware web gateways from config.
package gatewayfactory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ntub/domjudge-tool/internal/client/httpx"
	"github.com/ntub/domjudge-tool/internal/client/web"
	v7 "github.com/ntub/domjudge-tool/internal/client/web/v7"
	v8 "github.com/ntub/domjudge-tool/internal/client/web/v8"
	"github.com/ntub/domjudge-tool/internal/config"
)

// ErrUnsupportedVersion is returned when cfg.Version does not map to a
// supported DOMjudge web scraper.
var ErrUnsupportedVersion = errors.New("gatewayfactory: unsupported DOMjudge version")

// New constructs the web.Gateway implementation selected by cfg.Version.
//
// v8 import create/update/email/password/affiliation is completed in P2-4
// (importer routes these to the v4 REST API); until then a v8 import that
// creates or updates rows will fail with ErrUnsupportedOnV8. download-problems
// and rm-teams-and-users work on v8 today.
func New(_ context.Context, cfg *config.DomServerClient) (web.Gateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("gatewayfactory: config is nil")
	}
	family := versionFamily(cfg.Version)
	if family == "" {
		return nil, fmt.Errorf("gatewayfactory: version %q: %w", cfg.Version, ErrUnsupportedVersion)
	}
	session, err := newSession(cfg)
	if err != nil {
		return nil, err
	}
	switch family {
	case "7.3":
		gw, err := v7.New(v7.Options{
			Session:  session,
			Username: cfg.Username,
			Password: cfg.Password,
		})
		if err != nil {
			return nil, fmt.Errorf("gatewayfactory: new v7 gateway: %w", err)
		}
		return gw, nil
	case "8.3":
		gw, err := v8.New(v8.Options{
			Session:  session,
			Username: cfg.Username,
			Password: cfg.Password,
		})
		if err != nil {
			return nil, fmt.Errorf("gatewayfactory: new v8 gateway: %w", err)
		}
		return gw, nil
	}
	return nil, fmt.Errorf("gatewayfactory: version %q: %w", cfg.Version, ErrUnsupportedVersion)
}

func versionFamily(version string) string {
	switch {
	case version == "7.3" || strings.HasPrefix(version, "7.3."):
		return "7.3"
	case version == "8.3" || strings.HasPrefix(version, "8.3."):
		return "8.3"
	default:
		return ""
	}
}

func newSession(cfg *config.DomServerClient) (*httpx.SessionClient, error) {
	session, err := httpx.NewSession(httpx.SessionOptions{
		BaseURL:    cfg.Host,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	if err != nil {
		return nil, fmt.Errorf("gatewayfactory: new session: %w", err)
	}
	return session, nil
}

func derefSecondsDuration(p *float64) time.Duration {
	if p == nil {
		return 0
	}
	return time.Duration(*p * float64(time.Second))
}
