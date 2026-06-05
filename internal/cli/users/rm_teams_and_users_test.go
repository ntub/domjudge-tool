package users_test

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestRmTeamsAndUsers_IncludeCSV(t *testing.T) {
	t.Parallel()
	usersList := []model.User{
		{ID: "1", Username: "admin", TeamID: ptrStr("100")},
		{ID: "2", Username: "alice", TeamID: ptrStr("101")},
		{ID: "3", Username: "bob", TeamID: ptrStr("102")},
	}
	srv := httptest.NewServer(userListHandler(t, usersList))
	defer srv.Close()

	csvPath := writeTempCSV(t, "username,name,email,affiliation,password\nalice,Alice,,,secret\nbob,Bob,,,secret\n")
	gw := &mockGateway{}
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps(srv.URL, gw))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--include-csv", csvPath})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL, Username: "admin"}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := []string{"alice", "bob"}
	if !slices.Equal(gw.deleteUsersInclude, want) {
		t.Errorf("DeleteUsers include = %v, want %v", gw.deleteUsersInclude, want)
	}
	wantTeams := []string{"101", "102"}
	if !slices.Equal(gw.deleteTeamsInclude, wantTeams) {
		t.Errorf("DeleteTeams include = %v, want %v", gw.deleteTeamsInclude, wantTeams)
	}
}

func TestRmTeamsAndUsers_IncludeCSVWithExclude(t *testing.T) {
	t.Parallel()
	usersList := []model.User{
		{ID: "1", Username: "alice", TeamID: ptrStr("101")},
		{ID: "2", Username: "bob", TeamID: ptrStr("102")},
	}
	srv := httptest.NewServer(userListHandler(t, usersList))
	defer srv.Close()

	csvPath := writeTempCSV(t, "username,name,email,affiliation,password\nalice,Alice,,,secret\nbob,Bob,,,secret\n")
	gw := &mockGateway{}
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps(srv.URL, gw))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--include-csv", csvPath, "--exclude", "bob"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL, Username: "admin"}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := []string{"alice"}
	if !slices.Equal(gw.deleteUsersInclude, want) {
		t.Errorf("DeleteUsers include = %v, want %v", gw.deleteUsersInclude, want)
	}
}

func TestRmTeamsAndUsers_IncludeCSVRejectsEmptyList(t *testing.T) {
	t.Parallel()
	csvPath := writeTempCSV(t, "username,name,email,affiliation,password\n")
	gw := &mockGateway{}
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps("http://nowhere", gw))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--include-csv", csvPath})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: "http://nowhere", Username: "admin"}))

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil, want empty CSV error")
	}
	if !strings.Contains(err.Error(), "include-csv") || !strings.Contains(err.Error(), "no usernames") {
		t.Fatalf("error = %v, want include-csv no usernames", err)
	}
	if gw.loginCalls != 0 || gw.deleteUsersCalls != 0 || gw.deleteTeamsCalls != 0 {
		t.Fatalf("gateway was called on empty CSV: login=%d users=%d teams=%d",
			gw.loginCalls, gw.deleteUsersCalls, gw.deleteTeamsCalls)
	}
}

func TestRmTeamsAndUsers_IncludeCSVRejectsOnlyProtectedUsers(t *testing.T) {
	t.Parallel()
	csvPath := writeTempCSV(t, "username,name,email,affiliation,password\nadmin,Admin,,,secret\njudgehost,Judgehost,,,secret\n")
	gw := &mockGateway{}
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps("http://nowhere", gw))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--include-csv", csvPath})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: "http://nowhere", Username: "admin"}))

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil, want no usernames error")
	}
	if !strings.Contains(err.Error(), "no usernames to delete") {
		t.Fatalf("error = %v, want no usernames to delete", err)
	}
	if gw.loginCalls != 0 || gw.deleteUsersCalls != 0 || gw.deleteTeamsCalls != 0 {
		t.Fatalf("gateway was called on protected-only CSV: login=%d users=%d teams=%d",
			gw.loginCalls, gw.deleteUsersCalls, gw.deleteTeamsCalls)
	}
}

func TestRmTeamsAndUsers_IncludeCSVRejectsAllExcludedUsers(t *testing.T) {
	t.Parallel()
	csvPath := writeTempCSV(t, "username,name,email,affiliation,password\nalice,Alice,,,secret\n")
	gw := &mockGateway{}
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps("http://nowhere", gw))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--include-csv", csvPath, "--exclude", "alice"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: "http://nowhere", Username: "admin"}))

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil, want no usernames error")
	}
	if !strings.Contains(err.Error(), "no usernames to delete") {
		t.Fatalf("error = %v, want no usernames to delete", err)
	}
	if gw.loginCalls != 0 || gw.deleteUsersCalls != 0 || gw.deleteTeamsCalls != 0 {
		t.Fatalf("gateway was called when all CSV users were excluded: login=%d users=%d teams=%d",
			gw.loginCalls, gw.deleteUsersCalls, gw.deleteTeamsCalls)
	}
}

func TestRmTeamsAndUsers_IncludeCSVMutuallyExclusiveWithInclude(t *testing.T) {
	t.Parallel()
	csvPath := writeTempCSV(t, "username,name,email,affiliation,password\nalice,Alice,,,secret\n")
	cmd := users.NewRmTeamsAndUsersCmd(stubDeps("http://nowhere", &mockGateway{}))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--include-csv", csvPath, "--include", "alice"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: "http://nowhere", Username: "admin"}))

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil, want mutually exclusive flag error")
	}
	if !strings.Contains(err.Error(), "include") || !strings.Contains(err.Error(), "include-csv") {
		t.Fatalf("error = %v, want include/include-csv conflict", err)
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

func writeTempCSV(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.csv")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp csv: %v", err)
	}
	return path
}
