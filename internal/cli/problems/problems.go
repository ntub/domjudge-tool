// Package problems implements the "Problem Commands:" subtree of the
// domjudge-tool CLI: download-problems.
//
// The subtree depends on web.Gateway for both listing problems
// (GetProblems) and streaming the export ZIP for each one (Download,
// added in PR-7). The Deps factory mirrors cli/users so tests can
// swap in a stub gateway.
package problems

import (
	"context"
	"time"

	"github.com/ntub/domjudge-tool/internal/client/httpx"
	"github.com/ntub/domjudge-tool/internal/client/web"
	v7 "github.com/ntub/domjudge-tool/internal/client/web/v7"
	"github.com/ntub/domjudge-tool/internal/config"
)

// groupID matches the "problems" group registered in cli.NewRootCmd.
// docs/phase1/README.md §"Command tree shape" is the canonical
// source of truth.
const groupID = "problems"

// Deps groups the gateway factory the problems-subtree commands
// need. The default implementation builds a real v7 gateway; tests
// inject a stub returning the same web.Gateway interface.
type Deps struct {
	NewGateway func(ctx context.Context, cfg *config.DomServerClient) (web.Gateway, error)
}

// DefaultDeps wires Deps with the production v7 gateway constructor.
// cmd/domjudge-tool's blank import calls register(), which uses
// these defaults.
func DefaultDeps() Deps {
	return Deps{NewGateway: defaultNewGateway}
}

func defaultNewGateway(_ context.Context, cfg *config.DomServerClient) (web.Gateway, error) {
	session, err := httpx.NewSession(httpx.SessionOptions{
		BaseURL:    cfg.Host,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	if err != nil {
		return nil, err
	}
	return v7.New(v7.Options{
		Session:  session,
		Username: cfg.Username,
		Password: cfg.Password,
	})
}

func derefSecondsDuration(p *float64) time.Duration {
	if p == nil {
		return 0
	}
	return time.Duration(*p * float64(time.Second))
}
