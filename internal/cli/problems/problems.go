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
	NewGateway func(ctx context.Context, cfg *config.DomServerClient) (web.Gateway, error)
}

// DefaultDeps wires Deps with the production gateway constructor.
// cmd/domjudge-tool's blank import calls register(), which uses
// these defaults.
func DefaultDeps() Deps {
	return Deps{NewGateway: defaultNewGateway}
}

func defaultNewGateway(ctx context.Context, cfg *config.DomServerClient) (web.Gateway, error) {
	return gatewayfactory.New(ctx, cfg)
}
