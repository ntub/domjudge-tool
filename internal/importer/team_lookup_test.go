package importer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/model"
)

// teamRecoveryConfig drives teamRecoveryServer: it parameterises the
// list snapshot, the singular /users/{id} response, and the team list
// for a contest, so each PR-8 test case can wire just the surface it
// cares about.
type teamRecoveryConfig struct {
	// listSnapshot is the array returned by GET /api/v4/users.
	listSnapshot []model.User
	// singularUser is the body returned by GET /api/v4/users/{id}.
	// nil → 404. Set TeamID via singularUser.TeamID to simulate a v7
	// build that populates team_id only on the singular endpoint.
	singularUser *model.User
	// teams is the body returned by GET /api/v4/contests/{cid}/teams.
	teams []model.Team
	// contestID, when non-empty, locks which contest path the teams
	// handler accepts. Empty means "any".
	contestID string
	// teamsHits counts how many times the teams endpoint was hit so
	// tests can assert "must not be reached" branches.
	teamsHits *int32
}

// teamRecoveryServer mounts the three endpoints resolveTeamID can
// touch — GET /api/v4/users (list), GET /api/v4/users/{id}
// (singular), and GET /api/v4/contests/{cid}/teams — on a single
// httptest.Server. The returned client is wired against that server;
// its Users / Teams sub-resources are the ones the importer plumbs
// through resolveTeamID.
//
// Kept separate from the existing usersAPIServer helper in
// importer_test.go so PR-8 doesn't perturb the other importer
// regression tests (per CLAUDE.md "Compatibility Locks" / TDD spec).
func teamRecoveryServer(t *testing.T, cfg teamRecoveryConfig) (*apiv4.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v4/users":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(cfg.listSnapshot)
		case strings.HasPrefix(r.URL.Path, "/api/v4/users/"):
			if cfg.singularUser == nil {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(*cfg.singularUser)
		case strings.HasPrefix(r.URL.Path, "/api/v4/contests/") && strings.HasSuffix(r.URL.Path, "/teams"):
			if cfg.teamsHits != nil {
				atomic.AddInt32(cfg.teamsHits, 1)
			}
			if cfg.contestID != "" {
				want := fmt.Sprintf("/api/v4/contests/%s/teams", cfg.contestID)
				if r.URL.Path != want {
					t.Errorf("teams endpoint hit with path %q, want %q", r.URL.Path, want)
				}
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(cfg.teams)
		default:
			t.Errorf("unexpected api path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})
	return c, srv
}

// TestResolveTeamID_SingularEndpointPopulatesTeamID locks the v1.2.1
// recovery contract step 1: when /api/v4/users (list) leaves
// team_id nil but /api/v4/users/{id} populates it (a known v7
// behaviour), the importer recovers the id and feeds it into
// gw.UpdateTeam without falling through to the team-list lookup.
func TestResolveTeamID_SingularEndpointPopulatesTeamID(t *testing.T) {
	teamsHits := int32(0)
	listUser := model.User{
		ID:       "7",
		Username: "alice",
		Name:     "Old",
		Team:     strPtr("team-alpha"),
		TeamID:   nil, // v7 list snapshot omits it.
	}
	singular := listUser
	singular.TeamID = strPtr("99") // singular endpoint populates it.
	api, _ := teamRecoveryServer(t, teamRecoveryConfig{
		listSnapshot: []model.User{listUser},
		singularUser: &singular,
		teamsHits:    &teamsHits,
	})
	gw := &mockGateway{updateTeamID: "99", updateUserID: "7"}

	body := importCSVHeader + "\nalice,Test,alice@example.org,ntub,pw\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, api.Users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if len(gw.updateTeamCalls) != 1 {
		t.Fatalf("UpdateTeam calls = %d, want 1", len(gw.updateTeamCalls))
	}
	got := gw.updateTeamCalls[0].User.TeamID
	if got == nil || *got != "99" {
		t.Errorf("UpdateTeam User.TeamID = %v, want &\"99\"", got)
	}
	if hits := atomic.LoadInt32(&teamsHits); hits != 0 {
		t.Errorf("teams endpoint must not be hit when singular populates team_id; got %d hits", hits)
	}
}

// TestResolveTeamID_FallsBackToTeamListByName locks step 2: when both
// list and singular endpoints leave team_id nil, but the snapshot has
// a team display name AND a contest hint is wired, the importer
// matches against /api/v4/contests/{cid}/teams by team.Name.
func TestResolveTeamID_FallsBackToTeamListByName(t *testing.T) {
	listUser := model.User{
		ID:       "7",
		Username: "alice",
		Name:     "Old",
		Team:     strPtr("alpha"),
		TeamID:   nil,
	}
	singular := listUser // singular still has TeamID == nil.
	api, _ := teamRecoveryServer(t, teamRecoveryConfig{
		listSnapshot: []model.User{listUser},
		singularUser: &singular,
		teams: []model.Team{
			{ID: "5", Name: "other"},
			{ID: "42", Name: "alpha"}, // matches snapshot.Team via Name.
		},
		contestID: "11",
	})
	gw := &mockGateway{updateTeamID: "42", updateUserID: "7"}

	body := importCSVHeader + "\nalice,Test,alice@example.org,ntub,pw\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, api.Users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		API:                api,
		ContestHint:        "11",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if len(gw.updateTeamCalls) != 1 {
		t.Fatalf("UpdateTeam calls = %d, want 1", len(gw.updateTeamCalls))
	}
	got := gw.updateTeamCalls[0].User.TeamID
	if got == nil || *got != "42" {
		t.Errorf("UpdateTeam User.TeamID = %v, want &\"42\"", got)
	}
}

// TestResolveTeamID_FallsBackMatchesDisplayName covers the v7 server
// configuration where team.Name carries an internal slug and
// team.DisplayName carries the human-readable label that the user
// snapshot's team field copies. Matching both fields is the cheap
// defensive shape called out in the PR-8 spec.
func TestResolveTeamID_FallsBackMatchesDisplayName(t *testing.T) {
	listUser := model.User{
		ID:       "8",
		Username: "bob",
		Name:     "Old",
		Team:     strPtr("Team Alpha"), // human-readable label
		TeamID:   nil,
	}
	singular := listUser
	api, _ := teamRecoveryServer(t, teamRecoveryConfig{
		listSnapshot: []model.User{listUser},
		singularUser: &singular,
		teams: []model.Team{
			{ID: "5", Name: "team-beta", DisplayName: strPtr("Team Beta")},
			{ID: "9", Name: "team-alpha", DisplayName: strPtr("Team Alpha")}, // match via DisplayName.
		},
		contestID: "11",
	})
	gw := &mockGateway{updateTeamID: "9", updateUserID: "8"}

	body := importCSVHeader + "\nbob,Test,bob@example.org,ntub,pw\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, api.Users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		API:                api,
		ContestHint:        "11",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if len(gw.updateTeamCalls) != 1 {
		t.Fatalf("UpdateTeam calls = %d, want 1", len(gw.updateTeamCalls))
	}
	got := gw.updateTeamCalls[0].User.TeamID
	if got == nil || *got != "9" {
		t.Errorf("UpdateTeam User.TeamID = %v, want &\"9\"", got)
	}
}

// TestResolveTeamID_BothLookupsFail_ReturnsErrTeamIDMissing locks the
// terminal: when the singular endpoint and the team-list lookup both
// succeed but neither produces a team_id, the importer surfaces
// web.ErrTeamIDMissing wrapped with the offending username, and the
// gateway is never asked to update the team.
func TestResolveTeamID_BothLookupsFail_ReturnsErrTeamIDMissing(t *testing.T) {
	listUser := model.User{
		ID:       "7",
		Username: "alice",
		Name:     "Old",
		Team:     strPtr("alpha"),
		TeamID:   nil,
	}
	singular := listUser
	api, _ := teamRecoveryServer(t, teamRecoveryConfig{
		listSnapshot: []model.User{listUser},
		singularUser: &singular,
		teams:        []model.Team{}, // no teams → no match.
		contestID:    "11",
	})
	gw := &mockGateway{}

	body := importCSVHeader + "\nalice,Test,alice@example.org,ntub,pw\n"
	_, err := importer.ImportUsersTeams(context.Background(), gw, api.Users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		API:                api,
		ContestHint:        "11",
	})
	if err == nil {
		t.Fatal("expected error when both lookups fail to populate team_id")
	}
	if !errors.Is(err, web.ErrTeamIDMissing) {
		t.Errorf("err = %v, want errors.Is(err, web.ErrTeamIDMissing)", err)
	}
	if !strings.Contains(err.Error(), "alice") {
		t.Errorf("error %q must mention the offending username", err.Error())
	}
	if len(gw.updateTeamCalls) != 0 {
		t.Errorf("UpdateTeam was called %d times despite recovery failure, want 0", len(gw.updateTeamCalls))
	}
}

// TestResolveTeamID_TeamNameNil_SkipsListLookup locks the precondition
// guard: when snapshot.Team is nil (no display-name to match against),
// step 2 must not be attempted — the teams endpoint must not be hit —
// and the importer surfaces ErrTeamIDMissing immediately.
func TestResolveTeamID_TeamNameNil_SkipsListLookup(t *testing.T) {
	teamsHits := int32(0)
	listUser := model.User{
		ID:       "7",
		Username: "alice",
		Name:     "Old",
		Team:     nil, // no team display name.
		TeamID:   nil,
	}
	singular := listUser
	api, _ := teamRecoveryServer(t, teamRecoveryConfig{
		listSnapshot: []model.User{listUser},
		singularUser: &singular,
		teams: []model.Team{
			{ID: "9", Name: "should-not-be-matched"},
		},
		contestID: "11",
		teamsHits: &teamsHits,
	})
	gw := &mockGateway{}

	body := importCSVHeader + "\nalice,Test,alice@example.org,ntub,pw\n"
	_, err := importer.ImportUsersTeams(context.Background(), gw, api.Users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		API:                api,
		ContestHint:        "11",
	})
	if err == nil {
		t.Fatal("expected ErrTeamIDMissing when snapshot.Team is nil")
	}
	if !errors.Is(err, web.ErrTeamIDMissing) {
		t.Errorf("err = %v, want errors.Is(err, web.ErrTeamIDMissing)", err)
	}
	if hits := atomic.LoadInt32(&teamsHits); hits != 0 {
		t.Errorf("teams endpoint must not be hit when snapshot.Team is nil; got %d hits", hits)
	}
	if len(gw.updateTeamCalls) != 0 {
		t.Errorf("UpdateTeam was called %d times, want 0", len(gw.updateTeamCalls))
	}
}

// TestImportUsersTeams_UpdateRow_NilTeamID_AutoRecovers is the
// importer-level regression for the v1.2.0 dry-run failure: an
// existing user whose API list snapshot has team_id == nil but whose
// team still exists on the server is recovered via the team-list
// fallback and feeds the resolved id into gw.UpdateTeam.
func TestImportUsersTeams_UpdateRow_NilTeamID_AutoRecovers(t *testing.T) {
	listUser := model.User{
		ID:       "7",
		Username: "alice",
		Name:     "Old",
		Team:     strPtr("alpha"),
		TeamID:   nil,
	}
	singular := listUser // singular also nil — exercise the fallback.
	api, _ := teamRecoveryServer(t, teamRecoveryConfig{
		listSnapshot: []model.User{listUser},
		singularUser: &singular,
		teams:        []model.Team{{ID: "7", Name: "alpha"}},
		contestID:    "11",
	})
	gw := &mockGateway{updateTeamID: "7", updateUserID: "7"}

	body := importCSVHeader + "\nalice,Test User,alice@example.org,ntub,pw\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, api.Users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		API:                api,
		ContestHint:        "11",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if len(gw.updateTeamCalls) != 1 {
		t.Fatalf("UpdateTeam calls = %d, want 1", len(gw.updateTeamCalls))
	}
	got := gw.updateTeamCalls[0].User.TeamID
	if got == nil || *got != "7" {
		t.Errorf("UpdateTeam User.TeamID = %v, want &\"7\"", got)
	}
}

// TestImportUsersTeams_UpdateRow_LookupFails_ReturnsErrTeamIDMissing
// pins the operator-facing error path: when neither lookup populates
// team_id, the import aborts with the sentinel wrapped over the
// username, and gw.UpdateTeam is never reached.
func TestImportUsersTeams_UpdateRow_LookupFails_ReturnsErrTeamIDMissing(t *testing.T) {
	listUser := model.User{
		ID:       "7",
		Username: "alice",
		Name:     "Old",
		Team:     strPtr("alpha"),
		TeamID:   nil,
	}
	singular := listUser
	api, _ := teamRecoveryServer(t, teamRecoveryConfig{
		listSnapshot: []model.User{listUser},
		singularUser: &singular,
		teams:        []model.Team{}, // empty list → no match.
		contestID:    "11",
	})
	gw := &mockGateway{}

	body := importCSVHeader + "\nalice,Test,alice@example.org,ntub,pw\n"
	_, err := importer.ImportUsersTeams(context.Background(), gw, api.Users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		API:                api,
		ContestHint:        "11",
	})
	if err == nil {
		t.Fatal("expected ErrTeamIDMissing when team-list lookup fails")
	}
	if !errors.Is(err, web.ErrTeamIDMissing) {
		t.Errorf("err = %v, want errors.Is(err, web.ErrTeamIDMissing)", err)
	}
	if !strings.Contains(err.Error(), "alice") {
		t.Errorf("error %q must mention the offending username", err.Error())
	}
	if len(gw.updateTeamCalls) != 0 {
		t.Errorf("UpdateTeam was called %d times after unrecoverable nil team_id, want 0", len(gw.updateTeamCalls))
	}
}
