//go:build desktop

package main

import (
	"context"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/model"
)

// desktopFetchContests is the production ContestsFetchFunc.
func desktopFetchContests(ctx context.Context, p profile.ServerProfile) ([]model.Contest, error) {
	cfg := profileToConfig(p)
	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	return api.Contests.List(ctx)
}

// desktopFetchUsers is the production UsersFetchFunc.
func desktopFetchUsers(ctx context.Context, p profile.ServerProfile) ([]model.User, error) {
	cfg := profileToConfig(p)
	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	return api.Users.List(ctx, apiv4.UsersListOpts{})
}
