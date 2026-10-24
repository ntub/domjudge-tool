package cliinstall

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
)

// RunLauncher executes exactly the target beside the installed console launcher.
// The caller supplies its standard streams; cwd and environment are inherited.
// A child exit (including a nonzero exit) is returned without an extra error.
func RunLauncher(ctx context.Context, executablePath string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	target, err := launcherTarget(ctx, executablePath)
	if err != nil {
		return 1, err
	}
	// Both processes share the existing console and process group. The console
	// delivers Ctrl+C/Break to the child itself; handling it here keeps the
	// launcher alive to return the child's exit status. Do not use signal.Ignore
	// or CREATE_NEW_PROCESS_GROUP, which can disable Ctrl+C in the child.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	cmd := exec.CommandContext(ctx, target, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("starting bundled CLI %s: %w", target, err)
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return 1, fmt.Errorf("bundled CLI canceled: %w", ctx.Err())
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code := exit.ExitCode()
			if code < 0 {
				return 1, fmt.Errorf("bundled CLI terminated: %w", err)
			}
			return code, nil
		}
		return 1, fmt.Errorf("waiting for bundled CLI: %w", err)
	}
	return 0, nil
}

func launcherTarget(ctx context.Context, executablePath string) (string, error) {
	if !filepath.IsAbs(executablePath) {
		return "", errors.New("launcher executable path must be absolute")
	}
	launcher, err := filepath.EvalSymlinks(executablePath)
	if err != nil {
		return "", fmt.Errorf("resolving console launcher: %w", err)
	}
	documentPath := filepath.Join(filepath.Dir(filepath.Dir(launcher)), "target.json")
	document, err := observeObject(ctx, documentPath, true)
	if err != nil {
		return "", fmt.Errorf("reading installed CLI target: %w", err)
	}
	if !document.exists() || !document.info.Mode().IsRegular() {
		return "", errors.New("installed CLI target is missing or invalid; open the desktop app and Relink")
	}
	path, err := decodeTarget(document.data)
	if err != nil {
		return "", fmt.Errorf("installed CLI target is invalid; open the desktop app and Relink: %w", err)
	}
	if runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(path), ".exe") {
		return "", errors.New("installed CLI target must be an exact .exe path, not a shell command")
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("bundled CLI is unavailable; restore the app or Relink: %w", err)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("inspecting bundled CLI target: %w", err)
	}
	if !targetInfo.Mode().IsRegular() || (runtime.GOOS != "windows" && targetInfo.Mode()&0o111 == 0) {
		return "", errors.New("installed CLI target is not an executable file")
	}
	launcherInfo, err := os.Stat(launcher)
	if err != nil {
		return "", fmt.Errorf("inspecting console launcher: %w", err)
	}
	if os.SameFile(launcherInfo, targetInfo) || samePath(launcher, target) {
		return "", errors.New("installed CLI target forms a launcher cycle")
	}
	// Build info also identifies copied launchers from other versions. Reading
	// it does not execute the target, and stripped Go binaries retain this info.
	if info, err := buildinfo.ReadFile(target); err == nil && info.Path == "github.com/ntub/domjudge-tool/cmd/cli-launcher" {
		return "", errors.New("installed CLI target is another launcher, not the bundled CLI")
	}
	if launcherInfo.Size() == targetInfo.Size() {
		launcherHash, err := fileDigest(ctx, launcher)
		if err != nil {
			return "", err
		}
		targetHash, err := fileDigest(ctx, target)
		if err != nil {
			return "", err
		}
		if launcherHash == targetHash {
			return "", errors.New("installed CLI target is a copy of the launcher")
		}
	}
	return target, nil
}

// The launcher document has exactly one path field. A duplicate path field is
// corruption, even when encoding/json would otherwise accept its last value.
func decodeTarget(data []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", errors.New("expected a target object")
	}
	token, err = decoder.Token()
	if err != nil || token != "path" {
		return "", errors.New("expected exactly one path field")
	}
	var path string
	if err = decoder.Decode(&path); err != nil {
		return "", fmt.Errorf("reading target path: %w", err)
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') {
		return "", errors.New("expected exactly one path field")
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return "", errors.New("unexpected content after target object")
	}
	if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return "", errors.New("target path must be absolute")
	}
	return path, nil
}
