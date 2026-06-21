//go:build desktop

package main

import (
	"context"
	"fmt"
	"strings"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/util/password"
)

// desktopRunSetPassword implements handler.SetPasswordFunc.
// It uses the API accounts-import path (accounts.tsv) to set the password.
// Scraper fallback and per-user role changes are not supported from the
// desktop UI — use the CLI set-password command for those cases.
func desktopRunSetPassword(
	ctx context.Context,
	p profile.ServerProfile,
	opts handler.SetPasswordOpts,
) (handler.SetPasswordResult, error) {
	cfg := profileToConfig(p)
	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})

	users, err := api.Users.List(ctx, apiv4.UsersListOpts{})
	if err != nil {
		return handler.SetPasswordResult{}, fmt.Errorf("set-password: list users: %w", err)
	}

	var userID, fullName string
	for i := range users {
		if users[i].Username == opts.Username {
			userID = users[i].ID
			fullName = users[i].Name
			break
		}
	}
	if userID == "" {
		return handler.SetPasswordResult{}, fmt.Errorf("set-password: unknown user: %s", opts.Username)
	}
	if fullName == "" {
		fullName = opts.Username
	}

	pw := opts.Password
	generated := false
	if pw == "" {
		pw, err = password.Generate(0, password.DefaultCharset)
		if err != nil {
			return handler.SetPasswordResult{}, fmt.Errorf("set-password: generate password: %w", err)
		}
		generated = true
	}

	tsv := fmt.Sprintf("accounts\t1\nteam\t%s\t%s\t%s\n", fullName, opts.Username, pw)
	if _, err := api.Imports.Accounts(ctx, strings.NewReader(tsv)); err != nil {
		return handler.SetPasswordResult{}, fmt.Errorf("set-password: api imports accounts: %w", err)
	}

	return handler.SetPasswordResult{
		Username:  opts.Username,
		Password:  pw,
		Generated: generated,
	}, nil
}
