package users_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli/users"
)

const goldenTemplatePath = "../../../testdata/importer/template_python_golden.csv"

// TestImportUsersTeamsExample_GoldenCSV locks the current canonical
// import-users-teams.csv template emitted by the command.
func TestImportUsersTeamsExample_GoldenCSV(t *testing.T) {
	// Read the golden BEFORE Chdir flips cwd to the tempdir.
	want, err := os.ReadFile(goldenTemplatePath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	tmpDir := t.TempDir()
	chdir(t, tmpDir)

	cmd := users.NewImportUsersTeamsExampleCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tmpDir, "import-users-teams.csv"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("DoD #5 byte-identical FAILED\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestImportUsersTeamsExample_OutputFlag(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "custom-template.csv")

	cmd := users.NewImportUsersTeamsExampleCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--output", out})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected file at %s, got: %v", out, err)
	}
}

func TestImportUsersTeamsExample_NoArgs(t *testing.T) {
	t.Parallel()
	cmd := users.NewImportUsersTeamsExampleCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"unexpected"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute should fail with positional args (cobra.NoArgs)")
	}
}

func TestImportUsersTeamsExample_GroupedUnderUsers(t *testing.T) {
	t.Parallel()
	cmd := users.NewImportUsersTeamsExampleCmd()
	if cmd.GroupID != "users" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "users")
	}
}
