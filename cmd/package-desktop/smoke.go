package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// smokeArchive exercises only the independent CLI. It never starts a WebView,
// installs a command, contacts a DOMjudge, or reads the operator's config.
func smokeArchive(ctx context.Context, result bundleResult, meta bundleMetadata, stdout io.Writer) error {
	if meta.GOOS != runtime.GOOS || meta.GOARCH != runtime.GOARCH {
		return errors.New("archive smoke requires a native OS/architecture match")
	}
	checksum, err := archiveChecksum(ctx, result.Archive)
	if err != nil {
		return err
	}
	stored, err := os.ReadFile(result.Checksum)
	if err != nil {
		return fmt.Errorf("reading archive checksum: %w", err)
	}
	if string(stored) != checksum {
		return errors.New("archive SHA-256 checksum mismatch")
	}
	work, err := os.MkdirTemp("", "domjudge-desktop-smoke-*")
	if err != nil {
		return fmt.Errorf("creating smoke directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(work) }() // Best-effort cleanup must not mask a smoke failure.
	// Spaces and Unicode are intentional: a downloaded bundle can live anywhere.
	unpacked := filepath.Join(work, "unpacked bundle 空間")
	if err := unpackArchive(ctx, result.Archive, unpacked); err != nil {
		return fmt.Errorf("unpacking smoke artifact: %w", err)
	}
	layout := layoutFor(meta.GOOS)
	bundle := filepath.Join(unpacked, layout.root)
	manifest, err := os.ReadFile(filepath.Join(bundle, filepath.FromSlash(layout.manifest)))
	if err != nil {
		return fmt.Errorf("reading unpacked manifest: %w", err)
	}
	var actual bundleMetadata
	if err := json.Unmarshal(manifest, &actual); err != nil {
		return fmt.Errorf("decoding unpacked manifest: %w", err)
	}
	if actual != meta {
		return errors.New("unpacked manifest differs from the build identity")
	}
	cli := filepath.Join(bundle, filepath.FromSlash(layout.cli))
	if err := verifyBuildIdentity(ctx, cli, meta, "cli"); err != nil {
		return err
	}
	home := filepath.Join(work, "home")
	configDir := filepath.Join(home, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("creating isolated smoke home: %w", err)
	}
	env := append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "LOCALAPPDATA="+configDir, "APPDATA="+configDir, "XDG_CONFIG_HOME="+configDir, "NO_COLOR=1")
	for _, args := range [][]string{
		{"--help"},
		{"version"},
		{"gen-config", "--host", "https://example.org", "--username", "example", "--password", "placeholder", "--version", "8.3.1"},
	} {
		cmd := exec.CommandContext(ctx, cli, args...)
		cmd.Dir, cmd.Env = work, env
		output, err := cmd.CombinedOutput()
		if _, printErr := fmt.Fprintf(stdout, "$ domjudge-tool %s\n%s", strings.Join(args, " "), output); printErr != nil {
			return fmt.Errorf("printing smoke output: %w", printErr)
		}
		if err != nil {
			return fmt.Errorf("packaged CLI %s: %w", args[0], err)
		}
		switch args[0] {
		case "--help":
			if !strings.Contains(string(output), "Usage:") || !strings.Contains(string(output), "gen-config") {
				return errors.New("packaged CLI did not return command help")
			}
		case "version":
			short := meta.Revision
			if len(short) > 12 {
				short = short[:12]
			}
			if !strings.HasPrefix(string(output), "domjudge-tool "+meta.Version+" (") || !strings.Contains(string(output), short) {
				return errors.New("packaged CLI version/revision does not match the manifest")
			}
		}
	}
	config, err := os.ReadFile(filepath.Join(work, "domserver.json"))
	if err != nil {
		return fmt.Errorf("reading smoke-generated config: %w", err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(config, &values); err != nil {
		return fmt.Errorf("decoding smoke-generated config: %w", err)
	}
	for key, want := range map[string]string{"host": "https://example.org", "username": "example", "password": "placeholder", "version": "8.3.1"} {
		var got string
		if err := json.Unmarshal(values[key], &got); err != nil || got != want {
			return fmt.Errorf("smoke-generated config %s does not match the supplied value", key)
		}
	}
	_, err = fmt.Fprintln(stdout, "Native unpacked CLI smoke passed (help, version/revision, gen-config).")
	return err
}

func unpackArchive(ctx context.Context, path, dest string) error {
	if strings.HasSuffix(path, ".zip") {
		reader, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		defer func() { _ = reader.Close() }() // Read-only archive; each member is read and CRC-checked.
		for _, file := range reader.File {
			in, err := file.Open()
			if err != nil {
				return err
			}
			err = extractFile(ctx, dest, file.Name, file.Mode(), in)
			if err := errors.Join(err, in.Close()); err != nil {
				return err
			}
		}
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }() // Read-only archive; tar/gzip report data errors.
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() { _ = compressed.Close() }() // Read-only decompressor; the stream is consumed below.
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			// Read through gzip's trailer so its checksum is checked as well.
			_, err = io.Copy(io.Discard, compressed)
			return err
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			return fmt.Errorf("unsupported archive entry %q", header.Name)
		}
		if err := extractFile(ctx, dest, header.Name, header.FileInfo().Mode(), reader); err != nil {
			return err
		}
	}
}

func extractFile(ctx context.Context, dest, name string, mode fs.FileMode, in io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	local := filepath.FromSlash(name)
	if !filepath.IsLocal(local) || strings.Contains(name, "\\") || !mode.IsRegular() {
		return fmt.Errorf("unsafe archive entry %q", name)
	}
	path := filepath.Join(dest, local)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, in)
	return errors.Join(copyErr, file.Close())
}
