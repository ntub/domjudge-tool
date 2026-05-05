package submissions

import (
	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
)

// init registers the submissions subtree with cli's central registry.
// Triggered by cmd/domjudge-tool/main.go's blank import.
func init() { //nolint:gochecknoinits // registry wiring per CLAUDE.md
	cli.RegisterSubtree(register)
}

func register(root *cobra.Command) {
	root.AddCommand(NewDownloadSubmissionsCmd(DefaultDeps()))
}
