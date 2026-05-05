package v7

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/httpx"
	"github.com/ntub/domjudge-tool/internal/client/web"
)

const (
	loginFormFixture     = "../../../../testdata/web/v7/login_form_initial.html"
	postLoginLandingHTML = "../../../../testdata/web/v7/post_login_landing.html"
)

func mustReadFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return b
}

// newTestClient constructs a v7.Client wired to srv with sensible defaults.
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
		t.Fatalf("v7.New: %v", err)
	}
	return c
}

func TestLogin_HappyPath_ExtractsCSRFAndPostsCreds(t *testing.T) {
	loginForm := mustReadFixture(t, loginFormFixture)
	landing := mustReadFixture(t, postLoginLandingHTML)

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

func TestLogin_AssertsLandingOnJury(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`<form><input name="_csrf_token" value="x"></form>`))
		case http.MethodPost:
			http.Redirect(w, r, "/jury", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	if err := c.Login(context.Background()); err != nil {
		t.Errorf("expected nil error on /jury landing, got %v", err)
	}
}

func TestLogin_LandingNotJury_ReturnsErrLoginFailed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`<form><input name="_csrf_token" value="x"></form>`))
		case http.MethodPost:
			// Bad creds: stay on /login.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<form><input name="_csrf_token" value="y"></form>`))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	err := c.Login(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
}

func TestLogin_FormGETFails_WrapsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	err := c.Login(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "v7: login") {
		t.Errorf("error %q does not start with v7 login wrapper", err.Error())
	}
}

func TestLogin_ContextCancelled(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-block:
		}
	}))
	defer srv.Close()
	defer close(block)

	c := newTestClient(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Login(ctx)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}

func TestNew_RequiresSession(t *testing.T) {
	if _, err := New(Options{Username: "u", Password: "p"}); err == nil {
		t.Errorf("expected error for nil Session, got nil")
	}
}

func TestNew_ContestIDSetsCookieJar(t *testing.T) {
	var seenCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sc, err := httpx.NewSession(httpx.SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	c, err := New(Options{
		Session:   sc,
		Username:  "u",
		Password:  "p",
		ContestID: "19",
	})
	if err != nil {
		t.Fatalf("v7.New: %v", err)
	}

	resp, err := c.Session().GET(context.Background(), "/anything")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if !strings.Contains(seenCookie, "domjudge_cid=19") {
		t.Errorf("expected domjudge_cid=19 in Cookie header %q", seenCookie)
	}
}
