package web

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startWeb runs the web command until the returned cancel is called and
// returns its "Open in your browser:" URL.
func startWeb(t *testing.T, args ...string) (string, func() error) {
	t.Helper()
	cmd := NewWebCmd()
	out, w := io.Pipe()
	cmd.SetOut(w)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(append([]string{"--db", filepath.Join(t.TempDir(), "web.db")}, args...))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		err := cmd.ExecuteContext(ctx)
		_ = w.Close()
		done <- err
	}()
	lines := make(chan string)
	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			if u, ok := strings.CutPrefix(scanner.Text(), "Open in your browser: "); ok {
				lines <- u
			}
		}
		close(lines)
	}()
	stop := func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(10 * time.Second):
			t.Fatal("web command did not stop")
			return nil
		}
	}
	select {
	case u, ok := <-lines:
		if !ok {
			t.Fatalf("command exited before printing its URL: %v", stop())
		}
		return u, stop
	case <-time.After(10 * time.Second):
		_ = stop()
		t.Fatal("no URL printed")
		return "", nil
	}
}

func TestWebCmd_ServesTokenProtectedUI_Integration(t *testing.T) {
	t.Setenv(EnvToken, "")
	u, stop := startWeb(t, "--no-browser", "--port", "0")
	if !strings.HasPrefix(u, "http://127.0.0.1:") || !strings.Contains(u, "/?token=") {
		t.Fatalf("unexpected browser URL %q", u)
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("Server Profiles")) {
		t.Errorf("status %d; body lacks Server Profiles", resp.StatusCode)
	}
	base, _, _ := strings.Cut(u, "?")
	anon, err := http.Get(base)
	if err != nil {
		t.Fatalf("anonymous GET: %v", err)
	}
	_ = anon.Body.Close()
	if anon.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous status %d, want 401", anon.StatusCode)
	}
	if err := stop(); err != nil {
		t.Fatalf("command returned %v after interruption", err)
	}
}

func TestWebCmd_TokenFromEnvironmentAndFlagPrecedence(t *testing.T) {
	const envToken, flagToken = "env-token-0123456789", "flag-token-0123456789"
	t.Setenv(EnvToken, envToken)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "environment", args: nil, want: envToken},
		{name: "flag wins", args: []string{"--token", flagToken}, want: flagToken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, stop := startWeb(t, append([]string{"--no-browser", "--port", "0"}, tc.args...)...)
			if !strings.HasSuffix(u, "?token="+tc.want) {
				t.Errorf("URL %q does not carry token %q", u, tc.want)
			}
			if err := stop(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWebCmd_OpensBrowserWithPrintedURL(t *testing.T) {
	opened := make(chan string, 1)
	orig := browserOpener
	browserOpener = func(_ context.Context, u string) error { opened <- u; return nil }
	t.Cleanup(func() { browserOpener = orig })

	u, stop := startWeb(t, "--port", "0", "--auth", "none")
	defer func() {
		if err := stop(); err != nil {
			t.Fatal(err)
		}
	}()
	select {
	case got := <-opened:
		if got != u {
			t.Fatalf("opened %q, printed %q", got, u)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("browser was not opened")
	}
}

func TestWebCmd_RejectsInvalidFlags(t *testing.T) {
	t.Setenv(EnvPassword, "")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "none on wildcard host", args: []string{"--auth", "none", "--host", "0.0.0.0"}, want: "loopback"},
		{name: "basic without password", args: []string{"--auth", "basic"}, want: "password"},
		{name: "unknown auth", args: []string{"--auth", "oauth"}, want: "--auth"},
		{name: "cert without key", args: []string{"--tls-cert", "cert.pem"}, want: "--tls-key"},
		{name: "short token", args: []string{"--token", "short"}, want: "--token"},
		{name: "missing key pair", args: []string{"--tls-cert", "missing.pem", "--tls-key", "missing.key"}, want: "TLS"},
		{name: "positional argument", args: []string{"extra"}, want: "unknown command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewWebCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(append([]string{"--no-browser", "--port", "0", "--db", filepath.Join(t.TempDir(), "web.db")}, tc.args...))
			err := cmd.ExecuteContext(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err %v, want mention of %q", err, tc.want)
			}
		})
	}
}

func TestWebCmd_BasicAuthPasswordFromEnvironment_Integration(t *testing.T) {
	t.Setenv(EnvPassword, "s3cret-password")
	u, stop := startWeb(t, "--no-browser", "--port", "0", "--auth", "basic", "--username", "jury")
	defer func() {
		if err := stop(); err != nil {
			t.Fatal(err)
		}
	}()
	if strings.Contains(u, "token") {
		t.Fatalf("basic auth URL %q carries a token", u)
	}
	for _, tc := range []struct {
		user, pass string
		want       int
	}{
		{"jury", "s3cret-password", http.StatusOK},
		{"jury", "wrong", http.StatusUnauthorized},
	} {
		req, _ := http.NewRequest(http.MethodGet, u+"profiles", nil)
		req.SetBasicAuth(tc.user, tc.pass)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("password %q: status %d, want %d", tc.pass, resp.StatusCode, tc.want)
		}
	}
}
