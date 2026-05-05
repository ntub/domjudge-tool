package emails

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/email"
	"github.com/ntub/domjudge-tool/internal/importer"
)

// NewSendUserAccountsCmd builds the
// `send-user-accounts <csv> <template-dir>` subcommand. It loads the
// template, parses the CSV via importer.ParseUsersCSV, opens an SMTP
// sender via deps.Dial, then delegates to email.SendUserAccounts.
//
// SMTP credentials are kept on `--smtp-username` / `--smtp-password`
// (distinct from the DOMjudge admin auth in domserver.json).
func NewSendUserAccountsCmd(deps Deps) *cobra.Command {
	var (
		host         string
		port         int
		fromEmail    string
		useSSL       bool
		smtpUsername string
		smtpPassword string
		timeout      time.Duration
	)
	cmd := &cobra.Command{
		Use:   "send-user-accounts <csv-file> <template-dir>",
		Short: "Mail templated credential emails to a CSV-driven user list",
		Long: "Reads <csv-file> in the canonical import-users-teams CSV format\n" +
			"(BOM-tolerant). For each row, renders the templates under\n" +
			"<template-dir>/subject.txt + body.{html,txt}, then sends one\n" +
			"email via the SMTP server given by --host/--port. SMTP creds\n" +
			"are --smtp-username/--smtp-password, distinct from the\n" +
			"DOMjudge admin auth in domserver.json.",
		GroupID:       groupID,
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("send-user-accounts: config not on context (PreRunE missing)")
			}

			if err := ctx.Err(); err != nil {
				return fmt.Errorf("send-user-accounts: %w", err)
			}

			tmpl, err := email.LoadTemplate(args[1])
			if err != nil {
				return fmt.Errorf("send-user-accounts: template: %w", err)
			}

			csvFile, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("send-user-accounts: open csv: %w", err)
			}
			defer func() { _ = csvFile.Close() }()
			users, err := importer.ParseUsersCSV(csvFile)
			if err != nil {
				return fmt.Errorf("send-user-accounts: parse csv: %w", err)
			}

			sender, err := deps.Dial(ctx, email.DialOptions{
				Host:     host,
				Port:     port,
				UseSSL:   useSSL,
				Timeout:  timeout,
				Username: smtpUsername,
				Password: smtpPassword,
			})
			if err != nil {
				return fmt.Errorf("send-user-accounts: dial: %w", err)
			}
			defer func() { _ = sender.Close() }()

			if err := email.SendUserAccounts(ctx, sender, tmpl, users, cfg, email.SendOptions{
				FromEmail: fromEmail,
			}); err != nil {
				return fmt.Errorf("send-user-accounts: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Sent %d emails.\n", len(users))
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&host, "host", "localhost", "SMTP server host")
	flags.IntVar(&port, "port", 25, "SMTP server port")
	flags.StringVar(&fromEmail, "from-email", "noreply@localhost", "From: header value and recipient-fallback domain source")
	flags.BoolVar(&useSSL, "use-ssl", false, "open the SMTP connection with TLS (TLS-on-connect, not STARTTLS)")
	flags.StringVar(&smtpUsername, "smtp-username", "", "SMTP AUTH username (empty skips AUTH)")
	flags.StringVar(&smtpPassword, "smtp-password", "", "SMTP AUTH password (empty skips AUTH)")
	flags.DurationVar(&timeout, "timeout", 0, "SMTP dial timeout (0 = no timeout)")
	return cmd
}
