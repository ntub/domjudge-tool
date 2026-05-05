// Package submissions implements the "Submission Commands:" subtree
// of the domjudge-tool CLI: download-submissions (alias contest-files).
//
// PR-7 ports Python's `contest_files` semantics under a single leaf
// command. The single-id `submission_file` and listing `submission_list`
// commands are out of scope (see docs/phase1/PR-7-cli-rest.md).
package submissions

import (
	"context"
	"time"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/submissions"
)

// groupID matches the "submissions" group registered in cli.NewRootCmd.
const groupID = "submissions"

// DownloadFn matches the signature of submissions.Download. Tests
// inject a stub; production points at the real Layer-3 helper.
type DownloadFn = func(
	ctx context.Context,
	api *apiv4.Client,
	cid string,
	opts submissions.Options,
) (int, error)

// Deps groups the factories the submissions-subtree needs. NewAPI
// constructs an apiv4 client; Download is the use-case seam tests
// override.
type Deps struct {
	NewAPI   func(cfg *config.DomServerClient) *apiv4.Client
	Download DownloadFn
}

// DefaultDeps wires Deps with production constructors.
func DefaultDeps() Deps {
	return Deps{
		NewAPI:   defaultNewAPI,
		Download: submissions.Download,
	}
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
