package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// AuthMode selects how the web UI authenticates browser requests.
type AuthMode string

// Supported authentication modes.
const (
	AuthToken AuthMode = "token"
	AuthBasic AuthMode = "basic"
	AuthNone  AuthMode = "none"
)

// MinTokenLength is the shortest accepted user-supplied access token.
const MinTokenLength = 16

// NewToken returns a random 256-bit access token encoded as hex.
func NewToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("server: generate token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// guard holds the request checks applied before any UI route.
type guard struct {
	auth  AuthMode
	token string
	// cookieValue is the token in base64url form: a custom --token may
	// contain bytes that http.SetCookie would strip from a raw value.
	cookieValue        string
	username, password string
	// allowedHosts is non-nil for loopback listeners; it lists the exact
	// Host header values accepted, which defeats DNS rebinding.
	allowedHosts map[string]bool
	cookieName   string
	secure       bool
}

func newGuard(opts Options, port int) *guard {
	g := &guard{
		auth:        opts.Auth,
		token:       opts.Token,
		cookieValue: base64.RawURLEncoding.EncodeToString([]byte(opts.Token)),
		username:    opts.Username,
		password:    opts.Password,
		cookieName:  "domjudge_tool_web_" + strconv.Itoa(port),
		secure:      opts.TLSCertFile != "",
	}
	if IsLoopbackHost(opts.Host) {
		p := strconv.Itoa(port)
		// Browsers omit the scheme's default port from the Host header.
		defaultPort := (port == 80 && !g.secure) || (port == 443 && g.secure)
		listen := strings.ToLower(strings.Trim(opts.Host, "[]"))
		g.allowedHosts = map[string]bool{}
		for _, h := range []string{"127.0.0.1", "localhost", "::1", listen} {
			g.allowedHosts[net.JoinHostPort(h, p)] = true
			if defaultPort {
				g.allowedHosts[strings.TrimSuffix(net.JoinHostPort(h, p), ":"+p)] = true
			}
		}
	}
	return g
}

// protect wraps next with security headers, the loopback Host check,
// authentication, and cross-origin request protection, in that order.
func (g *guard) protect(next http.Handler) http.Handler {
	csrf := http.NewCrossOriginProtection().Handler(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		// same-origin keeps the Origin header on form POSTs, which
		// no-referrer would replace with "null".
		h.Set("Referrer-Policy", "same-origin")
		if g.allowedHosts != nil && !g.allowedHosts[strings.ToLower(r.Host)] {
			http.Error(w, "Forbidden: unexpected Host header.", http.StatusForbidden)
			return
		}
		if !g.authenticate(w, r) {
			return
		}
		csrf.ServeHTTP(w, r)
	})
}

// authenticate reports whether the request may proceed. It writes the
// response itself when it returns false.
func (g *guard) authenticate(w http.ResponseWriter, r *http.Request) bool {
	switch g.auth {
	case AuthNone:
		return true
	case AuthBasic:
		user, pass, ok := r.BasicAuth()
		if ok && equal(user, g.username) && equal(pass, g.password) {
			return true
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="domjudge-tool", charset="UTF-8"`)
		http.Error(w, "Unauthorized.", http.StatusUnauthorized)
		return false
	default:
		return g.authenticateToken(w, r)
	}
}

func (g *guard) authenticateToken(w http.ResponseWriter, r *http.Request) bool {
	query := r.URL.Query()
	if query.Has("token") {
		if !equal(query.Get("token"), g.token) || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			g.unauthorized(w)
			return false
		}
		http.SetCookie(w, &http.Cookie{
			Name:     g.cookieName,
			Value:    g.cookieValue,
			Path:     "/",
			HttpOnly: true,
			Secure:   g.secure,
			SameSite: http.SameSiteStrictMode,
		})
		// Drop the token from the address bar and history.
		query.Del("token")
		target := *r.URL
		target.RawQuery = query.Encode()
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target.RequestURI(), http.StatusSeeOther)
		return false
	}
	if c, err := r.Cookie(g.cookieName); err == nil && equal(c.Value, g.cookieValue) {
		return true
	}
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && equal(bearer, g.token) {
		return true
	}
	g.unauthorized(w)
	return false
}

func (g *guard) unauthorized(w http.ResponseWriter) {
	http.Error(w, "Unauthorized. Open the URL printed by `domjudge-tool web` in your terminal.", http.StatusUnauthorized)
}

// equal compares secrets in constant time, independent of their lengths.
func equal(got, want string) bool {
	a := sha256.Sum256([]byte(got))
	b := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

// IsLoopbackHost reports whether host names only this machine:
// "localhost" or a loopback IP address.
func IsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
