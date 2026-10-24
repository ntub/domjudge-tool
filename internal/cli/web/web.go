// Package web implements `domjudge-tool web`, which serves the local web
// UI and opens it in the default browser.
package web

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/webui/server"
)

// Environment variables read when the matching flag is not set.
const (
	EnvToken    = "DOMJUDGE_TOOL_WEB_TOKEN"
	EnvPassword = "DOMJUDGE_TOOL_WEB_PASSWORD"
)

// browserOpener opens url in the user's default browser. Tests replace it.
var browserOpener = openBrowser

// NewWebCmd builds the `web` subcommand.
func NewWebCmd() *cobra.Command {
	var (
		opts      server.Options
		auth      string
		noBrowser bool
	)
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Serve the local web UI and open it in your browser",
		Long: `Serve the domjudge-tool web UI until interrupted and open it in the
default browser. Server profiles, settings, and history are stored in --db.

By default the UI listens on 127.0.0.1 and requires the one-time token
URL printed on start. Use --auth basic with TLS before exposing it on a
network; --auth none is only accepted on a loopback --host.`,
		Example: `  domjudge-tool web
  domjudge-tool web --no-browser --port 0
  DOMJUDGE_TOOL_WEB_PASSWORD=change-me domjudge-tool web --host 0.0.0.0 \
    --auth basic --tls-cert cert.pem --tls-key key.pem`,
		GroupID: "general",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			opts.Auth = server.AuthMode(auth)
			if !c.Flags().Changed("token") {
				opts.Token = os.Getenv(EnvToken)
			}
			if !c.Flags().Changed("password") {
				opts.Password = os.Getenv(EnvPassword)
			}
			if opts.Auth == server.AuthToken && opts.Token == "" {
				token, err := server.NewToken()
				if err != nil {
					return err
				}
				opts.Token = token
			}
			if opts.DBPath == "" {
				path, err := server.DefaultDBPath()
				if err != nil {
					return err
				}
				opts.DBPath = path
			}
			srv, err := server.Start(opts)
			if err != nil {
				return fmt.Errorf("web: %w", err)
			}
			announce(c.OutOrStdout(), c.ErrOrStderr(), srv)
			if !noBrowser {
				if err := browserOpener(c.Context(), srv.BrowserURL()); err != nil {
					_, _ = fmt.Fprintf(c.ErrOrStderr(), "warning: could not open a browser (%v); open the URL above manually\n", err)
				}
			}
			if err := srv.Serve(c.Context()); err != nil {
				return fmt.Errorf("web: %w", err)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.Host, "host", "127.0.0.1", "listen address (IP or hostname)")
	f.IntVar(&opts.Port, "port", 9900, "listen port; 0 picks a free port")
	f.StringVar(&auth, "auth", string(server.AuthToken), "authentication: token, basic, or none (none requires a loopback --host)")
	f.StringVar(&opts.Token, "token", "", "fixed access token for --auth token, at least 16 characters (env "+EnvToken+"; default: random per run)")
	f.StringVar(&opts.Username, "username", "admin", "user name for --auth basic")
	f.StringVar(&opts.Password, "password", "", "password for --auth basic (env "+EnvPassword+")")
	f.StringVar(&opts.TLSCertFile, "tls-cert", "", "PEM certificate file; serves HTTPS together with --tls-key")
	f.StringVar(&opts.TLSKeyFile, "tls-key", "", "PEM private key file for --tls-cert")
	f.BoolVar(&noBrowser, "no-browser", false, "do not open a browser")
	f.StringVar(&opts.DBPath, "db", "", "SQLite file for profiles, settings, and history (default <user config dir>/domjudge-tool/domjudge-tool.db)")
	return cmd
}

// announce prints the stable start-up lines; "Open in your browser: " is a
// scripting contract. Write errors are ignored: the server still runs and
// the browser still opens.
func announce(stdout, stderr io.Writer, srv *server.Server) {
	_, _ = fmt.Fprintf(stdout, "domjudge-tool web is serving at %s\n", srv.URL())
	_, _ = fmt.Fprintf(stdout, "Open in your browser: %s\n", srv.BrowserURL())
	_, _ = fmt.Fprintln(stdout, "Press Ctrl+C to stop.")
	if srv.Exposed() && !srv.TLS() {
		_, _ = fmt.Fprintln(stderr, "warning: the web UI is reachable from other machines over plain HTTP; credentials and DOMjudge data travel unencrypted. Use --tls-cert and --tls-key.")
	}
}

func openBrowser(ctx context.Context, url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap the launcher without blocking the server; some launchers stay
	// attached to the browser they start.
	go func() { _ = cmd.Wait() }()
	return nil
}
