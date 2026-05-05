package users

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/importer"
)

// NewRmTeamsAndUsersCmd builds the `rm-teams-and-users` subcommand
// (alias `rm-teams`): drives importer.DeleteTeamsAndUsers with the
// --include / --exclude filter sets.
func NewRmTeamsAndUsersCmd(deps Deps) *cobra.Command {
	var include, exclude []string
	cmd := &cobra.Command{
		Use:           "rm-teams-and-users",
		Short:         "Delete users and their teams",
		GroupID:       groupID,
		Aliases:       []string{"rm-teams"},
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("rm-teams-and-users: config not on context")
			}
			api := deps.NewAPI(cfg)
			gw, err := deps.NewGateway(ctx, cfg)
			if err != nil {
				return fmt.Errorf("rm-teams-and-users: gateway: %w", err)
			}
			opts := importer.DeleteOptions{Include: include, Exclude: exclude}
			if err := importer.DeleteTeamsAndUsers(ctx, gw, api.Users, cfg, opts); err != nil {
				return fmt.Errorf("rm-teams-and-users: %w", err)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&include, "include", nil, "usernames to delete (default: all non-protected)")
	f.StringSliceVar(&exclude, "exclude", nil, "usernames to keep (in addition to admin/judgehost/cfg.username)")
	return cmd
}
