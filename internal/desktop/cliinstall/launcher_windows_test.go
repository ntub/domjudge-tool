package cliinstall

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// All native Windows subprocess tests use this test executable as a helper.
// A precompiled .test.exe is sufficient in an offline standard-user VM.
func TestWindowsConsoleHelper(t *testing.T) {
	switch os.Getenv("DOMJUDGE_TOOL_CONSOLE_HELPER") {
	case "launcher":
		if err := os.Setenv("DOMJUDGE_TOOL_CONSOLE_HELPER", "child"); err != nil {
			os.Exit(90)
		}
		path, err := os.Executable()
		if err != nil {
			os.Exit(91)
		}
		code, err := RunLauncher(context.Background(), path, []string{"-test.run=^TestWindowsConsoleHelper$"}, os.Stdin, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(code)
	case "child":
		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)
		fmt.Fprintln(os.Stdout, "ready")
		select {
		case <-interrupts:
			fmt.Fprintln(os.Stdout, "cancelled")
			os.Exit(130)
		case <-time.After(10 * time.Second):
			os.Exit(92)
		}
	case "sender":
		pid, err := strconv.Atoi(os.Getenv("DOMJUDGE_TOOL_CONSOLE_PID"))
		if err != nil {
			os.Exit(93)
		}
		kernel := windows.NewLazySystemDLL("kernel32.dll")
		// A sender may have no console. Detaching is intentionally best-effort.
		_, _, _ = kernel.NewProc("FreeConsole").Call()
		if result, _, err := kernel.NewProc("AttachConsole").Call(uintptr(pid)); result == 0 {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(94)
		}
		// AttachConsole resets handlers; protect only this sender, never inherit
		// an ignore-Ctrl+C attribute into the launcher or its child.
		handler := windows.NewCallback(func(uint32) uintptr { return 1 })
		if result, _, err := kernel.NewProc("SetConsoleCtrlHandler").Call(handler, 1); result == 0 {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(95)
		}
		event := uint32(windows.CTRL_C_EVENT)
		if os.Getenv("DOMJUDGE_TOOL_CONSOLE_EVENT") == "break" {
			event = windows.CTRL_BREAK_EVENT
		}
		if err := windows.GenerateConsoleCtrlEvent(event, 0); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(96)
		}
		os.Exit(0)
	}
}

func TestWindowsConsoleCancellation_Integration(t *testing.T) {
	for _, event := range []string{"ctrl-c", "break"} {
		t.Run(event, func(t *testing.T) {
			launcher, target := launcherFixture(t)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			mustRemove(t, launcher)
			copyTestExecutable(t, executable, launcher)
			// PE overlays are valid and distinguish the helper CLI image from the
			// helper launcher image without needing a Go compiler in the guest.
			file, err := os.OpenFile(target, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := file.WriteString("CLI helper overlay")
			if err := errors.Join(writeErr, file.Close()); err != nil {
				t.Fatal(err)
			}
			// CI runners can start the test process with Ctrl+C ignored, and a
			// new console inherits that attribute. Restore normal processing so
			// the test measures the launcher, not the runner's console setup.
			// Failure is harmless: the assertions below report any real gap.
			_, _, _ = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler").Call(0, 0)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, launcher, "-test.run=^TestWindowsConsoleHelper$")
			cmd.Env = append(os.Environ(), "DOMJUDGE_TOOL_CONSOLE_HELPER=launcher")
			cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
			output, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			})
			reader := bufio.NewReader(output)
			line, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(line) != "ready" {
				t.Fatalf("console readiness: %q, %v, %s", line, err, &stderr)
			}
			sender := exec.CommandContext(ctx, executable, "-test.run=^TestWindowsConsoleHelper$")
			sender.Env = append(os.Environ(), "DOMJUDGE_TOOL_CONSOLE_HELPER=sender", "DOMJUDGE_TOOL_CONSOLE_PID="+strconv.Itoa(cmd.Process.Pid), "DOMJUDGE_TOOL_CONSOLE_EVENT="+event)
			if output, err := sender.CombinedOutput(); err != nil {
				t.Fatalf("sending console event: %v, %s", err, output)
			}
			line, err = reader.ReadString('\n')
			if err != nil || strings.TrimSpace(line) != "cancelled" {
				t.Fatalf("child did not handle event: %q, %v", line, err)
			}
			var exit *exec.ExitError
			if err := cmd.Wait(); !errors.As(err, &exit) || exit.ExitCode() != 130 {
				t.Fatalf("launcher did not preserve canceled child status: %v, %s", err, &stderr)
			}
		})
	}
}

func TestWindowsModifiedLauncher_Integration(t *testing.T) {
	m, _ := testManager(t)
	s := installTest(t, m)
	writeTestFile(t, s.CommandPath, []byte("modified in place"), 0o755)
	got := inspectTest(t, m, "conflict")
	if got.CanUninstall || got.CanRelink {
		t.Fatalf("modified launcher still owned: %+v", got)
	}
	if _, err := m.Uninstall(context.Background(), got.Revision); err == nil {
		t.Fatal("removed modified launcher")
	}
}

func TestWindowsBusyObjects_Integration(t *testing.T) {
	for _, name := range []string{"command", "target"} {
		t.Run(name, func(t *testing.T) {
			m, _ := testManager(t)
			s := installTest(t, m)
			path := s.CommandPath
			if name == "target" {
				path = m.targetPath
			}
			p, err := windows.UTF16PtrFromString(path)
			if err != nil {
				t.Fatal(err)
			}
			handle, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
			if err != nil {
				t.Fatal(err)
			}
			operation := m.Uninstall
			if name == "target" {
				operation = m.Relink
			}
			got, operationErr := operation(context.Background(), s.Revision)
			closeErr := windows.CloseHandle(handle)
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			if operationErr == nil || got.State != "error" {
				t.Fatalf("busy object hidden: %+v, %v", got, operationErr)
			}
			current := inspectTest(t, m, "current")
			if _, err := m.Uninstall(context.Background(), current.Revision); err != nil {
				t.Fatalf("cannot remove after releasing busy handle: %v", err)
			}
		})
	}
}

func TestWindowsHardlinkLauncherCycle_Integration(t *testing.T) {
	launcher, target := launcherFixture(t)
	mustRemove(t, target)
	if err := os.Link(launcher, target); err != nil {
		t.Fatal(err)
	}
	if _, err := RunLauncher(context.Background(), launcher, nil, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("hardlinked launcher cycle accepted")
	}
}

func TestWindowsRejectsShellTarget_Integration(t *testing.T) {
	launcher, target := launcherFixture(t)
	shell := filepath.Join(filepath.Dir(target), "command.cmd")
	writeTestFile(t, shell, []byte("exit /b 0"), 0o600)
	writeLauncherTarget(t, launcher, shell)
	if _, err := RunLauncher(context.Background(), launcher, nil, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("shell target accepted")
	}
}
