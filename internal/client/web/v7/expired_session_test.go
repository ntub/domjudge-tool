package v7

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// loginRedirectMux returns an httptest server where every /jury/* path
// 302s to /login (session-expiry simulation) and /login serves a
// minimal login form. Used by the expired-session regression tests.
func loginRedirectMux(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<form action="/login" method="post">
<input type="hidden" name="_csrf_token" value="x">
<input type="text" name="_username">
<input type="password" name="_password">
</form>`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Any /jury/* path the v7 client tries to GET or POST is
		// redirected to /login, simulating a session-expiry redirect
		// loop.
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	return httptest.NewServer(mux)
}

// TestFetchDoc_RedirectsToLogin_ReturnsErrLoginFailed exercises the
// fetchDoc path used by every list/get op. Without the redirect-to-
// login guard, fetchDoc would parse the login HTML as if it were the
// requested page and return an empty result set silently.
func TestFetchDoc_RedirectsToLogin_ReturnsErrLoginFailed(t *testing.T) {
	srv := loginRedirectMux(t)
	defer srv.Close()
	c := newTestClient(t, srv)

	if _, err := c.GetAffiliations(context.Background()); !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("GetAffiliations: errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
	if _, err := c.GetProblems(context.Background(), web.ProblemFilter{}); !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("GetProblems: errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
	if err := c.DeleteUsers(context.Background(), []string{"someone"}, nil); !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("DeleteUsers: errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
	if err := c.DeleteTeams(context.Background(), []string{"someone"}, nil); !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("DeleteTeams: errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
}

// TestPostForm_GETRedirectsToLogin_ReturnsErrLoginFailed exercises
// the postForm path used by every write op. Without the guard, the
// caller would silently treat /login as a successful redirect and
// return ID="login" (or no error at all).
func TestPostForm_GETRedirectsToLogin_ReturnsErrLoginFailed(t *testing.T) {
	srv := loginRedirectMux(t)
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.CreateAffiliation(context.Background(), "x", "y", "z")
	if !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("CreateAffiliation: errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}

	if err := c.SetUserPassword(context.Background(), "1", "pw", []int{1}, true); !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("SetUserPassword: errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}

	teamID := "1"
	if _, _, err := c.CreateTeamAndUser(context.Background(), model.CreateUser{Username: "x", Name: "x"}, 1, 1, true); !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("CreateTeamAndUser: errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
	if _, _, err := c.UpdateTeam(context.Background(), model.User{Username: "x", Name: "x", TeamID: &teamID}, 1, 1, true); !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("UpdateTeam: errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
}

// TestPostForm_POSTRedirectsToLogin_ReturnsErrLoginFailed covers the
// case where the GET succeeds (form rendered) but the POST is the
// request that gets redirected to /login (e.g. the session expired
// between GET and POST). The naive "check final URL == /add path"
// guard lets this slip through.
func TestPostForm_POSTRedirectsToLogin_ReturnsErrLoginFailed(t *testing.T) {
	form := []byte(`<form action="" method="post">
<input type="hidden" name="_token" value="abc">
<input type="text" name="team_affiliation[shortname]">
<input type="text" name="team_affiliation[name]">
</form>`)
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/affiliations/add", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write(form)
		case http.MethodPost:
			http.Redirect(w, r, "/login", http.StatusFound)
		}
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<form><input name="_csrf_token" value="x"></form>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.CreateAffiliation(context.Background(), "x", "y", "z")
	if !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("CreateAffiliation: errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
}
