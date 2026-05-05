package users

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/importer"
)

// NewImportUsersTeamsExampleCmd builds the `import-users-teams-example`
// subcommand: writes the canonical CSV template (byte-identical to the
// Python tool's templates/csv/import-users-teams.csv) to a file. By
// default the file lands at ./import-users-teams.csv (Python parity);
// --output overrides the path.
func NewImportUsersTeamsExampleCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:           "import-users-teams-example",
		Short:         "Write the import-users-teams CSV template",
		GroupID:       groupID,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := os.Create(output)
			if err != nil {
				return fmt.Errorf("import-users-teams-example: create %s: %w", output, err)
			}
			defer func() { _ = f.Close() }()
			if err := importer.TemplateCSV(f); err != nil {
				return fmt.Errorf("import-users-teams-example: write template: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), output)
			return err
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "import-users-teams.csv", "output path for the template")
	return cmd
}
