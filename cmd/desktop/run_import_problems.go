//go:build desktop

package main

import (
	"context"
	"fmt"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	iproblems "github.com/ntub/domjudge-tool/internal/problems"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// desktopRunImportProblems is the production ImportProblemsFunc wired into the
// desktop app. It converts the saved temp-file opts into iproblems.ImportInput
// slices and delegates to iproblems.Import.
func desktopRunImportProblems(
	ctx context.Context,
	p profile.ServerProfile,
	opts handler.ImportProblemsOpts,
	rep progress.Reporter,
) (handler.ImportProblemsResult, error) {
	cfg := profileToConfig(p)

	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    cfg.Timeout,
	})

	if api.Imports == nil {
		return handler.ImportProblemsResult{}, fmt.Errorf("import-problems: imports resource unavailable")
	}

	inputs := make([]iproblems.ImportInput, len(opts.Files))
	for i, f := range opts.Files {
		inputs[i] = iproblems.ImportInput{Path: f.Path, Name: f.Name}
	}

	result, err := iproblems.Import(ctx, api.Imports, api.Problems, cfg.Version, opts.ContestID, inputs, opts.Upsert, rep)
	if err != nil {
		return handler.ImportProblemsResult{}, err
	}
	return handler.ImportProblemsResult{Imported: result.Imported}, nil
}
