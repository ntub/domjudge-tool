package scoreboard

import (
	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
)

// init registers the scoreboard subtree with cli's central registry.
func init() { //nolint:gochecknoinits // registry wiring per CLAUDE.md
	cli.RegisterSubtree(register)
}

func register(root *cobra.Command) {
	root.AddCommand(NewDownloadScoreboardCmd(DefaultDeps()))
}
