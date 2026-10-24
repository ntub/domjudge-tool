package cli_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// stubDeps returns a Deps with no preconfigured config/logger so
// commands run without touching the filesystem or network.
func stubDeps() cli.Deps {
	return cli.Deps{}
}

func TestRoot_RegistersGeneralAndUsersGroups(t *testing.T) {
	t.Parallel()
	root := cli.NewRootCmd(stubDeps())

	want := map[string]string{
		"general": "General Commands:",
		"users":   "User & Team Commands:",
	}
	got := map[string]string{}
	for _, g := range root.Groups() {
		got[g.ID] = g.Title
	}
	for id, title := range want {
		if got[id] != title {
			t.Errorf("group %q: title = %q, want %q", id, got[id], title)
		}
	}
}

// TestRoot_RegistersAllSixGroups locks the full Phase-1 group set
// from docs/phase1/README.md §"Command tree shape": general, users,
// problems, submissions, emails, scoreboard.
func TestRoot_RegistersAllSixGroups(t *testing.T) {
	t.Parallel()
	root := cli.NewRootCmd(stubDeps())

	want := map[string]string{
		"general":     "General Commands:",
		"users":       "User & Team Commands:",
		"problems":    "Problem Commands:",
		"submissions": "Submission Commands:",
		"emails":      "Email Commands:",
		"scoreboard":  "Scoreboard Commands:",
	}
	got := map[string]string{}
	for _, g := range root.Groups() {
		got[g.ID] = g.Title
	}
	for id, title := range want {
		if got[id] != title {
			t.Errorf("group %q: title = %q, want %q", id, got[id], title)
		}
	}
}

func TestRoot_PinsHelpAndCompletionToGeneral(t *testing.T) {
	t.Parallel()
	root := cli.NewRootCmd(stubDeps())
	// Cobra only auto-registers help/completion when at least one
	// subcommand exists. Add an in-test stub so we can read the
	// auto-generated commands' GroupIDs.
	root.AddCommand(&cobra.Command{Use: "stub", GroupID: "general", RunE: func(*cobra.Command, []string) error { return nil }})
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()

	help, _, err := root.Find([]string{"help"})
	if err != nil || help == nil {
		t.Fatalf("Find(help): cmd=%v err=%v", help, err)
	}
	if got, want := help.GroupID, "general"; got != want {
		t.Errorf("help GroupID = %q, want %q", got, want)
	}

	completion, _, err := root.Find([]string{"completion"})
	if err != nil || completion == nil {
		t.Fatalf("Find(completion): cmd=%v err=%v", completion, err)
	}
	if got, want := completion.GroupID, "general"; got != want {
		t.Errorf("completion GroupID = %q, want %q", got, want)
	}
}

func TestRoot_PersistentFlags_BindAtRoot(t *testing.T) {
	t.Parallel()
	root := cli.NewRootCmd(stubDeps())
	for _, name := range []string{"config", "verbose", "disable-ssl", "no-progress"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("persistent flag %q not registered", name)
		}
	}
	// Short forms wired.
	if root.PersistentFlags().ShorthandLookup("c") == nil {
		t.Errorf("persistent flag -c not registered")
	}
	if root.PersistentFlags().ShorthandLookup("v") == nil {
		t.Errorf("persistent flag -v not registered")
	}
}

// TestRootCmd_NoProgressFlag_Registered locks the persistent
// `--no-progress` flag added in PR-9: the flag must exist on the
// root, default to false, and bind no shorthand (only long form is
// part of the public CLI contract).
func TestRootCmd_NoProgressFlag_Registered(t *testing.T) {
	t.Parallel()
	root := cli.NewRootCmd(stubDeps())

	flag := root.PersistentFlags().Lookup("no-progress")
	if flag == nil {
		t.Fatalf("persistent flag --no-progress not registered")
	}
	if flag.DefValue != "false" {
		t.Errorf("--no-progress DefValue = %q, want %q", flag.DefValue, "false")
	}
	if flag.Shorthand != "" {
		t.Errorf("--no-progress Shorthand = %q, want empty (long form only)", flag.Shorthand)
	}
}

// TestRootCmd_PreRunE_InstallsReporter confirms the root PreRunE
// stashes a non-nil Reporter on ctx so cli/{group} subcommands and
// the libraries they call can pull it via ReporterFromContext without
// nil-checking. The probe subcommand RunE is the read site, mirroring
// TestRoot_VerboseSetsSlogLevel.
func TestRootCmd_PreRunE_InstallsReporter(t *testing.T) {
	t.Parallel()
	root := cli.NewRootCmd(stubDeps())

	var captured progress.Reporter
	probe := &cobra.Command{
		Use: "probe",
		RunE: func(cmd *cobra.Command, _ []string) error {
			captured = cli.ReporterFromContext(cmd.Context())
			return nil
		},
	}
	root.AddCommand(probe)
	root.SetArgs([]string{"probe"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if captured == nil {
		t.Fatalf("Reporter not stashed on ctx")
	}
	// Stderr is bound to a *bytes.Buffer in this test, which is not
	// a *os.File — progress.New must therefore return Nop, and the
	// Reporter must be safe to drive end-to-end without panicking.
	captured.Start(2, "probe")
	captured.Tick()
	captured.Finish()
}

// TestRootCmd_PreRunE_InstallsReporter_NoProgress confirms passing
// --no-progress still results in a non-nil Reporter on ctx (Nop), so
// downstream callers can always Tick.
func TestRootCmd_PreRunE_InstallsReporter_NoProgress(t *testing.T) {
	t.Parallel()
	root := cli.NewRootCmd(stubDeps())

	var captured progress.Reporter
	probe := &cobra.Command{
		Use: "probe",
		RunE: func(cmd *cobra.Command, _ []string) error {
			captured = cli.ReporterFromContext(cmd.Context())
			return nil
		},
	}
	root.AddCommand(probe)
	root.SetArgs([]string{"probe", "--no-progress"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if captured == nil {
		t.Fatalf("Reporter not stashed on ctx with --no-progress")
	}
}

func TestRoot_VerboseSetsSlogLevel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args      []string
		wantLevel slog.Level
	}{
		{args: []string{}, wantLevel: slog.LevelWarn},
		{args: []string{"-v"}, wantLevel: slog.LevelInfo},
		{args: []string{"-vv"}, wantLevel: slog.LevelDebug},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			t.Parallel()
			root := cli.NewRootCmd(stubDeps())
			var captured *slog.Logger
			probe := &cobra.Command{
				Use: "probe",
				RunE: func(cmd *cobra.Command, _ []string) error {
					captured, _ = cli.LoggerFromContext(cmd.Context())
					return nil
				},
			}
			root.AddCommand(probe)
			root.SetArgs(append([]string{"probe"}, tc.args...))
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			if err := root.ExecuteContext(context.Background()); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if captured == nil {
				t.Fatalf("logger not stashed on ctx")
			}
			if !captured.Enabled(context.Background(), tc.wantLevel) {
				t.Errorf("logger not enabled at %v", tc.wantLevel)
			}
			// Verify upper-bound: one level below shouldn't be enabled.
			oneBelow := tc.wantLevel - 4
			if oneBelow >= slog.LevelDebug-4 && captured.Enabled(context.Background(), oneBelow) {
				t.Errorf("logger leaked level %v (wanted floor %v)", oneBelow, tc.wantLevel)
			}
		})
	}
}

// TestRoot_VerboseRebindsSlogDefault verifies that verbose configuration also
// reaches subsystems using slog.Default(), including router and dedup logging.
func TestRoot_VerboseRebindsSlogDefault(t *testing.T) {
	root := cli.NewRootCmd(stubDeps())
	var buf bytes.Buffer
	probe := &cobra.Command{
		Use: "probe",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Restore the package default after the test so other
			// tests in the package don't see the buf-bound logger.
			defaultBefore := slog.Default()
			t.Cleanup(func() { slog.SetDefault(defaultBefore) })
			// The PreRunE installed the verbose logger on
			// slog.Default; emit a Debug entry and assert the
			// stderr-bound buf captured it.
			slog.Debug("probe-debug", "marker", "phase1.5")
			return nil
		},
	}
	root.AddCommand(probe)
	root.SetArgs([]string{"probe", "-vv"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&buf)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "probe-debug") {
		t.Errorf("slog.Default() not rebound; -vv didn't print probe-debug, stderr was:\n%s", buf.String())
	}
}

func TestExecute_Returns_FromSubcommand(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("boom")
	root := cli.NewRootCmd(stubDeps())
	root.AddCommand(&cobra.Command{
		Use:          "fail",
		SilenceUsage: true,
		RunE:         func(cmd *cobra.Command, _ []string) error { return wantErr },
	})
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"fail"})

	err := cli.RunRoot(context.Background(), root)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunRoot err = %v, want wraps %v", err, wantErr)
	}
}

func TestExecute_PrintsErrorOnce_NoUsage(t *testing.T) {
	t.Parallel()
	root := cli.NewRootCmd(stubDeps())
	root.AddCommand(&cobra.Command{
		Use:          "fail",
		SilenceUsage: true,
		RunE:         func(cmd *cobra.Command, _ []string) error { return errors.New("kaboom") },
	})
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"fail"})

	_ = cli.RunRoot(context.Background(), root)

	got := stderr.String()
	if !strings.Contains(got, "kaboom") {
		t.Errorf("stderr missing error string %q in:\n%s", "kaboom", got)
	}
	if strings.Contains(got, "Usage:") {
		t.Errorf("stderr should NOT contain Usage: (SilenceUsage), got:\n%s", got)
	}
	if strings.Count(got, "kaboom") > 1 {
		t.Errorf("stderr printed error more than once:\n%s", got)
	}
}
