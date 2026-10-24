package server

import (
	"context"
	"fmt"
	"io"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/exporter"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

// runExport is the production ExportFunc.
func runExport(
	ctx context.Context,
	p profile.ServerProfile,
	opts handler.ExportOpts,
	w io.Writer,
) error {
	cfg := profileToConfig(p)

	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
	})

	listOpts := apiv4.UsersListOpts{IDs: opts.IDs}
	if opts.TeamID != "" {
		v := opts.TeamID
		listOpts.TeamID = &v
	}

	list, err := api.Users.List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("export-users: list users: %w", err)
	}

	// DOMjudge 7.3.x ignores ?team_id= on /api/v4/users — re-filter
	// in memory so team_id filter behaves regardless of server version.
	if opts.TeamID != "" {
		list = filterUsersByTeamID(list, opts.TeamID)
	}

	f := exporter.FormatCSV
	if opts.Format == "json" {
		f = exporter.FormatJSON
	}
	return exporter.Users(list, f, w)
}

func filterUsersByTeamID(list []model.User, teamID string) []model.User {
	out := make([]model.User, 0, len(list))
	for i := range list {
		if list[i].TeamID != nil && *list[i].TeamID == teamID {
			out = append(out, list[i])
		}
	}
	return out
}
