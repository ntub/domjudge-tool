package problems

import (
	"archive/zip"
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
	"github.com/ntub/domjudge-tool/internal/model"
)

const maxProblemINIIdentityBytes = 64 * 1024

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
			if upsert {
				if api.Problems == nil {
					return fmt.Errorf("import-problems: api problems resource unavailable")
				}
				if err := resolveProblemUpserts(ctx, api.Problems, cfg.Version, cid, inputs); err != nil {
					return err
				}
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
	cmd.Flags().BoolVar(&upsert, "upsert", false, "create missing contest problems and update matching contest problems")
	return cmd
}

type problemZIPInput struct {
	path string
	id   string
	name string
}

type problemUpsertTarget struct {
	id         string
	uploadName string
}

type problemImporter interface {
	Problem(ctx context.Context, cid, problemID string, zip io.Reader) (string, error)
	ProblemPackageUpdate(ctx context.Context, cid, problemID, filename string, zip io.Reader) (string, error)
	ProblemPackage(ctx context.Context, cid, filename string, zip io.Reader) error
	ProblemPackageZipArray(ctx context.Context, cid, filename string, zip io.Reader) error
}

type problemLister interface {
	List(ctx context.Context, cid string) ([]model.Problem, error)
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
		if isDOMjudge7(version) {
			_, importErr = importer.Problem(ctx, cid, input.id, f)
		} else {
			_, importErr = importer.ProblemPackageUpdate(ctx, cid, input.id, input.name, f)
		}
	}
	if importErr != nil {
		return fmt.Errorf("import-problems: import %s as %s: %w%s",
			input.path, importProblemLabel(input), importErr, apiErrorBodySuffix(importErr))
	}
	return nil
}

func resolveProblemUpserts(
	ctx context.Context,
	lister problemLister,
	version, cid string,
	inputs []problemZIPInput,
) error {
	existing, err := lister.List(ctx, cid)
	if err != nil {
		return fmt.Errorf("import-problems: contest %s problems: %w", cid, err)
	}
	byIdentity, err := problemIndex(existing)
	if err != nil {
		return err
	}
	for i := range inputs {
		identities, err := inferProblemIdentities(version, inputs[i])
		if err != nil {
			return err
		}
		for _, identity := range identities {
			if target := byIdentity[identity]; target.id != "" {
				inputs[i].id = target.id
				if !isDOMjudge7(version) && target.uploadName != "" {
					inputs[i].name = target.uploadName
				}
				break
			}
		}
	}
	return nil
}

func problemIndex(problems []model.Problem) (map[string]problemUpsertTarget, error) {
	out := make(map[string]problemUpsertTarget, len(problems)*2)
	for _, p := range problems {
		if p.ID == "" {
			continue
		}
		target := problemTarget(p)
		if err := addProblemIdentity(out, p.ID, target); err != nil {
			return nil, err
		}
		if err := addProblemIdentity(out, p.ExternalID, target); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func problemTarget(p model.Problem) problemUpsertTarget {
	uploadID := strings.TrimSpace(p.ExternalID)
	if uploadID == "" {
		uploadID = strings.TrimSpace(p.ID)
	}
	target := problemUpsertTarget{id: p.ID}
	if uploadID != "" {
		target.uploadName = uploadID + ".zip"
	}
	return target
}

func addProblemIdentity(index map[string]problemUpsertTarget, identity string, target problemUpsertTarget) error {
	identity = strings.TrimSpace(identity)
	if identity == "" || target.id == "" {
		return nil
	}
	if existing := index[identity]; existing.id != "" && existing.id != target.id {
		return fmt.Errorf("import-problems: ambiguous problem identity %q matches both %s and %s", identity, existing.id, target.id)
	}
	index[identity] = target
	return nil
}

func inferProblemIdentities(version string, input problemZIPInput) ([]string, error) {
	if !isDOMjudge7(version) {
		if externalID, ok := problemExternalIDFromZip(input.path); ok {
			return []string{externalID}, nil
		}
	}
	identities := problemIdentitiesFromFilename(input.name)
	if len(identities) == 0 {
		return nil, fmt.Errorf("import-problems: cannot infer problem identity from %s", input.path)
	}
	return identities, nil
}

func problemExternalIDFromZip(path string) (string, bool) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = zr.Close() }()
	for _, file := range zr.File {
		if file.Name != "domjudge-problem.ini" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", false
		}
		body, err := io.ReadAll(io.LimitReader(rc, maxProblemINIIdentityBytes+1))
		_ = rc.Close()
		if err != nil {
			return "", false
		}
		if len(body) > maxProblemINIIdentityBytes {
			return "", false
		}
		return parseProblemExternalID(string(body))
	}
	return "", false
}

func parseProblemExternalID(ini string) (string, bool) {
	for _, line := range strings.Split(ini, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(strings.ToLower(key)) != "externalid" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if value != "" {
			return value, true
		}
	}
	return "", false
}

func problemIdentitiesFromFilename(filename string) []string {
	stem := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	var b strings.Builder
	for _, r := range stem {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	full := b.String()
	if full == "" {
		return nil
	}
	out := []string{full}
	trimmed := strings.Trim(full, "-_")
	if trimmed != "" && trimmed != full {
		out = append(out, trimmed)
	}
	if before, _, ok := strings.Cut(full, "_"); ok && before != "" && before != full {
		out = append(out, before)
	}
	if before, _, ok := strings.Cut(trimmed, "_"); ok && before != "" && before != trimmed {
		out = append(out, before)
	}
	return out
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
