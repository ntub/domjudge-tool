package emails

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/email"
)

// NewTemplateValidateCmd builds the `email-template-validate <dir>`
// subcommand. It loads the template via email.LoadTemplate and runs
// Template.Validate, reporting either "template OK: <dir>" on success
// or a wrapped error on failure.
func NewTemplateValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "email-template-validate <dir>",
		Short:         "Validate an email template directory",
		GroupID:       groupID,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			tmpl, err := email.LoadTemplate(dir)
			if err != nil {
				return fmt.Errorf("email-template-validate: %w", err)
			}
			if err := tmpl.Validate(); err != nil {
				return fmt.Errorf("email-template-validate: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "template OK: %s\n", dir)
			return err
		},
	}
}
