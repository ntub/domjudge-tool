package users

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/util/password"
)

// ErrUnknownUser is returned by set-password when the API user list
// has no entry matching the requested username.
var ErrUnknownUser = errors.New("set-password: unknown user")

// NewSetPasswordCmd builds the `set-password <username>` subcommand:
// looks up the user-id from the live API, then routes the reset through
// accounts.tsv import by default or the legacy scraper when roles are
// explicit. When --password is omitted the command generates one via
// internal/util/password and prints it to stdout once.
func NewSetPasswordCmd(deps Deps) *cobra.Command {
	var (
		pw          string
		passwordLen int
		charset     string
		enabled     bool
		userRoles   []int
	)
	cmd := &cobra.Command{
		Use:           "set-password <username>",
		Short:         "Reset a single user's password",
		GroupID:       groupID,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("set-password: config not on context (PreRunE missing)")
			}
			api := deps.NewAPI(cfg)
			snapshot, err := api.Users.List(ctx, apiv4.UsersListOpts{})
			if err != nil {
				return fmt.Errorf("set-password: list users: %w", err)
			}
			var userID, name string
			for i := range snapshot {
				if snapshot[i].Username == username {
					userID = snapshot[i].ID
					name = snapshot[i].Name
					break
				}
			}
			if userID == "" {
				return fmt.Errorf("%w: %s", ErrUnknownUser, username)
			}

			if pw == "" {
				cs := charset
				if cs == "" {
					cs = password.DefaultCharset
				}
				generated, err := password.Generate(passwordLen, cs)
				if err != nil {
					return fmt.Errorf("set-password: generate: %w", err)
				}
				pw = generated
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), pw)
			}

			roles := rolesFromFlag(cmd, userRoles)
			decision := deps.routerFor(cfg).Decide(ctx, importer.OpSetPassword, importer.Context{Roles: roles})
			switch decision.Path {
			case importer.PathAPI:
				if name == "" {
					name = username
				}
				tsv := setPasswordAccountsTSV(name, username, pw)
				if _, err := deps.accountsImporterFor(api).Accounts(ctx, strings.NewReader(tsv)); err != nil {
					return fmt.Errorf("set-password: api imports accounts: %w", err)
				}
				return nil
			case importer.PathScraper:
				return runSetPasswordScraper(ctx, deps, cfg, userID, pw, roles, enabled)
			default:
				return fmt.Errorf("set-password: router returned unknown path %q", decision.Path)
			}
		},
	}
	f := cmd.Flags()
	f.StringVar(&pw, "password", "", "new password (generated when omitted)")
	f.IntVar(&passwordLen, "password-length", 0, "length when generating (0 = util/password default)")
	f.StringVar(&charset, "password-pattern", "", "charset when generating (empty = util/password default)")
	f.BoolVar(&enabled, "enabled", true, "keep the user enabled after the password update")
	f.IntSliceVar(&userRoles, "user-roles", nil, "user role ids (comma-separated)")
	return cmd
}

func rolesFromFlag(cmd *cobra.Command, userRoles []int) []int {
	if !cmd.Flags().Changed("user-roles") {
		return nil
	}
	roles := append([]int(nil), userRoles...)
	if roles == nil {
		return []int{}
	}
	return roles
}

func runSetPasswordScraper(
	ctx context.Context,
	deps Deps,
	cfg *config.DomServerClient,
	userID, pw string,
	roles []int,
	enabled bool,
) error {
	gw, err := deps.NewGateway(ctx, cfg)
	if err != nil {
		return fmt.Errorf("set-password: gateway: %w", err)
	}
	if err := gw.Login(ctx); err != nil {
		return fmt.Errorf("set-password: login: %w", err)
	}
	// nolint:staticcheck // intentional scraper fallback when router elects PathScraper (--user-roles given or cfg.prefer_scraper).
	if err := gw.SetUserPassword(ctx, userID, pw, roles, enabled); err != nil {
		return fmt.Errorf("set-password: gateway: %w", err)
	}
	return nil
}

func setPasswordAccountsTSV(name, username, pw string) string {
	return fmt.Sprintf("accounts\t1\nteam\t%s\t%s\t%s\n", name, username, pw)
}
