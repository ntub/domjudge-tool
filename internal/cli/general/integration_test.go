package general_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/cli/general"
)

// TestGenConfig_UnderRoot_PersistentFlagInherited verifies that when
// gen-config is wired under the real root, the inherited
// --disable-ssl persistent flag does not collide with gen-config's
// local --disable-ssl flag.
func TestGenConfig_UnderRoot_PersistentFlagInherited(t *testing.T) {
	t.Parallel()
	root := cli.NewRootCmd(cli.Deps{})
	root.AddCommand(general.NewGenConfigCmd())
	root.AddCommand(general.NewVersionCmd())

	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "domserver.json")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"gen-config",
		"--host", "https://example.com",
		"--username", "user",
		"--password", "pass",
		"--output", out,
	})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute under root: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("config not written: %v", err)
	}
}
