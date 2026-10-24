//go:build darwin || linux

package cliinstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func objectID(_ string, info os.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("filesystem did not provide an object identity")
	}
	return fmt.Sprintf("%x:%x", stat.Dev, stat.Ino), nil
}

func acquireLock(ctx context.Context, path string) (*os.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening CLI installation lock: %w", err)
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = errors.New("lock is not an ordinary file")
	}
	if err == nil {
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			err = ErrBusy
		}
	}
	if err != nil {
		return nil, errors.Join(fmt.Errorf("locking CLI installation: %w", err), f.Close())
	}
	return f, nil // Closing the descriptor releases flock, including after a crash.
}

func publishNoReplace(source, target string) error {
	// Linkat without AT_SYMLINK_FOLLOW links the symlink itself. Both staging
	// and destination live in one directory/filesystem; EEXIST never overwrites.
	if err := unix.Linkat(unix.AT_FDCWD, source, unix.AT_FDCWD, target, 0); err != nil {
		return err
	}
	return os.Remove(source)
}

func replacePrivate(source, target string) error {
	return os.Rename(source, target)
}
