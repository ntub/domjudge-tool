package scoreboard

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	scorecore "github.com/ntub/domjudge-tool/internal/scoreboard"
)

// NewDownloadScoreboardCmd builds the `download-scoreboard` subcommand
// (alias `scoreboard-export`). Fetches the API scoreboard for the chosen
// contest via deps.Download, then writes a CSV to disk.
func NewDownloadScoreboardCmd(deps Deps) *cobra.Command {
	var (
		cid         int
		filename    string
		path        string
		urlOverride string
	)
	cmd := &cobra.Command{
		Use:     "download-scoreboard",
		Aliases: []string{"scoreboard-export"},
		Short:   "Export the public scoreboard as CSV",
		Long: "Fetches `<url>/api/v4/contests/<cid>/scoreboard` and writes\n" +
			"the reconstructed Phase 1 scoreboard CSV to --path/--filename.csv.",
		GroupID:       groupID,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("download-scoreboard: config not on context (PreRunE missing)")
			}
			apiCfg := *cfg
			apiCfg.Host = withCfgHost(cfg, urlOverride)
			newAPI := deps.NewAPI
			if newAPI == nil {
				newAPI = defaultNewAPI
			}
			download := deps.Download
			if download == nil {
				download = scorecore.Download
			}
			header, rows, err := download(ctx, newAPI(&apiCfg), strconv.Itoa(cid))
			if err != nil {
				return fmt.Errorf("download-scoreboard: %w", err)
			}

			outPath := filepath.Join(path, filename+".csv")
			if path == "" {
				outPath = filename + ".csv"
			}
			f, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("download-scoreboard: create %s: %w", outPath, err)
			}
			defer func() { _ = f.Close() }()

			cw := csv.NewWriter(f)
			cw.UseCRLF = true // Python csv default dialect: \r\n.
			if err := cw.Write(header); err != nil {
				return fmt.Errorf("download-scoreboard: write header: %w", err)
			}
			for _, row := range rows {
				if err := ctx.Err(); err != nil {
					return fmt.Errorf("download-scoreboard: %w", err)
				}
				if err := cw.Write(row); err != nil {
					return fmt.Errorf("download-scoreboard: write row: %w", err)
				}
			}
			cw.Flush()
			if err := cw.Error(); err != nil {
				return fmt.Errorf("download-scoreboard: flush: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s (%d rows).\n", outPath, len(rows))
			return nil
		},
	}
	flags := cmd.Flags()
	flags.IntVar(&cid, "cid", 0, "DOMjudge contest id (required)")
	flags.StringVar(&filename, "filename", "export", "output filename without extension (`.csv` is appended)")
	flags.StringVar(&path, "path", "", "directory to write the CSV under (default: cwd)")
	flags.StringVar(&urlOverride, "url", "", "override the scoreboard host (default: cfg.Host)")
	_ = cmd.MarkFlagRequired("cid")
	return cmd
}
