package problems

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
)

func TestParseProblemExternalID(t *testing.T) {
	tests := []struct {
		name   string
		ini    string
		wantID string
		wantOK bool
	}{
		{"empty", "", "", false},
		{"comments only", "# comment\n; comment", "", false},
		{"no equals", "externalid", "", false},
		{"wrong key", "timelimit = 1", "", false},
		{"valid lowercase", "externalid = hello-world", "hello-world", true},
		{"valid with double quotes", "EXTERNALID = \"quoted-id\"", "quoted-id", true},
		{"valid with single quotes", "ExternalId = 'single-quoted'", "single-quoted", true},
		{"empty value", "externalid = \"\"", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseProblemExternalID(tc.ini)
			if ok != tc.wantOK {
				t.Fatalf("parseProblemExternalID ok=%v, want %v", ok, tc.wantOK)
			}
			if got != tc.wantID {
				t.Fatalf("parseProblemExternalID got=%q, want %q", got, tc.wantID)
			}
		})
	}
}

func TestProblemExternalIDFromZip(t *testing.T) {
	const identity = "externalid = boundary-id\n#"
	tests := []struct {
		name   string
		entry  string
		body   string
		wantID string
		wantOK bool
	}{
		{"valid INI", "domjudge-problem.ini", "externalid = my-ext-id\n", "my-ext-id", true},
		{"no INI", "problem.yaml", "name: Hello\n", "", false},
		{"no external ID", "domjudge-problem.ini", "timelimit = 1\n", "", false},
		{"one byte below limit", "domjudge-problem.ini", identity + strings.Repeat("x", maxProblemINIIdentityBytes-len(identity)-1), "boundary-id", true},
		{"at limit", "domjudge-problem.ini", identity + strings.Repeat("x", maxProblemINIIdentityBytes-len(identity)), "boundary-id", true},
		{"one byte above limit", "domjudge-problem.ini", identity + strings.Repeat("x", maxProblemINIIdentityBytes-len(identity)+1), "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "problem.zip")
			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("create archive: %v", err)
			}
			t.Cleanup(func() { _ = f.Close() })
			zw := zip.NewWriter(f)
			w, err := zw.Create(tc.entry)
			if err != nil {
				t.Fatalf("create archive entry: %v", err)
			}
			if _, err := io.WriteString(w, tc.body); err != nil {
				t.Fatalf("write archive entry: %v", err)
			}
			if err := zw.Close(); err != nil {
				t.Fatalf("close archive: %v", err)
			}
			if err := f.Close(); err != nil {
				t.Fatalf("close archive file: %v", err)
			}

			got, ok := problemExternalIDFromZip(path)
			if got != tc.wantID || ok != tc.wantOK {
				t.Fatalf("problemExternalIDFromZip = (%q, %v), want (%q, %v)", got, ok, tc.wantID, tc.wantOK)
			}
		})
	}

	t.Run("missing archive", func(t *testing.T) {
		got, ok := problemExternalIDFromZip(filepath.Join(t.TempDir(), "missing.zip"))
		if got != "" || ok {
			t.Fatalf("missing archive = (%q, %v), want (empty, false)", got, ok)
		}
	})
	t.Run("malformed archive", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "invalid.zip")
		if err := os.WriteFile(path, []byte("not a ZIP archive"), 0o600); err != nil {
			t.Fatalf("write malformed archive: %v", err)
		}
		got, ok := problemExternalIDFromZip(path)
		if got != "" || ok {
			t.Fatalf("malformed archive = (%q, %v), want (empty, false)", got, ok)
		}
	})
}

func TestApiErrorBodySuffix(t *testing.T) {
	// Not an HTTPError
	if got := apiErrorBodySuffix(errors.New("plain error")); got != "" {
		t.Errorf("expected empty suffix for plain error, got %q", got)
	}

	// HTTPError with empty body
	if got := apiErrorBodySuffix(&apiv4.HTTPError{StatusCode: 400}); got != "" {
		t.Errorf("expected empty suffix for empty body, got %q", got)
	}

	// HTTPError with whitespace only body
	if got := apiErrorBodySuffix(&apiv4.HTTPError{StatusCode: 400, Body: []byte("   \n\t  ")}); got != "" {
		t.Errorf("expected empty suffix for whitespace body, got %q", got)
	}

	// HTTPError with short body
	shortErr := &apiv4.HTTPError{StatusCode: 400, Body: []byte("failed to import")}
	if got := apiErrorBodySuffix(shortErr); got != ": failed to import" {
		t.Errorf("got %q, want ': failed to import'", got)
	}

	// HTTPError with body > 500
	longErr := &apiv4.HTTPError{StatusCode: 400, Body: []byte(strings.Repeat("a", 600))}
	got := apiErrorBodySuffix(longErr)
	if !strings.HasSuffix(got, "...") || len(got) != 2+500+3 {
		t.Errorf("expected truncated body with suffix '...', got len %d: %q", len(got), got)
	}
}
