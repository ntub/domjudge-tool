package handler

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// DesktopPickDirectory opens a native directory-picker dialog on the current
// platform. It satisfies DirectoryPickerFunc and is used by both cmd/desktop
// and cmd/e2e-server.
func DesktopPickDirectory(ctx context.Context) (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "osascript", "-e", "POSIX path of (choose folder)")
	case "linux":
		cmd = exec.CommandContext(ctx, "zenity", "--file-selection", "--directory")
	case "windows":
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
			"Add-Type -AssemblyName System.Windows.Forms; $f = New-Object System.Windows.Forms.FolderBrowserDialog; [void]$f.ShowDialog(); $f.SelectedPath")
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
