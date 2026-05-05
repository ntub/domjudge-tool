package importer_test

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

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/exporter"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/model"
)

// TestImportUsersTeams_OutputCSV_Golden_API_vs_Scraper is the
// CONTRACT.md §4.4 byte-identical golden: the same input CSV, run
// through the scraper path or the API path, must produce a single
// import-users-teams-out.csv that matches testdata/importer/golden_out.csv
// to the byte. Drift in either path's mapping logic surfaces here.
//
// The test wires production ImportUsersTeams with a router pinned to
// the path under test; per-mode helpers stand up either a mockGateway
// (scraper) or an API fake server (api). For the api path the same
// mockGateway acts as the email-backfill fallback the API can't do.
func TestImportUsersTeams_OutputCSV_Golden_API_vs_Scraper(t *testing.T) {
	in := mustReadFixture(t, "input_5_rows.csv")
	want := mustReadFixture(t, "golden_out.csv")

	for _, mode := range []string{"scraper", "api"} {
		t.Run(mode, func(t *testing.T) {
			users, err := runImportInMode(t, mode, in)
			if err != nil {
				t.Fatalf("ImportUsersTeams: %v", err)
			}
			var out bytes.Buffer
			if err := exporter.CreateUsers(users, exporter.FormatCSV, &out); err != nil {
				t.Fatalf("exporter.CreateUsers: %v", err)
			}
			if !bytes.Equal(out.Bytes(), want) {
				t.Errorf("mode=%s output drift\n--- got (len=%d) ---\n%q\n--- want (len=%d) ---\n%q",
					mode, out.Len(), out.String(), len(want), string(want))
			}
		})
	}
}

func runImportInMode(t *testing.T, mode string, csvBody []byte) ([]model.CreateUser, error) {
	t.Helper()
	cid := 13337
	opts := importer.ImportOptions{
		CategoryID:         &cid,
		AffiliationCountry: "TWN",
	}
	switch mode {
	case "scraper":
		return runScraperImport(t, csvBody, opts)
	case "api":
		return runAPIImport(t, csvBody, opts)
	default:
		t.Fatalf("unknown mode %q", mode)
		return nil, nil
	}
}

func runScraperImport(t *testing.T, csvBody []byte, opts importer.ImportOptions) ([]model.CreateUser, error) {
	t.Helper()
	users := emptyUsersResource(t)
	gw := &mockGateway{createTeamID: "100", createUserID: "u1"}
	opts.Router = pinnedRouter{path: importer.PathScraper, reason: "test: scraper mode"}
	return importer.ImportUsersTeams(context.Background(), gw, users, bytes.NewReader(csvBody), opts)
}

func runAPIImport(t *testing.T, csvBody []byte, opts importer.ImportOptions) ([]model.CreateUser, error) {
	t.Helper()
	api, gw := newAPIModeServers(t)
	opts.Router = pinnedRouter{path: importer.PathAPI, reason: "test: api mode"}
	opts.API = api
	opts.ContestHint = "13337"
	return importer.ImportUsersTeams(context.Background(), gw, api.Users, bytes.NewReader(csvBody), opts)
}

func newAPIModeServers(t *testing.T) (*apiv4.Client, *mockGateway) {
	t.Helper()
	usersCallCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/users":
			usersCallCount++
			w.Header().Set("Content-Type", "application/json")
			// First call is the pre-flight snapshot from ImportUsersTeams;
			// it must be empty so every CSV row enters the create branch.
			// Subsequent calls (apiCreateFlow's post-import backfill)
			// surface the just-created users so the email update lookup
			// can resolve user ids.
			if usersCallCount == 1 {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":"100","username":"alice","name":"Alice","enabled":true,"roles":[]},{"id":"101","username":"bob","name":"Bob","enabled":true,"roles":[]},{"id":"102","username":"carol","name":"Carol","enabled":true,"roles":[]},{"id":"103","username":"dave","name":"Dave","enabled":true,"roles":[]},{"id":"104","username":"eve","name":"Eve","enabled":true,"roles":[]}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/contests":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"13337","shortname":"main","name":"Main","state":{"started":"2026-01-01T00:00:00+00:00"}}]`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v4/contests/") && strings.HasSuffix(r.URL.Path, "/organizations"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v4/users/"):
			// Drain the multipart body so the client side completes the
			// request, then echo a minimal id list. Body content is
			// validated separately in api_create_test.go.
			_ = r.ParseMultipartForm(1 << 20)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["ok"]`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	// In api mode the scraper gateway must NOT receive Create/Update/SetPw
	// calls — apiCreateFlow handles all three via bulk endpoints. The
	// only legitimate scraper hit is UpdateUserEmail for rows whose CSV
	// email is non-empty. Returning an error from CreateTeamAndUser
	// surfaces a regression where ImportUsersTeams silently falls back
	// to the scraper path.
	return c, &mockGateway{
		createTeamUserErr: errors.New("api mode: scraper CreateTeamAndUser must not run"),
	}
}

// emptyUsersResource returns a UsersResource backed by a fake server
// whose only job is to hand back an empty users snapshot. Used by the
// scraper mode of the golden test so ImportUsersTeams takes the
// create branch for every CSV row.
func emptyUsersResource(t *testing.T) *apiv4.UsersResource {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]model.User{})
	}))
	t.Cleanup(srv.Close)
	return apiv4.New(apiv4.Options{BaseURL: srv.URL}).Users
}

// pinnedRouter forces every Decide call onto the configured path so the
// test exercises one transport at a time.
type pinnedRouter struct {
	path   importer.Path
	reason string
}

func (p pinnedRouter) Decide(_ context.Context, _ importer.Op, _ importer.Context) importer.Decision {
	return importer.Decision{Path: p.path, Reason: p.reason}
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "importer", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}
