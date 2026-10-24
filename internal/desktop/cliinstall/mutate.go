package cliinstall

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// Install creates a command only when no integration objects already exist.
func (m *Manager) Install(ctx context.Context, revision string) (Status, error) {
	return m.mutate(ctx, revision, "install")
}

// Relink atomically switches the private target and repairs missing owned objects.
func (m *Manager) Relink(ctx context.Context, revision string) (Status, error) {
	return m.mutate(ctx, revision, "relink")
}

// Uninstall removes verified integration objects, never the bundled CLI or app.
func (m *Manager) Uninstall(ctx context.Context, revision string) (Status, error) {
	return m.mutate(ctx, revision, "uninstall")
}

func (m *Manager) mutate(ctx context.Context, revision, operation string) (status Status, err error) {
	i, err := m.inspect(ctx)
	if err != nil {
		return i.status, err
	}
	if revision == "" || revision != i.status.Revision {
		return i.status, ErrStaleRevision
	}
	if !allowed(i.status, operation) {
		return i.status, fmt.Errorf("cannot %s CLI integration: %s", operation, i.status.Reason)
	}
	if err := ensureDirectory(ctx, m.config.StateDir); err != nil {
		return mutationFailure(i.status, err)
	}
	lock, err := acquireLock(ctx, filepath.Join(m.config.StateDir, ".lock"))
	if err != nil {
		return mutationFailure(i.status, err)
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("releasing CLI installation lock: %w", closeErr))
			status, _ = mutationFailure(status, err)
		}
	}()
	// Never trust the pre-lock snapshot. Another app instance may have finished
	// an operation while this one was opening the persistent lock file.
	i, err = m.inspect(ctx)
	if err != nil {
		return i.status, err
	}
	if revision != i.status.Revision {
		return i.status, ErrStaleRevision
	}
	if !allowed(i.status, operation) {
		return i.status, fmt.Errorf("cannot %s CLI integration: %s", operation, i.status.Reason)
	}
	switch operation {
	case "install":
		err = m.install(ctx, i)
	case "relink":
		err = m.relink(ctx, i)
	case "uninstall":
		err = m.uninstall(ctx, i)
	}
	// Report real state even on partial I/O failure. A pending receipt retains
	// ownership of both sides of a private target replacement until committed.
	status, inspectErr := m.Inspect(ctx)
	err = errors.Join(err, inspectErr)
	if err != nil {
		return mutationFailure(status, err)
	}
	return status, nil
}

func allowed(s Status, operation string) bool {
	switch operation {
	case "install":
		return s.CanInstall
	case "relink":
		return s.CanRelink
	case "uninstall":
		return s.CanUninstall
	default:
		return false
	}
}

func mutationFailure(s Status, err error) (Status, error) {
	s.State, s.Reason = "error", err.Error()
	s.CanInstall, s.CanRelink, s.CanUninstall = false, false, false
	return s, err
}

func (m *Manager) install(ctx context.Context, i *inspection) (err error) {
	if err := ensureDirectory(ctx, filepath.Dir(m.commandPath)); err != nil {
		return err
	}
	targetTemp, target, err := m.stageTarget(ctx, i.status.CLIPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, removeTemporary(targetTemp)) }()
	commandTemp, commandID, err := m.stageCommand(ctx, i)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, removeTemporary(commandTemp)) }()
	r := receipt{Schema: 1, CommandPath: m.commandPath, CommandID: commandID, Target: target, LauncherSHA256: i.launcherSHA256}
	if err := m.saveReceipt(ctx, r, false); err != nil {
		return err
	}
	if err = publishNoReplace(targetTemp, m.targetPath); err != nil {
		return fmt.Errorf("creating private CLI target: %w", err)
	}
	if err = publishNoReplace(commandTemp, m.commandPath); err != nil {
		return fmt.Errorf("creating CLI command without replacement: %w", err)
	}
	return nil
}

func (m *Manager) relink(ctx context.Context, i *inspection) (err error) {
	targetTemp, target, err := m.stageTarget(ctx, i.status.CLIPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, removeTemporary(targetTemp)) }()
	r := *i.receipt
	if i.target.exists() && r.Pending != nil && i.target.id == r.Pending.ID {
		r.Target = *r.Pending
	}
	r.Pending = &target
	var commandTemp string
	if !i.command.exists() {
		if err := ensureDirectory(ctx, filepath.Dir(m.commandPath)); err != nil {
			return err
		}
		commandTemp, r.CommandID, err = m.stageCommand(ctx, i)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, removeTemporary(commandTemp)) }()
		r.LauncherSHA256 = i.launcherSHA256
	}
	if err := m.saveReceipt(ctx, r, true); err != nil {
		return err
	}
	if i.target.exists() {
		err = replacePrivate(targetTemp, m.targetPath)
	} else {
		err = publishNoReplace(targetTemp, m.targetPath)
	}
	if err != nil {
		return fmt.Errorf("replacing private CLI target: %w", err)
	}
	if commandTemp != "" {
		if err = publishNoReplace(commandTemp, m.commandPath); err != nil {
			return fmt.Errorf("repairing CLI command without replacement: %w", err)
		}
	}
	r.Target, r.Pending = target, nil
	return m.saveReceipt(ctx, r, true)
}

func (m *Manager) uninstall(ctx context.Context, i *inspection) error {
	if i.command.exists() {
		owned, err := m.ownsCommand(ctx, i.command, *i.receipt)
		if err != nil {
			return err
		}
		if !owned {
			return errors.New("command ownership changed before removal")
		}
		if err := removeChecked(ctx, m.commandPath, i.command); err != nil {
			return err
		}
	}
	if i.target.exists() {
		if err := removeChecked(ctx, m.targetPath, i.target); err != nil {
			return err
		}
	}
	// Keep the lock inode and directories: deleting them could let a concurrent
	// process lock a different inode while this process still holds the old one.
	return removeChecked(ctx, m.receiptPath, i.receiptObject)
}

func (m *Manager) stageTarget(ctx context.Context, cliPath string) (string, targetRecord, error) {
	path := filepath.Join(m.config.StateDir, ".target-"+rand.Text())
	if err := ctx.Err(); err != nil {
		return "", targetRecord{}, err
	}
	var err error
	if runtime.GOOS == "windows" {
		var data []byte
		data, err = json.Marshal(targetDocument{Path: cliPath})
		if err == nil {
			err = writeNewFile(ctx, path, data, 0o600)
		}
	} else {
		err = os.Symlink(cliPath, path)
	}
	if err != nil {
		return "", targetRecord{}, fmt.Errorf("preparing private CLI target: %w", err)
	}
	entry, err := observeObject(ctx, path, false)
	if err != nil {
		return "", targetRecord{}, errors.Join(err, removeTemporary(path))
	}
	return path, targetRecord{ID: entry.id, Path: cliPath}, nil
}

func (m *Manager) stageCommand(ctx context.Context, i *inspection) (path, identity string, err error) {
	path = filepath.Join(filepath.Dir(m.commandPath), ".domjudge-tool-"+rand.Text())
	if runtime.GOOS == "windows" {
		if err = copyNewFile(ctx, i.launcherPath, path); err != nil {
			return "", "", fmt.Errorf("copying bundled launcher: %w", err)
		}
		var digest string
		digest, err = fileDigest(ctx, path)
		if err == nil && digest != i.launcherSHA256 {
			err = errors.New("bundled launcher changed while copying it")
		}
		if err != nil {
			return "", "", errors.Join(err, removeTemporary(path))
		}
	} else {
		err = os.Symlink(m.targetPath, path)
	}
	if err != nil {
		return "", "", fmt.Errorf("preparing CLI command: %w", err)
	}
	entry, err := observeObject(ctx, path, false)
	if err != nil {
		return "", "", errors.Join(err, removeTemporary(path))
	}
	return path, entry.id, nil
}

func (m *Manager) saveReceipt(ctx context.Context, r receipt, replace bool) (err error) {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encoding CLI ownership receipt: %w", err)
	}
	path := filepath.Join(m.config.StateDir, ".receipt-"+rand.Text())
	if err = writeNewFile(ctx, path, data, 0o600); err != nil {
		return fmt.Errorf("writing CLI ownership receipt: %w", err)
	}
	defer func() { err = errors.Join(err, removeTemporary(path)) }()
	if replace {
		err = replacePrivate(path, m.receiptPath)
	} else {
		err = publishNoReplace(path, m.receiptPath)
	}
	if err != nil {
		return fmt.Errorf("publishing CLI ownership receipt: %w", err)
	}
	return nil
}

func ensureDirectory(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("creating integration directory %s: %w", path, err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("checking integration directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("integration directory is not an ordinary directory: %s", path)
	}
	return nil
}

func removeChecked(ctx context.Context, path string, expected object) error {
	actual, err := observeObject(ctx, path, expected.data != nil)
	if err != nil {
		return err
	}
	if !actual.exists() {
		return fmt.Errorf("integration object disappeared before removal: %s", path)
	}
	if actual.id != expected.id || actual.info.Mode() != expected.info.Mode() ||
		actual.info.Size() != expected.info.Size() || !actual.info.ModTime().Equal(expected.info.ModTime()) ||
		actual.link != expected.link || !bytes.Equal(actual.data, expected.data) {
		return fmt.Errorf("integration object changed before removal: %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing owned integration object %s: %w", path, err)
	}
	return nil
}

func removeTemporary(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing temporary integration file %s: %w", path, err)
	}
	return nil
}

func writeNewFile(ctx context.Context, path string, data []byte, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return errors.Join(err, removeTemporary(path))
	}
	return nil
}

func copyNewFile(ctx context.Context, source, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return errors.Join(err, src.Close())
	}
	_, err = io.Copy(dst, src)
	if err == nil {
		err = dst.Sync()
	}
	err = errors.Join(err, src.Close(), dst.Close(), ctx.Err())
	if err != nil {
		return errors.Join(err, removeTemporary(target))
	}
	return nil
}
