package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/config"
)

// LoadConfigPreRunE loads domserver.json from --config (or DOMSERVER_CONFIG
// when --config is left at its default) and stashes the resolved
// *config.DomServerClient on cmd.Context() via WithConfig.
//
// Subcommands attach this as PreRunE — never as the root's
// PersistentPreRunE — so commands that don't need a config file
// (gen-config, version, completion, help) skip the load.
//
// If the context already carries a config (via WithConfig — typically
// from a test that injected its own), file loading is skipped while
// command-line config overrides are still applied. This keeps tests
// that exercise the RunE in isolation free from having to stub a real
// domserver.json on disk.
//
// Subcommand RunEs MUST NOT re-load config; they fetch it via
// ConfigFromContext.
func LoadConfigPreRunE(cmd *cobra.Command, _ []string) error {
	cfg, ok := ConfigFromContext(cmd.Context())
	if !ok {
		flag := cmd.Flags().Lookup("config")
		path := ""
		if flag != nil {
			path = flag.Value.String()
			if !flag.Changed {
				if env := os.Getenv("DOMSERVER_CONFIG"); env != "" {
					path = env
				}
			}
		}
		if path == "" {
			path = "domserver.json"
		}
		var err error
		cfg, err = config.Load(path)
		if err != nil {
			return fmt.Errorf("cli: load config: %w", err)
		}
		cmd.SetContext(WithConfig(cmd.Context(), cfg))
	}
	return applyConfigFlagOverrides(cmd, cfg)
}

func applyConfigFlagOverrides(cmd *cobra.Command, cfg *config.DomServerClient) error {
	flag := cmd.Flags().Lookup("disable-ssl")
	if flag == nil || !flag.Changed {
		return nil
	}
	disableSSL, err := cmd.Flags().GetBool("disable-ssl")
	if err != nil {
		return fmt.Errorf("cli: read --disable-ssl: %w", err)
	}
	cfg.DisableSSL = disableSSL
	return nil
}
