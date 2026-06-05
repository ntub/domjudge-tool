package problems

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
)

// NewImportProblemsCmd builds the `import-problems <zip-or-folder>`
// subcommand. It imports one ZIP file or every top-level *.zip file in
// a folder into cfg.contest_id via DOMjudge's API v4 problem import
// endpoint.
func NewImportProblemsCmd(deps Deps) *cobra.Command {
	var problemID string
	cmd := &cobra.Command{
		Use:   "import-problems <zip-or-folder>",
		Short: "Import problem package ZIPs into the configured contest",
		Long: "Imports one problem package ZIP, or every top-level *.zip file\n" +
			"in a folder, into domserver.json contest_id. New imports omit\n" +
			"the optional problem field so DOMjudge creates the problem from\n" +
			"the package. Use --problem-id only when updating an existing\n" +
			"problem with a single ZIP input.",
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

			rep := cli.ReporterFromContext(ctx)
			rep.Start(len(inputs), "importing")
			defer rep.Finish()

			for _, input := range inputs {
				if err := ctx.Err(); err != nil {
					return fmt.Errorf("import-problems: %w", err)
				}
				if err := importProblemZIP(ctx, api.Imports, cfg.Version, cid, input); err != nil {
					return err
				}
				rep.Tick()
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Imported %d problems.\n", len(inputs))
			return nil
		},
	}
	cmd.Flags().StringVar(&problemID, "problem-id", "", "problem id for a single ZIP input")
	return cmd
}

type problemZIPInput struct {
	path string
	id   string
	name string
}

type problemImporter interface {
	Problem(ctx context.Context, cid, problemID string, zip io.Reader) (string, error)
	ProblemPackage(ctx context.Context, cid, filename string, zip io.Reader) error
	ProblemPackageZipArray(ctx context.Context, cid, filename string, zip io.Reader) error
}

func collectProblemZIPs(path, explicitProblemID string) ([]problemZIPInput, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("import-problems: stat %s: %w", path, err)
	}
	if !info.IsDir() {
		if !isZipPath(path) {
			return nil, fmt.Errorf("import-problems: %s is not a .zip file", path)
		}
		id := strings.TrimSpace(explicitProblemID)
		return []problemZIPInput{{path: path, id: id, name: filepath.Base(path)}}, nil
	}
	if strings.TrimSpace(explicitProblemID) != "" {
		return nil, fmt.Errorf("import-problems: --problem-id can only be used with a single ZIP file")
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("import-problems: read dir %s: %w", path, err)
	}
	inputs := make([]problemZIPInput, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isZipPath(entry.Name()) {
			continue
		}
		zipPath := filepath.Join(path, entry.Name())
		inputs = append(inputs, problemZIPInput{path: zipPath, name: entry.Name()})
	}
	sort.Slice(inputs, func(i, j int) bool {
		return inputs[i].path < inputs[j].path
	})
	if len(inputs) == 0 {
		return nil, fmt.Errorf("import-problems: no .zip files found in %s", path)
	}
	return inputs, nil
}

func importProblemZIP(ctx context.Context, importer problemImporter, version, cid string, input problemZIPInput) error {
	f, err := os.Open(input.path)
	if err != nil {
		return fmt.Errorf("import-problems: open %s: %w", input.path, err)
	}
	defer func() { _ = f.Close() }()
	var importErr error
	if input.id == "" {
		if isDOMjudge7(version) {
			importErr = importer.ProblemPackageZipArray(ctx, cid, input.name, f)
		} else {
			importErr = importer.ProblemPackage(ctx, cid, input.name, f)
		}
	} else {
		_, importErr = importer.Problem(ctx, cid, input.id, f)
	}
	if importErr != nil {
		return fmt.Errorf("import-problems: import %s as %s: %w%s",
			input.path, importProblemLabel(input), importErr, apiErrorBodySuffix(importErr))
	}
	return nil
}

func importProblemLabel(input problemZIPInput) string {
	if input.id != "" {
		return input.id
	}
	return "new problem"
}

func isDOMjudge7(version string) bool {
	return strings.HasPrefix(strings.TrimSpace(version), "7.")
}

func apiErrorBodySuffix(err error) string {
	var httpErr *apiv4.HTTPError
	if !errors.As(err, &httpErr) || len(httpErr.Body) == 0 {
		return ""
	}
	body := strings.TrimSpace(string(httpErr.Body))
	if body == "" {
		return ""
	}
	if len(body) > 500 {
		body = body[:500] + "..."
	}
	return ": " + body
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
