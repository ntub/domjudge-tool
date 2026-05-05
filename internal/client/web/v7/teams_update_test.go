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

const teamEditFixture = "../../../../testdata/web/v7/jury_team_edit.html"

func TestUpdateTeam_HappyPath(t *testing.T) {
	editForm := mustReadFixture(t, teamEditFixture)
	teamView := mustReadFixture(t, teamViewFixture)

	var posted struct {
		name, displayName, category, affiliation, enabled string
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams/55/edit", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write(editForm)
		case http.MethodPost:
			_ = r.ParseForm()
			posted.name = r.PostForm.Get("team[name]")
			posted.displayName = r.PostForm.Get("team[displayName]")
			posted.category = r.PostForm.Get("team[category]")
			posted.affiliation = r.PostForm.Get("team[affiliation]")
			posted.enabled = r.PostForm.Get("team[enabled]")
			http.Redirect(w, r, "/jury/teams/55", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury/teams/55", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(teamView)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	teamID := "55"
	user := model.User{
		Username: "bob",
		Name:     "Bob Builder",
		TeamID:   &teamID,
	}
	gotTeam, gotUser, err := c.UpdateTeam(context.Background(), user, 3, 9, false)
	if err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}
	if gotTeam != "55" {
		t.Errorf("teamID = %q, want 55", gotTeam)
	}
	if gotUser != "1" {
		t.Errorf("userID = %q, want 1", gotUser)
	}
	if posted.name != "bob" {
		t.Errorf("team[name] = %q, want bob", posted.name)
	}
	if posted.displayName != "Bob Builder" {
		t.Errorf("team[displayName] = %q, want Bob Builder", posted.displayName)
	}
	if posted.category != "3" {
		t.Errorf("team[category] = %q, want 3", posted.category)
	}
	if posted.affiliation != "9" {
		t.Errorf("team[affiliation] = %q, want 9", posted.affiliation)
	}
	if posted.enabled != "0" {
		t.Errorf("team[enabled] = %q, want 0 (disabled)", posted.enabled)
	}
}

func TestUpdateTeam_StayedOnEdit_ReturnsErr(t *testing.T) {
	editForm := mustReadFixture(t, teamEditFixture)
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams/55/edit", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(editForm)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	teamID := "55"
	user := model.User{Username: "x", Name: "x", TeamID: &teamID}
	_, _, err := c.UpdateTeam(context.Background(), user, 1, 1, true)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrUpdateFailed) {
		t.Errorf("errors.Is(err, web.ErrUpdateFailed) = false; err = %v", err)
	}
}

func TestUpdateTeam_TeamContestsNil_Dropped(t *testing.T) {
	formHTML := `<html><body><form>
<input type="hidden" name="_token" value="t">
<input type="text" name="team[name]" value="">
<select name="team[contests][]"><option value="1">A</option></select>
</form></body></html>`

	var contestsKeySeen bool
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams/55/edit", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(formHTML))
		case http.MethodPost:
			_ = r.ParseForm()
			_, contestsKeySeen = r.PostForm["team[contests][]"]
			http.Redirect(w, r, "/jury/teams/55", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury/teams/55", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<div class="container-fluid"><a href="/jury/users/9">u</a></div>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	teamID := "55"
	user := model.User{Username: "x", Name: "x", TeamID: &teamID}
	if _, _, err := c.UpdateTeam(context.Background(), user, 1, 1, true); err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}
	if contestsKeySeen {
		t.Errorf("team[contests][] must be dropped when nil; saw key in POST")
	}
}

func TestUpdateTeam_NilTeamID_ReturnsErr(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.NotFoundHandler()))
	user := model.User{Username: "x", Name: "x", TeamID: nil}
	_, _, err := c.UpdateTeam(context.Background(), user, 1, 1, true)
	if err == nil {
		t.Errorf("expected error for nil TeamID, got nil")
	}
}
