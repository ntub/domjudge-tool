package problems

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/config"
	iproblems "github.com/ntub/domjudge-tool/internal/problems"
)

// defaultFolder mirrors Python `_problems.py:18` — when the operator
// passes no `--folder`, write everything under ./export_problems/.
const defaultFolder = "export_problems"

// NewDownloadProblemsCmd builds the `download-problems` subcommand:
// logs in, lists problems via the jury web UI, then streams each
// problem's export ZIP to disk under the chosen folder.
//
// File naming mirrors Python `_problems.py:32`:
//
//	${folder}/${id}_${name}.zip   with all spaces in name → "-"
//
// Problems with an empty / nil ExportFilePath are silently skipped
// (Python parity: continue inside the loop).
func NewDownloadProblemsCmd(deps Deps) *cobra.Command {
	var (
		exclude []string
		only    []string
		folder  string
	)
	cmd := &cobra.Command{
		Use:   "download-problems",
		Short: "Download every problem's export ZIP",
		Long: "Logs in via the configured jury credentials, lists the contest's\n" +
			"problems, then writes each problem's export ZIP under the chosen\n" +
			"folder (default ./export_problems). --exclude / --only filter the\n" +
			"problem list before download.",
		GroupID:       groupID,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("download-problems: config not on context (PreRunE missing)")
			}
			effectiveOnly, scopedEmpty, err := resolveDownloadOnly(ctx, deps, cfg, only)
			if err != nil {
				return err
			}
			out := folder
			if out == "" {
				out = defaultFolder
			}
			// Create the output dir eagerly — mirrors original behaviour where
			// the folder is created even when scopedEmpty short-circuits.
			if err := os.MkdirAll(out, 0o755); err != nil {
				return fmt.Errorf("download-problems: mkdir %s: %w", out, err)
			}
			if scopedEmpty {
				rep := cli.ReporterFromContext(ctx)
				rep.Start(0, "downloading")
				rep.Finish()
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Processed 0 problems.\n")
				return nil
			}
			gw, err := deps.NewGateway(ctx, cfg)
			if err != nil {
				return fmt.Errorf("download-problems: gateway: %w", err)
			}
			if err := gw.Login(ctx); err != nil {
				return fmt.Errorf("download-problems: login: %w", err)
			}

			rep := cli.ReporterFromContext(ctx)
			filter := web.ProblemFilter{Exclude: exclude, Only: effectiveOnly}
			result, err := iproblems.Download(ctx, gw, filter, out, rep)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Processed %d problems.\n", result.Downloaded+result.Skipped)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringSliceVar(&exclude, "exclude", nil, "comma-separated problem ids to exclude")
	flags.StringSliceVar(&only, "only", nil, "comma-separated problem ids to keep (excludes others)")
	flags.StringVar(&folder, "folder", "", "output folder (default: export_problems)")
	return cmd
}

func resolveDownloadOnly(
	ctx context.Context,
	deps Deps,
	cfg *config.DomServerClient,
	only []string,
) (effectiveOnly []string, scopedEmpty bool, err error) {
	if len(only) > 0 || cfg == nil || cfg.ContestID == nil || *cfg.ContestID == "" {
		return only, false, nil
	}
	newAPI := deps.NewAPI
	if newAPI == nil {
		newAPI = defaultNewAPI
	}
	api := newAPI(cfg)
	if api == nil || api.Problems == nil {
		return nil, false, fmt.Errorf("download-problems: api problems resource unavailable")
	}
	probs, err := api.Problems.List(ctx, *cfg.ContestID)
	if err != nil {
		return nil, false, fmt.Errorf("download-problems: contest %s problems: %w", *cfg.ContestID, err)
	}
	ids := make([]string, 0, len(probs))
	for _, p := range probs {
		if p.ID != "" {
			ids = append(ids, p.ID)
		}
	}
	return ids, len(ids) == 0, nil
}
