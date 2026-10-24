package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testToken = "0123456789abcdef0123456789abcdef"

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// sessionCookie performs the ?token= exchange and returns the issued cookie.
func sessionCookie(t *testing.T, h http.Handler, token string) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/?token="+url.QueryEscape(token), nil)
	req.Host = "127.0.0.1:9900"
	cookies := serve(h, req).Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("token exchange issued %d cookies, want 1", len(cookies))
	}
	return cookies[0]
}

func TestGuard_LoopbackHostCheck(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		port int
		host string
		want int
	}{
		{"127.0.0.1", Options{Host: "127.0.0.1"}, 9900, "127.0.0.1:9900", http.StatusOK},
		{"localhost", Options{Host: "127.0.0.1"}, 9900, "localhost:9900", http.StatusOK},
		{"upper-case localhost", Options{Host: "127.0.0.1"}, 9900, "LOCALHOST:9900", http.StatusOK},
		{"ipv6 loopback", Options{Host: "127.0.0.1"}, 9900, "[::1]:9900", http.StatusOK},
		{"rebinding name", Options{Host: "127.0.0.1"}, 9900, "attacker.example.org:9900", http.StatusForbidden},
		{"other port", Options{Host: "127.0.0.1"}, 9900, "127.0.0.1:9901", http.StatusForbidden},
		{"missing port", Options{Host: "127.0.0.1"}, 9900, "127.0.0.1", http.StatusForbidden},
		{"custom loopback listen host", Options{Host: "127.0.0.2"}, 9900, "127.0.0.2:9900", http.StatusOK},
		{"other loopback address", Options{Host: "127.0.0.2"}, 9900, "127.0.0.3:9900", http.StatusForbidden},
		{"http default port omitted", Options{Host: "127.0.0.1"}, 80, "127.0.0.1", http.StatusOK},
		{"http default port omitted ipv6", Options{Host: "::1"}, 80, "[::1]", http.StatusOK},
		{"https default port omitted", Options{Host: "localhost", TLSCertFile: "cert.pem", TLSKeyFile: "key.pem"}, 443, "localhost", http.StatusOK},
		{"port 443 without tls keeps port", Options{Host: "127.0.0.1"}, 443, "127.0.0.1", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Auth = AuthNone
			h := newGuard(tc.opts, tc.port).protect(okHandler())
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tc.host
			if got := serve(h, req).Code; got != tc.want {
				t.Fatalf("Host %q: status %d, want %d", tc.host, got, tc.want)
			}
		})
	}
}

func TestGuard_ExposedListenerAcceptsAnyHost(t *testing.T) {
	h := newGuard(Options{Host: "0.0.0.0", Auth: AuthBasic, Username: "admin", Password: "secret"}, 9900).protect(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "test-station.example.org:9900"
	req.SetBasicAuth("admin", "secret")
	if got := serve(h, req).Code; got != http.StatusOK {
		t.Fatalf("status %d, want 200", got)
	}
}

func TestGuard_TokenAuth(t *testing.T) {
	h := newGuard(Options{Host: "127.0.0.1", Auth: AuthToken, Token: testToken}, 9900).protect(okHandler())
	cookie := sessionCookie(t, h, testToken)
	cases := []struct {
		name   string
		method string
		target string
		prep   func(*http.Request)
		want   int
	}{
		{name: "no credentials", method: http.MethodGet, target: "/profiles", want: http.StatusUnauthorized},
		{name: "wrong token", method: http.MethodGet, target: "/?token=wrong", want: http.StatusUnauthorized},
		{name: "token on POST", method: http.MethodPost, target: "/?token=" + testToken, want: http.StatusUnauthorized},
		{name: "valid cookie", method: http.MethodGet, target: "/profiles", prep: func(r *http.Request) { r.AddCookie(cookie) }, want: http.StatusOK},
		{name: "cookie of another port", method: http.MethodGet, target: "/profiles", prep: func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "domjudge_tool_web_9901", Value: cookie.Value})
		}, want: http.StatusUnauthorized},
		{name: "wrong cookie", method: http.MethodGet, target: "/profiles", prep: func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: cookie.Name, Value: "wrong-" + cookie.Value})
		}, want: http.StatusUnauthorized},
		{name: "bearer", method: http.MethodGet, target: "/profiles", prep: func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+testToken)
		}, want: http.StatusOK},
		{name: "wrong bearer", method: http.MethodGet, target: "/profiles", prep: func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer wrong")
		}, want: http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			req.Host = "127.0.0.1:9900"
			if tc.prep != nil {
				tc.prep(req)
			}
			if got := serve(h, req).Code; got != tc.want {
				t.Fatalf("status %d, want %d", got, tc.want)
			}
		})
	}
}

func TestGuard_TokenExchangeSetsCookieAndStripsToken(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tls    bool
		secure bool
	}{
		{name: "http"},
		{name: "https", tls: true, secure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := Options{Host: "127.0.0.1", Auth: AuthToken, Token: testToken}
			if tc.tls {
				opts.TLSCertFile, opts.TLSKeyFile = "cert.pem", "key.pem"
			}
			h := newGuard(opts, 9900).protect(okHandler())
			req := httptest.NewRequest(http.MethodGet, "/history?type=import&token="+testToken, nil)
			req.Host = "127.0.0.1:9900"
			rec := serve(h, req)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status %d, want 303", rec.Code)
			}
			if loc := rec.Header().Get("Location"); loc != "/history?type=import" {
				t.Fatalf("Location %q keeps the token or drops other parameters", loc)
			}
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("got %d cookies, want 1", len(cookies))
			}
			c := cookies[0]
			if c.Name != "domjudge_tool_web_9900" || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Secure != tc.secure {
				t.Fatalf("unexpected session cookie %+v", c)
			}
		})
	}
}

func TestGuard_CustomTokenSurvivesCookieRoundTrip(t *testing.T) {
	for _, token := range []string{testToken, `0123456789abcdef;"\ ,é`} {
		t.Run(token, func(t *testing.T) {
			h := newGuard(Options{Host: "127.0.0.1", Auth: AuthToken, Token: token}, 9900).protect(okHandler())
			req := httptest.NewRequest(http.MethodGet, "/profiles", nil)
			req.Host = "127.0.0.1:9900"
			req.AddCookie(sessionCookie(t, h, token))
			if got := serve(h, req).Code; got != http.StatusOK {
				t.Fatalf("replayed session cookie: status %d, want 200", got)
			}
		})
	}
}

func TestGuard_BasicAuth(t *testing.T) {
	h := newGuard(Options{Host: "127.0.0.1", Auth: AuthBasic, Username: "admin", Password: "s3cret"}, 9900).protect(okHandler())
	cases := []struct {
		name       string
		user, pass string
		set        bool
		want       int
	}{
		{name: "missing", want: http.StatusUnauthorized},
		{name: "wrong password", user: "admin", pass: "nope", set: true, want: http.StatusUnauthorized},
		{name: "wrong user", user: "root", pass: "s3cret", set: true, want: http.StatusUnauthorized},
		{name: "valid", user: "admin", pass: "s3cret", set: true, want: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = "127.0.0.1:9900"
			if tc.set {
				req.SetBasicAuth(tc.user, tc.pass)
			}
			rec := serve(h, req)
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusUnauthorized && !strings.HasPrefix(rec.Header().Get("WWW-Authenticate"), "Basic ") {
				t.Fatal("401 lacks a Basic challenge")
			}
		})
	}
}

func TestGuard_CrossOriginWritesRejected(t *testing.T) {
	h := newGuard(Options{Host: "127.0.0.1", Auth: AuthToken, Token: testToken}, 9900).protect(okHandler())
	cookie := sessionCookie(t, h, testToken)
	cases := []struct {
		name   string
		header http.Header
		want   int
	}{
		{name: "cross-site fetch metadata", header: http.Header{"Sec-Fetch-Site": {"cross-site"}}, want: http.StatusForbidden},
		{name: "foreign origin", header: http.Header{"Origin": {"https://attacker.example.org"}}, want: http.StatusForbidden},
		{name: "same origin", header: http.Header{"Sec-Fetch-Site": {"same-origin"}}, want: http.StatusOK},
		{name: "non-browser client", header: http.Header{}, want: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(url.Values{"theme": {"dark"}}.Encode()))
			req.Host = "127.0.0.1:9900"
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(cookie)
			for k, v := range tc.header {
				req.Header[k] = v
			}
			if got := serve(h, req).Code; got != tc.want {
				t.Fatalf("status %d, want %d", got, tc.want)
			}
		})
	}
}

func TestGuard_SecurityHeadersOnEveryResponse(t *testing.T) {
	h := newGuard(Options{Host: "127.0.0.1", Auth: AuthToken, Token: testToken}, 9900).protect(okHandler())
	for _, host := range []string{"127.0.0.1:9900", "attacker.example.org:9900"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		rec := serve(h, req)
		for name, want := range map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
			"Content-Security-Policy": "frame-ancestors 'none'",
			"Referrer-Policy":         "same-origin",
		} {
			if got := rec.Header().Get(name); got != want {
				t.Errorf("Host %s: %s = %q, want %q", host, name, got, want)
			}
		}
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1": true, "127.0.0.2": true, "localhost": true, "::1": true, "[::1]": true,
		"0.0.0.0": false, "::": false, "192.0.2.1": false, "test-station.example.org": false,
	} {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
