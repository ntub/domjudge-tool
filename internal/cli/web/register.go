package web

import (
	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
)

// init registers the web command with the cli package's central registry.
// AGENTS.md allows init() for this registry wiring only.
func init() { //nolint:gochecknoinits // registry wiring per AGENTS.md
	cli.RegisterSubtree(register)
}

func register(root *cobra.Command) {
	root.AddCommand(NewWebCmd())
}
