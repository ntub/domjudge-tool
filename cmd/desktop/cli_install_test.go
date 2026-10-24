//go:build desktop

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func isolateCLIPaths(t *testing.T) (root, home, config, local string) {
	t.Helper()
	root = t.TempDir()
	home = filepath.Join(root, "isolated home")
	config = filepath.Join(root, "isolated config")
	local = filepath.Join(root, "local app data")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("LOCALAPPDATA", local)
	// Windows integration must never use the roaming configuration directory.
	t.Setenv("APPDATA", filepath.Join(root, "roaming app data"))
	return root, home, config, local
}

func assertCLICompositionReadOnly(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("native composition or inspection wrote user directories: %v", entries)
	}
}

func TestDesktopCLIInstallConfigUsesIsolatedNativePaths_Integration(t *testing.T) {
	root, home, config, local := isolateCLIPaths(t)
	got, err := desktopCLIInstallConfig()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stateRoot := config
	switch runtime.GOOS {
	case "darwin":
		stateRoot = filepath.Join(home, "Library", "Application Support")
	case "windows":
		stateRoot = local
	}
	if got.ExecutablePath != executable || got.HomeDir != home || got.StateDir != filepath.Join(stateRoot, "domjudge-tool", "cli-install") {
		t.Fatalf("incorrect native configuration: %+v", got)
	}
	opts := desktopCLIInstallOptions("127.0.0.1:9900", "v1.2.3")
	if opts.Installer == nil || opts.UnavailableReason != "" || opts.Host != "127.0.0.1:9900" || opts.AppVersion != "v1.2.3" {
		t.Fatalf("incorrect native composition: %+v", opts)
	}
	status, err := opts.Installer.Inspect(context.Background())
	if err != nil || status.State != "unavailable" || status.Reason == "" || status.CanInstall || status.CanRelink || status.CanUninstall {
		t.Fatalf("raw development executable is installable: %+v, %v", status, err)
	}
	assertCLICompositionReadOnly(t, root)
}

func TestDesktopCLIInstallOptionsFailClosed_Integration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T)
	}{
		{
			name: "missing home",
			change: func(t *testing.T) {
				t.Setenv("HOME", "")
				t.Setenv("USERPROFILE", "")
			},
		},
		{
			name: "relative home rejected by manager",
			change: func(t *testing.T) {
				t.Setenv("HOME", "relative-home")
				t.Setenv("USERPROFILE", "relative-home")
			},
		},
		{
			name: "invalid state directory",
			change: func(t *testing.T) {
				switch runtime.GOOS {
				case "darwin":
					t.Setenv("HOME", "relative-home")
				case "windows":
					t.Setenv("LOCALAPPDATA", "")
				default:
					t.Setenv("XDG_CONFIG_HOME", "relative-config")
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _, _, _ := isolateCLIPaths(t)
			tc.change(t)
			opts := desktopCLIInstallOptions("127.0.0.1:9900", "dev")
			if opts.Installer != nil || !strings.HasPrefix(opts.UnavailableReason, "CLI integration is unavailable: ") {
				t.Fatalf("path resolution failure did not disable integration visibly: %+v", opts)
			}
			assertCLICompositionReadOnly(t, root)
		})
	}
}
