package submissions

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/submissions"
)

// NewDownloadSubmissionsCmd builds the `download-submissions <cid>`
// subcommand (alias `contest-files`). Delegates to the Layer-3
// `submissions.Download` orchestrator. Defaults match Python:
// Mode=2, Extract=true.
func NewDownloadSubmissionsCmd(deps Deps) *cobra.Command {
	var (
		mode    int
		path    string
		strict  bool
		extract bool
	)
	cmd := &cobra.Command{
		Use:     "download-submissions <cid>",
		Aliases: []string{"contest-files"},
		Short:   "Download every submission's source files for a contest",
		Long: "Downloads each submission's source archive for contest <cid>.\n" +
			"--mode picks the per-submission folder layout (1=team-first,\n" +
			"2=problem-first, other=contest_<cid>). --path prefixes the\n" +
			"layout. --extract unzips each archive in place.",
		GroupID:       groupID,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("download-submissions: config not on context (PreRunE missing)")
			}
			api := deps.NewAPI(cfg)

			opts := submissions.Options{
				Mode:       mode,
				OutputPath: path,
				Extract:    extract,
			}
			if cmd.Flags().Changed("strict") {
				v := strict
				opts.Strict = &v
			}

			n, err := deps.Download(ctx, api, args[0], opts)
			if err != nil {
				return fmt.Errorf("download-submissions: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %d submissions.\n", n)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.IntVar(&mode, "mode", 2, "folder layout: 1=team-first, 2=problem-first (default), other=contest_<cid>")
	flags.StringVar(&path, "path", "", "output path prefix prepended to the per-submission folder")
	flags.BoolVar(&strict, "strict", false, "forwarded to the apiv4 submissions list query (Python parity)")
	flags.BoolVar(&extract, "extract", true, "unzip each downloaded archive in place (Python is_extract default)")
	return cmd
}
