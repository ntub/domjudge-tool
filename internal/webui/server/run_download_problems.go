package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/client/web/gatewayfactory"
	"github.com/ntub/domjudge-tool/internal/problems"
	"github.com/ntub/domjudge-tool/internal/util/progress"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

// runDownloadProblems is the production DownloadFunc.
func runDownloadProblems(
	ctx context.Context,
	p profile.ServerProfile,
	opts handler.DownloadOpts,
	rep progress.Reporter,
) (handler.DownloadResult, error) {
	cfg := profileToConfig(p)

	gw, err := gatewayfactory.New(ctx, cfg)
	if err != nil {
		return handler.DownloadResult{}, fmt.Errorf("download-problems: gateway: %w", err)
	}
	if err := gw.Login(ctx); err != nil {
		return handler.DownloadResult{}, fmt.Errorf("download-problems: login: %w", err)
	}

	filter, err := resolveDownloadFilter(ctx, p, opts)
	if err != nil {
		return handler.DownloadResult{}, err
	}

	res, err := problems.Download(ctx, gw, filter, opts.OutDir, rep)
	if err != nil {
		return handler.DownloadResult{}, err
	}
	return handler.DownloadResult{
		Downloaded: res.Downloaded,
		Skipped:    res.Skipped,
		OutDir:     res.OutDir,
	}, nil
}

// resolveDownloadFilter builds the web.ProblemFilter for the request.
// When Only is empty and the profile has a ContestID, the API is used
// to enumerate problem IDs for that contest — mirroring CLI resolveDownloadOnly.
func resolveDownloadFilter(
	ctx context.Context,
	p profile.ServerProfile,
	opts handler.DownloadOpts,
) (web.ProblemFilter, error) {
	if len(opts.Only) > 0 || p.ContestID == nil || *p.ContestID == "" {
		return web.ProblemFilter{Only: opts.Only, Exclude: opts.Exclude}, nil
	}

	cfg := profileToConfig(p)
	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
	})
	probs, err := api.Problems.List(ctx, *p.ContestID)
	if err != nil {
		return web.ProblemFilter{}, fmt.Errorf("download-problems: contest %s problems: %w", *p.ContestID, err)
	}
	ids := make([]string, 0, len(probs))
	for _, pr := range probs {
		if pr.ID != "" {
			ids = append(ids, pr.ID)
		}
	}
	return web.ProblemFilter{Only: ids, Exclude: opts.Exclude}, nil
}

// defaultDownloadOutDir returns a sensible absolute default for the output
// directory — ~/Downloads/export_problems — so files are visible to the user
// regardless of where the .app bundle's working directory is.
func defaultDownloadOutDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "export_problems"
	}
	return filepath.Join(home, "Downloads", "export_problems")
}
