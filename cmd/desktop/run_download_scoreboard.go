//go:build desktop

package main

import (
	"context"
	"io"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/scoreboard"
)

// desktopRunDownloadScoreboard is the production DownloadScoreboardFunc.
func desktopRunDownloadScoreboard(
	ctx context.Context,
	p profile.ServerProfile,
	opts handler.DownloadScoreboardOpts,
	w io.Writer,
) error {
	cfg := profileToConfig(p)

	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    cfg.Timeout,
	})

	header, rows, err := scoreboard.Download(ctx, api, opts.ContestID, opts.ExcludeHidden)
	if err != nil {
		return err
	}
	return scoreboard.WriteCSV(header, rows, w)
}
