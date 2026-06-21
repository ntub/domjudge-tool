// Package problems provides use-case functions for managing contest
// problems — currently download. The Gateway is caller-supplied so
// tests can inject a stub without touching the network.
package problems

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/pathx"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// Result is the outcome of a Download call.
type Result struct {
	Downloaded int
	Skipped    int    // problems with no ExportFilePath; counted in progress total
	OutDir     string // absolute path of the output directory
}

// Download lists problems from gw using filter, creates outDir,
// then streams each export ZIP to disk under outDir.
//
// The caller is responsible for calling gw.Login before Download.
//
// Filename format mirrors Python _problems.py:32:
//
//	${outDir}/${id}_${name}.zip  (spaces in name → "-")
//
// Problems with an empty or nil ExportFilePath are silently skipped
// (Python parity: continue inside the loop). rep.Tick is NOT called
// for skipped items; rep.Finish always closes the progress at 100%.
func Download(
	ctx context.Context,
	gw web.Gateway,
	filter web.ProblemFilter,
	outDir string,
	rep progress.Reporter,
) (Result, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("problems: mkdir %s: %w", outDir, err)
	}
	abs, err := filepath.Abs(outDir)
	if err != nil {
		abs = outDir
	}

	items, err := gw.GetProblems(ctx, filter)
	if err != nil {
		return Result{}, fmt.Errorf("problems: list: %w", err)
	}

	rep.Start(len(items), "downloading")
	defer rep.Finish()

	var skipped int
	for _, p := range items {
		if err := ctx.Err(); err != nil {
			return Result{}, fmt.Errorf("problems: %w", err)
		}
		if p.ExportFilePath == nil || *p.ExportFilePath == "" {
			skipped++
			continue
		}
		if err := writeProblemZip(ctx, gw, abs, p); err != nil {
			return Result{}, err
		}
		rep.Tick()
	}

	return Result{
		Downloaded: len(items) - skipped,
		Skipped:    skipped,
		OutDir:     abs,
	}, nil
}

// writeProblemZip streams gw.Download(*p.ExportFilePath) into
// ${folder}/${p.ID}_${p.Name}.zip. The caller has already guarded
// against a nil/empty ExportFilePath.
func writeProblemZip(ctx context.Context, gw web.Gateway, folder string, p model.ProblemItem) error {
	rc, err := gw.Download(ctx, *p.ExportFilePath)
	if err != nil {
		return fmt.Errorf("problems: download %s: %w", *p.ExportFilePath, err)
	}
	defer func() { _ = rc.Close() }()

	filename := pathx.SafeFilename(fmt.Sprintf("%s_%s.zip", p.ID, p.Name))
	dst := filepath.Join(folder, filename)
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("problems: create %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("problems: write %s: %w", dst, err)
	}
	return nil
}
