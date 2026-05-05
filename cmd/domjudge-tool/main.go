// Package main is the entry point for the domjudge-tool CLI.
//
// All business logic lives under internal/cli; this file only builds
// a signal-aware ctx (SIGINT, SIGTERM) and hands off to cli.Execute.
// Pre-PR-7 the binary exposes the general + users command groups; PR-7
// adds the remaining four groups (problems, submissions, emails,
// scoreboard).
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	// Blank imports trigger each subtree's init(), which calls
	// cli.RegisterSubtree to attach its commands to the root. The
	// registry pattern keeps cli a leaf package (no imports of
	// cli/{group}) so each group can depend on cli for ctx helpers.
	"github.com/ntub/domjudge-tool/internal/cli"
	_ "github.com/ntub/domjudge-tool/internal/cli/emails"
	_ "github.com/ntub/domjudge-tool/internal/cli/general"
	_ "github.com/ntub/domjudge-tool/internal/cli/problems"
	_ "github.com/ntub/domjudge-tool/internal/cli/scoreboard"
	_ "github.com/ntub/domjudge-tool/internal/cli/submissions"
	_ "github.com/ntub/domjudge-tool/internal/cli/users"
)

func main() {
	os.Exit(run())
}

// run separates os.Exit from defer cleanup so signal-handler teardown
// always runs (gocritic exitAfterDefer).
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Execute(ctx); err != nil {
		return 1
	}
	return 0
}
