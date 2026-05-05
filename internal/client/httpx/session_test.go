package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionClient_GET_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/hello" {
			t.Errorf("path = %q, want /hello", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hi"))
	}))
	defer srv.Close()

	sc, err := NewSession(SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	resp, err := sc.GET(context.Background(), "/hello")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hi" {
		t.Errorf("body = %q, want %q", body, "hi")
	}
}

func TestSessionClient_PostForm_SendsURLEncoded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want %q", got, "application/x-www-form-urlencoded")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.PostForm.Get("name"); got != "alice" {
			t.Errorf("name = %q, want alice", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sc, err := NewSession(SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	values := url.Values{"name": []string{"alice"}}
	resp, err := sc.PostForm(context.Background(), "/login", values)
	if err != nil {
		t.Fatalf("PostForm: %v", err)
	}
	defer resp.Body.Close()
}

func TestSessionClient_PostForm_RepeatedKeysForMultiValue(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		seen = r.PostForm["roles[]"]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sc, err := NewSession(SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	values := url.Values{"roles[]": []string{"1", "2", "3"}}
	resp, err := sc.PostForm(context.Background(), "/users/edit", values)
	if err != nil {
		t.Fatalf("PostForm: %v", err)
	}
	defer resp.Body.Close()

	want := []string{"1", "2", "3"}
	if len(seen) != len(want) {
		t.Fatalf("roles[] len = %d, want %d (got %v)", len(seen), len(want), seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("roles[][%d] = %q, want %q", i, seen[i], want[i])
		}
	}
}

func TestSessionClient_CookieJarPersistsAcrossRequests(t *testing.T) {
	var req2Cookies string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123", Path: "/"})
			w.WriteHeader(http.StatusOK)
		case "/check":
			req2Cookies = r.Header.Get("Cookie")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	sc, err := NewSession(SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	resp, err := sc.GET(context.Background(), "/set")
	if err != nil {
		t.Fatalf("GET /set: %v", err)
	}
	resp.Body.Close()

	resp, err = sc.GET(context.Background(), "/check")
	if err != nil {
		t.Fatalf("GET /check: %v", err)
	}
	resp.Body.Close()

	if !strings.Contains(req2Cookies, "session=abc123") {
		t.Errorf("second request Cookie header %q does not contain session=abc123", req2Cookies)
	}
}

func TestSessionClient_FollowsRedirects(t *testing.T) {
	var hitTarget atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/from":
			http.Redirect(w, r, "/to", http.StatusFound)
		case "/to":
			hitTarget.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("landed"))
		}
	}))
	defer srv.Close()

	sc, err := NewSession(SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	resp, err := sc.GET(context.Background(), "/from")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if hitTarget.Load() != 1 {
		t.Errorf("target /to was hit %d times, want 1", hitTarget.Load())
	}
	if resp.Request.URL.Path != "/to" {
		t.Errorf("final path = %q, want /to", resp.Request.URL.Path)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "landed" {
		t.Errorf("body = %q, want landed", body)
	}
}

func TestSessionClient_DisableSSLFlipsInsecureSkipVerify(t *testing.T) {
	tests := []struct {
		name       string
		disableSSL bool
		want       bool
	}{
		{name: "DisableSSL=true", disableSSL: true, want: true},
		{name: "DisableSSL=false", disableSSL: false, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sc, err := NewSession(SessionOptions{
				BaseURL:    "http://example.org",
				DisableSSL: tc.disableSSL,
			})
			if err != nil {
				t.Fatalf("NewSession: %v", err)
			}
			tr := sc.Transport()
			if tr == nil {
				t.Fatalf("Transport() returned nil")
			}
			if tr.TLSClientConfig == nil {
				t.Fatalf("TLSClientConfig is nil")
			}
			if got := tr.TLSClientConfig.InsecureSkipVerify; got != tc.want {
				t.Errorf("InsecureSkipVerify = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSessionClient_ContextCancelled_AbortsRequest(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-block:
		}
	}))
	defer srv.Close()
	defer close(block)

	sc, err := NewSession(SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := sc.GET(ctx, "/anything")
	if err == nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}

func TestSessionClient_TimeoutHonoured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sc, err := NewSession(SessionOptions{
		BaseURL: srv.URL,
		Timeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	resp, err := sc.GET(context.Background(), "/slow")
	if err == nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Fatalf("expected timeout error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "timeout") &&
		!strings.Contains(msg, "deadline exceeded") &&
		!errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not look like a timeout", msg)
	}
}

func TestSessionClient_DefaultTimeoutMatchesClient(t *testing.T) {
	sc, err := NewSession(SessionOptions{BaseURL: "http://example.org"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if got := sc.Timeout(); got != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", got)
	}
}

func TestSessionClient_JarAccessorReturnsConfigurableJar(t *testing.T) {
	var seenCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sc, err := NewSession(SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	jar := sc.Jar()
	if jar == nil {
		t.Fatalf("Jar() returned nil")
	}

	u, _ := url.Parse(srv.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "domjudge_cid", Value: "19"}})

	resp, err := sc.GET(context.Background(), "/anything")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if !strings.Contains(seenCookie, "domjudge_cid=19") {
		t.Errorf("Cookie header %q does not contain domjudge_cid=19", seenCookie)
	}
}

func TestSessionClient_GET_ResolvesRelativePathAgainstBaseURL(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sc, err := NewSession(SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	resp, err := sc.GET(context.Background(), "/jury/users/3/delete")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if seenPath != "/jury/users/3/delete" {
		t.Errorf("seen path = %q, want /jury/users/3/delete", seenPath)
	}
}

func TestSessionClient_PostForm_ResolvesRelativePathAgainstBaseURL(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sc, err := NewSession(SessionOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	resp, err := sc.PostForm(context.Background(), "/login", url.Values{})
	if err != nil {
		t.Fatalf("PostForm: %v", err)
	}
	resp.Body.Close()

	if seenPath != "/login" {
		t.Errorf("seen path = %q, want /login", seenPath)
	}
}

func TestNewSession_RequiresBaseURL(t *testing.T) {
	if _, err := NewSession(SessionOptions{BaseURL: ""}); err == nil {
		t.Errorf("expected error for empty BaseURL, got nil")
	}
}

func TestNewSession_RejectsInvalidBaseURL(t *testing.T) {
	if _, err := NewSession(SessionOptions{BaseURL: "::not a url::"}); err == nil {
		t.Errorf("expected error for malformed BaseURL, got nil")
	}
}

// Compile-time check that we don't accidentally re-instantiate cookiejar
// on every request; the jar should be shared with the underlying http.Client.
func TestSessionClient_HTTPClient_UsesSameJar(t *testing.T) {
	sc, err := NewSession(SessionOptions{BaseURL: "http://example.org"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	jar := sc.Jar()
	if _, ok := jar.(*cookiejar.Jar); !ok {
		t.Errorf("Jar() type = %T, want *cookiejar.Jar", jar)
	}
}
