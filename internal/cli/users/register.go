package users

import (
	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
)

// init registers the users subtree with the cli package's central
// registry. CLAUDE.md "Things to avoid" allows init() solely for this
// pattern.
func init() { //nolint:gochecknoinits // registry wiring per CLAUDE.md
	cli.RegisterSubtree(register)
}

func register(root *cobra.Command) {
	deps := DefaultDeps()
	root.AddCommand(NewImportUsersTeamsCmd(deps))
	root.AddCommand(NewImportUsersTeamsExampleCmd())
	root.AddCommand(NewExportUsersCmd(deps))
	root.AddCommand(NewRmTeamsAndUsersCmd(deps))
	root.AddCommand(NewSetPasswordCmd(deps))
}
