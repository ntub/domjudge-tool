//go:build desktop

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ntub/domjudge-tool/internal/desktop/cliinstall"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
)

// Native path discovery belongs at composition, never in HTTP or preview code.
func desktopCLIInstallOptions(host, version string) handler.CLIInstallOptions {
	opts := handler.CLIInstallOptions{Host: host, AppVersion: version}
	config, err := desktopCLIInstallConfig()
	if err != nil {
		opts.UnavailableReason = "CLI integration is unavailable: " + err.Error()
		return opts
	}
	manager, err := cliinstall.New(config)
	if err != nil {
		opts.UnavailableReason = "CLI integration is unavailable: " + err.Error()
		return opts
	}
	opts.Installer = manager
	return opts
}

func desktopCLIInstallConfig() (cliinstall.Config, error) {
	executable, err := os.Executable()
	if err != nil {
		return cliinstall.Config{}, fmt.Errorf("resolve desktop executable: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return cliinstall.Config{}, fmt.Errorf("resolve user home: %w", err)
	}
	var stateRoot string
	if runtime.GOOS == "windows" {
		stateRoot = os.Getenv("LOCALAPPDATA")
		if stateRoot == "" {
			return cliinstall.Config{}, fmt.Errorf("LOCALAPPDATA is not set")
		}
	} else {
		stateRoot, err = os.UserConfigDir()
		if err != nil {
			return cliinstall.Config{}, fmt.Errorf("resolve user configuration directory: %w", err)
		}
	}
	return cliinstall.Config{
		ExecutablePath: executable,
		HomeDir:        home,
		StateDir:       filepath.Join(stateRoot, "domjudge-tool", "cli-install"),
	}, nil
}
