package server

import (
	"context"
	"io"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/scoreboard"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

// runDownloadScoreboard is the production DownloadScoreboardFunc.
func runDownloadScoreboard(
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
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})

	header, rows, err := scoreboard.Download(ctx, api, opts.ContestID, opts.ExcludeHidden)
	if err != nil {
		return err
	}
	return scoreboard.WriteCSV(header, rows, w)
}
