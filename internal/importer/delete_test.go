package importer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/model"
)

// strPtr is a small helper to create *string literals inside test
// tables without inflating each entry.
func strPtr(s string) *string { return &s }

// userListServer spins up an httptest.Server that answers
// GET /api/v4/users with the supplied snapshot, and returns a
// pre-wired apiv4.UsersResource pointing at it. The server is
// registered with t.Cleanup so the test does not need to track it.
func userListServer(t *testing.T, snapshot []model.User) *apiv4.UsersResource {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/users" {
			t.Errorf("unexpected api path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(snapshot)
	}))
	t.Cleanup(srv.Close)
	c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})
	return c.Users
}

// sortedCopy returns a sorted copy of s; tests use it for set-equality
// comparisons that don't care about order.
func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func TestDeleteTeamsAndUsers_DefaultProtectsAdmin(t *testing.T) {
	cfg := &config.DomServerClient{Username: "configuser"}
	users := userListServer(t, []model.User{
		{ID: "1", Username: "admin", TeamID: strPtr("100")},
		{ID: "2", Username: "judgehost", TeamID: strPtr("101")},
		{ID: "3", Username: "configuser", TeamID: strPtr("102")},
		{ID: "4", Username: "alice", TeamID: strPtr("103")},
	})
	gw := &mockGateway{}

	err := importer.DeleteTeamsAndUsers(context.Background(), gw, users, cfg, importer.DeleteOptions{
		Include: []string{"admin", "judgehost", "configuser", "alice"},
	})
	if err != nil {
		t.Fatalf("DeleteTeamsAndUsers: %v", err)
	}

	for _, name := range []string{"admin", "judgehost", "configuser"} {
		for _, got := range gw.deleteUsersIncludeCap {
			if got == name {
				t.Errorf("protected user %q leaked into DeleteUsers include", name)
			}
		}
		for _, got := range gw.deleteUsersExcludeCap {
			if got == name {
				t.Errorf("protected user %q leaked into DeleteUsers exclude", name)
			}
		}
	}

	if !reflect.DeepEqual(gw.deleteUsersIncludeCap, []string{"alice"}) {
		t.Errorf("DeleteUsers include = %v, want [alice]", gw.deleteUsersIncludeCap)
	}
}

func TestDeleteTeamsAndUsers_IncludeExclude(t *testing.T) {
	cfg := &config.DomServerClient{Username: "configuser"}
	users := userListServer(t, []model.User{
		{ID: "1", Username: "alice", TeamID: strPtr("11")},
		{ID: "2", Username: "bob", TeamID: strPtr("22")},
		{ID: "3", Username: "carol", TeamID: strPtr("33")},
	})
	gw := &mockGateway{}

	err := importer.DeleteTeamsAndUsers(context.Background(), gw, users, cfg, importer.DeleteOptions{
		Include: []string{"alice", "bob"},
		Exclude: []string{"bob"},
	})
	if err != nil {
		t.Fatalf("DeleteTeamsAndUsers: %v", err)
	}

	if !reflect.DeepEqual(gw.deleteUsersIncludeCap, []string{"alice"}) {
		t.Errorf("DeleteUsers include = %v, want [alice]", gw.deleteUsersIncludeCap)
	}
}

func TestDeleteTeamsAndUsers_EmptyInclude_DeletesAll(t *testing.T) {
	cfg := &config.DomServerClient{Username: "configuser"}
	users := userListServer(t, []model.User{
		{ID: "1", Username: "admin", TeamID: strPtr("100")},
		{ID: "2", Username: "alice", TeamID: strPtr("101")},
		{ID: "3", Username: "bob", TeamID: strPtr("102")},
	})
	gw := &mockGateway{}

	err := importer.DeleteTeamsAndUsers(context.Background(), gw, users, cfg, importer.DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteTeamsAndUsers: %v", err)
	}

	got := sortedCopy(gw.deleteUsersIncludeCap)
	want := []string{"alice", "bob"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DeleteUsers include (sorted) = %v, want %v", got, want)
	}
}

func TestDeleteTeamsAndUsers_DerivesTeamIDs(t *testing.T) {
	cfg := &config.DomServerClient{Username: "configuser"}
	users := userListServer(t, []model.User{
		{ID: "1", Username: "alice", TeamID: strPtr("11")},
		{ID: "2", Username: "bob", TeamID: strPtr("22")},
		{ID: "3", Username: "carol", TeamID: strPtr("33")},
	})
	gw := &mockGateway{}

	err := importer.DeleteTeamsAndUsers(context.Background(), gw, users, cfg, importer.DeleteOptions{
		Include: []string{"alice", "bob"},
		Exclude: []string{"carol"},
	})
	if err != nil {
		t.Fatalf("DeleteTeamsAndUsers: %v", err)
	}

	gotInclude := sortedCopy(gw.deleteTeamsIncludeCap)
	wantInclude := []string{"11", "22"}
	if !reflect.DeepEqual(gotInclude, wantInclude) {
		t.Errorf("DeleteTeams include = %v, want %v", gotInclude, wantInclude)
	}

	gotExclude := sortedCopy(gw.deleteTeamsExcludeCap)
	wantExclude := []string{"33"}
	if !reflect.DeepEqual(gotExclude, wantExclude) {
		t.Errorf("DeleteTeams exclude = %v, want %v", gotExclude, wantExclude)
	}
}

func TestDeleteTeamsAndUsers_BothGatewayCalls(t *testing.T) {
	cfg := &config.DomServerClient{Username: "configuser"}
	users := userListServer(t, []model.User{
		{ID: "1", Username: "alice", TeamID: strPtr("11")},
	})
	gw := &mockGateway{}

	err := importer.DeleteTeamsAndUsers(context.Background(), gw, users, cfg, importer.DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteTeamsAndUsers: %v", err)
	}

	// Locate DeleteUsers and DeleteTeams in the call order; first must
	// precede second.
	posUsers, posTeams := -1, -1
	for i, name := range gw.callOrder {
		switch name {
		case "DeleteUsers":
			if posUsers == -1 {
				posUsers = i
			}
		case "DeleteTeams":
			if posTeams == -1 {
				posTeams = i
			}
		}
	}
	if posUsers == -1 {
		t.Fatalf("DeleteUsers never called; callOrder=%v", gw.callOrder)
	}
	if posTeams == -1 {
		t.Fatalf("DeleteTeams never called; callOrder=%v", gw.callOrder)
	}
	if posUsers >= posTeams {
		t.Errorf("DeleteUsers must precede DeleteTeams; callOrder=%v", gw.callOrder)
	}
	if gw.loginCalls == 0 {
		t.Errorf("Login was never invoked; callOrder=%v", gw.callOrder)
	}
}
