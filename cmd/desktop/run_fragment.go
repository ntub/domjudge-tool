//go:build desktop

package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/model"
)

// desktopFetchContests is the production ContestsFetchFunc.
func desktopFetchContests(ctx context.Context, p profile.ServerProfile) ([]model.Contest, error) {
	cfg := profileToConfig(p)
	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	return api.Contests.List(ctx)
}

// desktopFetchUsers is the production UsersFetchFunc.
func desktopFetchUsers(ctx context.Context, p profile.ServerProfile) ([]model.User, error) {
	cfg := profileToConfig(p)
	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	return api.Users.List(ctx, apiv4.UsersListOpts{})
}

// desktopFetchProblems is the production ProblemsFetchFunc.
func desktopFetchProblems(ctx context.Context, p profile.ServerProfile, contestID string) ([]model.Problem, error) {
	cfg := profileToConfig(p)
	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	return api.Contests.ListProblems(ctx, contestID)
}

// desktopPickDirectory triggers a native directory picker on different platforms.
func desktopPickDirectory(ctx context.Context) (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "osascript", "-e", "POSIX path of (choose folder)")
	case "linux":
		cmd = exec.CommandContext(ctx, "zenity", "--file-selection", "--directory")
	case "windows":
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
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
