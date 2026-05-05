package v7

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

const (
	teamsAddFixture = "../../../../testdata/web/v7/jury_teams_add.html"
	teamViewFixture = "../../../../testdata/web/v7/jury_team_view.html"
)

func strPtr(s string) *string { return &s }

func TestCreateTeamAndUser_HappyPath(t *testing.T) {
	addForm := mustReadFixture(t, teamsAddFixture)
	teamView := mustReadFixture(t, teamViewFixture)

	var posted struct {
		name, displayName, category, affiliation, enabled, addUser, username string
		contestsKeySeen                                                      bool
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams/add", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write(addForm)
		case http.MethodPost:
			_ = r.ParseForm()
			posted.name = r.PostForm.Get("team[name]")
			posted.displayName = r.PostForm.Get("team[displayName]")
			posted.category = r.PostForm.Get("team[category]")
			posted.affiliation = r.PostForm.Get("team[affiliation]")
			posted.enabled = r.PostForm.Get("team[enabled]")
			posted.addUser = r.PostForm.Get("team[addUserForTeam]")
			posted.username = r.PostForm.Get("team[users][0][username]")
			_, posted.contestsKeySeen = r.PostForm["team[contests][]"]
			http.Redirect(w, r, "/jury/teams/99", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury/teams/99", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(teamView)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	user := model.CreateUser{Username: "alice", Name: "Alice Wonderland"}
	teamID, userID, err := c.CreateTeamAndUser(context.Background(), user, 7, 22, true)
	if err != nil {
		t.Fatalf("CreateTeamAndUser: %v", err)
	}
	if teamID != "99" {
		t.Errorf("teamID = %q, want 99", teamID)
	}
	if userID != "1" {
		// jury_team_view.html has the user link <a href="/jury/users/1">.
		t.Errorf("userID = %q, want 1", userID)
	}
	if posted.name != "alice" {
		t.Errorf("team[name] = %q, want alice", posted.name)
	}
	if posted.displayName != "Alice Wonderland" {
		t.Errorf("team[displayName] = %q, want Alice Wonderland", posted.displayName)
	}
	if posted.category != "7" {
		t.Errorf("team[category] = %q, want 7", posted.category)
	}
	if posted.affiliation != "22" {
		t.Errorf("team[affiliation] = %q, want 22", posted.affiliation)
	}
	if posted.enabled != "1" {
		t.Errorf("team[enabled] = %q, want 1", posted.enabled)
	}
	if posted.addUser != "1" {
		t.Errorf("team[addUserForTeam] = %q, want 1", posted.addUser)
	}
	if posted.username != "alice" {
		t.Errorf("team[users][0][username] = %q, want alice", posted.username)
	}
	// The team[contests][] select in our fixture has no selected option,
	// so the key MUST be dropped from the POST per Python v7.py:69.
	if posted.contestsKeySeen {
		t.Errorf("team[contests][] should be dropped when form yields nil; saw it in POST")
	}
}

func TestCreateTeamAndUser_DropsTeamContestsWhenNil(t *testing.T) {
	// Synthetic minimal form: only the team[contests][] select with no
	// selected option. Verifies the nilDrop logic in isolation.
	formHTML := `<html><body><form>
<input type="hidden" name="_token" value="t">
<input type="text" name="team[name]" value="">
<select name="team[contests][]"><option value="1">A</option></select>
</form></body></html>`

	var contestsKeySeen bool
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams/add", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(formHTML))
		case http.MethodPost:
			_ = r.ParseForm()
			_, contestsKeySeen = r.PostForm["team[contests][]"]
			http.Redirect(w, r, "/jury/teams/1", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury/teams/1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<div class="container-fluid"><a href="/jury/users/9">u</a></div>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.CreateTeamAndUser(context.Background(), model.CreateUser{Username: "x", Name: "x"}, 1, 1, true); err != nil {
		t.Fatalf("CreateTeamAndUser: %v", err)
	}
	if contestsKeySeen {
		t.Errorf("team[contests][] must be dropped when form yields nil; saw key in POST")
	}
}

func TestCreateTeamAndUser_StayedOnAdd_ReturnsErrCreateFailed(t *testing.T) {
	addForm := mustReadFixture(t, teamsAddFixture)
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams/add", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(addForm)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, _, err := c.CreateTeamAndUser(context.Background(), model.CreateUser{Username: "x", Name: "x"}, 1, 1, true)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrCreateFailed) {
		t.Errorf("errors.Is(err, web.ErrCreateFailed) = false; err = %v", err)
	}
}

func TestCreateTeamAndUser_TeamViewMissingUserLink_ReturnsErr(t *testing.T) {
	addForm := mustReadFixture(t, teamsAddFixture)
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams/add", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write(addForm)
		case http.MethodPost:
			http.Redirect(w, r, "/jury/teams/99", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury/teams/99", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1>no container-fluid here</h1></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, _, err := c.CreateTeamAndUser(context.Background(), model.CreateUser{Username: "x", Name: "x"}, 1, 1, true)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "user link") {
		t.Errorf("error %q does not mention 'user link'", err.Error())
	}
}

func TestCreateTeamAndUser_ContextCancelled(t *testing.T) {
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

	_, _, err := c.CreateTeamAndUser(ctx, model.CreateUser{Username: "x", Name: "x"}, 1, 1, true)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}

var _ = strPtr // exported for use by later tests
