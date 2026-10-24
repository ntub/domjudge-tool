// Package server composes the local web UI: the SQLite stores, the
// production operation functions, the security middleware, and the HTTP
// server started by `domjudge-tool web`.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
	"github.com/ntub/domjudge-tool/internal/webui/settings"
)

// Options configures one web UI server.
type Options struct {
	Host string
	// Port 0 picks a free port.
	Port               int
	Auth               AuthMode
	Token              string // required for AuthToken
	Username, Password string // required for AuthBasic
	TLSCertFile        string
	TLSKeyFile         string
	DBPath             string
}

// Validate rejects option combinations that are unsafe or incomplete.
func (o Options) Validate() error {
	if strings.TrimSpace(o.Host) == "" {
		return errors.New("--host must not be empty")
	}
	if o.Port < 0 || o.Port > 65535 {
		return fmt.Errorf("--port %d is outside 0-65535", o.Port)
	}
	switch o.Auth {
	case AuthToken:
		if len(o.Token) < MinTokenLength {
			return fmt.Errorf("--token must be at least %d characters", MinTokenLength)
		}
	case AuthBasic:
		if o.Username == "" || o.Password == "" {
			return errors.New("--auth basic needs --username and a password (--password or DOMJUDGE_TOOL_WEB_PASSWORD)")
		}
		if strings.Contains(o.Username, ":") {
			return errors.New("--username must not contain ':'")
		}
	case AuthNone:
		if !IsLoopbackHost(o.Host) {
			return errors.New("--auth none is only allowed with a loopback --host such as 127.0.0.1")
		}
	default:
		return fmt.Errorf("--auth %q is not one of token, basic, none", o.Auth)
	}
	if (o.TLSCertFile == "") != (o.TLSKeyFile == "") {
		return errors.New("--tls-cert and --tls-key must be given together")
	}
	if o.DBPath == "" {
		return errors.New("--db must not be empty")
	}
	return nil
}

// DefaultDBPath returns <user config dir>/domjudge-tool/domjudge-tool.db,
// creating the directory with owner-only permissions.
func DefaultDBPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("server: locate user config directory: %w", err)
	}
	dir = filepath.Join(dir, "domjudge-tool")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("server: create %s: %w", dir, err)
	}
	return filepath.Join(dir, "domjudge-tool.db"), nil
}

// Server is a bound, not yet serving, web UI.
type Server struct {
	opts     Options
	gdb      *gorm.DB
	listener net.Listener
	handler  http.Handler
	tls      *tls.Config
	port     int
}

// Start validates opts, opens the database, and binds the listener.
// Call Serve to accept requests; Serve releases every resource.
func Start(opts Options) (*Server, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	var tlsConfig *tls.Config
	if opts.TLSCertFile != "" {
		cert, err := tls.LoadX509KeyPair(opts.TLSCertFile, opts.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("server: load TLS key pair: %w", err)
		}
		tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}
	gdb, err := db.Open(opts.DBPath)
	if err != nil {
		return nil, err
	}
	s := &Server{opts: opts, gdb: gdb, tls: tlsConfig}
	ok := false
	defer func() {
		if !ok {
			_ = s.closeDB()
		}
	}()
	profiles, err := profile.NewStore(gdb)
	if err != nil {
		return nil, err
	}
	history, err := oplog.NewStore(gdb)
	if err != nil {
		return nil, err
	}
	appSettings, err := settings.NewStore(gdb)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(strings.Trim(opts.Host, "[]"), strconv.Itoa(opts.Port)))
	if err != nil {
		return nil, fmt.Errorf("server: listen: %w", err)
	}
	s.listener = listener
	s.port = listener.Addr().(*net.TCPAddr).Port
	s.handler = newGuard(opts, s.port).protect(newMux(profiles, history, appSettings))
	ok = true
	return s, nil
}

// URL is the server's base URL as reached through --host.
func (s *Server) URL() string {
	return s.urlFor(strings.Trim(s.opts.Host, "[]"))
}

// BrowserURL is the URL to open in a browser on this machine. A wildcard
// listener is reached through loopback; token authentication appends the
// one-time login query.
func (s *Server) BrowserURL() string {
	host := strings.Trim(s.opts.Host, "[]")
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
		if ip.To4() == nil {
			host = "::1"
		}
	}
	u := s.urlFor(host)
	if s.opts.Auth == AuthToken {
		u += "?" + url.Values{"token": {s.opts.Token}}.Encode()
	}
	return u
}

// Exposed reports whether the listener accepts connections from other
// machines.
func (s *Server) Exposed() bool {
	return !IsLoopbackHost(s.opts.Host)
}

// TLS reports whether the server uses HTTPS.
func (s *Server) TLS() bool {
	return s.tls != nil
}

func (s *Server) urlFor(host string) string {
	scheme := "http"
	if s.tls != nil {
		scheme = "https"
	}
	return scheme + "://" + net.JoinHostPort(host, strconv.Itoa(s.port)) + "/"
}

// Serve handles requests until ctx is cancelled, then shuts down. Request
// contexts derive from ctx, so long-running operations and event streams
// stop with the server. Serve always closes the listener and database.
func (s *Server) Serve(ctx context.Context) error {
	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		TLSConfig:         s.tls,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() {
		if s.tls != nil {
			errc <- srv.ServeTLS(s.listener, "", "")
			return
		}
		errc <- srv.Serve(s.listener)
	}()
	var serveErr error
	select {
	case serveErr = <-errc:
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
		}
		cancel()
		serveErr = <-errc
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(serveErr, s.closeDB())
}

func (s *Server) closeDB() error {
	sqlDB, err := s.gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
