// Package problems implements the "Problem Commands:" subtree of the
// domjudge-tool CLI: download-problems and import-problems.
//
// The subtree depends on web.Gateway for scraper-backed listing and
// download, and on api/v4 for problem package import. The Deps factory
// mirrors cli/users so tests can swap in stubs.
package problems

import (
	"context"
	"time"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/client/web/gatewayfactory"
	"github.com/ntub/domjudge-tool/internal/config"
)

// groupID matches the "problems" group registered in cli.NewRootCmd.
// docs/phase1/README.md §"Command tree shape" is the canonical
// source of truth.
const groupID = "problems"

// Deps groups the gateway factory the problems-subtree commands
// need. The default implementation builds a real version-aware gateway;
// tests inject a stub returning the same web.Gateway interface.
type Deps struct {
	NewAPI     func(cfg *config.DomServerClient) *apiv4.Client
	NewGateway func(ctx context.Context, cfg *config.DomServerClient) (web.Gateway, error)
}

// DefaultDeps wires Deps with the production gateway constructor.
// cmd/domjudge-tool's blank import calls register(), which uses
// these defaults.
func DefaultDeps() Deps {
	return Deps{
		NewAPI:     defaultNewAPI,
		NewGateway: defaultNewGateway,
	}
}

func defaultNewAPI(cfg *config.DomServerClient) *apiv4.Client {
	return apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
}

func defaultNewGateway(ctx context.Context, cfg *config.DomServerClient) (web.Gateway, error) {
	return gatewayfactory.New(ctx, cfg)
}

func derefSecondsDuration(p *float64) time.Duration {
	if p == nil {
		return 0
	}
	return time.Duration(*p * float64(time.Second))
}
