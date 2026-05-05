package emails

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/email"
)

// NewTemplateExampleCmd builds the `email-template-example` subcommand,
// which writes a ready-to-edit template directory containing all six
// fixed placeholders.
func NewTemplateExampleCmd() *cobra.Command {
	var (
		output string
		force  bool
	)
	cmd := &cobra.Command{
		Use:           "email-template-example",
		Short:         "Write an example email template directory with all six placeholders",
		GroupID:       groupID,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := email.WriteTemplateScaffold(output, force); err != nil {
				if errors.Is(err, email.ErrTemplateDirExists) {
					return fmt.Errorf("email-template-example: %w (use --force to overwrite)", err)
				}
				return fmt.Errorf("email-template-example: %w", err)
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), output)
			return err
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&output, "output", "o", "./email-template", "output directory for the template")
	flags.BoolVarP(&force, "force", "f", false, "overwrite existing template files")
	return cmd
}
