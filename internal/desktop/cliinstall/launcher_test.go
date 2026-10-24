package cliinstall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func launcherFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	launcher := filepath.Join(root, "state 日本語", "bin", "domjudge-tool.exe")
	target := filepath.Join(root, "App with spaces", "cli", "domjudge-tool.exe")
	writeTestFile(t, launcher, []byte("launcher identity"), 0o755)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyTestExecutable(t, executable, target)
	writeLauncherTarget(t, launcher, target)
	return launcher, target
}

func copyTestExecutable(t *testing.T, source, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := copyNewFile(context.Background(), source, target); err != nil {
		t.Fatal(err)
	}
}

func writeLauncherTarget(t *testing.T, launcher, target string) {
	t.Helper()
	data, err := json.Marshal(map[string]string{"path": target})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(filepath.Dir(filepath.Dir(launcher)), "target.json"), data, 0o600)
}

type childReport struct {
	Args                          []string
	Input, Directory, Environment string
}

func TestLauncherChildHelper(t *testing.T) {
	if os.Getenv("DOMJUDGE_TOOL_LAUNCHER_TEST_CHILD") != "1" {
		return
	}
	if os.Getenv("DOMJUDGE_TOOL_LAUNCHER_TEST_WAIT") == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	var args []string
	for index, arg := range os.Args {
		if arg == "--" {
			args = os.Args[index+1:]
			break
		}
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(90)
	}
	dir, err := os.Getwd()
	if err != nil {
		os.Exit(91)
	}
	if err := json.NewEncoder(os.Stdout).Encode(childReport{args, string(input), dir, os.Getenv("DOMJUDGE_TOOL_LAUNCHER_TEST_ENV")}); err != nil {
		os.Exit(92)
	}
	fmt.Fprint(os.Stderr, "child stderr\n")
	os.Exit(37)
}

func TestLauncherPreservesProcessBoundary_Integration(t *testing.T) {
	launcher, _ := launcherFixture(t)
	t.Setenv("DOMJUDGE_TOOL_LAUNCHER_TEST_CHILD", "1")
	t.Setenv("DOMJUDGE_TOOL_LAUNCHER_TEST_ENV", "environment 日本語 value")
	arguments := []string{"", "plain", "two words", "日本語", `a"b`, `trailing\`, `back\\\"quote`, "&|<>^%PATH%", "line\nbreak"}
	args := append([]string{"-test.run=^TestLauncherChildHelper$", "--"}, arguments...)
	var stdout, stderr bytes.Buffer
	code, err := RunLauncher(context.Background(), launcher, args, strings.NewReader("stdin 日本語\n"), &stdout, &stderr)
	if err != nil || code != 37 {
		t.Fatalf("RunLauncher = %d, %v; stderr=%s", code, err, &stderr)
	}
	var report childReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("child stdout: %s: %v", &stdout, err)
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.Args, arguments) || report.Input != "stdin 日本語\n" || report.Directory != dir || report.Environment != "environment 日本語 value" {
		t.Fatalf("process boundary changed: %+v", report)
	}
	if stderr.String() != "child stderr\n" {
		t.Fatalf("stderr = %q", &stderr)
	}
}

func TestLauncherRejectsInvalidTargets_Integration(t *testing.T) {
	for _, name := range []string{"missing document", "corrupt document", "extra field", "duplicate field", "relative path", "missing executable", "directory", "self", "copy of launcher"} {
		t.Run(name, func(t *testing.T) {
			launcher, target := launcherFixture(t)
			document := filepath.Join(filepath.Dir(filepath.Dir(launcher)), "target.json")
			switch name {
			case "missing document":
				mustRemove(t, document)
			case "corrupt document":
				writeTestFile(t, document, []byte("{"), 0o600)
			case "extra field":
				writeTestFile(t, document, []byte(`{"path":"unused","other":true}`), 0o600)
			case "duplicate field":
				encoded, err := json.Marshal(target)
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, document, []byte(fmt.Sprintf(`{"path":%s,"path":%s}`, encoded, encoded)), 0o600)
			case "relative path":
				writeLauncherTarget(t, launcher, "domjudge-tool")
			case "missing executable":
				mustRemove(t, target)
			case "directory":
				mustRemove(t, target)
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
			case "self":
				writeLauncherTarget(t, launcher, launcher)
			case "copy of launcher":
				writeTestFile(t, target, []byte("launcher identity"), 0o755)
			}
			var output bytes.Buffer
			code, err := RunLauncher(context.Background(), launcher, nil, strings.NewReader(""), &output, &output)
			if err == nil || code == 0 || output.Len() != 0 {
				t.Fatalf("invalid target ran: code=%d error=%v output=%q", code, err, &output)
			}
		})
	}
}

func TestLauncherCancellation_Integration(t *testing.T) {
	launcher, _ := launcherFixture(t)
	t.Setenv("DOMJUDGE_TOOL_LAUNCHER_TEST_CHILD", "1")
	t.Setenv("DOMJUDGE_TOOL_LAUNCHER_TEST_WAIT", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	code, err := RunLauncher(ctx, launcher, []string{"-test.run=^TestLauncherChildHelper$"}, nil, io.Discard, io.Discard)
	if !errors.Is(err, context.DeadlineExceeded) || code == 0 {
		t.Fatalf("canceled launcher = %d, %v", code, err)
	}
}

func TestLauncherRejectsAliases_Integration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink fixture; Windows standard-user hardlinks covered separately")
	}
	for _, name := range []string{"symlink", "hardlink"} {
		t.Run(name, func(t *testing.T) {
			launcher, target := launcherFixture(t)
			mustRemove(t, target)
			var err error
			if name == "symlink" {
				err = os.Symlink(launcher, target)
			} else {
				err = os.Link(launcher, target)
			}
			if err != nil {
				t.Fatal(err)
			}
			code, err := RunLauncher(context.Background(), launcher, nil, nil, io.Discard, io.Discard)
			if err == nil || code == 0 {
				t.Fatalf("launcher alias accepted: %d, %v", code, err)
			}
		})
	}
}
