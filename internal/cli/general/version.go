// Package general implements the "General Commands:" subtree of the
// domjudge-tool CLI: gen-config (write domserver.json from flags) and
// version (print build identifier).
package general

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// groupID matches the "general" group registered in cli.NewRootCmd.
// The literal is duplicated to avoid an import cycle (cli depends on
// this package for wiring; this package would loop back if it imported
// cli for the constant). docs/phase1/README.md §"Command tree shape"
// is the canonical source of truth.
const groupID = "general"

// Version is the build version. goreleaser injects the git tag via
// ldflags: -X github.com/ntub/domjudge-tool/internal/cli/general.Version=...
// When unset (e.g. plain `go build` or test runs), versionLine falls
// back to runtime/debug.ReadBuildInfo (which `go install <module>@<tag>`
// populates).
var Version string

// NewVersionCmd builds the `version` subcommand. It writes a single
// build-identifier line to cmd.OutOrStdout and accepts no positional
// args.
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         "Print build version",
		GroupID:       groupID,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), versionLine())
			return err
		},
	}
}

// versionLine assembles the build identifier from runtime/debug. The
// format is stable: "domjudge-tool <version> (<short-revision>[+dirty], <go-version>)".
func versionLine() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "domjudge-tool (unknown build)"
	}
	version := Version
	if version == "" {
		version = info.Main.Version
		if version == "" || version == "(devel)" {
			version = "(devel)"
		}
	}
	var revision, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				modified = "+dirty"
			}
		}
	}
	if revision != "" {
		short := revision
		if len(short) > 12 {
			short = short[:12]
		}
		return fmt.Sprintf("domjudge-tool %s (%s%s, %s)", version, short, modified, info.GoVersion)
	}
	return fmt.Sprintf("domjudge-tool %s (%s)", version, info.GoVersion)
}
