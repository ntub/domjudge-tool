package v7

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
)

// TestUpdateTeam_PreservesTextareaContent locks the fix for Codex
// finding #3: a textarea with existing content (e.g. team[members]
// or team[comments]) must round-trip through GET + parse + POST so
// the value is not silently cleared by an UpdateTeam call that only
// changes the team's name.
func TestUpdateTeam_PreservesTextareaContent(t *testing.T) {
	formHTML := `<html><body><form>
<input type="hidden" name="_token" value="t">
<input type="text" name="team[name]" value="oldname">
<input type="text" name="team[displayName]" value="">
<select name="team[category]"><option value="3" selected="selected">x</option></select>
<select name="team[affiliation]"><option value="9" selected="selected">y</option></select>
<select name="team[enabled]"><option value="1" selected="selected">Yes</option></select>
<textarea name="team[members]">Alice
Bob
Charlie</textarea>
<textarea name="team[comments]">existing comment text</textarea>
</form></body></html>`

	var posted struct {
		members  string
		comments string
		name     string
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams/55/edit", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(formHTML))
		case http.MethodPost:
			_ = r.ParseForm()
			posted.members = r.PostForm.Get("team[members]")
			posted.comments = r.PostForm.Get("team[comments]")
			posted.name = r.PostForm.Get("team[name]")
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
	user := model.User{Username: "newname", Name: "New Display", TeamID: &teamID}
	if _, _, err := c.UpdateTeam(context.Background(), user, 3, 9, true); err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}
	// Override fields take effect.
	if posted.name != "newname" {
		t.Errorf("team[name] = %q, want newname", posted.name)
	}
	// Textarea content survives the round trip.
	if posted.members != "Alice\nBob\nCharlie" {
		t.Errorf("team[members] = %q, want preserved Alice/Bob/Charlie", posted.members)
	}
	if posted.comments != "existing comment text" {
		t.Errorf("team[comments] = %q, want preserved", posted.comments)
	}
}

// TestUpdateTeam_PreservesMultiSelectContestAssignments locks the fix
// for Codex finding #3: a multi-select that already has multiple
// options selected must preserve every selected value across the
// re-POST. The previous implementation took only the first selected
// option, silently breaking team-to-contest associations whenever
// any other team field was edited.
func TestUpdateTeam_PreservesMultiSelectContestAssignments(t *testing.T) {
	formHTML := `<html><body><form>
<input type="hidden" name="_token" value="t">
<input type="text" name="team[name]" value="oldname">
<select name="team[category]"><option value="3" selected="selected">x</option></select>
<select name="team[affiliation]"><option value="9" selected="selected">y</option></select>
<select name="team[enabled]"><option value="1" selected="selected">Yes</option></select>
<select name="team[contests][]" multiple="multiple">
  <option value="11">11</option>
  <option value="12" selected="selected">12</option>
  <option value="13" selected="selected">13</option>
  <option value="14">14</option>
  <option value="15" selected="selected">15</option>
</select>
</form></body></html>`

	var contests []string
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/teams/55/edit", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(formHTML))
		case http.MethodPost:
			_ = r.ParseForm()
			contests = append([]string(nil), r.PostForm["team[contests][]"]...)
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
	user := model.User{Username: "newname", Name: "New Display", TeamID: &teamID}
	if _, _, err := c.UpdateTeam(context.Background(), user, 3, 9, true); err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}
	want := []string{"12", "13", "15"}
	if !reflect.DeepEqual(contests, want) {
		t.Errorf("team[contests][] = %v, want %v", contests, want)
	}
}
