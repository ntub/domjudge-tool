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

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

const maxProblemINIIdentityBytes = 64 * 1024

// ImportInput describes one problem ZIP to import.
type ImportInput struct {
	Path string // local filesystem path to the ZIP file (must already be closed/flushed)
	Name string // filename to send as the multipart filename header
	ID   string // optional problem ID; empty = create new, non-empty = update existing
}

// ImportResult is the outcome of an Import call.
type ImportResult struct {
	Imported int
}

// ProblemImporter is the subset of *apiv4.ImportsResource used by Import.
type ProblemImporter interface {
	Problem(ctx context.Context, cid, problemID string, zip io.Reader) (string, error)
	ProblemPackageUpdate(ctx context.Context, cid, problemID, filename string, zip io.Reader) (string, error)
	ProblemPackage(ctx context.Context, cid, filename string, zip io.Reader) error
	ProblemPackageZipArray(ctx context.Context, cid, filename string, zip io.Reader) error
}

// ProblemLister is the subset of *apiv4.ProblemsResource used by upsert resolution.
type ProblemLister interface {
	List(ctx context.Context, cid string) ([]model.Problem, error)
}

// Import uploads each input ZIP to the DOMjudge contest identified by cid.
// When upsert is true, lister is queried for existing contest problems and each
// input is matched to an existing problem; matched inputs are sent as updates.
// rep tracks upload progress; rep.Start and rep.Finish are called by Import.
func Import(
	ctx context.Context,
	importer ProblemImporter,
	lister ProblemLister,
	version, cid string,
	inputs []ImportInput,
	upsert bool,
	rep progress.Reporter,
) (ImportResult, error) {
	if upsert {
		if lister == nil {
			return ImportResult{}, fmt.Errorf("import-problems: lister required for upsert mode")
		}
		if err := resolveProblemUpserts(ctx, lister, version, cid, inputs); err != nil {
			return ImportResult{}, err
		}
	}

	rep.Start(len(inputs), "importing")
	defer rep.Finish()

	for i := range inputs {
		if err := ctx.Err(); err != nil {
			return ImportResult{}, fmt.Errorf("import-problems: %w", err)
		}
		if err := importProblemZIP(ctx, importer, version, cid, inputs[i]); err != nil {
			return ImportResult{}, err
		}
		rep.Tick()
	}
	return ImportResult{Imported: len(inputs)}, nil
}

// problemUpsertTarget holds the resolved target for an upsert match.
type problemUpsertTarget struct {
	id         string
	uploadName string
}

func importProblemZIP(ctx context.Context, importer ProblemImporter, version, cid string, input ImportInput) error {
	f, err := os.Open(input.Path)
	if err != nil {
		return fmt.Errorf("import-problems: open %s: %w", input.Path, err)
	}
	defer func() { _ = f.Close() }()
	var importErr error
	if input.ID == "" {
		if isDOMjudge7(version) {
			importErr = importer.ProblemPackageZipArray(ctx, cid, input.Name, f)
		} else {
			importErr = importer.ProblemPackage(ctx, cid, input.Name, f)
		}
	} else {
		if isDOMjudge7(version) {
			_, importErr = importer.Problem(ctx, cid, input.ID, f)
		} else {
			_, importErr = importer.ProblemPackageUpdate(ctx, cid, input.ID, input.Name, f)
		}
	}
	if importErr != nil {
		return fmt.Errorf("import-problems: import %s as %s: %w%s",
			input.Path, importProblemLabel(input), importErr, apiErrorBodySuffix(importErr))
	}
	return nil
}

func resolveProblemUpserts(
	ctx context.Context,
	lister ProblemLister,
	version, cid string,
	inputs []ImportInput,
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
				inputs[i].ID = target.id
				if !isDOMjudge7(version) && target.uploadName != "" {
					inputs[i].Name = target.uploadName
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

func inferProblemIdentities(version string, input ImportInput) ([]string, error) {
	if !isDOMjudge7(version) {
		if externalID, ok := problemExternalIDFromZip(input.Path); ok {
			return []string{externalID}, nil
		}
	}
	identities := problemIdentitiesFromFilename(input.Name)
	if len(identities) == 0 {
		return nil, fmt.Errorf("import-problems: cannot infer problem identity from %s", input.Path)
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

// SortInputs sorts a slice of ImportInput by path — exposed for CLI use.
func SortInputs(inputs []ImportInput) {
	sort.Slice(inputs, func(i, j int) bool {
		return inputs[i].Path < inputs[j].Path
	})
}

func importProblemLabel(input ImportInput) string {
	if input.ID != "" {
		return input.ID
	}
	return "new problem"
}

// IsDOMjudge7 reports whether version is a 7.x DOMjudge server.
// Exported so callers (CLI, desktop) can branch on API shape without
// re-implementing the check.
func IsDOMjudge7(version string) bool {
	return isDOMjudge7(version)
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
