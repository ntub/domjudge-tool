package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// Deps groups injectable dependencies the command tree needs.
//
// Phase 1 keeps this struct empty by default; subcommands construct
// their layer-2/-3 collaborators (apiv4.Client, web.Gateway, etc.)
// from the loaded *config.DomServerClient at RunE time, not at root
// construction. The struct exists so future cross-cutting overrides
// (e.g. an injected http.RoundTripper for tests) have a place to land
// without revisiting every subcommand constructor signature.
type Deps struct{}

// Group IDs — keep these strings in sync with docs/phase1/README.md
// §"Command tree shape" and with the GroupID literals on each leaf
// command (subpackages set GroupID inline so this package doesn't have
// to import them, and they don't have to import this package — the
// directional split avoids an import cycle while keeping the wiring
// flat).
const (
	groupGeneral     = "general"
	groupUsers       = "users"
	groupProblems    = "problems"
	groupSubmissions = "submissions"
	groupEmails      = "emails"
	groupScoreboard  = "scoreboard"
)

// NewRootCmd builds the Cobra root.
//
// The returned *cobra.Command has no business logic on its RunE; it
// shows --help when invoked with no subcommand. PR-7 registers all
// six Phase-1 groups; subcommands attach to whichever GroupID their
// leaf package defines.
func NewRootCmd(_ Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "domjudge-tool",
		Short:         "Manage DOMjudge contest servers",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringP("config", "c", "domserver.json", "path to domserver.json")
	cmd.PersistentFlags().CountP("verbose", "v", "increase log verbosity (-v info, -vv debug)")
	cmd.PersistentFlags().Bool("disable-ssl", false, "disable TLS verification (overrides config)")
	cmd.PersistentFlags().Bool("no-progress", false,
		"disable the progress bar (auto-disabled when stderr is not a TTY)")

	cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		count, err := c.Flags().GetCount("verbose")
		if err != nil {
			return fmt.Errorf("cli: read --verbose: %w", err)
		}
		logger := slog.New(slog.NewTextHandler(c.ErrOrStderr(), &slog.HandlerOptions{
			Level: levelForVerbose(count),
		}))
		c.SetContext(WithLogger(c.Context(), logger))
		// Subsystems that don't take ctx (router decision telemetry,
		// importer dedup debug lines) read slog.Default(); rebind here
		// so -v / -vv reaches them. Closes #5 of v1.1.1 hotfix.
		slog.SetDefault(logger)

		// Construct a Reporter once, here, so library packages can
		// pull it from ctx without re-implementing the TTY / env /
		// flag gate at every call site. progress.New returns Nop
		// when stderr isn't a TTY, NO_COLOR is set, TERM=dumb, or
		// --no-progress is true.
		noProg, _ := c.Flags().GetBool("no-progress")
		rep := progress.New(c.ErrOrStderr(), noProg)
		c.SetContext(WithReporter(c.Context(), rep))
		return nil
	}

	cmd.AddGroup(
		&cobra.Group{ID: groupGeneral, Title: "General Commands:"},
		&cobra.Group{ID: groupUsers, Title: "User & Team Commands:"},
		&cobra.Group{ID: groupProblems, Title: "Problem Commands:"},
		&cobra.Group{ID: groupSubmissions, Title: "Submission Commands:"},
		&cobra.Group{ID: groupEmails, Title: "Email Commands:"},
		&cobra.Group{ID: groupScoreboard, Title: "Scoreboard Commands:"},
	)
	cmd.SetHelpCommandGroupID(groupGeneral)
	cmd.SetCompletionCommandGroupID(groupGeneral)

	return cmd
}

// subtrees holds the registration callbacks supplied by sub-packages
// via RegisterSubtree. A registry-wiring slice (rather than a direct
// import from cli into cli/{group}) keeps the dependency graph one-way
// and lets sub-packages depend on cli for ctx helpers without a cycle.
// CLAUDE.md "Things to avoid" allows init() solely for this pattern.
var subtrees []func(*cobra.Command)

// RegisterSubtree records a function that adds one sub-package's
// commands to the root. cli/{group}/register.go calls this from
// init(); cmd/domjudge-tool/main.go triggers the inits via blank
// imports.
func RegisterSubtree(fn func(*cobra.Command)) {
	subtrees = append(subtrees, fn)
}

// Execute is the entry point invoked by cmd/domjudge-tool/main.go. It
// builds the root, applies every registered subtree, runs it with the
// supplied ctx, and prints any returned error exactly once on stderr
// (Cobra's SilenceErrors path).
func Execute(ctx context.Context) error {
	root := NewRootCmd(Deps{})
	for _, fn := range subtrees {
		fn(root)
	}
	return RunRoot(ctx, root)
}

// RunRoot is the testable seam for Execute: callers (tests) inject a
// pre-configured root, RunRoot drives it.
func RunRoot(ctx context.Context, root *cobra.Command) error {
	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(root.ErrOrStderr(), err)
		return err
	}
	return nil
}

// levelForVerbose maps the --verbose count to a slog.Level. 0 → warn,
// 1 → info, ≥2 → debug.
func levelForVerbose(count int) slog.Level {
	switch {
	case count >= 2:
		return slog.LevelDebug
	case count == 1:
		return slog.LevelInfo
	default:
		return slog.LevelWarn
	}
}
