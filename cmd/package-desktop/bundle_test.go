package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPackageBundle_Integration(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			t.Run(goos+"/"+goarch, func(t *testing.T) {
				work := filepath.Join(t.TempDir(), "bundle inputs 空間")
				inputs := newBundleInputs(t, work)
				meta := bundleMetadata{Schema: 1, Version: "v1.2.3-rc.1+test", Revision: strings.Repeat("a", 40), GOOS: goos, GOARCH: goarch}
				output := filepath.Join(work, "unpacked")
				artifacts := filepath.Join(work, "artifacts")
				result, err := packageBundle(context.Background(), meta, inputs, output, artifacts)
				if err != nil {
					t.Fatal(err)
				}
				root, ext := "domjudge-tool", ".zip"
				cli, gui, manifest := "cli/domjudge-tool", "domjudge-tool-app", "bundle.json"
				want := map[string]string{"LICENSE": "test license\n"}
				switch goos {
				case "darwin":
					root = "domjudge-tool.app"
					cli = "Contents/Resources/cli/domjudge-tool"
					gui = "Contents/MacOS/domjudge-tool-app"
					manifest = "Contents/Resources/bundle.json"
					want = map[string]string{
						"Contents/Resources/LICENSE":    "test license\n",
						"Contents/Resources/icons.icns": "icns fixture\x00",
					}
				case "windows":
					cli += ".exe"
					gui += ".exe"
					want["resources/cli-launcher.exe"] = "launcher fixture\x00"
				default:
					ext = ".tar.gz"
				}
				want[cli] = "CLI fixture\x00"
				want[gui] = "GUI fixture\x00"
				wantName := "domjudge-tool-desktop_" + meta.Version + "_" + goos + "_" + goarch + ext
				if result.Bundle != output || filepath.Base(result.Archive) != wantName || result.Checksum != result.Archive+".sha256" {
					t.Fatalf("unexpected output paths: %+v", result)
				}
				members := readArchive(t, result.Archive)
				for name, content := range want {
					member, ok := members[root+"/"+name]
					if !ok || string(member.data) != content {
						t.Errorf("archive %s = %q, want %q", name, member.data, content)
					}
					unpacked, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(name)))
					if err != nil || string(unpacked) != content {
						t.Errorf("unpacked %s = %q, %v", name, unpacked, err)
					}
				}
				for _, name := range []string{cli, gui} {
					if mode := members[root+"/"+name].mode.Perm(); mode != 0o755 {
						t.Errorf("%s archive mode = %o, want 755", name, mode)
					}
				}
				var got bundleMetadata
				if err := json.Unmarshal(members[root+"/"+manifest].data, &got); err != nil || got != meta {
					t.Fatalf("manifest = %+v, %v; want %+v", got, err, meta)
				}
				wantCount := len(want) + 1
				if goos == "darwin" {
					wantCount++
					plist := members[root+"/Contents/Info.plist"].data
					var parsed anyPlist
					if err := xml.Unmarshal(plist, &parsed); err != nil {
						t.Fatalf("invalid packaged plist: %v", err)
					}
					if !bytes.Contains(plist, []byte(meta.Version)) || bytes.Contains(plist, []byte("{{")) {
						t.Errorf("plist version was not rendered: %s", plist)
					}
				}
				if len(members) != wantCount {
					t.Errorf("archive has %d files, want %d: %v", len(members), wantCount, members)
				}
				archive := mustRead(t, result.Archive)
				wantChecksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), wantName)
				if got := string(mustRead(t, result.Checksum)); got != wantChecksum {
					t.Errorf("checksum = %q, want %q", got, wantChecksum)
				}
				mustWrite(t, filepath.Join(output, "obsolete-file"), "old bundle content", 0o644)
				if _, err := packageBundle(context.Background(), meta, inputs, output, artifacts); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(output, "obsolete-file")); !os.IsNotExist(err) {
					t.Errorf("obsolete content survived a rebuild: %v", err)
				}
				if !bytes.Equal(archive, mustRead(t, result.Archive)) {
					t.Error("same bundle inputs produced different archive bytes")
				}
			})
		}
	}
}

func TestPackageBundleFailureKeepsPreviousOutput_Integration(t *testing.T) {
	for _, failure := range []string{"missing CLI", "missing GUI", "missing launcher", "bad plist", "unsafe version", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			work := t.TempDir()
			inputs := newBundleInputs(t, work)
			meta := bundleMetadata{Schema: 1, Version: "v1.2.3", Revision: strings.Repeat("b", 40), GOOS: "windows", GOARCH: "arm64"}
			output := filepath.Join(work, "existing")
			if _, err := packageBundle(context.Background(), meta, inputs, output, filepath.Join(work, "archives")); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(output, "keep"), "previous working bundle", 0o644)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch failure {
			case "missing CLI":
				inputs.CLI += ".missing"
			case "missing GUI":
				inputs.GUI += ".missing"
			case "missing launcher":
				inputs.Launcher += ".missing"
			case "bad plist":
				meta.GOOS = "darwin"
				mustWrite(t, inputs.InfoPlist, "{{", 0o644)
			case "unsafe version":
				meta.Version = "../escape"
			case "cancelled":
				cancel()
			}
			if _, err := packageBundle(ctx, meta, inputs, output, filepath.Join(work, "archives")); err == nil {
				t.Fatal("packaging unexpectedly succeeded")
			}
			if got := string(mustRead(t, filepath.Join(output, "keep"))); got != "previous working bundle" {
				t.Fatalf("previous output changed: %q", got)
			}
		})
	}
}

func TestPackageBundleRejectsSymlinks_Integration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows standard users need not have symlink privileges")
	}
	for _, target := range []string{"input", "output"} {
		t.Run(target, func(t *testing.T) {
			work := t.TempDir()
			inputs := newBundleInputs(t, work)
			output := filepath.Join(work, "bundle")
			if target == "input" {
				link := filepath.Join(work, "linked-cli")
				if err := os.Symlink(inputs.CLI, link); err != nil {
					t.Fatal(err)
				}
				inputs.CLI = link
			} else if err := os.Symlink(filepath.Join(work, "untouched"), output); err != nil {
				t.Fatal(err)
			}
			meta := bundleMetadata{Schema: 1, Version: "dev", Revision: strings.Repeat("c", 40), GOOS: "linux", GOARCH: "amd64"}
			if _, err := packageBundle(context.Background(), meta, inputs, output, filepath.Join(work, "archives")); err == nil {
				t.Fatal("symlink input/output unexpectedly accepted")
			}
		})
	}
}

func TestPackageBundlePreservesForeignDirectory_Integration(t *testing.T) {
	work := t.TempDir()
	inputs := newBundleInputs(t, work)
	output := filepath.Join(work, "notes")
	mustWrite(t, filepath.Join(output, "uncommitted.txt"), "keep this data", 0o600)
	meta := bundleMetadata{Schema: 1, Version: "dev", Revision: "test", GOOS: "linux", GOARCH: "amd64"}
	if _, err := packageBundle(context.Background(), meta, inputs, output, filepath.Join(work, "archives")); err == nil {
		t.Error("packaging accepted an unrelated nonempty directory")
	}
	if got := string(mustRead(t, filepath.Join(output, "uncommitted.txt"))); got != "keep this data" {
		t.Fatalf("foreign data changed: %q", got)
	}
}

func TestPackageBundleRejectsAliasedArtifactOverlap_Integration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows standard users need not have symlink privileges")
	}
	work := t.TempDir()
	inputs := newBundleInputs(t, work)
	output := filepath.Join(work, "bundle")
	meta := bundleMetadata{Schema: 1, Version: "dev", Revision: "test", GOOS: "linux", GOARCH: "amd64"}
	if _, err := packageBundle(context.Background(), meta, inputs, output, filepath.Join(work, "archives")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(output, "keep"), "old bundle data", 0o600)
	alias := filepath.Join(work, "alias")
	if err := os.Symlink(work, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := packageBundle(context.Background(), meta, inputs, output, filepath.Join(alias, "bundle", "archives")); err == nil {
		t.Error("packaging accepted physically overlapping destinations")
	}
	if got := string(mustRead(t, filepath.Join(output, "keep"))); got != "old bundle data" {
		t.Fatalf("overlap rejection changed previous bundle: %q", got)
	}
}

func TestPackageBundleRejectsCaseAliasOverlap_Integration(t *testing.T) {
	work := t.TempDir()
	inputs := newBundleInputs(t, work)
	output := filepath.Join(work, "bundle")
	meta := bundleMetadata{Schema: 1, Version: "dev", Revision: "test", GOOS: "linux", GOARCH: "amd64"}
	if _, err := packageBundle(context.Background(), meta, inputs, output, filepath.Join(work, "archives")); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(work, "BUNDLE")
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(alias)
	if os.IsNotExist(err) {
		t.Skip("test volume is case-sensitive")
	}
	if err != nil || !os.SameFile(info, aliasInfo) {
		t.Fatalf("case alias does not identify the same directory: %v", err)
	}
	mustWrite(t, filepath.Join(output, "keep"), "old bundle data", 0o600)
	if _, err := packageBundle(context.Background(), meta, inputs, output, filepath.Join(alias, "archives")); err == nil {
		t.Error("packaging accepted a case alias inside its output")
	}
	if got := string(mustRead(t, filepath.Join(output, "keep"))); got != "old bundle data" {
		t.Fatalf("case alias rejection changed previous bundle: %q", got)
	}
}

func TestPackageBundleRejectsNewCaseAliasOverlap_Integration(t *testing.T) {
	work := t.TempDir()
	mustWrite(t, filepath.Join(work, "probe"), "case probe", 0o600)
	if _, err := os.Stat(filepath.Join(work, "PROBE")); os.IsNotExist(err) {
		t.Skip("test volume is case-sensitive")
	} else if err != nil {
		t.Fatal(err)
	}
	inputs := newBundleInputs(t, work)
	output := filepath.Join(work, "bundle")
	meta := bundleMetadata{Schema: 1, Version: "dev", Revision: "test", GOOS: "linux", GOARCH: "amd64"}
	_, err := packageBundle(context.Background(), meta, inputs, output, filepath.Join(work, "BUNDLE", "archives"))
	if err == nil || !strings.Contains(err.Error(), "must not overlap") {
		t.Fatalf("new case alias destinations were not rejected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "domjudge-tool-app")); !os.IsNotExist(err) {
		t.Fatalf("overlapping destinations published a bundle: %v", err)
	}
}

func TestFailedBundleReplacementPreservesRecovery_Integration(t *testing.T) {
	for _, restoreFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("restore-fails-%t", restoreFails), func(t *testing.T) {
			work := t.TempDir()
			stage := filepath.Join(work, "stage")
			staged, output := filepath.Join(stage, "bundle"), filepath.Join(work, "output")
			mustWrite(t, filepath.Join(staged, "data"), "new bundle", 0o600)
			mustWrite(t, filepath.Join(output, "data"), "previous bundle", 0o600)
			var backup string
			rename := func(from, to string) error {
				if from == output {
					backup = to
				}
				if from == staged || (restoreFails && from == backup) {
					return os.ErrPermission
				}
				return os.Rename(from, to)
			}
			err := replaceBundle(context.Background(), staged, output, rename)
			if err == nil {
				t.Fatal("replacement failure was hidden")
			}
			// The caller always cleans its temporary build stage on return.
			if cleanupErr := os.RemoveAll(stage); cleanupErr != nil {
				t.Fatal(cleanupErr)
			}
			recovery := output
			if restoreFails {
				recovery = backup
				if !strings.Contains(err.Error(), backup) {
					t.Errorf("rollback failure omits recovery path: %v", err)
				}
			}
			if got := string(mustRead(t, filepath.Join(recovery, "data"))); got != "previous bundle" {
				t.Fatalf("previous bundle lost during recovery: %q", got)
			}
		})
	}
}

type anyPlist struct {
	XMLName xml.Name `xml:"plist"`
}

type archiveMember struct {
	data []byte
	mode os.FileMode
}

func readArchive(t *testing.T, path string) map[string]archiveMember {
	t.Helper()
	members := make(map[string]archiveMember)
	add := func(name string, mode os.FileMode, reader io.Reader) {
		if mode.IsDir() {
			return
		}
		if !mode.IsRegular() || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.Contains(name, "../") {
			t.Fatalf("unsafe archive member %q: %v", name, mode)
		}
		if _, exists := members[name]; exists {
			t.Fatalf("duplicate archive member %q", name)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		members[name] = archiveMember{data: data, mode: mode}
	}
	if strings.HasSuffix(path, ".zip") {
		reader, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		for _, file := range reader.File {
			r, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			add(file.Name, file.Mode(), r)
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
		}
		return members
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return members
		}
		if err != nil {
			t.Fatal(err)
		}
		add(header.Name, header.FileInfo().Mode(), reader)
	}
}

func newBundleInputs(t *testing.T, work string) bundleInputs {
	t.Helper()
	inputs := bundleInputs{
		CLI:       filepath.Join(work, "source", "cli"),
		GUI:       filepath.Join(work, "source", "gui"),
		Launcher:  filepath.Join(work, "source", "launcher"),
		InfoPlist: filepath.Join(work, "source", "Info.plist"),
		Icon:      filepath.Join(work, "source", "icons.icns"),
		License:   filepath.Join(work, "source", "LICENSE"),
	}
	mustWrite(t, inputs.CLI, "CLI fixture\x00", 0o755)
	mustWrite(t, inputs.GUI, "GUI fixture\x00", 0o755)
	mustWrite(t, inputs.Launcher, "launcher fixture\x00", 0o755)
	mustWrite(t, inputs.InfoPlist, `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleShortVersionString</key><string>{{.Version}}</string></dict></plist>`, 0o644)
	mustWrite(t, inputs.Icon, "icns fixture\x00", 0o644)
	mustWrite(t, inputs.License, "test license\n", 0o644)
	return inputs
}

func mustWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPackagedMacVersion_Integration(t *testing.T) {
	for _, tc := range []struct{ version, want string }{
		{version: "v1.2.3", want: "1.2.3"},
		{version: "v1.2.3-rc.1+build", want: "1.2.3"},
		{version: "dev", want: "0.0.0"},
		{version: "abc1234-dirty", want: "0.0.0"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			work := t.TempDir()
			inputs := newBundleInputs(t, work)
			inputs.InfoPlist = filepath.Join("..", "..", "build", "darwin", "Info.plist")
			meta := bundleMetadata{Schema: 1, Version: tc.version, Revision: strings.Repeat("d", 40), GOOS: "darwin", GOARCH: "arm64"}
			result, err := packageBundle(context.Background(), meta, inputs, filepath.Join(work, "bundle"), filepath.Join(work, "archives"))
			if err != nil {
				t.Fatal(err)
			}
			plist := mustRead(t, filepath.Join(result.Bundle, "Contents", "Info.plist"))
			for key, value := range map[string]string{
				"CFBundleShortVersionString": tc.want,
				"CFBundleVersion":            tc.want,
				"DTBundleVersion":            tc.version,
				"DTBundleRevision":           meta.Revision,
			} {
				want := "<key>" + key + "</key>\n  <string>" + value + "</string>"
				if !bytes.Contains(plist, []byte(want)) {
					t.Errorf("packaged plist omits %q: %s", want, plist)
				}
			}
		})
	}
}
