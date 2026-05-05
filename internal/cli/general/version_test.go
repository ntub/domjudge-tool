package general_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli/general"
)

func TestVersion_PrintsVersionLine(t *testing.T) {
	t.Parallel()
	cmd := general.NewVersionCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.HasPrefix(got, "domjudge-tool ") {
		t.Errorf("output should start with 'domjudge-tool ', got %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("output should end with newline, got %q", got)
	}
}

func TestVersion_NoArgsAccepted(t *testing.T) {
	t.Parallel()
	cmd := general.NewVersionCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"unexpected-positional"})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute with positional should error (cobra.NoArgs)")
	}
}

func TestVersion_GroupedUnderGeneral(t *testing.T) {
	t.Parallel()
	cmd := general.NewVersionCmd()
	if cmd.GroupID != "general" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "general")
	}
}

func TestVersion_HasNoPreRunE(t *testing.T) {
	t.Parallel()
	cmd := general.NewVersionCmd()
	// version must not attach LoadConfigPreRunE — running it without
	// any config on disk should still succeed. Locks the "skip
	// config-load" contract spelled out in
	// docs/phase1/README.md §6.
	if cmd.PreRunE != nil {
		t.Errorf("version should have no PreRunE")
	}
	if cmd.PersistentPreRunE != nil {
		t.Errorf("version should have no PersistentPreRunE")
	}
}
