package users

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/exporter"
	"github.com/ntub/domjudge-tool/internal/importer"
)

// importFn matches importer.ImportUsersTeamsWithReport's signature so tests can
// stub the call without an HTTP server. The compile-time check below
// guards against signature drift.
type importFn = func(
	context.Context,
	web.Gateway,
	*apiv4.UsersResource,
	io.Reader,
	importer.ImportOptions,
) (importer.ImportUsersTeamsResult, error)

// importUsersTeamsFn is the package-level seam: production points at
// the real importer; tests in package users replace it.
var importUsersTeamsFn importFn = importer.ImportUsersTeamsWithReport

// NewImportUsersTeamsCmd builds the `import-users-teams <file>`
// subcommand: drives importer.ImportUsersTeamsWithReport against the
// supplied CSV, writing the resulting CreateUser slice to
// ./import-users-teams-out.csv (Python parity).
//
// Deviation from Python: --ignore-existing and --delete-existing are
// MarkFlagsMutuallyExclusive. Python silently let delete win when
// both were set; a flag-validation rejection makes the conflict
// visible to the operator.
func NewImportUsersTeamsCmd(deps Deps) *cobra.Command {
	var (
		categoryID         int
		affiliationID      int
		userRoles          []int
		enabled            bool
		affiliationCountry string
		ignoreExisting     bool
		deleteExisting     bool
		passwordLen        int
		passwordCharset    string
		newPassword        bool
	)
	cmd := &cobra.Command{
		Use:           "import-users-teams <file>",
		Short:         "Reconcile a CSV against the live user list",
		GroupID:       groupID,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("import-users-teams: config not on context")
			}
			file, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("import-users-teams: open %s: %w", args[0], err)
			}
			defer func() { _ = file.Close() }()

			api := deps.NewAPI(cfg)
			gw, err := deps.NewGateway(ctx, cfg)
			if err != nil {
				return fmt.Errorf("import-users-teams: gateway: %w", err)
			}

			country := affiliationCountry
			if country == "" && cfg.AffiliationCountry != nil {
				country = *cfg.AffiliationCountry
			}
			opts := importer.ImportOptions{
				CategoryID:         intPtrFromFlagOrConfig(cmd, "category-id", categoryID, cfg.CategoryID),
				AffiliationID:      intPtrFromFlagOrConfig(cmd, "affiliation-id", affiliationID, cfg.AffiliationID),
				UserRoles:          intSliceFromFlagOrConfig(cmd, "user-roles", userRoles, cfg.UserRoles),
				AffiliationCountry: country,
				Enabled:            enabled,
				IgnoreExisting:     ignoreExisting,
				DeleteExisting:     deleteExisting,
				NewPassword:        newPassword,
				PasswordLength:     passwordLen,
				PasswordCharset:    passwordCharset,
				Router:             deps.routerFor(cfg),
				API:                api,
				ContestHint:        derefString(cfg.ContestID),
				Reporter:           cli.ReporterFromContext(ctx),
			}

			result, err := importUsersTeamsFn(ctx, gw, api.Users, file, opts)
			if err != nil {
				return fmt.Errorf("import-users-teams: %w", err)
			}

			const outputFile = "import-users-teams-out.csv"
			out, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("import-users-teams: create output: %w", err)
			}
			defer func() { _ = out.Close() }()
			if err := exporter.CreateUsers(result.Users, exporter.FormatCSV, out); err != nil {
				return fmt.Errorf("import-users-teams: write output: %w", err)
			}
			printImportSummary(cmd, result.Summary, outputFile)
			return nil
		},
	}

	f := cmd.Flags()
	f.IntVar(&categoryID, "category-id", 0, "default category id")
	f.IntVar(&affiliationID, "affiliation-id", 0, "fallback affiliation id when CSV column is empty")
	f.IntSliceVar(&userRoles, "user-roles", nil, "user role ids (comma-separated)")
	f.BoolVar(&enabled, "enabled", true, "leave each created user enabled")
	f.StringVar(&affiliationCountry, "affiliation-country", "", "country for new affiliations (default: cfg.affiliation_country)")
	f.BoolVar(&ignoreExisting, "ignore-existing", false, "skip rows whose username already exists")
	f.BoolVar(&deleteExisting, "delete-existing", false, "delete then recreate rows whose username already exists")
	f.IntVar(&passwordLen, "password-length", 0, "length when generating passwords (0 = util/password default)")
	f.StringVar(&passwordCharset, "password-pattern", "", "charset when generating passwords")
	f.BoolVar(&newPassword, "new-password", false, "always regenerate passwords (even when CSV provides one)")

	cmd.MarkFlagsMutuallyExclusive("ignore-existing", "delete-existing")
	return cmd
}

// printImportSummary writes a multiline stdout summary for successful
// imports. Keep stdout user-facing and machine-readable enough for
// shell logs; progress and diagnostics stay on stderr.
func printImportSummary(cmd *cobra.Command, summary importer.ImportUsersTeamsSummary, outputFile string) {
	createdNames := "none"
	if len(summary.AffiliationsCreatedNames) > 0 {
		createdNames = strings.Join(summary.AffiliationsCreatedNames, ", ")
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"Import summary:\n"+
			"  Input rows: %d\n"+
			"  Processed rows: %d\n"+
			"  Created users: %d\n"+
			"  Updated users: %d\n"+
			"  Skipped existing users: %d\n"+
			"  Affiliations created: %d (%s)\n"+
			"  Affiliations reused: %d unique / %d rows\n"+
			"  Fallback affiliation-id rows: %d (id=%d)\n"+
			"  Output file: %s\n",
		summary.InputRows,
		summary.ProcessedRows,
		summary.CreatedUsers,
		summary.UpdatedUsers,
		summary.SkippedExistingUsers,
		summary.AffiliationsCreated,
		createdNames,
		summary.AffiliationsReusedUnique,
		summary.AffiliationsReusedRows,
		summary.FallbackAffiliationIDRows,
		summary.FallbackAffiliationID,
		outputFile,
	)
}

func intPtrFromFlagOrConfig(cmd *cobra.Command, name string, flagValue int, cfgValue *int) *int {
	if !cmd.Flags().Changed(name) {
		if cfgValue == nil {
			return nil
		}
		v := *cfgValue
		return &v
	}
	v := flagValue
	return &v
}

func intSliceFromFlagOrConfig(cmd *cobra.Command, name string, flagValue, cfgValue []int) []int {
	if !cmd.Flags().Changed(name) {
		return cloneIntSlice(cfgValue)
	}
	return cloneIntSlice(flagValue)
}

func cloneIntSlice(xs []int) []int {
	if xs == nil {
		return nil
	}
	return append([]int{}, xs...)
}
