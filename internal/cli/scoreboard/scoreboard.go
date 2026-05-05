// Package scoreboard implements the "Scoreboard Commands:" subtree
// of the domjudge-tool CLI: download-scoreboard (alias scoreboard-export).
//
// The CLI takes the URL + cid as inputs and delegates the fetch+parse
// to internal/scoreboard, then writes a CSV to the chosen path.
package scoreboard

import (
	"context"
	"time"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/config"
	scorecore "github.com/ntub/domjudge-tool/internal/scoreboard"
)

// groupID matches the "scoreboard" group registered in cli.NewRootCmd.
const groupID = "scoreboard"

// DownloadFn is the seam tests inject to bypass the live HTTP fetch.
type DownloadFn = func(ctx context.Context, api *apiv4.Client, cid string) ([]string, [][]string, error)

// Deps groups the dependencies the scoreboard subcommand needs.
type Deps struct {
	Download DownloadFn
	NewAPI   func(cfg *config.DomServerClient) *apiv4.Client
}

// DefaultDeps wires Deps with a production API client and downloader.
func DefaultDeps() Deps {
	return Deps{Download: scorecore.Download, NewAPI: defaultNewAPI}
}

func defaultNewAPI(cfg *config.DomServerClient) *apiv4.Client {
	return apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
}

func derefSecondsDuration(p *float64) time.Duration {
	if p == nil {
		return 0
	}
	return time.Duration(*p * float64(time.Second))
}

// withCfgHost picks the API host: --url flag wins; otherwise cfg.Host.
func withCfgHost(cfg *config.DomServerClient, override string) string {
	if override != "" {
		return override
	}
	return cfg.Host
}
