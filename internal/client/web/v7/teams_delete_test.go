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

const teamsListFixture = "../../../../testdata/web/v7/jury_teams_list.html"

func TestDeleteTeams_HappyPath(t *testing.T) {
	listHTML := mustReadFixture(t, teamsListFixture)

	var deletePosts atomic.Int32
	var clarificationPosts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(listHTML)
	})
	mux.HandleFunc("/jury/teams/1/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("delete method = %s, want POST", r.Method)
		}
		deletePosts.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/jury/clarifications/send", func(w http.ResponseWriter, _ *http.Request) {
		// Should NOT be hit — that's a[-1], the clarification link, not delete.
		clarificationPosts.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	// Row 1's first anchor text is "297"; row 2's is "404".
	if err := c.DeleteTeams(context.Background(), []string{"297"}, nil); err != nil {
		t.Fatalf("DeleteTeams: %v", err)
	}
	if got := deletePosts.Load(); got != 1 {
		t.Errorf("delete POSTs = %d, want 1", got)
	}
	if got := clarificationPosts.Load(); got != 0 {
		t.Errorf("clarifications/send POSTed %d times — should be 0; this is the a[-2] vs a[-1] guard", got)
	}
}

// TestDeleteTeams_PostHrefIsSecondToLastAnchor explicitly locks the
// Python a[-2] selector at v7.py:192 against drift to a[-1]. Without
// this test the row-anchor offset could silently regress to last-anchor
// (which is the clarification link, not delete) and tests would still
// pass because no POST is posted to a non-existent /delete route.
func TestDeleteTeams_PostHrefIsSecondToLastAnchor(t *testing.T) {
	// Synthetic minimal fixture with three anchors: edit (a[-3]),
	// delete (a[-2]), clarification (a[-1]). DeleteTeams must POST
	// the middle one.
	html := `<html><body>
<table><tbody>
<tr>
  <td><a href="/jury/teams/1">team-x</a></td>
  <td><a href="/jury/teams/1/edit">edit</a></td>
  <td><a href="/jury/teams/1/delete">delete</a></td>
  <td><a href="/jury/clarifications/send?teamto=1">clarify</a></td>
</tr>
</tbody></table>
</body></html>`

	var deleteHit, editHit, clarifyHit atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	})
	mux.HandleFunc("/jury/teams/1/delete", func(w http.ResponseWriter, _ *http.Request) {
		deleteHit.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/jury/teams/1/edit", func(w http.ResponseWriter, _ *http.Request) {
		editHit.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/jury/clarifications/send", func(w http.ResponseWriter, _ *http.Request) {
		clarifyHit.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteTeams(context.Background(), []string{"team-x"}, nil); err != nil {
		t.Fatalf("DeleteTeams: %v", err)
	}
	if deleteHit.Load() != 1 {
		t.Errorf("delete = %d, want 1", deleteHit.Load())
	}
	if editHit.Load() != 0 {
		t.Errorf("edit POSTed %d times — DeleteTeams must use a[-2], not a[-3] (or earlier)", editHit.Load())
	}
	if clarifyHit.Load() != 0 {
		t.Errorf("clarification POSTed %d times — DeleteTeams must use a[-2], not a[-1]", clarifyHit.Load())
	}
}

func TestDeleteTeams_PythonInvertedFilter_LocksKnownQuirk(t *testing.T) {
	listHTML := mustReadFixture(t, teamsListFixture)

	var deletePosts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(listHTML)
	})
	mux.HandleFunc("/jury/teams/1/delete", func(w http.ResponseWriter, _ *http.Request) {
		deletePosts.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteTeams(context.Background(), nil, nil); err != nil {
		t.Fatalf("DeleteTeams: %v", err)
	}
	if got := deletePosts.Load(); got != 0 {
		t.Errorf("nil include posted %d delete(s), want 0 (Python quirk at v7.py:188)", got)
	}
}

func TestDeleteTeams_500FromList_WrapsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.DeleteTeams(context.Background(), []string{"x"}, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrServerError) {
		t.Errorf("errors.Is(err, web.ErrServerError) = false; err = %v", err)
	}
}
