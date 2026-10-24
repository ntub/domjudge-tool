package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown flag", args: []string{"-unknown"}, want: "flag provided but not defined"},
		{name: "positional input", args: []string{"other"}, want: "positional arguments"},
		{name: "unsupported OS", args: []string{"-goos", "freebsd"}, want: "unsupported desktop OS"},
		{name: "unsupported architecture", args: []string{"-goarch", "386"}, want: "unsupported desktop architecture"},
		{name: "unsafe version", args: []string{"-version", "../escape"}, want: "unsafe filename"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			err := run(context.Background(), tc.args, &output, &output)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRunRejectsRepositoryAlias_Integration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows standard users need not have symlink privileges")
	}
	work := t.TempDir()
	t.Chdir(work)
	alias := filepath.Join(t.TempDir(), "repository")
	if err := os.Symlink(work, alias); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := run(context.Background(), []string{"-output", alias}, &output, &output)
	if err == nil || !strings.Contains(err.Error(), "must not replace the repository") {
		t.Fatalf("repository alias was not rejected before building: %v; %s", err, &output)
	}
}

func TestRunHelp(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"-help"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-version", "-goos", "-goarch", "-output", "-artifacts", "-smoke"} {
		if !strings.Contains(output.String(), flag) {
			t.Errorf("help omits %s: %s", flag, output.String())
		}
	}
}

func TestNativeCLIArchiveSmoke_Integration(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := resolveMetadata(context.Background(), root, "v1.2.3-packaging-test", runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "native CLI 空間")
	inputs := newBundleInputs(t, work)
	inputs.CLI = filepath.Join(work, "real-cli")
	if runtime.GOOS == "windows" {
		inputs.CLI += ".exe"
	}
	var output bytes.Buffer
	if err := buildComponent(context.Background(), root, meta, "cli", inputs.CLI, &output, &output); err != nil {
		t.Fatalf("building actual CLI: %v\n%s", err, &output)
	}
	result, err := packageBundle(context.Background(), meta, inputs, filepath.Join(work, "bundle"), filepath.Join(work, "archives"))
	if err != nil {
		t.Fatal(err)
	}
	if err := smokeArchive(context.Background(), result, meta, &output); err != nil {
		t.Fatalf("smoking actual packaged CLI: %v\n%s", err, &output)
	}
	if !strings.Contains(output.String(), "domjudge-tool "+meta.Version) || !strings.Contains(output.String(), "gen-config") {
		t.Fatalf("smoke did not exercise the actual CLI: %s", &output)
	}
	// The checksum is verified before any packaged program can execute.
	if err := os.WriteFile(result.Checksum, []byte("corrupt checksum\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := smokeArchive(context.Background(), result, meta, &output); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("corrupt checksum error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("executed a CLI after checksum failure: %s", &output)
	}
}
