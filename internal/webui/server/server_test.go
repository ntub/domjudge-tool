package server

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validOptions(t *testing.T) Options {
	t.Helper()
	return Options{Host: "127.0.0.1", Port: 0, Auth: AuthToken, Token: testToken, DBPath: filepath.Join(t.TempDir(), "web.db")}
}

func TestOptions_Validate(t *testing.T) {
	cases := []struct {
		name    string
		change  func(*Options)
		wantErr string
	}{
		{name: "valid token", change: func(*Options) {}},
		{name: "empty host", change: func(o *Options) { o.Host = " " }, wantErr: "--host"},
		{name: "port too large", change: func(o *Options) { o.Port = 65536 }, wantErr: "--port"},
		{name: "negative port", change: func(o *Options) { o.Port = -1 }, wantErr: "--port"},
		{name: "short token", change: func(o *Options) { o.Token = "short" }, wantErr: "--token"},
		{name: "basic without password", change: func(o *Options) { o.Auth, o.Username = AuthBasic, "admin" }, wantErr: "password"},
		{name: "basic colon user", change: func(o *Options) { o.Auth, o.Username, o.Password = AuthBasic, "a:b", "pw" }, wantErr: "--username"},
		{name: "basic valid", change: func(o *Options) { o.Auth, o.Username, o.Password = AuthBasic, "admin", "pw" }},
		{name: "none on loopback", change: func(o *Options) { o.Auth = AuthNone }},
		{name: "none on localhost", change: func(o *Options) { o.Auth, o.Host = AuthNone, "localhost" }},
		{name: "none on wildcard", change: func(o *Options) { o.Auth, o.Host = AuthNone, "0.0.0.0" }, wantErr: "loopback"},
		{name: "unknown auth", change: func(o *Options) { o.Auth = "oauth" }, wantErr: "--auth"},
		{name: "cert without key", change: func(o *Options) { o.TLSCertFile = "cert.pem" }, wantErr: "--tls-key"},
		{name: "key without cert", change: func(o *Options) { o.TLSKeyFile = "key.pem" }, wantErr: "--tls-cert"},
		{name: "no db", change: func(o *Options) { o.DBPath = "" }, wantErr: "--db"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := validOptions(t)
			tc.change(&o)
			err := o.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %v, want mention of %q", err, tc.wantErr)
			}
		})
	}
}

func TestServer_BrowserURL(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		tls  bool
		want string
		url  string
	}{
		{
			name: "loopback token", opts: Options{Host: "127.0.0.1", Auth: AuthToken, Token: testToken},
			url: "http://127.0.0.1:9900/", want: "http://127.0.0.1:9900/?token=" + testToken,
		},
		{
			name: "ipv4 wildcard", opts: Options{Host: "0.0.0.0", Auth: AuthBasic},
			url: "http://0.0.0.0:9900/", want: "http://127.0.0.1:9900/",
		},
		{
			name: "ipv6 wildcard", opts: Options{Host: "::", Auth: AuthBasic},
			url: "http://[::]:9900/", want: "http://[::1]:9900/",
		},
		{
			name: "bracketed ipv6 loopback", opts: Options{Host: "[::1]", Auth: AuthNone},
			url: "http://[::1]:9900/", want: "http://[::1]:9900/",
		},
		{
			name: "tls hostname", opts: Options{Host: "test-station.example.org", Auth: AuthBasic}, tls: true,
			url: "https://test-station.example.org:9900/", want: "https://test-station.example.org:9900/",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{opts: tc.opts, port: 9900}
			if tc.tls {
				s.tls = &tls.Config{MinVersion: tls.VersionTLS12}
			}
			if got := s.URL(); got != tc.url {
				t.Errorf("URL() = %q, want %q", got, tc.url)
			}
			if got := s.BrowserURL(); got != tc.want {
				t.Errorf("BrowserURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestServer_StartServeShutdown_Integration(t *testing.T) {
	opts := validOptions(t)
	srv, err := Start(opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	resp, err := client.Get(srv.BrowserURL())
	if err != nil {
		cancel()
		t.Fatalf("GET token URL: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Server Profiles") {
		t.Errorf("token login: status %d, body lacks Server Profiles", resp.StatusCode)
	}
	if resp.Request.URL.RawQuery != "" {
		t.Errorf("final URL still has a query: %s", resp.Request.URL)
	}
	anon, err := http.Get(srv.URL() + "static/webui.css")
	if err != nil {
		t.Fatalf("anonymous GET: %v", err)
	}
	_ = anon.Body.Close()
	if anon.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous static asset: status %d, want 401", anon.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v after cancellation", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not stop after cancellation")
	}
	if _, err := os.Stat(opts.DBPath); err != nil {
		t.Fatalf("database file not created: %v", err)
	}
	// The database handle is closed: the file can be removed and recreated.
	if err := os.Remove(opts.DBPath); err != nil {
		t.Fatalf("remove database after shutdown: %v", err)
	}
}

func TestStart_RejectsInvalidOptionsBeforeOpeningDB(t *testing.T) {
	opts := validOptions(t)
	opts.Auth = AuthNone
	opts.Host = "0.0.0.0"
	if _, err := Start(opts); err == nil {
		t.Fatal("Start accepted --auth none on a wildcard host")
	}
	if _, err := os.Stat(opts.DBPath); !os.IsNotExist(err) {
		t.Fatalf("database was created for invalid options: %v", err)
	}
}

func TestStart_PortInUse(t *testing.T) {
	first, err := Start(validOptions(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- first.Serve(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	opts := validOptions(t)
	opts.Port = first.port
	if _, err := Start(opts); err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("second Start on port %d: err %v, want listen error", first.port, err)
	}
}
