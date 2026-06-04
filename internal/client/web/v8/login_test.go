package v8

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/httpx"
	"github.com/ntub/domjudge-tool/internal/client/web"
)

const (
	loginFixture       = "../../../../testdata/web/v8/login.html"
	landingFixture     = "../../../../testdata/web/v8/post_login_landing.html"
	problemsFixture    = "../../../../testdata/web/v8/problems.html"
	usersFixture       = "../../../../testdata/web/v8/users.html"
	teamsFixture       = "../../../../testdata/web/v8/teams.html"
	userConfirmFixture = "../../../../testdata/web/v8/user_delete_confirm.html"
	teamConfirmFixture = "../../../../testdata/web/v8/team_delete_confirm.html"
)

func mustReadFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return b
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	sc, err := httpx.NewSession(httpx.SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("httpx.NewSession: %v", err)
	}
	c, err := New(Options{
		Session:  sc,
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("v8.New: %v", err)
	}
	return c
}

func TestLogin_HappyPath_ExtractsCSRFAndPostsCreds(t *testing.T) {
	loginForm := mustReadFixture(t, loginFixture)
	landing := mustReadFixture(t, landingFixture)

	var (
		gotCSRF     string
		gotUsername string
		gotPassword string
		postSeen    atomic.Int32
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(loginForm)
		case http.MethodPost:
			postSeen.Add(1)
			_ = r.ParseForm()
			gotCSRF = r.PostForm.Get("_csrf_token")
			gotUsername = r.PostForm.Get("_username")
			gotPassword = r.PostForm.Get("_password")
			http.Redirect(w, r, "/jury", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(landing)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if postSeen.Load() != 1 {
		t.Errorf("POST /login hit %d times, want 1", postSeen.Load())
	}
	if gotCSRF != "TEST_CSRF_TOKEN" {
		t.Errorf("posted _csrf_token = %q, want TEST_CSRF_TOKEN", gotCSRF)
	}
	if gotUsername != "admin" {
		t.Errorf("posted _username = %q, want admin", gotUsername)
	}
	if gotPassword != "secret" {
		t.Errorf("posted _password = %q, want secret", gotPassword)
	}
}

func TestLogin_LandingNotJury_ReturnsErrLoginFailed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`<form><input name="_csrf_token" value="x"></form>`))
		case http.MethodPost:
			_, _ = w.Write([]byte(`<form><input name="_csrf_token" value="y"></form>`))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.Login(context.Background())
	if !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
}
