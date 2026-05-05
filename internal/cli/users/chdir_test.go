package users_test

import (
	"os"
	"testing"
)

// chdir changes the test process's working directory and restores it
// at test cleanup. Go 1.24's t.Chdir is unavailable on the project's
// pinned go1.23 toolchain.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}
