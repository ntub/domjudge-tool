// Package cliinstall manages explicit per-user integration of the bundled CLI.
// It never discovers home directories, edits PATH, or executes inspected files.
package cliinstall

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config contains absolute paths resolved by the native application, not HTTP.
type Config struct {
	ExecutablePath string
	HomeDir        string
	StateDir       string
}

// Status describes observed disk state, never another terminal's PATH lookup.
type Status struct {
	State, BundlePath, CLIPath, CommandPath, TargetPath string
	Version, Revision, Reason, PathNote                 string
	CanInstall, CanRelink, CanUninstall                 bool
}

// Manager has immutable configuration and uses an OS lock for all mutations.
type Manager struct {
	config      Config
	commandPath string
	targetPath  string
	receiptPath string
}

var (
	// ErrStaleRevision means disk state changed since the submitted inspection.
	ErrStaleRevision = errors.New("CLI installation changed; inspect it again before retrying")
	// ErrBusy means another process is changing this installation.
	ErrBusy = errors.New("CLI installation is busy; try again after the other operation finishes")
)

// New validates explicit configuration without reading or modifying disk.
func New(cfg Config) (*Manager, error) {
	for _, field := range []struct{ name, value string }{
		{"executable", cfg.ExecutablePath}, {"home", cfg.HomeDir}, {"state directory", cfg.StateDir},
	} {
		if !filepath.IsAbs(field.value) || strings.ContainsRune(field.value, 0) {
			return nil, fmt.Errorf("CLI installation %s must be an absolute path", field.name)
		}
	}
	cfg.ExecutablePath = filepath.Clean(cfg.ExecutablePath)
	cfg.HomeDir = filepath.Clean(cfg.HomeDir)
	cfg.StateDir = filepath.Clean(cfg.StateDir)
	m := &Manager{config: cfg, receiptPath: filepath.Join(cfg.StateDir, "receipt.json")}
	m.commandPath = filepath.Join(cfg.HomeDir, ".local", "bin", "domjudge-tool")
	m.targetPath = filepath.Join(cfg.StateDir, "target")
	if runtime.GOOS == "windows" {
		m.commandPath = filepath.Join(cfg.StateDir, "bin", "domjudge-tool.exe")
		m.targetPath += ".json"
	}
	if samePath(cfg.StateDir, cfg.HomeDir) || samePath(cfg.StateDir, filepath.Dir(m.commandPath)) ||
		samePath(cfg.ExecutablePath, m.commandPath) || samePath(cfg.ExecutablePath, m.targetPath) {
		return nil, errors.New("CLI installation paths overlap")
	}
	return m, nil
}

type bundleManifest struct {
	Schema   int    `json:"schema"`
	Version  string `json:"version"`
	Revision string `json:"revision"`
	GOOS     string `json:"goos"`
	GOARCH   string `json:"goarch"`
}

type targetRecord struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type receipt struct {
	Schema         int           `json:"schema"`
	CommandPath    string        `json:"command_path"`
	CommandID      string        `json:"command_id"`
	LauncherSHA256 string        `json:"launcher_sha256,omitempty"`
	Target         targetRecord  `json:"target"`
	Pending        *targetRecord `json:"pending,omitempty"`
}

type targetDocument struct {
	Path string `json:"path"`
}

type object struct {
	info os.FileInfo
	id   string
	data []byte
	link string
}

func (o object) exists() bool { return o.info != nil }

type inspection struct {
	status          Status
	receipt         *receipt
	command, target object
	receiptObject   object
	bundleOK        bool
	launcherPath    string
	launcherSHA256  string
	fingerprint     hash.Hash
}

// Inspect reads state without creating directories, acquiring locks, or running programs.
func (m *Manager) Inspect(ctx context.Context) (Status, error) {
	i, err := m.inspect(ctx)
	return i.status, err
}

func (m *Manager) inspect(ctx context.Context) (i *inspection, err error) {
	i = &inspection{fingerprint: sha256.New()}
	i.status.CommandPath = m.commandPath
	i.status.PathNote = "PATH is not changed. Add the command directory to your shell's PATH yourself, or use the exact command path. This app cannot check another terminal's PATH, aliases, or command cache."
	_, _ = fmt.Fprintln(i.fingerprint, m.config.ExecutablePath, m.config.HomeDir, m.config.StateDir) // hash.Hash.Write cannot fail.
	defer func() {
		if err != nil {
			i.status.State, i.status.Reason = "error", err.Error()
			i.status.CanInstall, i.status.CanRelink, i.status.CanUninstall = false, false, false
			_, _ = fmt.Fprintln(i.fingerprint, err) // hash.Hash.Write cannot fail.
		}
		i.status.Revision = hex.EncodeToString(i.fingerprint.Sum(nil))
	}()
	if err := ctx.Err(); err != nil {
		return i, err
	}
	if err := m.discover(ctx, i); err != nil {
		return i, err
	}
	for _, dir := range []string{m.config.StateDir, filepath.Dir(m.commandPath)} {
		entry, readErr := i.observe(ctx, dir, false)
		if readErr != nil {
			return i, readErr
		}
		if entry.exists() && (!entry.info.IsDir() || entry.info.Mode()&os.ModeSymlink != 0) {
			i.conflict("Integration directory is not an ordinary directory: " + dir)
			return i, nil
		}
	}
	lock, lockErr := observeObject(ctx, filepath.Join(m.config.StateDir, ".lock"), false)
	if lockErr != nil {
		return i, lockErr
	}
	if lock.exists() && !lock.info.Mode().IsRegular() {
		_, _ = fmt.Fprintln(i.fingerprint, lock.id, lock.info.Mode(), lock.link) // hash.Hash.Write cannot fail.
		i.conflict("The installation lock is not an ordinary file; it will not be followed or replaced.")
		return i, nil
	}
	i.command, err = i.observe(ctx, m.commandPath, false)
	if err != nil {
		return i, err
	}
	i.target, err = i.observe(ctx, m.targetPath, runtime.GOOS == "windows")
	if err != nil {
		return i, err
	}
	i.receiptObject, err = i.observe(ctx, m.receiptPath, true)
	if err != nil {
		return i, err
	}
	if !i.receiptObject.exists() {
		if i.command.exists() || i.target.exists() {
			i.conflict("An integration object exists without an ownership receipt; it will not be changed.")
		} else if i.bundleOK {
			i.status.State, i.status.Reason, i.status.CanInstall = "absent", "The bundled CLI is not installed for this user.", true
		}
		return i, nil
	}
	if !i.receiptObject.info.Mode().IsRegular() {
		i.conflict("The ownership receipt is not an ordinary file.")
		return i, nil
	}
	var r receipt
	if err = decodeDocument(i.receiptObject.data, &r); err != nil {
		return i, fmt.Errorf("reading CLI ownership receipt: %w", err)
	}
	if r.Schema != 1 || r.CommandID == "" || !samePath(r.CommandPath, m.commandPath) || !validTargetRecord(r.Target) ||
		(r.Pending != nil && !validTargetRecord(*r.Pending)) || (runtime.GOOS == "windows" && !validDigest(r.LauncherSHA256)) {
		return i, errors.New("CLI ownership receipt has invalid or mismatched fields")
	}
	i.receipt = &r
	if i.command.exists() {
		owned, checkErr := m.ownsCommand(ctx, i.command, r)
		if checkErr != nil {
			return i, checkErr
		}
		if !owned {
			i.conflict("The command was replaced or modified; the receipt does not authorize changing it.")
			return i, nil
		}
	}
	if i.target.exists() {
		path, owned := ownedTarget(i.target, r)
		if !owned {
			i.conflict("The private target was replaced or modified; it will not be changed.")
			return i, nil
		}
		i.status.TargetPath = path
	}
	i.status.CanUninstall = true
	if !i.bundleOK {
		return i, nil
	}
	i.status.CanRelink = true
	if !i.command.exists() || !i.target.exists() {
		i.status.State, i.status.Reason = "broken", "An owned integration object is missing. Relink repairs it; Uninstall removes the remaining owned objects."
		return i, nil
	}
	destination, readErr := i.observe(ctx, i.status.TargetPath, false)
	if readErr != nil {
		if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
			return i, readErr
		}
		// Source availability does not change verified integration ownership.
		_, _ = fmt.Fprintln(i.fingerprint, readErr) // hash.Hash.Write cannot fail.
		i.status.State = "broken"
		i.status.Reason = fmt.Sprintf("The installed CLI target cannot be inspected: %v. Relink to this app or uninstall the owned integration.", readErr)
		return i, nil
	}
	if !executableObject(destination) {
		i.status.State, i.status.Reason = "broken", "The installed CLI target is missing or is not an executable file. Move the app back or Relink to this app."
	} else if samePath(i.status.TargetPath, i.status.CLIPath) {
		i.status.State, i.status.Reason = "current", "The command points to this app's bundled CLI."
	} else {
		i.status.State, i.status.Reason = "other", "The command points to another app location. Relink explicitly switches it to this app."
	}
	return i, nil
}

func (i *inspection) conflict(reason string) {
	i.status.State, i.status.Reason = "conflict", reason
	i.status.CanInstall, i.status.CanRelink, i.status.CanUninstall = false, false, false
}

func (m *Manager) discover(ctx context.Context, i *inspection) error {
	i.status.State = "unavailable"
	i.status.Reason = "CLI integration requires a complete native desktop bundle."
	executable := m.config.ExecutablePath
	if runtime.GOOS == "darwin" && translocated(executable) {
		i.status.Reason = "App Translocation prevents a stable CLI link. Move the app to a permanent location and reopen it."
		return nil
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			i.status.Reason = "The desktop executable is missing."
			return nil
		}
		return fmt.Errorf("resolving desktop executable: %w", err)
	}
	if runtime.GOOS == "darwin" && translocated(resolved) {
		i.status.Reason = "App Translocation prevents a stable CLI link. Move the app to a permanent location and reopen it."
		return nil
	}
	base := filepath.Dir(resolved)
	root := base
	name := "domjudge-tool"
	switch runtime.GOOS {
	case "darwin":
		if filepath.Base(base) != "MacOS" || filepath.Base(filepath.Dir(base)) != "Contents" {
			return nil
		}
		root = filepath.Dir(filepath.Dir(base))
		base = filepath.Join(filepath.Dir(base), "Resources")
	case "windows":
		name += ".exe"
	}
	i.status.BundlePath, i.status.CLIPath = root, filepath.Join(base, "cli", name)
	for _, path := range []string{resolved, i.status.CLIPath} {
		entry, readErr := i.observe(ctx, path, false)
		if readErr != nil {
			return readErr
		}
		if !executableObject(entry) {
			i.status.Reason = "A required bundle executable is missing or is not executable: " + path
			return nil
		}
	}
	metadata, err := i.observe(ctx, filepath.Join(base, "bundle.json"), true)
	if err != nil {
		return err
	}
	if !metadata.exists() || !metadata.info.Mode().IsRegular() {
		i.status.Reason = "Bundle metadata is missing or is not an ordinary file."
		return nil
	}
	var manifest bundleManifest
	if err := decodeDocument(metadata.data, &manifest); err != nil || manifest.Schema != 1 ||
		manifest.Version == "" || manifest.Revision == "" || manifest.GOOS != runtime.GOOS || manifest.GOARCH != runtime.GOARCH {
		i.status.Reason = "Bundle metadata is corrupt, unsupported, or does not match this platform."
		return nil
	}
	i.status.Version = manifest.Version
	if runtime.GOOS == "windows" {
		i.launcherPath = filepath.Join(root, "resources", "cli-launcher.exe")
		launcher, readErr := i.observe(ctx, i.launcherPath, false)
		if readErr != nil {
			return readErr
		}
		if !executableObject(launcher) {
			i.status.Reason = "The bundled console launcher is missing or is not an ordinary file."
			return nil
		}
		i.launcherSHA256, err = fileDigest(ctx, i.launcherPath)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(i.fingerprint, i.launcherSHA256) // hash.Hash.Write cannot fail.
	}
	i.bundleOK = true
	return nil
}

func (i *inspection) observe(ctx context.Context, path string, contents bool) (object, error) {
	o, err := observeObject(ctx, path, contents)
	_, _ = fmt.Fprintln(i.fingerprint, path) // hash.Hash.Write cannot fail.
	if o.exists() {
		// Directory timestamps change when the lock is created, so they cannot be
		// part of the revision. File identities and contents capture owned state.
		if !o.info.IsDir() {
			_, _ = fmt.Fprintln(i.fingerprint, o.id, o.info.Mode(), o.info.Size(), o.info.ModTime().UnixNano(), o.link) // hash.Hash.Write cannot fail.
			_, _ = i.fingerprint.Write(o.data)                                                                          // hash.Hash.Write cannot fail.
		}
	}
	return o, err
}

func observeObject(ctx context.Context, path string, contents bool) (object, error) {
	var o object
	if err := ctx.Err(); err != nil {
		return o, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		return o, fmt.Errorf("inspecting %s: %w", path, err)
	}
	o.info = info
	o.id, err = objectID(path, info)
	if err != nil {
		return o, fmt.Errorf("identifying %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		o.link, err = os.Readlink(path)
	} else if contents && info.Mode().IsRegular() {
		var f *os.File
		f, err = os.Open(path)
		if err == nil {
			o.data, err = io.ReadAll(io.LimitReader(f, 65537))
			err = errors.Join(err, f.Close())
			if len(o.data) > 65536 {
				err = errors.Join(err, errors.New("state document exceeds 64 KiB"))
			}
		}
	}
	if err != nil {
		return o, fmt.Errorf("reading %s: %w", path, err)
	}
	return o, nil
}

func (m *Manager) ownsCommand(ctx context.Context, o object, r receipt) (bool, error) {
	if o.id != r.CommandID {
		return false, nil
	}
	if runtime.GOOS != "windows" {
		return o.info.Mode()&os.ModeSymlink != 0 && o.link == m.targetPath, nil
	}
	if !o.info.Mode().IsRegular() {
		return false, nil
	}
	digest, err := fileDigest(ctx, m.commandPath)
	return digest == r.LauncherSHA256, err
}

func ownedTarget(o object, r receipt) (string, bool) {
	for _, expected := range []*targetRecord{&r.Target, r.Pending} {
		if expected == nil || o.id != expected.ID {
			continue
		}
		if runtime.GOOS != "windows" {
			if o.info.Mode()&os.ModeSymlink != 0 && o.link == expected.Path {
				return expected.Path, true
			}
		} else if o.info.Mode().IsRegular() {
			path, err := decodeTarget(o.data)
			if err == nil && path == expected.Path {
				return path, true
			}
		}
	}
	return "", false
}

func validTargetRecord(r targetRecord) bool {
	return r.ID != "" && filepath.IsAbs(r.Path) && !strings.ContainsRune(r.Path, 0)
}

func validDigest(value string) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == sha256.Size
}

func executableObject(o object) bool {
	return o.exists() && o.info.Mode().IsRegular() && (runtime.GOOS == "windows" || o.info.Mode()&0o111 != 0)
}

func decodeDocument(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("expected a single JSON document")
	}
	return nil
}

func fileDigest(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}
	h := sha256.New()
	_, err = io.Copy(h, f)
	err = errors.Join(err, f.Close(), ctx.Err())
	if err != nil {
		return "", fmt.Errorf("hashing %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func translocated(path string) bool {
	return strings.Contains(strings.ToLower(filepath.ToSlash(path)), "/apptranslocation/")
}
