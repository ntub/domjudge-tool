package problems

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	iproblems "github.com/ntub/domjudge-tool/internal/problems"
)

// NewImportProblemsCmd builds the `import-problems <zip-or-folder>`
// subcommand. It imports one ZIP file or every top-level *.zip file in
// a folder into cfg.contest_id via DOMjudge's API v4 problem import
// endpoint.
func NewImportProblemsCmd(deps Deps) *cobra.Command {
	var (
		problemID string
		upsert    bool
	)
	cmd := &cobra.Command{
		Use:   "import-problems <zip-or-folder>",
		Short: "Import problem package ZIPs into the configured contest",
		Long: "Imports one problem package ZIP, or every top-level *.zip file\n" +
			"in a folder, into domserver.json contest_id. New imports omit\n" +
			"the optional problem field so DOMjudge creates the problem from\n" +
			"the package. Use --upsert to update matching contest problems\n" +
			"and create missing ones. Use --problem-id only when updating an\n" +
			"existing problem with a single ZIP input.",
		GroupID:       groupID,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cli.LoadConfigPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, ok := cli.ConfigFromContext(ctx)
			if !ok {
				return fmt.Errorf("import-problems: config not on context (PreRunE missing)")
			}
			cid := strings.TrimSpace(derefString(cfg.ContestID))
			if cid == "" {
				return fmt.Errorf("import-problems: domserver.json contest_id is required")
			}
			if upsert && strings.TrimSpace(problemID) != "" {
				return fmt.Errorf("import-problems: --upsert and --problem-id are mutually exclusive")
			}
			inputs, err := collectProblemZIPs(args[0], problemID)
			if err != nil {
				return err
			}
			newAPI := deps.NewAPI
			if newAPI == nil {
				newAPI = defaultNewAPI
			}
			api := newAPI(cfg)
			if api == nil || api.Imports == nil {
				return fmt.Errorf("import-problems: api imports resource unavailable")
			}
			if upsert && api.Problems == nil {
				return fmt.Errorf("import-problems: api problems resource unavailable")
			}

			rep := cli.ReporterFromContext(ctx)
			result, err := iproblems.Import(
				ctx, api.Imports, api.Problems,
				cfg.Version, cid, inputs, upsert, rep,
			)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Imported %d problems.\n", result.Imported)
			return nil
		},
	}
	cmd.Flags().StringVar(&problemID, "problem-id", "", "problem id for a single ZIP input")
	cmd.Flags().BoolVar(&upsert, "upsert", false, "create missing contest problems and update matching contest problems")
	return cmd
}

func collectProblemZIPs(path, explicitProblemID string) ([]iproblems.ImportInput, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("import-problems: stat %s: %w", path, err)
	}
	if !info.IsDir() {
		if !isZipPath(path) {
			return nil, fmt.Errorf("import-problems: %s is not a .zip file", path)
		}
		id := strings.TrimSpace(explicitProblemID)
		return []iproblems.ImportInput{{Path: path, ID: id, Name: filepath.Base(path)}}, nil
	}
	if strings.TrimSpace(explicitProblemID) != "" {
		return nil, fmt.Errorf("import-problems: --problem-id can only be used with a single ZIP file")
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("import-problems: read dir %s: %w", path, err)
	}
	inputs := make([]iproblems.ImportInput, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isZipPath(entry.Name()) {
			continue
		}
		zipPath := filepath.Join(path, entry.Name())
		inputs = append(inputs, iproblems.ImportInput{Path: zipPath, Name: entry.Name()})
	}
	sort.Slice(inputs, func(i, j int) bool {
		return inputs[i].Path < inputs[j].Path
	})
	if len(inputs) == 0 {
		return nil, fmt.Errorf("import-problems: no .zip files found in %s", path)
	}
	return inputs, nil
}

func isZipPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".zip")
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
