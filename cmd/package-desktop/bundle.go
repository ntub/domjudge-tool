package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"time"
)

type bundleMetadata struct {
	Schema   int    `json:"schema"`
	Version  string `json:"version"`
	Revision string `json:"revision"`
	GOOS     string `json:"goos"`
	GOARCH   string `json:"goarch"`
}

type bundleInputs struct {
	CLI, GUI, Launcher, InfoPlist, Icon, License string
}

type bundleResult struct {
	Bundle, Archive, Checksum string
}

type bundleLayout struct {
	root, cli, gui, manifest, launcher, extension string
}

func layoutFor(goos string) bundleLayout {
	switch goos {
	case "darwin":
		return bundleLayout{
			root: "domjudge-tool.app", cli: "Contents/Resources/cli/domjudge-tool",
			gui: "Contents/MacOS/domjudge-tool-app", manifest: "Contents/Resources/bundle.json", extension: ".zip",
		}
	case "windows":
		return bundleLayout{
			root: "domjudge-tool", cli: "cli/domjudge-tool.exe", gui: "domjudge-tool-app.exe",
			manifest: "bundle.json", launcher: "resources/cli-launcher.exe", extension: ".zip",
		}
	default:
		return bundleLayout{
			root: "domjudge-tool", cli: "cli/domjudge-tool", gui: "domjudge-tool-app",
			manifest: "bundle.json", extension: ".tar.gz",
		}
	}
}

func validateMetadata(meta bundleMetadata) error {
	if meta.Schema != 1 || meta.Revision == "" {
		return errors.New("bundle schema must be 1 and revision must be set")
	}
	if meta.GOOS != "darwin" && meta.GOOS != "linux" && meta.GOOS != "windows" {
		return fmt.Errorf("unsupported desktop OS %q", meta.GOOS)
	}
	if meta.GOARCH != "amd64" && meta.GOARCH != "arm64" {
		return fmt.Errorf("unsupported desktop architecture %q", meta.GOARCH)
	}
	if meta.Version == "" || meta.Version == "." || meta.Version == ".." {
		return errors.New("version must be a nonempty filename-safe build identifier")
	}
	for _, char := range meta.Version {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._+-", char) {
			continue
		}
		return fmt.Errorf("version %q contains an unsafe filename/linker character; supply -version", meta.Version)
	}
	return nil
}

type bundleFile struct {
	name, source string
	data         []byte
	mode         fs.FileMode
}

// packageBundle stages the entire artifact before replacing an existing local
// build. It never copies arbitrary source-tree contents into a distribution.
func packageBundle(ctx context.Context, meta bundleMetadata, inputs bundleInputs, output, artifacts string) (bundleResult, error) {
	var result bundleResult
	if err := validateMetadata(meta); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	layout := layoutFor(meta.GOOS)
	manifest, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return result, fmt.Errorf("encoding bundle manifest: %w", err)
	}
	files := []bundleFile{
		{name: layout.cli, source: inputs.CLI, mode: 0o755},
		{name: layout.gui, source: inputs.GUI, mode: 0o755},
		{name: layout.manifest, data: append(manifest, '\n'), mode: 0o644},
	}
	if meta.GOOS == "darwin" {
		plist, err := renderPlist(ctx, inputs.InfoPlist, meta)
		if err != nil {
			return result, err
		}
		files = append(files,
			bundleFile{name: "Contents/Info.plist", data: plist, mode: 0o644},
			bundleFile{name: "Contents/Resources/icons.icns", source: inputs.Icon, mode: 0o644},
			bundleFile{name: "Contents/Resources/LICENSE", source: inputs.License, mode: 0o644},
		)
	} else {
		files = append(files, bundleFile{name: "LICENSE", source: inputs.License, mode: 0o644})
	}
	if meta.GOOS == "windows" {
		files = append(files, bundleFile{name: layout.launcher, source: inputs.Launcher, mode: 0o755})
	}
	slices.SortFunc(files, func(a, b bundleFile) int { return strings.Compare(a.name, b.name) })
	if info, err := os.Lstat(output); err == nil && !info.IsDir() {
		return result, fmt.Errorf("bundle output is not a directory: %s", output)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return result, fmt.Errorf("inspecting bundle output: %w", err)
	}
	if err := validateExistingBundle(output, meta.GOOS); err != nil {
		return result, err
	}
	resolvedOutput, err := resolveExistingAncestors(output)
	if err != nil {
		return result, fmt.Errorf("resolving bundle output: %w", err)
	}
	resolvedArtifacts, err := resolveExistingAncestors(artifacts)
	if err != nil {
		return result, fmt.Errorf("resolving artifact directory: %w", err)
	}
	for _, file := range files {
		if file.source != "" {
			source, err := resolveExistingAncestors(file.source)
			if err != nil {
				return result, fmt.Errorf("resolving bundle input: %w", err)
			}
			if contains, err := pathWithin(resolvedOutput, source); err != nil {
				return result, err
			} else if contains {
				return result, errors.New("bundle output must not contain its source inputs")
			}
		}
	}
	if err := validateDestinationOverlap(resolvedOutput, resolvedArtifacts); err != nil {
		return result, err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return result, fmt.Errorf("creating output parent: %w", err)
	}
	stage, err := os.MkdirTemp(filepath.Dir(output), ".package-desktop-")
	if err != nil {
		return result, fmt.Errorf("creating bundle stage: %w", err)
	}
	defer func() { _ = os.RemoveAll(stage) }() // Best-effort cleanup must not mask a packaging failure.
	bundle := filepath.Join(stage, layout.root)
	for _, file := range files {
		if err := writeBundleFile(ctx, bundle, file); err != nil {
			return result, fmt.Errorf("assembling %s: %w", file.name, err)
		}
	}
	name := "domjudge-tool-desktop_" + meta.Version + "_" + meta.GOOS + "_" + meta.GOARCH + layout.extension
	archive := filepath.Join(stage, name)
	if err := writeArchive(ctx, archive, bundle, layout, files); err != nil {
		return result, fmt.Errorf("writing bundle archive: %w", err)
	}
	checksum, err := archiveChecksum(ctx, archive)
	if err != nil {
		return result, err
	}
	if err := os.WriteFile(archive+".sha256", []byte(checksum), 0o644); err != nil {
		return result, fmt.Errorf("writing archive checksum: %w", err)
	}
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		return result, fmt.Errorf("creating artifact directory: %w", err)
	}
	// Directory creation can reveal aliases on case-insensitive volumes.
	if err := validateDestinationOverlap(resolvedOutput, resolvedArtifacts); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := replaceBundle(ctx, bundle, output, os.Rename); err != nil {
		return result, err
	}
	result = bundleResult{Bundle: output, Archive: filepath.Join(artifacts, name), Checksum: filepath.Join(artifacts, name+".sha256")}
	// Staging under the bundle parent keeps bundle replacement on one volume;
	// copying the archive also supports a separate artifact volume.
	for _, file := range []struct{ source, dest string }{{archive, result.Archive}, {archive + ".sha256", result.Checksum}} {
		if err := copyArtifact(ctx, file.source, file.dest); err != nil {
			return bundleResult{}, err
		}
	}
	return result, nil
}

func pathWithin(parent, child string) (bool, error) {
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false, err
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false, err
	}
	if rel, err := filepath.Rel(parentAbs, childAbs); err == nil && filepath.IsLocal(rel) {
		return true, nil
	}
	parentInfo, err := os.Stat(parentAbs)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("identifying destination ancestor %s: %w", parentAbs, err)
	}
	// SameFile handles case aliases without assuming the volume's case policy.
	for path := childAbs; ; path = filepath.Dir(path) {
		info, err := os.Stat(path)
		if err == nil && os.SameFile(parentInfo, info) {
			return true, nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("identifying destination ancestor %s: %w", path, err)
		}
		if filepath.Dir(path) == path {
			return false, nil
		}
	}
}

func validateDestinationOverlap(output, artifacts string) error {
	for _, pair := range [][2]string{{output, artifacts}, {artifacts, output}} {
		if contains, err := pathWithin(pair[0], pair[1]); err != nil {
			return err
		} else if contains {
			return errors.New("bundle and artifact directories must not overlap")
		}
	}
	return nil
}

// Resolve existing ancestors even when the final build directory is not present.
func resolveExistingAncestors(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		return resolved, err
	}
	parent := filepath.Dir(absolute)
	if parent == absolute {
		return "", err
	}
	resolved, err = resolveExistingAncestors(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(absolute)), nil
}

func validateExistingBundle(output, goos string) error {
	dir, err := os.Open(output)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("opening existing bundle: %w", err)
	}
	_, readErr := dir.Readdirnames(1)
	if err := dir.Close(); err != nil {
		return fmt.Errorf("closing existing bundle directory: %w", err)
	}
	if errors.Is(readErr, io.EOF) {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("reading existing bundle directory: %w", readErr)
	}
	layout := layoutFor(goos)
	for _, name := range []string{layout.manifest, layout.cli, layout.gui} {
		info, err := os.Lstat(filepath.Join(output, filepath.FromSlash(name)))
		if err != nil {
			return fmt.Errorf("refusing to replace an unrecognized bundle directory %s: %w", output, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace an unrecognized bundle directory: %s", output)
		}
	}
	file, err := os.Open(filepath.Join(output, filepath.FromSlash(layout.manifest)))
	if err != nil {
		return fmt.Errorf("opening previous bundle manifest: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, 65537))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return fmt.Errorf("reading previous bundle manifest: %w", err)
	}
	var previous bundleMetadata
	if len(data) > 65536 || json.Unmarshal(data, &previous) != nil || validateMetadata(previous) != nil || previous.GOOS != goos {
		return fmt.Errorf("refusing to replace a directory with invalid bundle metadata: %s", output)
	}
	return nil
}

func renderPlist(ctx context.Context, path string, meta bundleMetadata) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading Info.plist template: %w", err)
	}
	tmpl, err := template.New("Info.plist").Option("missingkey=error").Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing Info.plist template: %w", err)
	}
	var output bytes.Buffer
	values := struct {
		bundleMetadata
		MacVersion string
	}{bundleMetadata: meta, MacVersion: macVersion(meta.Version)}
	if err := tmpl.Execute(&output, values); err != nil {
		return nil, fmt.Errorf("rendering Info.plist: %w", err)
	}
	return output.Bytes(), nil
}

// Apple's version keys are numeric. Preserve the exact source version/revision
// in DTBundleVersion/DTBundleRevision and bundle.json, including dev builds.
func macVersion(version string) string {
	base, _, _ := strings.Cut(strings.TrimPrefix(version, "v"), "-")
	base, _, _ = strings.Cut(base, "+")
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return "0.0.0"
	}
	for _, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return "0.0.0"
		}
	}
	return base
}

func writeBundleFile(ctx context.Context, root string, file bundleFile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(file.name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if file.source == "" {
		return os.WriteFile(path, file.data, file.mode)
	}
	info, err := os.Lstat(file.source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("input must be a regular file: %s", file.source)
	}
	in, err := os.Open(file.source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() // Read-only input; io.Copy reports read errors.
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, file.mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close())
}

func replaceBundle(ctx context.Context, staged, output string, rename func(string, string) error) error {
	// A failed rollback must survive the caller's staging-directory cleanup.
	backup := filepath.Dir(staged) + ".previous"
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(backup); !errors.Is(err, fs.ErrNotExist) {
		if err != nil {
			return fmt.Errorf("inspecting bundle recovery path: %w", err)
		}
		return fmt.Errorf("bundle recovery path already exists: %s", backup)
	}
	moved := false
	if err := rename(output, backup); err == nil {
		moved = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("moving previous local bundle: %w", err)
	}
	if err := rename(staged, output); err != nil {
		if moved {
			if restoreErr := rename(backup, output); restoreErr != nil {
				return fmt.Errorf("replacing local bundle: %w; restoring previous bundle: %w; previous bundle retained at %s", err, restoreErr, backup)
			}
		}
		return fmt.Errorf("replacing local bundle: %w", err)
	}
	if moved {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("removing previous bundle at %s: %w", backup, err)
		}
	}
	return nil
}

func copyArtifact(ctx context.Context, source, dest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("opening artifact: %w", err)
	}
	defer func() { _ = in.Close() }() // Read-only source; io.Copy reports read errors.
	out, err := os.CreateTemp(filepath.Dir(dest), ".artifact-")
	if err != nil {
		return fmt.Errorf("creating artifact: %w", err)
	}
	defer func() { _ = os.Remove(out.Name()) }() // Rename consumes this path; failure cleanup is best-effort.
	_, copyErr := io.Copy(out, in)
	if err := errors.Join(copyErr, out.Chmod(0o644), out.Close()); err != nil {
		return fmt.Errorf("copying artifact: %w", err)
	}
	if err := os.Rename(out.Name(), dest); err != nil {
		return fmt.Errorf("replacing artifact: %w", err)
	}
	return nil
}

func writeArchive(ctx context.Context, path, bundle string, layout bundleLayout, files []bundleFile) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if layout.extension == ".zip" {
		writer := zip.NewWriter(file)
		err := writeArchiveFiles(ctx, bundle, layout.root, files, func(name string, info fs.FileInfo, mode fs.FileMode) (io.Writer, error) {
			header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)}
			header.SetMode(mode)
			return writer.CreateHeader(header)
		})
		return errors.Join(err, writer.Close(), file.Close())
	}
	compressed := gzip.NewWriter(file)
	writer := tar.NewWriter(compressed)
	err = writeArchiveFiles(ctx, bundle, layout.root, files, func(name string, info fs.FileInfo, mode fs.FileMode) (io.Writer, error) {
		header := &tar.Header{Name: name, Mode: int64(mode.Perm()), Size: info.Size(), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			return nil, err
		}
		return writer, nil
	})
	return errors.Join(err, writer.Close(), compressed.Close(), file.Close())
}

func writeArchiveFiles(ctx context.Context, bundle, root string, files []bundleFile, header func(string, fs.FileInfo, fs.FileMode) (io.Writer, error)) error {
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		in, err := os.Open(filepath.Join(bundle, filepath.FromSlash(file.name)))
		if err != nil {
			return err
		}
		info, statErr := in.Stat()
		if statErr != nil {
			return errors.Join(statErr, in.Close())
		}
		out, headerErr := header(root+"/"+file.name, info, file.mode)
		if headerErr != nil {
			return errors.Join(headerErr, in.Close())
		}
		_, copyErr := io.Copy(out, in)
		if err := errors.Join(copyErr, in.Close()); err != nil {
			return err
		}
	}
	return nil
}

func archiveChecksum(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening archive for checksum: %w", err)
	}
	defer func() { _ = file.Close() }() // Read-only archive; io.Copy reports read errors.
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hashing archive: %w", err)
	}
	return fmt.Sprintf("%x  %s\n", hash.Sum(nil), filepath.Base(path)), nil
}
