package importer_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/importer"
)

// TestImportUsersTeams_API_IgnoreExisting_SkipsKnownRow locks the
// hybrid contract: when --ignore-existing is set and a CSV row's
// username appears in the live snapshot, ImportUsersTeams must drop
// the row before the API path renders TSVs. The accounts.tsv body
// captured by the fake server therefore must NOT contain that
// username.
func TestImportUsersTeams_API_IgnoreExisting_SkipsKnownRow(t *testing.T) {
	cap := &branchCapture{}
	srv := newBranchServer(t, cap, branchServerOpts{
		// alice already exists in the snapshot; bob is new.
		usersFirstPage:  `[{"id":"7","username":"alice","name":"Old Alice","enabled":true,"roles":[]}]`,
		usersSecondPage: `[{"id":"7","username":"alice","name":"Old Alice","enabled":true,"roles":[]},{"id":"8","username":"bob","name":"Bob","enabled":true,"roles":[]}]`,
	})
	t.Cleanup(srv.Close)

	api := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGateway{}
	cid := 13337
	body := importCSVHeader +
		"\nalice,Alice,alice@example.org,ACME,pw1" +
		"\nbob,Bob,bob@example.org,ACME,pw2\n"

	if _, err := importer.ImportUsersTeams(context.Background(), gw, api.Users, strings.NewReader(body), importer.ImportOptions{
		CategoryID:         &cid,
		AffiliationCountry: "TWN",
		IgnoreExisting:     true,
		Router:             pinnedRouter{path: importer.PathAPI, reason: "test: api mode"},
		API:                api,
		ContestHint:        "13337",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	got := cap.accountsTSV()
	if strings.Contains(got, "alice") {
		t.Errorf("accounts.tsv must not contain ignored row 'alice'; got %q", got)
	}
	if !strings.Contains(got, "bob") {
		t.Errorf("accounts.tsv missing surviving row 'bob'; got %q", got)
	}
}

// TestImportUsersTeams_API_DeleteExisting_CallsScraperFirst locks the
// hybrid sequence the spec calls "混合的關鍵 case": with
// --delete-existing the gateway DeleteUsers + DeleteTeams fire BEFORE
// the API import POSTs. After preDelete the CSV row is treated as a
// fresh create and reaches the API path.
func TestImportUsersTeams_API_DeleteExisting_CallsScraperFirst(t *testing.T) {
	cap := &branchCapture{}
	srv := newBranchServer(t, cap, branchServerOpts{
		// alice exists in the initial snapshot so preDelete fires for it.
		usersFirstPage:  `[{"id":"7","username":"alice","name":"Old Alice","enabled":true,"team_id":"42","roles":[]}]`,
		usersSecondPage: `[{"id":"7","username":"alice","name":"Old Alice","enabled":true,"team_id":"42","roles":[]}]`,
	})
	t.Cleanup(srv.Close)

	api := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGateway{}
	cid := 13337
	body := importCSVHeader + "\nalice,Alice,,ACME,pw1\n"

	if _, err := importer.ImportUsersTeams(context.Background(), gw, api.Users, strings.NewReader(body), importer.ImportOptions{
		CategoryID:         &cid,
		AffiliationCountry: "TWN",
		DeleteExisting:     true,
		Router:             pinnedRouter{path: importer.PathAPI, reason: "test: api mode"},
		API:                api,
		ContestHint:        "13337",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if gw.deleteUsersCalls != 1 {
		t.Errorf("DeleteUsers calls = %d, want 1", gw.deleteUsersCalls)
	}
	if gw.deleteTeamsCalls != 1 {
		t.Errorf("DeleteTeams calls = %d, want 1", gw.deleteTeamsCalls)
	}
	// preDelete must have fired before the first import POST. The fake
	// server records the POST timestamps via cap.firstAccountsPOSTOrder.
	deleteUsersIdx := indexOf(gw.callOrder, "DeleteUsers")
	deleteTeamsIdx := indexOf(gw.callOrder, "DeleteTeams")
	if deleteUsersIdx < 0 || deleteTeamsIdx < 0 {
		t.Fatalf("DeleteUsers/DeleteTeams missing from gateway callOrder=%v", gw.callOrder)
	}
	if deleteUsersIdx >= deleteTeamsIdx {
		t.Errorf("DeleteUsers must precede DeleteTeams; got order %v", gw.callOrder)
	}
	if cap.accountsTSV() == "" {
		t.Error("accounts.tsv was never POSTed; api path failed to run after preDelete")
	}
	if !strings.Contains(cap.accountsTSV(), "alice") {
		t.Errorf("accounts.tsv missing recreated row 'alice'; got %q", cap.accountsTSV())
	}
}

// indexOf returns the position of name in xs, or -1 when absent.
func indexOf(xs []string, name string) int {
	for i, s := range xs {
		if s == name {
			return i
		}
	}
	return -1
}

// --- helpers shared by the two tests above -----------------------------

type branchServerOpts struct {
	usersFirstPage  string
	usersSecondPage string
}

type branchCapture struct {
	mu              sync.Mutex
	accountsBody    string
	accountsHitOnce bool
}

func (c *branchCapture) accountsTSV() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accountsBody
}

func (c *branchCapture) writeAccounts(body string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accountsHitOnce {
		return
	}
	c.accountsBody = body
	c.accountsHitOnce = true
}

func newBranchServer(t *testing.T, cap *branchCapture, opts branchServerOpts) *httptest.Server {
	t.Helper()
	usersCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/users":
			usersCount++
			w.Header().Set("Content-Type", "application/json")
			if usersCount == 1 {
				_, _ = w.Write([]byte(opts.usersFirstPage))
				return
			}
			_, _ = w.Write([]byte(opts.usersSecondPage))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/contests":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"13337","shortname":"main","name":"Main","state":{"started":"2026-01-01T00:00:00+00:00"}}]`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v4/contests/") && strings.HasSuffix(r.URL.Path, "/organizations"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/users/accounts":
			_ = r.ParseMultipartForm(1 << 20)
			f, _, _ := r.FormFile("tsv")
			if f != nil {
				body, _ := io.ReadAll(f)
				_ = f.Close()
				cap.writeAccounts(string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["ok"]`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v4/users/"):
			_ = r.ParseMultipartForm(1 << 20)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["ok"]`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}
