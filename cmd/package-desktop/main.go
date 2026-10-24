// Command package-desktop builds the independent Desktop and CLI together.
// Local packaging and native CI use this same entry point.
package main

import (
	"context"
	"debug/buildinfo"
	"debug/pe"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "package-desktop:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("package-desktop", flag.ContinueOnError)
	flags.SetOutput(stderr)
	version := flags.String("version", "", "build version (default: git describe --tags --always --dirty)")
	goos := flags.String("goos", runtime.GOOS, "target OS: darwin, linux, windows")
	goarch := flags.String("goarch", runtime.GOARCH, "target architecture: amd64, arm64")
	output := flags.String("output", "", "unpacked bundle directory (default: bin/domjudge-tool.app on macOS, bin/desktop/domjudge-tool elsewhere)")
	artifacts := flags.String("artifacts", "dist/desktop", "directory for the archive and SHA-256 checksum")
	smoke := flags.Bool("smoke", false, "unpack and run the native bundled CLI help, version, and gen-config")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("positional arguments are not supported")
	}
	// Validate the request before invoking build tools or touching output paths.
	checkVersion := *version
	if checkVersion == "" {
		checkVersion = "dev"
	}
	if err := validateMetadata(bundleMetadata{Schema: 1, Version: checkVersion, Revision: "pending", GOOS: *goos, GOARCH: *goarch}); err != nil {
		return err
	}
	native := *goos == runtime.GOOS && *goarch == runtime.GOARCH
	if !native && *goos != "windows" {
		return errors.New("macOS and Linux Desktop packaging requires a native runner with its platform SDK")
	}
	if *smoke && !native {
		return errors.New("-smoke requires the native target OS and architecture; cross-compilation is not native proof")
	}
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("finding repository directory: %w", err)
	}
	if *output == "" {
		if *goos == "darwin" {
			*output = filepath.Join("bin", "domjudge-tool.app")
		} else {
			*output = filepath.Join("bin", "desktop", "domjudge-tool")
		}
	}
	resolvedOutput, err := resolveExistingAncestors(*output)
	if err != nil {
		return fmt.Errorf("resolving bundle output: %w", err)
	}
	resolvedRoot, err := resolveExistingAncestors(root)
	if err != nil {
		return fmt.Errorf("resolving repository directory: %w", err)
	}
	if contains, err := pathWithin(resolvedOutput, resolvedRoot); err != nil {
		return err
	} else if contains {
		return errors.New("bundle output must not replace the repository or its ancestors")
	}
	meta, err := resolveMetadata(ctx, root, *version, *goos, *goarch)
	if err != nil {
		return err
	}
	buildDir, err := os.MkdirTemp("", "domjudge-desktop-build-")
	if err != nil {
		return fmt.Errorf("creating build directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(buildDir) }() // Best-effort cleanup must not mask a build failure.
	suffix := ""
	if meta.GOOS == "windows" {
		suffix = ".exe"
	}
	inputs := bundleInputs{
		CLI:       filepath.Join(buildDir, "domjudge-tool"+suffix),
		GUI:       filepath.Join(buildDir, "domjudge-tool-app"+suffix),
		Launcher:  filepath.Join(buildDir, "cli-launcher.exe"),
		InfoPlist: filepath.Join(root, "build", "darwin", "Info.plist"),
		Icon:      filepath.Join(root, "build", "darwin", "icons.icns"),
		License:   filepath.Join(root, "LICENSE"),
	}
	for _, component := range []struct{ kind, path string }{{"cli", inputs.CLI}, {"gui", inputs.GUI}, {"launcher", inputs.Launcher}} {
		if component.kind == "launcher" && meta.GOOS != "windows" {
			continue
		}
		if _, err := fmt.Fprintf(stdout, "Building %s %s/%s (%s, %s)\n", component.kind, meta.GOOS, meta.GOARCH, meta.Version, meta.Revision); err != nil {
			return fmt.Errorf("printing build progress: %w", err)
		}
		if err := buildComponent(ctx, root, meta, component.kind, component.path, stdout, stderr); err != nil {
			return err
		}
	}
	result, err := packageBundle(ctx, meta, inputs, *output, *artifacts)
	if err != nil {
		return err
	}
	if *smoke {
		if err := smokeArchive(ctx, result, meta, stdout); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "Bundle: %s\nArchive: %s\nSHA-256: %s\n", result.Bundle, result.Archive, result.Checksum)
	return err
}

func resolveMetadata(ctx context.Context, root, version, goos, goarch string) (bundleMetadata, error) {
	var meta bundleMetadata
	revision, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return meta, err
	}
	if version == "" {
		version, err = gitOutput(ctx, root, "describe", "--tags", "--always", "--dirty")
		if err != nil {
			return meta, err
		}
	}
	meta = bundleMetadata{Schema: 1, Version: version, Revision: revision, GOOS: goos, GOARCH: goarch}
	return meta, validateMetadata(meta)
}

func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolving source identity with git %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func buildComponent(ctx context.Context, root string, meta bundleMetadata, kind, output string, stdout, stderr io.Writer) error {
	cgo, pkg, ldflags := "0", "./cmd/domjudge-tool", "-s -w"
	args := []string{"build", "-trimpath", "-buildvcs=true"}
	switch kind {
	case "cli":
		ldflags += " -X github.com/ntub/domjudge-tool/internal/cli/general.Version=" + meta.Version
	case "gui":
		pkg = "./cmd/desktop"
		args = append(args, "-tags", "desktop")
		ldflags += " -X main.appVersion=" + meta.Version
		if meta.GOOS == "windows" {
			ldflags += " -H windowsgui"
		} else {
			cgo = "1"
		}
	case "launcher":
		pkg = "./cmd/cli-launcher"
	default:
		return fmt.Errorf("unknown build component %q", kind)
	}
	args = append(args, "-ldflags", ldflags, "-o", output, pkg)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+meta.GOOS, "GOARCH="+meta.GOARCH, "CGO_ENABLED="+cgo)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building %s: %w", kind, err)
	}
	return verifyBuildIdentity(ctx, output, meta, kind)
}

func verifyBuildIdentity(ctx context.Context, path string, meta bundleMetadata, kind string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s build identity: %w", kind, err)
	}
	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["GOOS"] != meta.GOOS || settings["GOARCH"] != meta.GOARCH || settings["vcs.revision"] != meta.Revision {
		return fmt.Errorf("%s build identity does not match bundle metadata", kind)
	}
	if meta.GOOS == "windows" {
		file, err := pe.Open(path)
		if err != nil {
			return fmt.Errorf("reading Windows %s subsystem: %w", kind, err)
		}
		defer func() { _ = file.Close() }() // Read-only PE inspection; parsing errors are checked above.
		header, ok := file.OptionalHeader.(*pe.OptionalHeader64)
		want := uint16(pe.IMAGE_SUBSYSTEM_WINDOWS_CUI)
		if kind == "gui" {
			want = pe.IMAGE_SUBSYSTEM_WINDOWS_GUI
		}
		if !ok || header.Subsystem != want {
			return fmt.Errorf("unexpected executable subsystem for Windows %s", kind)
		}
	}
	return nil
}
