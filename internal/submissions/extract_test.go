package submissions_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/submissions"
)

// makeZip writes a zip with the given entries to disk and returns the
// zip's path. Entries are written in slice order; ExtractSingleFile
// is expected to alphabetise on read so the output is deterministic.
func makeZip(t *testing.T, dir string, entries map[string][]byte) string {
	t.Helper()
	zipPath := filepath.Join(dir, "in.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zw.Create: %v", err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("zw.Write: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zw.Close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return zipPath
}

func TestExtractSingleFile_HappyPath_RenamesAndRemovesZip(t *testing.T) {
	dir := t.TempDir()
	zipPath := makeZip(t, dir, map[string][]byte{
		"main.cpp": []byte("int main(){}"),
	})

	out, err := submissions.ExtractSingleFile(zipPath, "accepted_42")
	if err != nil {
		t.Fatalf("ExtractSingleFile: %v", err)
	}
	if out != filepath.Join(dir, "accepted_42.cpp") {
		t.Errorf("returned path %q, want %q", out, filepath.Join(dir, "accepted_42.cpp"))
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(body, []byte("int main(){}")) {
		t.Errorf("extracted body mismatch")
	}
	if _, err := os.Stat(zipPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("zip not removed: stat err = %v", err)
	}
}

func TestExtractSingleFile_MultiFile_PicksAlphabeticallyFirst(t *testing.T) {
	dir := t.TempDir()
	zipPath := makeZip(t, dir, map[string][]byte{
		"zeta.py":  []byte("z"),
		"alpha.py": []byte("a"),
	})
	out, err := submissions.ExtractSingleFile(zipPath, "wrong_answer_7")
	if err != nil {
		t.Fatalf("ExtractSingleFile: %v", err)
	}
	if filepath.Base(out) != "wrong_answer_7.py" {
		t.Errorf("base = %q, want wrong_answer_7.py", filepath.Base(out))
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != "a" {
		t.Errorf("picked wrong file: body = %q, want %q", string(body), "a")
	}
}

func TestExtractSingleFile_EmptyArchiveError(t *testing.T) {
	dir := t.TempDir()
	zipPath := makeZip(t, dir, map[string][]byte{})

	_, err := submissions.ExtractSingleFile(zipPath, "judgement_1")
	if !errors.Is(err, submissions.ErrEmptyArchive) {
		t.Errorf("errors.Is(err, ErrEmptyArchive) = false; err = %v", err)
	}
}

func TestExtractSingleFile_RejectsTraversalEntry(t *testing.T) {
	dir := t.TempDir()
	zipPath := makeZip(t, dir, map[string][]byte{
		"../../etc/passwd": []byte("evil"),
	})
	_, err := submissions.ExtractSingleFile(zipPath, "x")
	if err == nil {
		t.Fatal("expected error for traversal entry, got nil")
	}
	if !errors.Is(err, submissions.ErrUnsafeEntry) {
		t.Errorf("errors.Is(err, ErrUnsafeEntry) = false; err = %v", err)
	}
}

func TestExtractSingleFile_ZipNotFound(t *testing.T) {
	_, err := submissions.ExtractSingleFile("/no/such/file.zip", "x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
