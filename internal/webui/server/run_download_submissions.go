package server

import (
	"context"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/submissions"
	"github.com/ntub/domjudge-tool/internal/util/progress"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

// runDownloadSubmissions is the production DownloadSubmissionsFunc.
func runDownloadSubmissions(
	ctx context.Context,
	p profile.ServerProfile,
	opts handler.DownloadSubmissionsOpts,
	rep progress.Reporter,
) (handler.DownloadSubmissionsResult, error) {
	cfg := profileToConfig(p)

	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})

	n, err := submissions.Download(ctx, api, opts.ContestID, submissions.Options{
		Mode:       opts.Mode,
		OutputPath: opts.OutDir,
		ProblemIDs: opts.ProblemIDs,
		Extract:    opts.Extract,
		Reporter:   rep,
	})
	return handler.DownloadSubmissionsResult{Downloaded: n}, err
}
