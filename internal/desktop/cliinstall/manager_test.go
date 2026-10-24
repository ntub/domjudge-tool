package cliinstall

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func testBundle(t *testing.T, root string) string {
	t.Helper()
	base := root
	executable := filepath.Join(root, "domjudge-tool-app")
	name := "domjudge-tool"
	if runtime.GOOS == "darwin" {
		executable = filepath.Join(root, "Contents", "MacOS", "domjudge-tool-app")
		base = filepath.Join(root, "Contents", "Resources")
	}
	if runtime.GOOS == "windows" {
		executable += ".exe"
		name += ".exe"
		writeTestFile(t, filepath.Join(root, "resources", "cli-launcher.exe"), []byte("native console launcher"), 0o755)
	}
	writeTestFile(t, executable, []byte("desktop"), 0o755)
	writeTestFile(t, filepath.Join(base, "cli", name), []byte("bundled CLI"), 0o755)
	metadata, err := json.Marshal(map[string]any{
		"schema": 1, "version": "v1.2.3", "revision": "test-revision", "goos": runtime.GOOS, "goarch": runtime.GOARCH,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(base, "bundle.json"), metadata, 0o600)
	return executable
}

func writeTestFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func testManager(t *testing.T) (*Manager, Config) {
	t.Helper()
	root := t.TempDir()
	cfg := Config{
		ExecutablePath: testBundle(t, filepath.Join(root, "App space 日本語.app")),
		HomeDir:        filepath.Join(root, "home"),
		StateDir:       filepath.Join(root, "state"),
	}
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return m, cfg
}

func inspectTest(t *testing.T, m *Manager, want string) Status {
	t.Helper()
	s, err := m.Inspect(context.Background())
	if err != nil {
		t.Fatalf("Inspect: %v (%+v)", err, s)
	}
	if s.State != want || s.Revision == "" {
		t.Fatalf("Inspect = %+v, want state %q and a revision", s, want)
	}
	return s
}

func installTest(t *testing.T, m *Manager) Status {
	t.Helper()
	s := inspectTest(t, m, "absent")
	got, err := m.Install(context.Background(), s.Revision)
	if err != nil || got.State != "current" {
		t.Fatalf("Install = %+v, %v", got, err)
	}
	return got
}

func TestNewRejectsImplicitPaths(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"empty", Config{}},
		{"relative executable", Config{"relative", root, filepath.Join(root, "state")}},
		{"relative home", Config{filepath.Join(root, "app"), "home", filepath.Join(root, "state")}},
		{"relative state", Config{filepath.Join(root, "app"), root, "state"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.cfg); err == nil {
				t.Fatal("New accepted implicit or relative paths")
			}
		})
	}
}

func TestInspectReadOnly_Integration(t *testing.T) {
	m, cfg := testManager(t)
	s := inspectTest(t, m, "absent")
	if !s.CanInstall || s.CanRelink || s.CanUninstall || s.Version != "v1.2.3" || s.PathNote == "" {
		t.Fatalf("incorrect initial status: %+v", s)
	}
	for _, path := range []string{cfg.HomeDir, cfg.StateDir} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Inspect created %s: %v", path, err)
		}
	}
	if again := inspectTest(t, m, "absent"); again.Revision != s.Revision {
		t.Fatal("read-only inspection changed its revision")
	}
}

func TestUnavailableBundles_Integration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *Manager, Config)
	}{
		{"missing executable", func(t *testing.T, _ *Manager, cfg Config) { mustRemove(t, cfg.ExecutablePath) }},
		{"missing metadata", func(t *testing.T, m *Manager, _ Config) {
			s := inspectTest(t, m, "absent")
			mustRemove(t, filepath.Join(filepath.Dir(filepath.Dir(s.CLIPath)), "bundle.json"))
		}},
		{"invalid metadata", func(t *testing.T, m *Manager, _ Config) {
			s := inspectTest(t, m, "absent")
			writeTestFile(t, filepath.Join(filepath.Dir(filepath.Dir(s.CLIPath)), "bundle.json"), []byte(`{"schema":99}`), 0o600)
		}},
		{"missing CLI", func(t *testing.T, m *Manager, _ Config) { mustRemove(t, inspectTest(t, m, "absent").CLIPath) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, cfg := testManager(t)
			tc.change(t, m, cfg)
			s := inspectTest(t, m, "unavailable")
			if s.CanInstall || s.CanRelink || s.Reason == "" {
				t.Fatalf("unavailable bundle allowed mutation: %+v", s)
			}
			if _, err := m.Install(context.Background(), s.Revision); err == nil {
				t.Fatal("installed unavailable bundle")
			}
			if _, err := os.Lstat(cfg.StateDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rejected installation created state: %v", err)
			}
		})
	}
}

func TestLifecycleAndStaleRevision_Integration(t *testing.T) {
	m, cfg := testManager(t)
	before := inspectTest(t, m, "absent")
	installed := installTest(t, m)
	if installed.TargetPath != installed.CLIPath || !installed.CanUninstall || installed.CanInstall {
		t.Fatalf("incorrect installed status: %+v", installed)
	}
	for _, operation := range []func(context.Context, string) (Status, error){m.Install, m.Relink, m.Uninstall} {
		if _, err := operation(context.Background(), before.Revision); !errors.Is(err, ErrStaleRevision) {
			t.Fatalf("stale mutation error = %v", err)
		}
	}
	otherExecutable := testBundle(t, filepath.Join(filepath.Dir(cfg.HomeDir), "Other.app"))
	other, err := New(Config{otherExecutable, cfg.HomeDir, cfg.StateDir})
	if err != nil {
		t.Fatal(err)
	}
	otherStatus := inspectTest(t, other, "other")
	if !otherStatus.CanRelink || otherStatus.TargetPath != installed.CLIPath {
		t.Fatalf("other installation = %+v", otherStatus)
	}
	publicBefore, err := os.Lstat(installed.CommandPath)
	if err != nil {
		t.Fatal(err)
	}
	relinked, err := other.Relink(context.Background(), otherStatus.Revision)
	if err != nil || relinked.State != "current" || relinked.TargetPath != otherStatus.CLIPath {
		t.Fatalf("Relink = %+v, %v", relinked, err)
	}
	publicAfter, err := os.Lstat(installed.CommandPath)
	if err != nil || !os.SameFile(publicBefore, publicAfter) {
		t.Fatalf("Relink replaced public command: %v", err)
	}
	uninstalled, err := other.Uninstall(context.Background(), relinked.Revision)
	if err != nil || uninstalled.State != "absent" {
		t.Fatalf("Uninstall = %+v, %v", uninstalled, err)
	}
	for _, path := range []string{installed.CommandPath, filepath.Join(cfg.StateDir, "receipt.json")} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("owned object retained: %s, %v", path, err)
		}
	}
	if _, err := os.Stat(installed.CLIPath); err != nil {
		t.Fatalf("Uninstall removed App CLI: %v", err)
	}
}

func TestForeignCommandNeverOverwritten_Integration(t *testing.T) {
	for _, kind := range []string{"file", "directory"} {
		t.Run(kind, func(t *testing.T) {
			m, cfg := testManager(t)
			before := inspectTest(t, m, "absent")
			if kind == "file" {
				writeTestFile(t, before.CommandPath, []byte("foreign"), 0o755)
			} else if err := os.MkdirAll(before.CommandPath, 0o700); err != nil {
				t.Fatal(err)
			}
			s := inspectTest(t, m, "conflict")
			if s.CanInstall || s.CanRelink || s.CanUninstall {
				t.Fatalf("foreign command offered mutation: %+v", s)
			}
			for _, operation := range []func(context.Context, string) (Status, error){m.Install, m.Relink, m.Uninstall} {
				if _, err := operation(context.Background(), s.Revision); err == nil {
					t.Fatal("foreign command accepted mutation")
				}
			}
			if kind == "file" {
				got, err := os.ReadFile(before.CommandPath)
				if err != nil || string(got) != "foreign" {
					t.Fatalf("foreign bytes changed: %q, %v", got, err)
				}
			}
			if runtime.GOOS != "windows" {
				if _, err := os.Lstat(cfg.StateDir); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("conflicting operation created state: %v", err)
				}
			}
		})
	}
}

func TestReceiptAloneDoesNotAuthorize_Integration(t *testing.T) {
	for _, kind := range []string{"modified command", "missing receipt", "corrupt receipt", "modified target"} {
		t.Run(kind, func(t *testing.T) {
			m, cfg := testManager(t)
			s := installTest(t, m)
			want := "conflict"
			switch kind {
			case "modified command":
				mustRemove(t, s.CommandPath)
				writeTestFile(t, s.CommandPath, []byte("foreign replacement"), 0o755)
			case "missing receipt":
				mustRemove(t, filepath.Join(cfg.StateDir, "receipt.json"))
			case "corrupt receipt":
				writeTestFile(t, filepath.Join(cfg.StateDir, "receipt.json"), []byte("invalid"), 0o600)
				want = "error"
			case "modified target":
				name := "target"
				if runtime.GOOS == "windows" {
					name = "target.json"
				}
				mustRemove(t, filepath.Join(cfg.StateDir, name))
				writeTestFile(t, filepath.Join(cfg.StateDir, name), []byte("foreign target"), 0o600)
			}
			got, err := m.Inspect(context.Background())
			if got.State != want || got.CanUninstall || got.CanRelink || got.CanInstall || (want == "error" && err == nil) {
				t.Fatalf("Inspect = %+v, %v", got, err)
			}
			if _, err := m.Uninstall(context.Background(), got.Revision); err == nil {
				t.Fatal("receipt authorized destructive action on changed objects")
			}
		})
	}
}

func TestMissingObjectsRepairAndRemoval_Integration(t *testing.T) {
	for _, object := range []string{"command", "target", "CLI", "bundle"} {
		t.Run(object, func(t *testing.T) {
			m, cfg := testManager(t)
			s := installTest(t, m)
			switch object {
			case "command":
				mustRemove(t, s.CommandPath)
			case "target":
				name := "target"
				if runtime.GOOS == "windows" {
					name = "target.json"
				}
				mustRemove(t, filepath.Join(cfg.StateDir, name))
			case "CLI":
				mustRemove(t, s.CLIPath)
			case "bundle":
				mustRemove(t, cfg.ExecutablePath)
			}
			got, err := m.Inspect(context.Background())
			if err != nil || !got.CanUninstall || (got.State != "broken" && got.State != "unavailable") {
				t.Fatalf("missing %s: %+v, %v", object, got, err)
			}
			if object == "command" || object == "target" {
				if !got.CanRelink {
					t.Fatalf("missing integration cannot be repaired: %+v", got)
				}
				got, err = m.Relink(context.Background(), got.Revision)
				if err != nil || got.State != "current" {
					t.Fatalf("repair: %+v, %v", got, err)
				}
			}
			if _, err := m.Uninstall(context.Background(), got.Revision); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConcurrentInstallers_Integration(t *testing.T) {
	m, cfg := testManager(t)
	revision := inspectTest(t, m, "absent").Revision
	var wg sync.WaitGroup
	errorsCh := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			other, err := New(cfg)
			if err == nil {
				_, err = other.Install(context.Background(), revision)
			}
			errorsCh <- err
		})
	}
	wg.Wait()
	close(errorsCh)
	successes := 0
	for err := range errorsCh {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, ErrStaleRevision) && !errors.Is(err, ErrBusy) {
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful installers = %d", successes)
	}
	inspectTest(t, m, "current")
}

func TestCanceledOperationsDoNotMutate_Integration(t *testing.T) {
	m, cfg := testManager(t)
	s := inspectTest(t, m, "absent")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Install(ctx, s.Revision); !errors.Is(err, context.Canceled) {
		t.Fatalf("Install error = %v", err)
	}
	if _, err := os.Lstat(cfg.StateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled operation mutated state: %v", err)
	}
}

func TestTranslocationRefused_Integration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS App Translocation")
	}
	root := t.TempDir()
	cfg := Config{testBundle(t, filepath.Join(root, "AppTranslocation", "token", "d", "App.app")), filepath.Join(root, "home"), filepath.Join(root, "state")}
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := inspectTest(t, m, "unavailable")
	if !strings.Contains(strings.ToLower(s.Reason), "translocation") {
		t.Fatalf("reason = %q", s.Reason)
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}
