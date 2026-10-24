package server

import (
	"context"
	"fmt"
	"strings"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web/gatewayfactory"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/util/password"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

// runSetPassword implements handler.SetPasswordFunc.
// Password writes use the same version-aware transport policy as the CLI.
func runSetPassword(
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

	decision := importer.NewDefaultRouter(cfg).Decide(ctx, importer.OpSetPassword, importer.Context{})
	switch decision.Path {
	case importer.PathAPI:
		tsv := fmt.Sprintf("accounts\t1\nteam\t%s\t%s\t%s\n", fullName, opts.Username, pw)
		if _, err := api.Imports.Accounts(ctx, strings.NewReader(tsv)); err != nil {
			return handler.SetPasswordResult{}, fmt.Errorf("set-password: api imports accounts: %w", err)
		}
	case importer.PathScraper:
		gw, err := gatewayfactory.New(ctx, cfg)
		if err != nil {
			return handler.SetPasswordResult{}, fmt.Errorf("set-password: gateway: %w", err)
		}
		if err := gw.Login(ctx); err != nil {
			return handler.SetPasswordResult{}, fmt.Errorf("set-password: login: %w", err)
		}
		//nolint:staticcheck // The router selects the supported write transport.
		if err := gw.SetUserPassword(ctx, userID, pw, nil, true); err != nil {
			return handler.SetPasswordResult{}, fmt.Errorf("set-password: gateway: %w", err)
		}
	default:
		return handler.SetPasswordResult{}, fmt.Errorf("set-password: router returned unknown path %q", decision.Path)
	}

	return handler.SetPasswordResult{
		Username:  opts.Username,
		Password:  pw,
		Generated: generated,
	}, nil
}
