package submissions

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrEmptyArchive is returned by ExtractSingleFile when the zip has
// no regular file entries.
var ErrEmptyArchive = errors.New("submissions: empty archive")

// ErrUnsafeEntry is returned when the chosen archive entry's name
// contains a path-traversal component. Defense-in-depth — the caller
// already controls the write path; this rejects malformed archives
// before any I/O happens.
var ErrUnsafeEntry = errors.New("submissions: unsafe entry path")

// ExtractSingleFile opens zipPath, picks the alphabetically-first
// regular file entry, copies it next to the zip with a new name of
// `<newStem><ext>` (where ext is taken from the picked entry), and
// removes the original zip on success. Returns the path of the
// extracted file.
//
// Mirrors Python `services/api/v4/submissions.py:80-92`'s is_extract
// path with one safety upgrade: the entry order is locked
// alphabetically, while Python relied on `glob`'s undefined order.
func ExtractSingleFile(zipPath, newStem string) (string, error) {
	rc, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("submissions: open zip %s: %w", zipPath, err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = rc.Close()
		}
	}()

	entries := make([]*zip.File, 0, len(rc.File))
	for _, f := range rc.File {
		if f.FileInfo().IsDir() {
			continue
		}
		entries = append(entries, f)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("%w: %s", ErrEmptyArchive, zipPath)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	pick := entries[0]

	if strings.Contains(pick.Name, "..") || strings.HasPrefix(pick.Name, "/") {
		return "", fmt.Errorf("%w: %q", ErrUnsafeEntry, pick.Name)
	}

	ext := filepath.Ext(pick.Name)
	out := filepath.Join(filepath.Dir(zipPath), newStem+ext)

	src, err := pick.Open()
	if err != nil {
		return "", fmt.Errorf("submissions: open entry: %w", err)
	}
	dst, err := os.Create(out)
	if err != nil {
		_ = src.Close()
		return "", fmt.Errorf("submissions: create %s: %w", out, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = src.Close()
		_ = dst.Close()
		return "", fmt.Errorf("submissions: extract %s: %w", pick.Name, err)
	}
	if err := src.Close(); err != nil {
		_ = dst.Close()
		return "", fmt.Errorf("submissions: close entry: %w", err)
	}
	if err := dst.Close(); err != nil {
		return "", fmt.Errorf("submissions: close output: %w", err)
	}

	// Close the reader before removing the underlying file (Windows
	// cannot delete an open file; the test suite runs cross-platform
	// in CI).
	if err := rc.Close(); err != nil {
		return "", fmt.Errorf("submissions: close zip: %w", err)
	}
	closed = true
	if err := os.Remove(zipPath); err != nil {
		return "", fmt.Errorf("submissions: remove zip: %w", err)
	}
	return out, nil
}
