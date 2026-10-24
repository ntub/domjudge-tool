package cliinstall

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLockProcessHelper(t *testing.T) {
	path := os.Getenv("DOMJUDGE_TOOL_LOCK_TEST_PATH")
	if path == "" {
		return
	}
	lock, err := acquireLock(context.Background(), path)
	if err != nil {
		os.Exit(90)
	}
	if _, err := io.WriteString(os.Stdout, "locked\n"); err != nil {
		os.Exit(91)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err := lock.Close(); err != nil {
		os.Exit(92)
	}
	os.Exit(0)
}

func TestCrossProcessLockAndCrashRelease_Integration(t *testing.T) {
	m, cfg := testManager(t)
	s := installTest(t, m)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestLockProcessHelper$")
	cmd.Env = append(os.Environ(), "DOMJUDGE_TOOL_LOCK_TEST_PATH="+filepath.Join(cfg.StateDir, ".lock"))
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("lock helper readiness: %q, %v", line, err)
	}
	if _, err := m.Relink(context.Background(), s.Revision); !errors.Is(err, ErrBusy) {
		t.Fatalf("cross-process mutation error = %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("killed helper unexpectedly exited successfully")
	}
	if _, err := m.Relink(context.Background(), s.Revision); err != nil {
		t.Fatalf("lock retained after process exit: %v", err)
	}
}

func TestNoReplacePublication_Integration(t *testing.T) {
	for _, kind := range []string{"file", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			source, destination := filepath.Join(root, "source"), filepath.Join(root, "command")
			writeTestFile(t, source, []byte("ours"), 0o600)
			if kind == "file" {
				writeTestFile(t, destination, []byte("foreign"), 0o600)
			} else if err := os.Mkdir(destination, 0o700); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(destination)
			if err != nil {
				t.Fatal(err)
			}
			if err := publishNoReplace(source, destination); err == nil {
				t.Fatal("publication overwrote destination")
			}
			after, err := os.Lstat(destination)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("destination changed: %v", err)
			}
			if kind == "file" {
				data, err := os.ReadFile(destination)
				if err != nil || string(data) != "foreign" {
					t.Fatalf("foreign data changed: %q, %v", data, err)
				}
			}
		})
	}
}

func TestInterruptedRelinkReceipt_Integration(t *testing.T) {
	for _, phase := range []string{"before target replacement", "after target replacement"} {
		t.Run(phase, func(t *testing.T) {
			m, cfg := testManager(t)
			installTest(t, m)
			otherPath := testBundle(t, filepath.Join(filepath.Dir(cfg.HomeDir), "upgraded.app"))
			other, err := New(Config{otherPath, cfg.HomeDir, cfg.StateDir})
			if err != nil {
				t.Fatal(err)
			}
			i, err := other.inspect(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			temporary, target, err := other.stageTarget(context.Background(), i.status.CLIPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = removeTemporary(temporary) })
			r := *i.receipt
			r.Pending = &target
			if err := other.saveReceipt(context.Background(), r, true); err != nil {
				t.Fatal(err)
			}
			want := "other"
			if phase == "after target replacement" {
				if err := replacePrivate(temporary, other.targetPath); err != nil {
					t.Fatal(err)
				}
				want = "current"
			}
			s := inspectTest(t, other, want)
			if _, err := other.Relink(context.Background(), s.Revision); err != nil {
				t.Fatalf("cannot finish interrupted relink: %v", err)
			}
			inspectTest(t, other, "current")
		})
	}
}

func TestBundleMoveAndUpgrade_Integration(t *testing.T) {
	m, cfg := testManager(t)
	s := installTest(t, m)
	resolvedExecutable, err := filepath.EvalSymlinks(cfg.ExecutablePath)
	if err != nil {
		t.Fatal(err)
	}
	public, err := os.Lstat(s.CommandPath)
	if err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(filepath.Dir(filepath.Dir(s.CLIPath)), "bundle.json")
	data, err := json.Marshal(bundleManifest{1, "v2.0.0", "upgrade", runtime.GOOS, runtime.GOARCH})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, metadata, data, 0o600)
	upgraded := inspectTest(t, m, "current")
	if upgraded.Version != "v2.0.0" || upgraded.Revision == s.Revision {
		t.Fatalf("upgrade not observed: %+v", upgraded)
	}
	if _, err := m.Uninstall(context.Background(), s.Revision); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("upgrade accepted stale form: %v", err)
	}
	newRoot := filepath.Join(filepath.Dir(s.BundlePath), "Moved 日本語.app")
	if err := os.Rename(s.BundlePath, newRoot); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(s.BundlePath, resolvedExecutable)
	if err != nil {
		t.Fatal(err)
	}
	moved, err := New(Config{filepath.Join(newRoot, relative), cfg.HomeDir, cfg.StateDir})
	if err != nil {
		t.Fatal(err)
	}
	broken := inspectTest(t, moved, "broken")
	if _, err := moved.Relink(context.Background(), broken.Revision); err != nil {
		t.Fatal(err)
	}
	got := inspectTest(t, moved, "current")
	after, err := os.Lstat(got.CommandPath)
	if err != nil || !os.SameFile(public, after) {
		t.Fatalf("move/relink changed public identity: %v", err)
	}
}

func TestForgedReceiptPaths_Integration(t *testing.T) {
	m, cfg := testManager(t)
	s := installTest(t, m)
	i, err := m.inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r := *i.receipt
	r.CommandPath = filepath.Join(cfg.HomeDir, "foreign")
	if err := m.saveReceipt(context.Background(), r, true); err != nil {
		t.Fatal(err)
	}
	got, err := m.Inspect(context.Background())
	if err == nil || got.State != "error" || got.CanUninstall {
		t.Fatalf("forged receipt accepted: %+v, %v", got, err)
	}
	if _, err := os.Lstat(s.CommandPath); err != nil {
		t.Fatal(err)
	}
}

func TestUninstallLeavesUnrelatedState_Integration(t *testing.T) {
	m, cfg := testManager(t)
	s := installTest(t, m)
	unrelated := filepath.Join(cfg.StateDir, "notes.json")
	writeTestFile(t, unrelated, []byte("user owned"), 0o600)
	if _, err := m.Uninstall(context.Background(), s.Revision); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(unrelated)
	if err != nil || string(data) != "user owned" {
		t.Fatalf("unrelated state changed: %q, %v", data, err)
	}
	installTest(t, m)
}
