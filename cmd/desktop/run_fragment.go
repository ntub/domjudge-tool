//go:build desktop

package main

import (
	"context"
	"fmt"

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

// desktopFetchProblems is the production ProblemsFetchFunc.
func desktopFetchProblems(ctx context.Context, p profile.ServerProfile, contestID string) ([]model.Problem, error) {
	cfg := profileToConfig(p)
	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	return api.Contests.ListProblems(ctx, contestID)
}

// desktopFetchTeams is the production TeamsFetchFunc.
func desktopFetchTeams(ctx context.Context, p profile.ServerProfile) ([]model.Team, error) {
	cfg := profileToConfig(p)
	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	cid := ""
	if p.ContestID != nil {
		cid = *p.ContestID
	}
	if cid == "" {
		contests, err := api.Contests.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list contests: %w", err)
		}
		if len(contests) == 0 {
			return nil, fmt.Errorf("no contests found on server")
		}
		cid = contests[0].ID
	}
	return api.Teams.List(ctx, cid)
}
