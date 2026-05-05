package users

import (
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/exporter"
	"github.com/ntub/domjudge-tool/internal/model"
)

// NewExportUsersCmd builds the `export-users` subcommand (alias
// `user-list`): fetches the live user list, optionally filters by
// --ids / --team-id, then writes CSV or JSON via internal/exporter.
//
// Without --output the bytes go to stdout; with --output they go to
// the file (and stdout stays empty so shell-pipe mistakes do not
// produce a duplicate).
func NewExportUsersCmd(deps Deps) *cobra.Command {
	var (
		idsCSV string
		teamID string
		format string
		output string
	)
	cmd := &cobra.Command{
		Use:           "export-users",
		Short:         "Export users to CSV or JSON",
		GroupID:       groupID,
		Aliases:       []string{"user-list"},
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := exporterFormat(format)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("export-users: config not on context")
			}
			api := deps.NewAPI(cfg)
			opts := apiv4.UsersListOpts{IDs: splitCSV(idsCSV)}
			if teamID != "" {
				v := teamID
				opts.TeamID = &v
			}
			list, err := api.Users.List(ctx, opts)
			if err != nil {
				return fmt.Errorf("export-users: list users: %w", err)
			}
			list = filterByIDs(list, opts.IDs)
			// DOMjudge 7.3.x ignores ?team_id= on /api/v4/users —
			// re-filter in memory so --team-id behaves regardless of
			// server compliance. Closes #7 of v1.1.1 hotfix.
			list = filterByTeamID(list, teamID)

			var w io.Writer
			w = cmd.OutOrStdout()
			if output != "" {
				file, err := os.Create(output)
				if err != nil {
					return fmt.Errorf("export-users: create %s: %w", output, err)
				}
				defer func() { _ = file.Close() }()
				w = file
			}
			if err := exporter.Users(list, f, w); err != nil {
				return fmt.Errorf("export-users: %w", err)
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&idsCSV, "ids", "", "comma-separated user ids to include")
	flags.StringVar(&teamID, "team-id", "", "filter by team id")
	flags.StringVar(&format, "format", "csv", "output format: csv | json")
	flags.StringVarP(&output, "output", "o", "", "output file (default stdout)")
	return cmd
}

func exporterFormat(s string) (exporter.Format, error) {
	switch s {
	case "csv":
		return exporter.FormatCSV, nil
	case "json":
		return exporter.FormatJSON, nil
	default:
		return "", fmt.Errorf("export-users: unknown --format %q (want csv or json)", s)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	out := make([]string, 0)
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func filterByIDs(list []model.User, ids []string) []model.User {
	if len(ids) == 0 {
		return list
	}
	out := make([]model.User, 0, len(ids))
	for i := range list {
		if slices.Contains(ids, list[i].ID) {
			out = append(out, list[i])
		}
	}
	return out
}

// filterByTeamID compensates for DOMjudge 7.3.x's broken ?team_id=
// query parameter (the server ignores it and returns every user).
// teamID == "" means "no filter requested".
func filterByTeamID(list []model.User, teamID string) []model.User {
	if teamID == "" {
		return list
	}
	out := make([]model.User, 0, len(list))
	for i := range list {
		if list[i].TeamID != nil && *list[i].TeamID == teamID {
			out = append(out, list[i])
		}
	}
	return out
}
