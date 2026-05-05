package problems

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/pathx"
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
			gw, err := deps.NewGateway(ctx, cfg)
			if err != nil {
				return fmt.Errorf("download-problems: gateway: %w", err)
			}
			if err := gw.Login(ctx); err != nil {
				return fmt.Errorf("download-problems: login: %w", err)
			}
			items, err := gw.GetProblems(ctx, web.ProblemFilter{Exclude: exclude, Only: only})
			if err != nil {
				return fmt.Errorf("download-problems: list: %w", err)
			}

			out := folder
			if out == "" {
				out = defaultFolder
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return fmt.Errorf("download-problems: mkdir %s: %w", out, err)
			}

			for _, p := range items {
				if err := ctx.Err(); err != nil {
					return fmt.Errorf("download-problems: %w", err)
				}
				if p.ExportFilePath == nil || *p.ExportFilePath == "" {
					continue
				}
				if err := writeProblemZip(ctx, gw, out, p); err != nil {
					return err
				}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Processed %d problems.\n", len(items))
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringSliceVar(&exclude, "exclude", nil, "comma-separated problem ids to exclude")
	flags.StringSliceVar(&only, "only", nil, "comma-separated problem ids to keep (excludes others)")
	flags.StringVar(&folder, "folder", "", "output folder (default: export_problems)")
	return cmd
}

// writeProblemZip streams gw.Download(*p.ExportFilePath) into a
// path-safe `${folder}/${p.ID}_${p.Name}.zip`. The caller has already
// guarded against a nil/empty export path.
func writeProblemZip(ctx context.Context, gw web.Gateway, folder string, p model.ProblemItem) error {
	rc, err := gw.Download(ctx, *p.ExportFilePath)
	if err != nil {
		return fmt.Errorf("download-problems: download %s: %w", *p.ExportFilePath, err)
	}
	defer func() { _ = rc.Close() }()

	filename := pathx.SafeFilename(fmt.Sprintf("%s_%s.zip", p.ID, p.Name))
	dst := filepath.Join(folder, filename)
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("download-problems: create %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("download-problems: write %s: %w", dst, err)
	}
	return nil
}
