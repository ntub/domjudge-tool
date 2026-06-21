//go:build desktop

package main

import (
	"context"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web/gatewayfactory"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// desktopRunDelete is the production DeleteFunc wired into the desktop app.
// It converts a ServerProfile to the legacy config format, builds real API
// and web-gateway clients, and calls the existing CLI delete logic.
func desktopRunDelete(
	ctx context.Context,
	p profile.ServerProfile,
	opts handler.DeleteOpts,
	rep progress.Reporter,
) (handler.DeleteResult, error) {
	cfg := profileToConfig(p)

	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
	})

	gw, err := gatewayfactory.New(ctx, cfg)
	if err != nil {
		return handler.DeleteResult{}, err
	}

	rep.Start(4, "removing")

	// Step 1: list current users to derive include/exclude sets.
	snapshot, err := api.Users.List(ctx, apiv4.UsersListOpts{})
	if err != nil {
		return handler.DeleteResult{}, err
	}
	rep.Tick()

	protected := map[string]bool{"admin": true, "judgehost": true}
	if cfg.Username != "" {
		protected[cfg.Username] = true
	}

	includeUsers := resolveInclude(snapshot, protected, opts.Include, opts.Exclude)

	// Derive team IDs from the snapshot for dry-run preview.
	teamIdx := make(map[string]string, len(snapshot))
	for i := range snapshot {
		if snapshot[i].TeamID != nil {
			teamIdx[snapshot[i].Username] = *snapshot[i].TeamID
		}
	}
	includeTeams := projectIDs(includeUsers, teamIdx)

	rep.Tick()

	if opts.DryRun {
		rep.Finish()
		return handler.DeleteResult{
			PlannedUsers: includeUsers,
			PlannedTeams: includeTeams,
			DryRun:       true,
		}, nil
	}

	// Real delete: delegate to the existing importer which handles login + gateway calls.
	delOpts := importer.DeleteOptions{Include: opts.Include, Exclude: opts.Exclude}
	if err := importer.DeleteTeamsAndUsers(ctx, gw, api.Users, cfg, delOpts); err != nil {
		return handler.DeleteResult{}, err
	}
	rep.Tick()
	rep.Tick()
	rep.Finish()

	return handler.DeleteResult{
		DeletedUsers: len(includeUsers),
		DeletedTeams: len(includeTeams),
	}, nil
}

// resolveInclude computes the final list of usernames to delete.
// When include is nil, all non-protected snapshot users are candidates.
func resolveInclude(snapshot []model.User, protected map[string]bool, include, exclude []string) []string {
	excludeSet := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		if !protected[name] {
			excludeSet[name] = true
		}
	}

	candidates := include
	if candidates == nil {
		candidates = make([]string, 0, len(snapshot))
		for i := range snapshot {
			candidates = append(candidates, snapshot[i].Username)
		}
	}

	out := make([]string, 0, len(candidates))
	for _, name := range candidates {
		if protected[name] || excludeSet[name] {
			continue
		}
		out = append(out, name)
	}
	return out
}

func projectIDs(usernames []string, idx map[string]string) []string {
	out := make([]string, 0, len(usernames))
	for _, name := range usernames {
		if id, ok := idx[name]; ok {
			out = append(out, id)
		}
	}
	return out
}
