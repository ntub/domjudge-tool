package users

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/importer"
)

// NewRmTeamsAndUsersCmd builds the `rm-teams-and-users` subcommand
// (alias `rm-teams`): drives importer.DeleteTeamsAndUsers with the
// --include / --exclude filter sets.
func NewRmTeamsAndUsersCmd(deps Deps) *cobra.Command {
	var include, exclude []string
	var includeCSV string
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
			if includeCSV != "" {
				parsed, err := readDeleteIncludeCSV(includeCSV)
				if err != nil {
					return fmt.Errorf("rm-teams-and-users: %w", err)
				}
				if !hasDeletableUsername(parsed, exclude, cfg.Username) {
					return fmt.Errorf("rm-teams-and-users: include-csv: no usernames to delete after applying protected/exclude filters")
				}
				include = parsed
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
	f.StringVar(&includeCSV, "include-csv", "", "import-users-teams CSV whose usernames should be deleted")
	f.StringSliceVar(&exclude, "exclude", nil, "usernames to keep (in addition to admin/judgehost/cfg.username)")
	cmd.MarkFlagsMutuallyExclusive("include", "include-csv")
	return cmd
}

func hasDeletableUsername(include, exclude []string, cfgUsername string) bool {
	protected := map[string]bool{"admin": true, "judgehost": true}
	if cfgUsername != "" {
		protected[cfgUsername] = true
	}
	excluded := make(map[string]bool, len(exclude))
	for _, username := range exclude {
		excluded[username] = true
	}
	for _, username := range include {
		if protected[username] || excluded[username] {
			continue
		}
		return true
	}
	return false
}

func readDeleteIncludeCSV(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("include-csv: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	users, err := importer.ParseUsersCSV(f)
	if err != nil {
		return nil, fmt.Errorf("include-csv: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("include-csv: no usernames to delete")
	}
	include := make([]string, 0, len(users))
	for _, user := range users {
		include = append(include, user.Username)
	}
	return include, nil
}
