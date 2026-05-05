package users_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli/users"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/model"
)

const sampleCSV = `username,name,email,affiliation,password
alice,Alice,alice@example.com,,wonderland
bob,Bob,,,builder
`

// importHandler routes the api-first create flow's read endpoints
// against the in-memory state used by TestImportUsersTeams_HappyPath.
// /users starts empty so every CSV row enters the create cohort, then
// flips to a populated snapshot once /users/accounts has been POSTed
// so the email backfill can resolve user_ids. PR-3 added the contests
// + organizations endpoints; the four bulk import POSTs swallow any
// multipart payload and respond with a placeholder id list.
func importHandler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	var accountsImported bool
	mux.HandleFunc("/api/v4/users", func(w http.ResponseWriter, _ *http.Request) {
		if !accountsImported {
			_ = json.NewEncoder(w).Encode([]model.User{})
			return
		}
		_, _ = w.Write([]byte(`[{"id":"1","username":"alice","name":"Alice","enabled":true,"roles":[]},{"id":"2","username":"bob","name":"Bob","enabled":true,"roles":[]}]`))
	})
	mux.HandleFunc("/api/v4/contests", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"1","shortname":"main","name":"Main","state":{"started":"2026-01-01T00:00:00+00:00"}}]`))
	})
	mux.HandleFunc("/api/v4/contests/1/organizations", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	for _, p := range []string{
		"/api/v4/users/groups",
		"/api/v4/users/organizations",
		"/api/v4/users/teams",
	} {
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseMultipartForm(1 << 20)
			_, _ = w.Write([]byte(`["ok"]`))
		})
	}
	mux.HandleFunc("/api/v4/users/accounts", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		accountsImported = true
		_, _ = w.Write([]byte(`["ok"]`))
	})
	return mux
}

// importGateway wraps mockGateway and synthesizes (teamID, userID) so
// importer.ImportUsersTeams completes without per-row API errors.
type importGateway struct {
	*mockGateway
}

func (g *importGateway) CreateTeamAndUser(_ context.Context, user model.CreateUser, _, _ int, _ bool) (string, string, error) {
	return "T-" + user.Username, "U-" + user.Username, nil
}

func TestImportUsersTeams_HappyPath(t *testing.T) {
	srv := httptest.NewServer(importHandler(t))
	defer srv.Close()

	gw := &importGateway{mockGateway: &mockGateway{}}

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "in.csv")
	if err := os.WriteFile(csvPath, []byte(sampleCSV), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	chdir(t, tmpDir)

	cmd := users.NewImportUsersTeamsCmd(stubDeps(srv.URL, gw))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{csvPath})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "import-users-teams-out.csv")); err != nil {
		t.Errorf("expected output CSV at import-users-teams-out.csv, got: %v", err)
	}
	if gw.loginCalls != 1 {
		t.Errorf("Login calls = %d, want 1", gw.loginCalls)
	}
	// v1.1.1 #6: success summary on stdout. sampleCSV has 2 rows, both
	// new (snapshot starts empty) so created=2, updated=0, skipped=0.
	got := stdout.String()
	if !strings.Contains(got, "Imported 2 users") ||
		!strings.Contains(got, "created 2") ||
		!strings.Contains(got, "updated 0") ||
		!strings.Contains(got, "skipped 0") {
		t.Errorf("stdout %q should contain summary 'Imported 2 users (created 2, updated 0, skipped 0)'", got)
	}
}

func TestImportUsersTeams_RequiresFile(t *testing.T) {
	t.Parallel()
	cmd := users.NewImportUsersTeamsCmd(stubDeps("http://x", &mockGateway{}))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{}))

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute should fail when CSV path missing (cobra.ExactArgs)")
	}
}

func TestImportUsersTeams_MutuallyExclusiveFlags(t *testing.T) {
	t.Parallel()
	cmd := users.NewImportUsersTeamsCmd(stubDeps("http://x", &mockGateway{}))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"some.csv", "--ignore-existing", "--delete-existing"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{}))

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Execute should reject --ignore-existing + --delete-existing")
	}
	if !strings.Contains(err.Error(), "ignore-existing") ||
		!strings.Contains(err.Error(), "delete-existing") {
		t.Errorf("err = %q, want mention of both flags", err)
	}
}

func TestImportUsersTeams_GroupedUnderUsers(t *testing.T) {
	t.Parallel()
	cmd := users.NewImportUsersTeamsCmd(stubDeps("http://x", &mockGateway{}))
	if cmd.GroupID != "users" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "users")
	}
}

func TestImportUsersTeams_NonexistentCSV_WrapsErrNotExist(t *testing.T) {
	t.Parallel()
	cmd := users.NewImportUsersTeamsCmd(stubDeps("http://x", &mockGateway{}))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"/no/such/in.csv"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: "http://x"}))

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Execute should fail when CSV is missing")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want wraps os.ErrNotExist", err)
	}
}
