package users_test

import (
	"bytes"
	"encoding/json"
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

func strPtr(s string) *string { return &s }

func TestExportUsers_FormatCSV_Stdout(t *testing.T) {
	t.Parallel()
	usersList := []model.User{
		{ID: "1", Username: "alice", Name: "Alice"},
		{ID: "2", Username: "bob", Name: "Bob"},
	}
	srv := httptest.NewServer(userListHandler(t, usersList))
	defer srv.Close()

	cmd := users.NewExportUsersCmd(stubDeps(srv.URL, nil))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "csv"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	wantHeader := "last_login_time,first_login_time,team,roles,id,username,name,email,last_ip,ip,enabled,team_id,affiliation,password\n"
	if !strings.HasPrefix(got, wantHeader) {
		t.Errorf("output should start with locked CSV header, got:\n%s", got)
	}
	if !strings.Contains(got, "alice") || !strings.Contains(got, "bob") {
		t.Errorf("output missing user rows:\n%s", got)
	}
}

func TestExportUsers_FormatJSON_Stdout(t *testing.T) {
	t.Parallel()
	usersList := []model.User{{ID: "1", Username: "alice"}}
	srv := httptest.NewServer(userListHandler(t, usersList))
	defer srv.Close()

	cmd := users.NewExportUsersCmd(stubDeps(srv.URL, nil))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "json"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &arr); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
	}
	if len(arr) != 1 || arr[0]["username"] != "alice" {
		t.Errorf("unexpected json: %+v", arr)
	}
}

func TestExportUsers_OutputFlag(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(userListHandler(t, []model.User{{ID: "1", Username: "alice"}}))
	defer srv.Close()

	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "users.csv")
	cmd := users.NewExportUsersCmd(stubDeps(srv.URL, nil))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "csv", "--output", out})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Errorf("stdout should be empty when --output is given, got %q", got)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(body), "alice") {
		t.Errorf("output file missing user row, got:\n%s", body)
	}
}

func TestExportUsers_FilterByIDs(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(userListHandler(t, []model.User{
		{ID: "1", Username: "alice"},
		{ID: "2", Username: "bob"},
		{ID: "3", Username: "carol"},
	}))
	defer srv.Close()

	cmd := users.NewExportUsersCmd(stubDeps(srv.URL, nil))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "json", "--ids", "1,3"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(arr) != 2 {
		t.Errorf("len = %d, want 2 (IDs filter), got %+v", len(arr), arr)
	}
}

// TestExportUsers_FilterByTeamID_ServerDoesNotFilter verifies local filtering
// when a server ignores the /api/v4/users team_id query parameter.
func TestExportUsers_FilterByTeamID_ServerDoesNotFilter(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(userListHandler(t, []model.User{
		{ID: "1", Username: "alice", TeamID: strPtr("3")},
		{ID: "2", Username: "bob", TeamID: strPtr("5")},
		{ID: "3", Username: "carol", TeamID: strPtr("5")},
		{ID: "4", Username: "dave", TeamID: strPtr("9")},
	}))
	defer srv.Close()

	cmd := users.NewExportUsersCmd(stubDeps(srv.URL, nil))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "json", "--team-id", "5"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &arr); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
	}
	if len(arr) != 2 {
		t.Fatalf("len = %d, want 2 (only team_id=5), output=%s", len(arr), stdout.String())
	}
	for _, u := range arr {
		if u["team_id"] != "5" {
			t.Errorf("team_id = %v, want 5", u["team_id"])
		}
	}
}

func TestExportUsers_AliasUserList(t *testing.T) {
	t.Parallel()
	cmd := users.NewExportUsersCmd(stubDeps("http://x", nil))
	if !slices.Contains(cmd.Aliases, "user-list") {
		t.Errorf("Aliases = %v, want to contain user-list", cmd.Aliases)
	}
}

func TestExportUsers_GroupedUnderUsers(t *testing.T) {
	t.Parallel()
	cmd := users.NewExportUsersCmd(stubDeps("http://x", nil))
	if cmd.GroupID != "users" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "users")
	}
}

func TestExportUsers_FormatJSON_EmptyListIsEmptyArray(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(userListHandler(t, []model.User{}))
	defer srv.Close()

	cmd := users.NewExportUsersCmd(stubDeps(srv.URL, nil))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "json"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "[]" {
		t.Errorf("empty list output = %q, want %q (JSON null is forbidden)", got, "[]")
	}
}

func TestExportUsers_RejectsBadFormat(t *testing.T) {
	t.Parallel()
	cmd := users.NewExportUsersCmd(stubDeps("http://x", nil))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "yaml"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: "http://x"}))

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute should reject --format yaml")
	}
}
