package general

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/config"
)

// NewGenConfigCmd builds the `gen-config` subcommand: writes a
// domserver.json from CLI flags via config.Save.
//
// The command is intentionally non-interactive (Phase 1 DoD #4 requires
// a byte-identical file given identical flags). Required flags
// --host, --username, --password are enforced via MarkFlagRequired;
// optional flags map to *T fields on config.DomServerClient and stay
// nil unless cmd.Flags().Changed reports them set.
func NewGenConfigCmd() *cobra.Command {
	var (
		host, username, password string
		output                   string
		version                  string
		apiVersion               string
		disableSSL               bool
		timeout                  float64
		maxConnections           int
		maxKeepaliveConnections  int
		categoryID               int
		affiliationID            int
		affiliationCountry       string
		userRoles                []int
		contestID                string
	)

	cmd := &cobra.Command{
		Use:           "gen-config",
		Short:         "Write a domserver.json from flags",
		GroupID:       groupID,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		Example:       "  domjudge-tool gen-config --host https://example.com --username admin --password s3cr3t",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.Default()
			cfg.Host = host
			cfg.Username = username
			cfg.Password = password
			cfg.DisableSSL = disableSSL
			if version != "" {
				cfg.Version = version
			}
			if apiVersion != "" {
				cfg.APIVersion = apiVersion
			}
			if affiliationCountry != "" {
				v := affiliationCountry
				cfg.AffiliationCountry = &v
			}
			if cmd.Flags().Changed("timeout") {
				v := timeout
				cfg.Timeout = &v
			}
			if cmd.Flags().Changed("max-connections") {
				v := maxConnections
				cfg.MaxConnections = &v
			}
			if cmd.Flags().Changed("max-keepalive-connections") {
				v := maxKeepaliveConnections
				cfg.MaxKeepaliveConnections = &v
			}
			if cmd.Flags().Changed("category-id") {
				v := categoryID
				cfg.CategoryID = &v
			}
			if cmd.Flags().Changed("affiliation-id") {
				v := affiliationID
				cfg.AffiliationID = &v
			}
			if len(userRoles) > 0 {
				cfg.UserRoles = userRoles
			}
			if contestID != "" {
				v := contestID
				cfg.ContestID = &v
			}
			if err := config.Save(cfg, output); err != nil {
				return fmt.Errorf("gen-config: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s.\n", output)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&host, "host", "", "DOMjudge host URL (required)")
	f.StringVar(&username, "username", "", "admin / api_reader / api_writer username (required)")
	f.StringVar(&password, "password", "", "user password (required)")
	f.StringVarP(&output, "output", "o", "domserver.json", "output path for the config file")
	f.StringVar(&version, "version", "7.3.2", "DOMjudge version")
	f.StringVar(&apiVersion, "api-version", "v4", "DOMjudge API version")
	f.BoolVar(&disableSSL, "disable-ssl", false, "skip TLS verification")
	f.Float64Var(&timeout, "timeout", 0, "HTTP timeout in seconds")
	f.IntVar(&maxConnections, "max-connections", 0, "HTTP max connections")
	f.IntVar(&maxKeepaliveConnections, "max-keepalive-connections", 0, "HTTP max keepalive connections")
	f.IntVar(&categoryID, "category-id", 0, "default category id")
	f.IntVar(&affiliationID, "affiliation-id", 0, "default affiliation id")
	f.StringVar(&affiliationCountry, "affiliation-country", "TWN", "default affiliation country")
	f.IntSliceVar(&userRoles, "user-roles", nil, "default user role ids (comma-separated)")
	f.StringVar(&contestID, "contest-id", "", "default contest id (numeric or shortname)")

	for _, name := range []string{"host", "username", "password"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}
