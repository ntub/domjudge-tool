package users_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/cli/users"
	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/model"
)

// userListHandler returns an http.Handler that serves model.Users at
// /api/v4/users (matching apiv4.UsersResource.List).
func userListHandler(t *testing.T, list []model.User) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/users" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	})
}

type fakeAccountsImporter struct {
	calls []string
	err   error
}

func (f *fakeAccountsImporter) Accounts(ctx context.Context, r io.Reader) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	f.calls = append(f.calls, string(body))
	if f.err != nil {
		return nil, f.err
	}
	return []string{"alice"}, nil
}

// stubAPIDeps wires a Deps using an httptest server URL. The gateway
// factory returns the supplied gw verbatim.
func stubDeps(srvURL string, gw web.Gateway) users.Deps {
	return stubDepsWithAccounts(srvURL, gw, &fakeAccountsImporter{})
}

func stubDepsWithAccounts(srvURL string, gw web.Gateway, accounts *fakeAccountsImporter) users.Deps {
	return users.Deps{
		NewAPI: func(_ *config.DomServerClient) *apiv4.Client {
			return apiv4.New(apiv4.Options{BaseURL: srvURL, Username: "admin", Password: "secret"})
		},
		NewGateway: func(_ context.Context, _ *config.DomServerClient) (web.Gateway, error) {
			return gw, nil
		},
		NewAccountsImporter: func(_ *apiv4.Client) users.AccountsImporter {
			return accounts
		},
	}
}

func ctxWithCfg(cfg *config.DomServerClient) context.Context {
	return cli.WithConfig(context.Background(), cfg)
}

func TestSetPassword_DefaultGoesAPI(t *testing.T) {
	t.Parallel()

	users0 := []model.User{
		{ID: "42", Username: "alice", Name: "Alice Team"},
		{ID: "7", Username: "bob"},
	}
	srv := httptest.NewServer(userListHandler(t, users0))
	defer srv.Close()

	gw := &mockGateway{}
	accounts := &fakeAccountsImporter{}
	cmd := users.NewSetPasswordCmd(stubDepsWithAccounts(srv.URL, gw, accounts))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"alice", "--password", "s3cr3t"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL, Username: "admin", Password: "secret"}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gw.loginCalls != 0 {
		t.Errorf("Login called %d times, want 0 for API path", gw.loginCalls)
	}
	if len(gw.setPasswordCalls) != 0 {
		t.Fatalf("SetUserPassword calls = %d, want 0 for API path", len(gw.setPasswordCalls))
	}
	if len(accounts.calls) != 1 {
		t.Fatalf("Accounts calls = %d, want 1", len(accounts.calls))
	}
	wantTSV := "accounts\t1\nteam\tAlice Team\talice\ts3cr3t\n"
	if accounts.calls[0] != wantTSV {
		t.Fatalf("accounts.tsv mismatch\nwant: %q\n got: %q", wantTSV, accounts.calls[0])
	}
}

func TestSetPassword_UnknownUser(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(userListHandler(t, []model.User{{ID: "1", Username: "alice"}}))
	defer srv.Close()

	gw := &mockGateway{}
	accounts := &fakeAccountsImporter{}
	cmd := users.NewSetPasswordCmd(stubDepsWithAccounts(srv.URL, gw, accounts))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"bogus", "--password", "x"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Execute should fail for unknown user")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("err = %q, want it to mention the username", err)
	}
	if gw.loginCalls != 0 || len(gw.setPasswordCalls) != 0 {
		t.Errorf("gateway should not be hit when the user is missing: %+v", gw)
	}
	if len(accounts.calls) != 0 {
		t.Errorf("accounts importer should not be hit when the user is missing: %+v", accounts.calls)
	}
}

func TestSetPassword_GeneratedPasswordPrintedOnce(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(userListHandler(t, []model.User{{ID: "1", Username: "alice"}}))
	defer srv.Close()

	gw := &mockGateway{}
	accounts := &fakeAccountsImporter{}
	cmd := users.NewSetPasswordCmd(stubDepsWithAccounts(srv.URL, gw, accounts))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"alice"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(gw.setPasswordCalls) != 0 {
		t.Fatalf("SetUserPassword calls = %d, want 0 for API path", len(gw.setPasswordCalls))
	}
	if len(accounts.calls) != 1 {
		t.Fatalf("Accounts calls = %d, want 1", len(accounts.calls))
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("generated password should be printed once, got %q", stdout.String())
	}
	pw := lines[0]
	if pw == "" {
		t.Fatalf("generated password is empty")
	}
	wantTSV := "accounts\t1\nteam\talice\talice\t" + pw + "\n"
	if accounts.calls[0] != wantTSV {
		t.Fatalf("accounts.tsv mismatch\nwant: %q\n got: %q", wantTSV, accounts.calls[0])
	}
}

func TestSetPassword_RolesFlagGoesScraper(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(userListHandler(t, []model.User{{ID: "1", Username: "alice"}}))
	defer srv.Close()

	gw := &mockGateway{}
	accounts := &fakeAccountsImporter{}
	cmd := users.NewSetPasswordCmd(stubDepsWithAccounts(srv.URL, gw, accounts))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"alice", "--password", "rotated", "--user-roles", "3,4"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(accounts.calls) != 0 {
		t.Fatalf("Accounts calls = %d, want 0 for scraper path", len(accounts.calls))
	}
	if gw.loginCalls != 1 {
		t.Errorf("Login called %d times, want 1 for scraper path", gw.loginCalls)
	}
	if len(gw.setPasswordCalls) != 1 {
		t.Fatalf("SetUserPassword calls = %d, want 1", len(gw.setPasswordCalls))
	}
	got := gw.setPasswordCalls[0].Roles
	want := []int{3, 4}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Roles = %v, want %v", got, want)
	}
}

func TestSetPassword_V8ConfigWithoutRolesGoesScraper(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(userListHandler(t, []model.User{{ID: "1", Username: "alice"}}))
	defer srv.Close()

	gw := &mockGateway{}
	accounts := &fakeAccountsImporter{}
	cmd := users.NewSetPasswordCmd(stubDepsWithAccounts(srv.URL, gw, accounts))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"alice", "--password", "rotated"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{
		Host:     srv.URL,
		Username: "admin",
		Password: "secret",
		Version:  "8.3.1",
	}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(accounts.calls) != 0 {
		t.Fatalf("Accounts calls = %d, want 0 for scraper path", len(accounts.calls))
	}
	if gw.loginCalls != 1 {
		t.Errorf("Login called %d times, want 1 for scraper path", gw.loginCalls)
	}
	if len(gw.setPasswordCalls) != 1 {
		t.Fatalf("SetUserPassword calls = %d, want 1", len(gw.setPasswordCalls))
	}
	got := gw.setPasswordCalls[0]
	if got.UserID != "1" || got.Password != "rotated" || got.Roles != nil {
		t.Errorf("scraper call = %+v, want user 1 password rotated nil roles", got)
	}
}

func TestSetPassword_PreferScraperConfigGoesScraper(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(userListHandler(t, []model.User{{ID: "1", Username: "alice"}}))
	defer srv.Close()

	gw := &mockGateway{}
	accounts := &fakeAccountsImporter{}
	cmd := users.NewSetPasswordCmd(stubDepsWithAccounts(srv.URL, gw, accounts))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"alice", "--password", "rotated"})
	preferScraper := true
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL, PreferScraper: &preferScraper}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(accounts.calls) != 0 {
		t.Fatalf("Accounts calls = %d, want 0 for scraper path", len(accounts.calls))
	}
	if gw.loginCalls != 1 {
		t.Errorf("Login called %d times, want 1 for scraper path", gw.loginCalls)
	}
	if len(gw.setPasswordCalls) != 1 {
		t.Fatalf("SetUserPassword calls = %d, want 1", len(gw.setPasswordCalls))
	}
	got := gw.setPasswordCalls[0]
	if got.UserID != "1" || got.Password != "rotated" || got.Roles != nil {
		t.Errorf("scraper call = %+v, want user 1 password rotated nil roles", got)
	}
}

func TestSetPassword_RequiresUsername(t *testing.T) {
	t.Parallel()
	cmd := users.NewSetPasswordCmd(stubDeps("http://nowhere", &mockGateway{}))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{}))

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute should reject missing positional (cobra.ExactArgs)")
	}
}

func TestSetPassword_AdminAllowed(t *testing.T) {
	t.Parallel()
	// admin is in the default-protect set for *delete*, but
	// set-password does not consult that set: an operator can reset
	// the admin password.
	srv := httptest.NewServer(userListHandler(t, []model.User{{ID: "1", Username: "admin"}}))
	defer srv.Close()

	gw := &mockGateway{}
	accounts := &fakeAccountsImporter{}
	cmd := users.NewSetPasswordCmd(stubDepsWithAccounts(srv.URL, gw, accounts))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"admin", "--password", "rotated"})
	cmd.SetContext(ctxWithCfg(&config.DomServerClient{Host: srv.URL, Username: "admin"}))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(gw.setPasswordCalls) != 0 {
		t.Errorf("admin reset should default to API, gateway calls: %+v", gw.setPasswordCalls)
	}
	if len(accounts.calls) != 1 {
		t.Fatalf("Accounts calls = %d, want 1", len(accounts.calls))
	}
	wantTSV := "accounts\t1\nteam\tadmin\tadmin\trotated\n"
	if accounts.calls[0] != wantTSV {
		t.Fatalf("accounts.tsv mismatch\nwant: %q\n got: %q", wantTSV, accounts.calls[0])
	}
}

func TestSetPassword_GroupedUnderUsers(t *testing.T) {
	t.Parallel()
	cmd := users.NewSetPasswordCmd(stubDeps("http://nowhere", &mockGateway{}))
	if cmd.GroupID != "users" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "users")
	}
}
