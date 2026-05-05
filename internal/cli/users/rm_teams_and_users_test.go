package users_test

import (
	"bytes"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli/users"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/model"
)

func ptrStr(s string) *string { return &s }

func TestRmTeamsAndUsers_DefaultProtects(t *testing.T) {
	t.Parallel()
	usersList := []model.User{
		{ID: "1", Username: "admin", TeamID: ptrStr("100")},
		{ID: "2", Username: "alice", TeamID: ptrStr("101")},
		{ID: "3", Username: "bob", TeamID: ptrStr("102")},
		{ID: "4", Username: "judgehost", TeamID: ptrStr("103")},
	}
	srv := httptest.NewServer(userListHandler(t, usersList))
	defer srv.Close()

	gw := &mockGateway{}
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps(srv.URL, gw))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL, Username: "admin"}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gw.loginCalls != 1 {
		t.Errorf("Login called %d, want 1", gw.loginCalls)
	}
	// Default-protect set: admin, judgehost, cfg.Username (=admin).
	// alice + bob remain.
	want := []string{"alice", "bob"}
	if !slices.Equal(gw.deleteUsersInclude, want) {
		t.Errorf("DeleteUsers include = %v, want %v", gw.deleteUsersInclude, want)
	}
	wantTeams := []string{"101", "102"}
	if !slices.Equal(gw.deleteTeamsInclude, wantTeams) {
		t.Errorf("DeleteTeams include = %v, want %v", gw.deleteTeamsInclude, wantTeams)
	}
}

func TestRmTeamsAndUsers_IncludeExcludeFlags(t *testing.T) {
	t.Parallel()
	usersList := []model.User{
		{ID: "1", Username: "admin", TeamID: ptrStr("100")},
		{ID: "2", Username: "alice", TeamID: ptrStr("101")},
		{ID: "3", Username: "bob", TeamID: ptrStr("102")},
	}
	srv := httptest.NewServer(userListHandler(t, usersList))
	defer srv.Close()

	gw := &mockGateway{}
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps(srv.URL, gw))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--include", "alice", "--include", "bob", "--exclude", "bob"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL, Username: "admin"}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := []string{"alice"} // bob excluded; admin protected (and not in include anyway)
	if !slices.Equal(gw.deleteUsersInclude, want) {
		t.Errorf("DeleteUsers include = %v, want %v", gw.deleteUsersInclude, want)
	}
}

func TestRmTeamsAndUsers_AliasRmTeams(t *testing.T) {
	t.Parallel()
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps("http://nowhere", &mockGateway{}))
	got := cmd.Aliases
	if !slices.Contains(got, "rm-teams") {
		t.Errorf("Aliases = %v, want to contain rm-teams", got)
	}
}

func TestRmTeamsAndUsers_GroupedUnderUsers(t *testing.T) {
	t.Parallel()
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps("http://nowhere", &mockGateway{}))
	if cmd.GroupID != "users" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "users")
	}
}
