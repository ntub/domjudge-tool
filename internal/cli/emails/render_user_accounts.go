package emails

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/email"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/util/pathx"
)

// NewRenderUserAccountsCmd builds the
// `render-user-accounts <csv-file> <template-dir>` subcommand. It
// reuses the same template loading, CSV parsing, and message-build
// path as send-user-accounts, but writes each rendered RFC 5322
// message to <output-dir>/<safe-username>.eml instead of dialling
// SMTP.
func NewRenderUserAccountsCmd() *cobra.Command {
	var (
		outputDir string
		fromEmail string
		force     bool
	)
	cmd := &cobra.Command{
		Use:           "render-user-accounts <csv-file> <template-dir>",
		Short:         "Render templated credential emails to .eml files on disk",
		GroupID:       groupID,
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("render-user-accounts: config not on context (PreRunE missing)")
			}

			tmpl, err := email.LoadTemplate(args[1])
			if err != nil {
				return fmt.Errorf("render-user-accounts: template: %w", err)
			}
			if err := tmpl.Validate(); err != nil {
				return fmt.Errorf("render-user-accounts: template: %w", err)
			}

			csvFile, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("render-user-accounts: open csv: %w", err)
			}
			defer func() { _ = csvFile.Close() }()
			users, err := importer.ParseUsersCSV(csvFile)
			if err != nil {
				return fmt.Errorf("render-user-accounts: parse csv: %w", err)
			}

			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("render-user-accounts: mkdir: %w", err)
			}

			opts := email.SendOptions{FromEmail: fromEmail}
			for _, user := range users {
				name := pathx.SafeFilename(user.Username)
				if name == "" {
					name = "unknown"
				}
				destPath := filepath.Join(outputDir, name+".eml")
				if !force {
					if _, err := os.Stat(destPath); err == nil {
						return fmt.Errorf("render-user-accounts: %s already exists (use --force to overwrite)", destPath)
					}
				}
				msg, _, err := email.BuildEML(tmpl, user, cfg, opts)
				if err != nil {
					return fmt.Errorf("render-user-accounts: build for %s: %w", user.Username, err)
				}
				if err := os.WriteFile(destPath, msg, 0o644); err != nil {
					return fmt.Errorf("render-user-accounts: write %s: %w", destPath, err)
				}
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Rendered %d emails to %s.\n", len(users), outputDir)
			return err
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&outputDir, "output-dir", "d", "./rendered-emails", "output directory for .eml files")
	flags.StringVar(&fromEmail, "from-email", "noreply@localhost", "From: header value and recipient-fallback domain source")
	flags.BoolVar(&force, "force", false, "overwrite existing .eml files")
	return cmd
}
