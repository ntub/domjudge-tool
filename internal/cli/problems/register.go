package problems

import (
	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
)

// init registers the problems subtree with cli's central registry.
// Triggered when cmd/domjudge-tool/main.go imports this package
// (blank import).
func init() { //nolint:gochecknoinits // registry wiring per CLAUDE.md
	cli.RegisterSubtree(register)
}

func register(root *cobra.Command) {
	root.AddCommand(NewDownloadProblemsCmd(DefaultDeps()))
}
