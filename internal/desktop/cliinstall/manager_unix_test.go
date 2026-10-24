//go:build darwin || linux

package cliinstall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func TestUnixLinkOwnership_Integration(t *testing.T) {
	for _, kind := range []string{"foreign symlink", "dangling symlink", "same destination replacement", "state symlink", "lock symlink", "target fifo"} {
		t.Run(kind, func(t *testing.T) {
			m, cfg := testManager(t)
			s := inspectTest(t, m, "absent")
			foreign := filepath.Join(filepath.Dir(cfg.HomeDir), "foreign")
			writeTestFile(t, foreign, []byte("foreign"), 0o600)
			want := "conflict"
			switch kind {
			case "foreign symlink", "dangling symlink":
				if err := os.MkdirAll(filepath.Dir(s.CommandPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if kind == "dangling symlink" {
					foreign += "-missing"
				}
				if err := os.Symlink(foreign, s.CommandPath); err != nil {
					t.Fatal(err)
				}
			case "same destination replacement":
				s = installTest(t, m)
				// Keep the old inode alive so the filesystem cannot recycle its ID.
				if err := os.Rename(s.CommandPath, s.CommandPath+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(m.targetPath, s.CommandPath); err != nil {
					t.Fatal(err)
				}
			case "state symlink":
				if err := os.Symlink(filepath.Dir(foreign), cfg.StateDir); err != nil {
					t.Fatal(err)
				}
			case "lock symlink":
				if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(foreign, filepath.Join(cfg.StateDir, ".lock")); err != nil {
					t.Fatal(err)
				}
				inspectTest(t, m, "conflict")
				if lock, err := acquireLock(context.Background(), filepath.Join(cfg.StateDir, ".lock")); err == nil {
					_ = lock.Close()
					t.Fatal("lock acquisition followed a symlink")
				}
				if _, err := m.Install(context.Background(), s.Revision); err == nil {
					t.Fatal("symlink lock accepted")
				}
				data, err := os.ReadFile(foreign)
				if err != nil || string(data) != "foreign" {
					t.Fatalf("lock target changed: %q, %v", data, err)
				}
				return
			case "target fifo":
				if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := syscall.Mkfifo(m.targetPath, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got := inspectTest(t, m, want)
			if got.CanInstall || got.CanRelink || got.CanUninstall {
				t.Fatalf("foreign object permits mutation: %+v", got)
			}
		})
	}
}

func TestPermissionFailuresRemainVisible_Integration(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses Unix permission bits")
	}
	for _, name := range []string{"unreadable receipt", "inaccessible state", "read-only public directory", "read-only private directory", "non-executable CLI"} {
		t.Run(name, func(t *testing.T) {
			m, cfg := testManager(t)
			s := inspectTest(t, m, "absent")
			path, mode := cfg.StateDir, os.FileMode(0)
			switch name {
			case "unreadable receipt":
				s = installTest(t, m)
				path = m.receiptPath
			case "inaccessible state":
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "read-only public directory":
				path, mode = filepath.Dir(s.CommandPath), 0o500
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "read-only private directory":
				mode = 0o500
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "non-executable CLI":
				path, mode = s.CLIPath, 0o600
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o700) })
			if name == "read-only public directory" || name == "read-only private directory" {
				got, err := m.Install(context.Background(), s.Revision)
				if err == nil || !errors.Is(err, os.ErrPermission) || got.State != "error" {
					t.Fatalf("permission failure hidden: %+v, %v", got, err)
				}
				return
			}
			got, err := m.Inspect(context.Background())
			if name == "non-executable CLI" {
				if err != nil || got.State != "unavailable" {
					t.Fatalf("nonexecutable CLI: %+v, %v", got, err)
				}
			} else if err == nil || !errors.Is(err, os.ErrPermission) || got.State != "error" {
				t.Fatalf("permission failure hidden: %+v, %v", got, err)
			}
		})
	}
}

func TestInaccessibleOldSourceRecovery_Integration(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses Unix permission bits")
	}
	for _, action := range []string{"relink", "uninstall"} {
		t.Run(action, func(t *testing.T) {
			oldManager, cfg := testManager(t)
			old := installTest(t, oldManager)
			cfg.ExecutablePath = testBundle(t, filepath.Join(filepath.Dir(cfg.HomeDir), "Other.app"))
			m, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(old.BundlePath, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(old.BundlePath, 0o700) })
			_, sourceErr := os.Lstat(old.CLIPath)
			if !errors.Is(sourceErr, os.ErrPermission) {
				t.Fatalf("old source must be inaccessible: %v", sourceErr)
			}
			s := inspectTest(t, m, "broken")
			if !s.CanRelink || !s.CanUninstall || !strings.Contains(s.Reason, sourceErr.Error()) {
				t.Fatalf("owned integration cannot recover or hides the source error: %+v", s)
			}
			var got Status
			if action == "relink" {
				got, err = m.Relink(context.Background(), s.Revision)
				if err != nil || got.State != "current" || got.TargetPath != s.CLIPath {
					t.Fatalf("relink inaccessible source = %+v, %v", got, err)
				}
			} else {
				got, err = m.Uninstall(context.Background(), s.Revision)
				if err != nil || got.State != "absent" {
					t.Fatalf("uninstall inaccessible source = %+v, %v", got, err)
				}
				if _, err := os.Lstat(s.CommandPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("uninstall retained command: %v", err)
				}
			}
			if err := os.Chmod(old.BundlePath, 0o700); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(old.CLIPath)
			if err != nil || string(data) != "bundled CLI" {
				t.Fatalf("recovery modified the old source: %q, %v", data, err)
			}
		})
	}
}

func TestHostInstalledCommandLifecycle_Integration(t *testing.T) {
	m, cfg := testManager(t)
	before := inspectTest(t, m, "absent")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	mustRemove(t, before.CLIPath)
	copyTestExecutable(t, executable, before.CLIPath)
	s := installTest(t, m)
	publicTarget, err := os.Readlink(s.CommandPath)
	if err != nil || publicTarget != filepath.Join(cfg.StateDir, "target") {
		t.Fatalf("public indirection = %q, %v", publicTarget, err)
	}
	privateTarget, err := os.Readlink(publicTarget)
	if err != nil || privateTarget != s.CLIPath {
		t.Fatalf("private indirection = %q, %v", privateTarget, err)
	}
	t.Setenv("PATH", filepath.Dir(s.CommandPath))
	t.Setenv("DOMJUDGE_TOOL_LAUNCHER_TEST_CHILD", "1")
	t.Setenv("DOMJUDGE_TOOL_LAUNCHER_TEST_ENV", "host lifecycle")
	path, err := exec.LookPath("domjudge-tool")
	if err != nil || path != s.CommandPath {
		t.Fatalf("isolated PATH lookup = %q, %v", path, err)
	}
	args := []string{"", "日本語", "space value", `quote"value`}
	cmd := exec.Command(path, append([]string{"-test.run=^TestLauncherChildHelper$", "--"}, args...)...)
	cmd.Stdin = strings.NewReader("host stdin")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 37 {
		t.Fatalf("installed subprocess: %v, %s", err, &stderr)
	}
	var report childReport
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.Args, args) || report.Input != "host stdin" || report.Environment != "host lifecycle" {
		t.Fatalf("installed command changed process boundary: %+v", report)
	}
	if _, err := m.Uninstall(context.Background(), s.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("domjudge-tool"); err == nil {
		t.Fatal("removed command still resolves through isolated PATH")
	}
	if _, err := os.Stat(s.CLIPath); err != nil {
		t.Fatalf("host lifecycle removed bundled executable: %v", err)
	}
}
