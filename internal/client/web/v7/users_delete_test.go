package v7

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

const usersListFixture = "../../../../testdata/web/v7/jury_users_list.html"

func TestDeleteUsers_HappyPath_DeletesIncludedNotExcluded(t *testing.T) {
	listHTML := mustReadFixture(t, usersListFixture)

	var deletePosts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(listHTML)
	})
	mux.HandleFunc("/jury/users/1/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("delete method = %s, want POST", r.Method)
		}
		deletePosts.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	// Include row 1 (student001) and row 2 (student002) but exclude
	// row 2 → only row 1 gets a POST.
	include := []string{"student001", "student002"}
	exclude := []string{"student002"}
	if err := c.DeleteUsers(context.Background(), include, exclude); err != nil {
		t.Fatalf("DeleteUsers: %v", err)
	}
	if got := deletePosts.Load(); got != 1 {
		t.Errorf("delete POSTs = %d, want 1", got)
	}
}

// TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk
//
// QUIRK: see docs/ROADMAP.md (Phase 1 quirks). Python `services/web/v7.py:161`
// short-circuits the filter as `lower_name not in include or lower_name
// in exclude`, which means an empty `include` set causes EVERY row to
// be skipped — DeleteUsers issues zero POSTs.
//
// The user base relies on this behaviour as an accidental safety net
// (calling DeleteUsers with no include argument is a no-op rather than
// "delete everyone"). Do NOT "fix" this without an ADR.
func TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk(t *testing.T) {
	listHTML := mustReadFixture(t, usersListFixture)

	var deletePosts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(listHTML)
	})
	mux.HandleFunc("/jury/users/1/delete", func(w http.ResponseWriter, _ *http.Request) {
		deletePosts.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	// Empty include → quirk: zero POSTs.
	if err := c.DeleteUsers(context.Background(), nil, nil); err != nil {
		t.Fatalf("DeleteUsers (empty include): %v", err)
	}
	if got := deletePosts.Load(); got != 0 {
		t.Errorf("nil include + nil exclude posted %d delete(s), want 0 (Python quirk)", got)
	}

	// Same with empty []string slices.
	deletePosts.Store(0)
	if err := c.DeleteUsers(context.Background(), []string{}, []string{}); err != nil {
		t.Fatalf("DeleteUsers (empty []string): %v", err)
	}
	if got := deletePosts.Load(); got != 0 {
		t.Errorf("empty slices posted %d delete(s), want 0 (Python quirk)", got)
	}
}

func TestDeleteUsers_NotFoundUsernamesIgnored(t *testing.T) {
	listHTML := mustReadFixture(t, usersListFixture)

	var deletePosts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(listHTML)
	})
	mux.HandleFunc("/jury/users/1/delete", func(w http.ResponseWriter, _ *http.Request) {
		deletePosts.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteUsers(context.Background(), []string{"no-such-user"}, nil); err != nil {
		t.Fatalf("DeleteUsers: %v", err)
	}
	if got := deletePosts.Load(); got != 0 {
		t.Errorf("delete POSTs = %d, want 0 (no match)", got)
	}
}

func TestDeleteUsers_POSTRedirectsToLogin_ReturnsErrLoginFailed(t *testing.T) {
	listHTML := `<html><body><table><tbody>
<tr><td><a href="/jury/users/1/delete">student001</a></td></tr>
</tbody></table></body></html>`

	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(listHTML))
	})
	mux.HandleFunc("/jury/users/1/delete", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<form><input name="_csrf_token" value="x"></form>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.DeleteUsers(context.Background(), []string{"student001"}, nil)
	if !errors.Is(err, web.ErrLoginFailed) {
		t.Errorf("errors.Is(err, web.ErrLoginFailed) = false; err = %v", err)
	}
}

func TestDeleteUsers_500FromList_WrapsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.DeleteUsers(context.Background(), []string{"a"}, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrServerError) {
		t.Errorf("errors.Is(err, web.ErrServerError) = false; err = %v", err)
	}
}
