// Command cli-launcher is the independent console entry point installed on Windows.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ntub/domjudge-tool/internal/desktop/cliinstall"
)

func main() {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "domjudge-tool launcher:", err)
		os.Exit(1)
	}
	code, err := cliinstall.RunLauncher(context.Background(), executable, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "domjudge-tool launcher:", err)
	}
	os.Exit(code)
}
