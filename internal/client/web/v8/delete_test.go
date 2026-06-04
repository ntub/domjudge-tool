package v8

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDeleteUsers_GetsConfirmAndPostsDeleteForm(t *testing.T) {
	usersHTML := mustReadFixture(t, usersFixture)
	confirmHTML := mustReadFixture(t, userConfirmFixture)

	var confirmGets, deletePosts atomic.Int32
	var postedCSRF string
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(usersHTML)
	})
	mux.HandleFunc("/jury/users/11/delete", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			confirmGets.Add(1)
			_, _ = w.Write(confirmHTML)
		case http.MethodPost:
			deletePosts.Add(1)
			_ = r.ParseForm()
			postedCSRF = r.PostForm.Get("_csrf_token")
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("delete method = %s, want GET or POST", r.Method)
		}
	})
	mux.HandleFunc("/jury/users/22/delete", func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("excluded user delete route should not be hit")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteUsers(context.Background(), []string{"anne26a", "cindy950093"}, []string{"cindy950093"}); err != nil {
		t.Fatalf("DeleteUsers: %v", err)
	}
	if confirmGets.Load() != 1 {
		t.Errorf("confirm GETs = %d, want 1", confirmGets.Load())
	}
	if deletePosts.Load() != 1 {
		t.Errorf("delete POSTs = %d, want 1", deletePosts.Load())
	}
	if postedCSRF != "" {
		t.Errorf("posted _csrf_token = %q, want empty because v8 confirm fixture has no hidden CSRF input", postedCSRF)
	}
}

func TestDeleteTeams_GetsConfirmAndPostsSecondToLastAnchor(t *testing.T) {
	teamsHTML := mustReadFixture(t, teamsFixture)
	confirmHTML := mustReadFixture(t, teamConfirmFixture)

	var confirmGets, deletePosts, clarificationPosts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(teamsHTML)
	})
	mux.HandleFunc("/jury/teams/10/delete", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			confirmGets.Add(1)
			_, _ = w.Write(confirmHTML)
		case http.MethodPost:
			deletePosts.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("delete method = %s, want GET or POST", r.Method)
		}
	})
	mux.HandleFunc("/jury/clarifications/send", func(w http.ResponseWriter, _ *http.Request) {
		clarificationPosts.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteTeams(context.Background(), []string{"10"}, nil); err != nil {
		t.Fatalf("DeleteTeams: %v", err)
	}
	if confirmGets.Load() != 1 {
		t.Errorf("confirm GETs = %d, want 1", confirmGets.Load())
	}
	if deletePosts.Load() != 1 {
		t.Errorf("delete POSTs = %d, want 1", deletePosts.Load())
	}
	if clarificationPosts.Load() != 0 {
		t.Errorf("clarification POSTs = %d, want 0", clarificationPosts.Load())
	}
}

func TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk(t *testing.T) {
	usersHTML := mustReadFixture(t, usersFixture)

	var deleteHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(usersHTML)
	})
	mux.HandleFunc("/jury/users/11/delete", func(w http.ResponseWriter, _ *http.Request) {
		deleteHits.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteUsers(context.Background(), nil, nil); err != nil {
		t.Fatalf("DeleteUsers nil include: %v", err)
	}
	if deleteHits.Load() != 0 {
		t.Errorf("nil include posted %d delete(s), want 0 (Python quirk)", deleteHits.Load())
	}
	if err := c.DeleteUsers(context.Background(), []string{}, []string{}); err != nil {
		t.Fatalf("DeleteUsers empty include: %v", err)
	}
	if deleteHits.Load() != 0 {
		t.Errorf("empty include posted %d delete(s), want 0 (Python quirk)", deleteHits.Load())
	}
}

func TestDeleteTeams_PythonInvertedFilter_LocksKnownQuirk(t *testing.T) {
	teamsHTML := mustReadFixture(t, teamsFixture)

	var deleteHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(teamsHTML)
	})
	mux.HandleFunc("/jury/teams/10/delete", func(w http.ResponseWriter, _ *http.Request) {
		deleteHits.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteTeams(context.Background(), nil, nil); err != nil {
		t.Fatalf("DeleteTeams: %v", err)
	}
	if deleteHits.Load() != 0 {
		t.Errorf("nil include posted %d delete(s), want 0 (Python quirk)", deleteHits.Load())
	}
}
