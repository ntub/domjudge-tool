//go:build desktop

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
	"github.com/ntub/domjudge-tool/internal/email"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/util/pathx"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

func desktopEmailScaffold(_ context.Context, dir string, force bool) error {
	return email.WriteTemplateScaffold(dir, force)
}

func desktopEmailValidate(_ context.Context, dir string) error {
	tmpl, err := email.LoadTemplate(dir)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return tmpl.Validate()
}

func desktopEmailRender(_ context.Context, opts handler.EmailRenderOpts) (int, error) {
	tmpl, err := email.LoadTemplate(opts.TemplateDir)
	if err != nil {
		return 0, fmt.Errorf("render: load template: %w", err)
	}
	if err := tmpl.Validate(); err != nil {
		return 0, fmt.Errorf("render: validate template: %w", err)
	}

	f, err := os.Open(opts.CSVPath)
	if err != nil {
		return 0, fmt.Errorf("render: open csv: %w", err)
	}
	defer func() { _ = f.Close() }()

	users, err := importer.ParseUsersCSV(f)
	if err != nil {
		return 0, fmt.Errorf("render: parse csv: %w", err)
	}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return 0, fmt.Errorf("render: mkdir: %w", err)
	}

	cfg := &config.DomServerClient{}
	sendOpts := email.SendOptions{FromEmail: "noreply@localhost"}

	for _, user := range users {
		name := pathx.SafeFilename(user.Username)
		if name == "" {
			name = "unknown"
		}
		destPath := filepath.Join(opts.OutputDir, name+".eml")
		if !opts.Force {
			if _, err := os.Stat(destPath); err == nil {
				return 0, fmt.Errorf("render: %s already exists (enable force to overwrite)", destPath)
			}
		}
		msg, _, err := email.BuildEML(tmpl, user, cfg, sendOpts)
		if err != nil {
			return 0, fmt.Errorf("render: build for %s: %w", user.Username, err)
		}
		if err := os.WriteFile(destPath, msg, 0o644); err != nil {
			return 0, fmt.Errorf("render: write %s: %w", destPath, err)
		}
	}
	return len(users), nil
}

func desktopEmailSend(ctx context.Context, opts handler.EmailSendOpts, smtp settings.AppSettings, rep progress.Reporter) (int, error) {
	tmpl, err := email.LoadTemplate(opts.TemplateDir)
	if err != nil {
		return 0, fmt.Errorf("send: load template: %w", err)
	}
	if err := tmpl.Validate(); err != nil {
		return 0, fmt.Errorf("send: validate template: %w", err)
	}

	f, err := os.Open(opts.CSVPath)
	if err != nil {
		return 0, fmt.Errorf("send: open csv: %w", err)
	}
	defer func() { _ = f.Close() }()

	users, err := importer.ParseUsersCSV(f)
	if err != nil {
		return 0, fmt.Errorf("send: parse csv: %w", err)
	}

	sender, err := email.Dial(ctx, email.DialOptions{
		Host:     smtp.SMTPHost,
		Port:     smtp.SMTPPort,
		UseSSL:   smtp.SMTPUseTLS,
		Username: smtp.SMTPUsername,
		Password: smtp.SMTPPassword,
	})
	if err != nil {
		return 0, fmt.Errorf("send: dial smtp: %w", err)
	}
	defer func() { _ = sender.Close() }()

	cfg := &config.DomServerClient{}
	sendOpts := email.SendOptions{
		FromEmail: smtp.SMTPFromEmail,
		Reporter:  rep,
	}
	if err := email.SendUserAccounts(ctx, sender, tmpl, users, cfg, sendOpts); err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}
	return len(users), nil
}
