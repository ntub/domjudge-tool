package general

import (
	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
)

// init registers the general subtree with the cli package's central
// registry. Triggered when cmd/domjudge-tool/main.go imports this
// package (blank import). CLAUDE.md "Things to avoid" allows init()
// for this exact pattern (registry wiring only).
func init() { //nolint:gochecknoinits // registry wiring per CLAUDE.md
	cli.RegisterSubtree(register)
}

func register(root *cobra.Command) {
	root.AddCommand(NewGenConfigCmd())
	root.AddCommand(NewVersionCmd())
}
