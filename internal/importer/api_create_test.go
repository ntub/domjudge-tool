package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// TestResolveContestID_HintProvided_PassThrough verifies that a numeric
// hint short-circuits the contests List call — the test http server
// fails the test if hit. cfg.ContestID="2" should resolve to "2".
func TestResolveContestID_HintProvided_PassThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected hit on %s for numeric hint", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	got, err := resolveContestID(context.Background(), c, "2")
	if err != nil {
		t.Fatalf("resolveContestID: %v", err)
	}
	if got != "2" {
		t.Fatalf("got %q, want 2", got)
	}
}

// TestResolveContestID_EmptyHint_OneActive_AutoSelect locks the
// "single active contest is auto-selected" backward-compat path.
func TestResolveContestID_EmptyHint_OneActive_AutoSelect(t *testing.T) {
	const list = `[
	  {"id":"1","shortname":"old","name":"Old","state":{"started":"2025-01-01T00:00:00+00:00","ended":"2025-02-01T00:00:00+00:00"}},
	  {"id":"7","shortname":"current","name":"Current","state":{"started":"2026-01-01T00:00:00+00:00"}}
	]`
	srv := contestsListServer(t, list)
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	got, err := resolveContestID(context.Background(), c, "")
	if err != nil {
		t.Fatalf("resolveContestID: %v", err)
	}
	if got != "7" {
		t.Fatalf("got %q, want 7", got)
	}
}

// TestResolveContestID_EmptyHint_NoActive_Error covers the
// zero-active branch: every contest has both started and ended set,
// so resolveContestID returns an error mentioning "set contest_id".
func TestResolveContestID_EmptyHint_NoActive_Error(t *testing.T) {
	const list = `[
	  {"id":"1","shortname":"old","name":"Old","state":{"started":"2025-01-01T00:00:00+00:00","ended":"2025-02-01T00:00:00+00:00"}}
	]`
	srv := contestsListServer(t, list)
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	_, err := resolveContestID(context.Background(), c, "")
	if err == nil {
		t.Fatal("expected error when no active contest")
	}
	if !strings.Contains(err.Error(), "set contest_id") {
		t.Errorf("err %q should mention 'set contest_id'", err.Error())
	}
}

// TestResolveContestID_EmptyHint_MultipleActive_Error covers the
// multi-active branch. The error must enumerate the candidate ids so
// the operator knows which one to pin.
func TestResolveContestID_EmptyHint_MultipleActive_Error(t *testing.T) {
	const list = `[
	  {"id":"1","shortname":"a","name":"A","state":{"started":"2026-01-01T00:00:00+00:00"}},
	  {"id":"2","shortname":"b","name":"B","state":{"started":"2026-01-01T00:00:00+00:00"}}
	]`
	srv := contestsListServer(t, list)
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	_, err := resolveContestID(context.Background(), c, "")
	if err == nil {
		t.Fatal("expected error when multiple active contests")
	}
	if !strings.Contains(err.Error(), "set contest_id") {
		t.Errorf("err %q should mention 'set contest_id'", err.Error())
	}
	if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "2") {
		t.Errorf("err %q should enumerate active ids 1 and 2", err.Error())
	}
}

// TestFilterActiveContests_TimeFields covers the v7 reality: the
// /api/v4/contests list response on DOMjudge 7.3.x doesn't include the
// nested `state` object — only `start_time` / `end_time`. Active means
// "started ≤ now < ended" using those wall-clock fields. The legacy
// state-based path stays as a fallback so older or alt-server responses
// keep working (locked by TestFilterActiveContests_LegacyStateRespected).
func TestFilterActiveContests_TimeFields(t *testing.T) {
	now := time.Now()
	rfc := func(d time.Duration) *string {
		s := now.Add(d).UTC().Format(time.RFC3339)
		return &s
	}
	cases := []struct {
		name      string
		contest   model.Contest
		wantCount int
	}{
		{
			name:      "StartTimeNilEndTimeNil_NotActive",
			contest:   model.Contest{ID: "1"},
			wantCount: 0,
		},
		{
			name:      "StartTimeBeforeNow_EndTimeAfterNow_Active",
			contest:   model.Contest{ID: "2", StartTime: rfc(-time.Hour), EndTime: rfc(time.Hour)},
			wantCount: 1,
		},
		{
			name:      "EndTimeBeforeNow_NotActive",
			contest:   model.Contest{ID: "3", StartTime: rfc(-2 * time.Hour), EndTime: rfc(-time.Hour)},
			wantCount: 0,
		},
		{
			name:      "EndTimeNil_StartTimeBeforeNow_Active",
			contest:   model.Contest{ID: "4", StartTime: rfc(-time.Hour)},
			wantCount: 1,
		},
		{
			name:      "StartTimeAfterNow_NotActive",
			contest:   model.Contest{ID: "5", StartTime: rfc(time.Hour)},
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterActiveContests([]model.Contest{tc.contest})
			if len(got) != tc.wantCount {
				t.Fatalf("filterActiveContests = %d active, want %d (contest=%+v)", len(got), tc.wantCount, tc.contest)
			}
		})
	}
}

// TestFilterActiveContests_LegacyStateRespected pins the fallback
// behaviour for responses that DO carry the nested state object (older
// responses or non-list endpoints). When StartTime is nil but
// State.Started is set, the contest is still treated as active.
func TestFilterActiveContests_LegacyStateRespected(t *testing.T) {
	started := "2026-01-01T00:00:00+00:00"
	contests := []model.Contest{
		{ID: "1", State: model.ContestState{Started: &started}},
		{ID: "2"},
	}
	got := filterActiveContests(contests)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("got %+v, want one active contest with ID=1", got)
	}
}

// TestFilterActiveContests_PrefersTimeOverState locks the precedence
// rule: when StartTime is set, the time-fields path decides — even if a
// stale state.started would say "active". Avoids double-counting.
func TestFilterActiveContests_PrefersTimeOverState(t *testing.T) {
	now := time.Now()
	pastEnd := now.Add(-time.Hour).UTC().Format(time.RFC3339)
	pastStart := now.Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	stale := "2026-01-01T00:00:00+00:00"
	contests := []model.Contest{
		{
			ID:        "1",
			StartTime: &pastStart,
			EndTime:   &pastEnd,
			State:     model.ContestState{Started: &stale}, // would say "still active"
		},
	}
	got := filterActiveContests(contests)
	if len(got) != 0 {
		t.Fatalf("got %d active, want 0 (time-fields say ended)", len(got))
	}
}

// TestResolveContestID_EmptyHint_TimeBased_AutoSelect drives the v7
// scenario end-to-end: list response carries start_time/end_time only,
// resolveContestID picks the active one.
func TestResolveContestID_EmptyHint_TimeBased_AutoSelect(t *testing.T) {
	now := time.Now().UTC()
	pastStart := now.Add(-time.Hour).Format(time.RFC3339)
	futureEnd := now.Add(time.Hour).Format(time.RFC3339)
	body := fmt.Sprintf(`[{"id":"42","shortname":"main","name":"Main","start_time":%q,"end_time":%q}]`, pastStart, futureEnd)
	srv := contestsListServer(t, body)
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	got, err := resolveContestID(context.Background(), c, "")
	if err != nil {
		t.Fatalf("resolveContestID: %v", err)
	}
	if got != "42" {
		t.Fatalf("got %q, want 42", got)
	}
}

// TestApiCreateFlow_HappyPath_3Rows is the contract test: same input
// CSV drives 5 POST calls (groups, organizations, teams, accounts) +
// 2 GET calls (organizations list, users list). The fake server captures
// each multipart body so the test asserts the TSV bytes match the
// pure-builder output.
func TestApiCreateFlow_HappyPath_3Rows(t *testing.T) {
	rows := []model.CreateUser{
		{Username: "alice", Name: "Alice", Affiliation: stringPtr("ACME"), Password: stringPtr("pw-alice")},
		{Username: "bob", Name: "Bob", Affiliation: stringPtr("ACME"), Password: stringPtr("pw-bob")},
		{Username: "carol", Name: "Carol", Affiliation: stringPtr("NTUB"), Password: stringPtr("pw-carol")},
	}

	cap := newAPICapture()
	srv := newAPIFakeServer(t, cap, apiFakeOpts{
		contests:      `[{"id":"2","shortname":"main","name":"Main","state":{"started":"2026-01-01T00:00:00+00:00"}}]`,
		organizations: `[{"id":"1","shortname":"NTUB","name":"NTUB","country":"TWN"}]`,
		users:         `[]`,
	})
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGW{}
	categoryID := 13337
	opts := ImportOptions{
		CategoryID:         &categoryID,
		AffiliationCountry: "TWN",
	}

	out, err := apiCreateFlow(context.Background(), c, gw, rows, "2", opts)
	if err != nil {
		t.Fatalf("apiCreateFlow: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}

	// All three rows reported as new (IsExist=&false) and pass passwords through.
	for i, row := range out {
		if row.IsExist == nil || *row.IsExist {
			t.Errorf("out[%d].IsExist = %v, want &false", i, row.IsExist)
		}
		if row.Password == nil || *row.Password == "" {
			t.Errorf("out[%d].Password missing", i)
		}
	}

	// Group import fired exactly once with the configured CategoryID.
	if got := cap.groupsTSV(); !strings.Contains(got, "13337\tp1.5-imported\n") {
		t.Errorf("groups.tsv missing expected row, got %q", got)
	}
	// Organizations import fired with the missing org only (ACME).
	missing := cap.orgsBody()
	if len(missing) != 1 || missing[0].Name != "ACME" {
		t.Errorf("organizations payload = %+v, want one ACME entry", missing)
	}
	// Teams TSV body is byte-identical to formatTeamsTSV output for the same input.
	want := formatTeamsTSV(rows, "13337", map[string]model.Organization{
		"ACME": missing[0],
		"NTUB": {ID: "1", ShortName: "NTUB", Name: "NTUB", Country: "TWN"},
	})
	if got := cap.teamsTSV(); got != want {
		t.Errorf("teams2.tsv\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
	if got := cap.accountsTSV(); got != formatAccountsTSV(rows) {
		t.Errorf("accounts.tsv\n--- got ---\n%q\n--- want ---\n%q", got, formatAccountsTSV(rows))
	}
}

// TestApiCreateFlow_EmailRow_FallsBackToScraper locks the contract that
// rows whose CSV email is non-empty trigger gw.UpdateUserEmail once
// the API import completes — the API has no email column so we keep
// the scraper for that single field.
func TestApiCreateFlow_EmailRow_FallsBackToScraper(t *testing.T) {
	rows := []model.CreateUser{
		{Username: "alice", Name: "Alice", Email: stringPtr("alice@example.org"), Affiliation: stringPtr("NTUB"), Password: stringPtr("pw")},
		{Username: "bob", Name: "Bob", Affiliation: stringPtr("NTUB"), Password: stringPtr("pw")},
	}
	cap := newAPICapture()
	srv := newAPIFakeServer(t, cap, apiFakeOpts{
		organizations: `[{"id":"1","shortname":"NTUB","name":"NTUB","country":"TWN"}]`,
		users:         `[{"id":"55","username":"alice","name":"Alice","enabled":true,"roles":[]},{"id":"56","username":"bob","name":"Bob","enabled":true,"roles":[]}]`,
	})
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGW{}
	if _, err := apiCreateFlow(context.Background(), c, gw, rows, "2", ImportOptions{}); err != nil {
		t.Fatalf("apiCreateFlow: %v", err)
	}

	if len(gw.emailCalls) != 1 {
		t.Fatalf("UpdateUserEmail called %d times, want 1", len(gw.emailCalls))
	}
	if gw.emailCalls[0].userID != "55" {
		t.Errorf("UpdateUserEmail userID = %q, want 55", gw.emailCalls[0].userID)
	}
	if gw.emailCalls[0].email != "alice@example.org" {
		t.Errorf("UpdateUserEmail email = %q, want alice@example.org", gw.emailCalls[0].email)
	}
}

// TestApiCreateFlow_NoCategoryID_SkipsGroupsImport pins that the
// optional groups upsert only fires when the operator passed
// --category-id. Without it the import endpoint must not be hit.
func TestApiCreateFlow_NoCategoryID_SkipsGroupsImport(t *testing.T) {
	rows := []model.CreateUser{{Username: "alice", Name: "Alice", Password: stringPtr("pw")}}
	cap := newAPICapture()
	srv := newAPIFakeServer(t, cap, apiFakeOpts{
		organizations: `[]`,
		users:         `[]`,
	})
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGW{}
	if _, err := apiCreateFlow(context.Background(), c, gw, rows, "2", ImportOptions{}); err != nil {
		t.Fatalf("apiCreateFlow: %v", err)
	}
	if cap.groupsTSV() != "" {
		t.Errorf("groups.tsv was hit despite no CategoryID, body = %q", cap.groupsTSV())
	}
}

// TestApiCreateFlow_NoDuplicateOrgsOnReimport is the v1.1.1 #4
// regression: re-importing a CSV whose affiliations already exist in
// the contest must not call the bulk POST /users/organizations
// endpoint at all. The live v7 station observed the opposite (extra
// id=4 / id=5 duplicate orgs); the protective test pins the expected
// behaviour even after the v1.1.1 default revert keeps this code
// dormant — Phase 2 reactivation needs this lock in place.
func TestApiCreateFlow_NoDuplicateOrgsOnReimport(t *testing.T) {
	rows := []model.CreateUser{
		{Username: "alice", Name: "Alice", Affiliation: stringPtr("一甲"), Password: stringPtr("pw")},
		{Username: "bob", Name: "Bob", Affiliation: stringPtr("二乙"), Password: stringPtr("pw")},
	}
	cap := newAPICapture()
	srv := newAPIFakeServer(t, cap, apiFakeOpts{
		organizations: `[
			{"id":"2","shortname":"一甲","name":"一甲","country":"TWN"},
			{"id":"3","shortname":"二乙","name":"二乙","country":"TWN"}
		]`,
		users: `[]`,
	})
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGW{}
	if _, err := apiCreateFlow(context.Background(), c, gw, rows, "2", ImportOptions{}); err != nil {
		t.Fatalf("apiCreateFlow: %v", err)
	}
	if cap.orgsCalls() != 0 {
		t.Errorf("organizations import called %d times, want 0 (orgs already exist)", cap.orgsCalls())
	}
}

// TestApiCreateFlow_DedupTrimsTrailingWhitespace covers the most
// likely root cause for the live v7 duplicate-org bug: CSV parsers
// often leave \r or trailing spaces on the last column, so the dedup
// lookup misses an existing org whose name is byte-equal after trim.
func TestApiCreateFlow_DedupTrimsTrailingWhitespace(t *testing.T) {
	rows := []model.CreateUser{
		{Username: "alice", Name: "Alice", Affiliation: stringPtr("一甲 "), Password: stringPtr("pw")},
		{Username: "bob", Name: "Bob", Affiliation: stringPtr("一甲\r"), Password: stringPtr("pw")},
	}
	cap := newAPICapture()
	srv := newAPIFakeServer(t, cap, apiFakeOpts{
		organizations: `[{"id":"2","shortname":"一甲","name":"一甲","country":"TWN"}]`,
		users:         `[]`,
	})
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGW{}
	if _, err := apiCreateFlow(context.Background(), c, gw, rows, "2", ImportOptions{}); err != nil {
		t.Fatalf("apiCreateFlow: %v", err)
	}
	if cap.orgsCalls() != 0 {
		t.Errorf("organizations import called %d times, want 0 (whitespace-only diff must dedup)", cap.orgsCalls())
	}
}

// TestApiCreateFlow_NoMissingOrgs_SkipsOrganizationsImport pins that
// when every CSV affiliation already exists in the contest's org list,
// the bulk organizations endpoint must not be POSTed.
func TestApiCreateFlow_NoMissingOrgs_SkipsOrganizationsImport(t *testing.T) {
	rows := []model.CreateUser{
		{Username: "alice", Name: "Alice", Affiliation: stringPtr("NTUB"), Password: stringPtr("pw")},
	}
	cap := newAPICapture()
	srv := newAPIFakeServer(t, cap, apiFakeOpts{
		organizations: `[{"id":"1","shortname":"NTUB","name":"NTUB","country":"TWN"}]`,
		users:         `[]`,
	})
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGW{}
	if _, err := apiCreateFlow(context.Background(), c, gw, rows, "2", ImportOptions{}); err != nil {
		t.Fatalf("apiCreateFlow: %v", err)
	}
	if cap.orgsCalls() != 0 {
		t.Errorf("organizations import called %d times, want 0", cap.orgsCalls())
	}
}

// TestApiCreateFlow_GenerateNewPassword_PopulatesOutput pins that an
// empty CSV password column yields a generated password in the
// returned []CreateUser, mirroring scraper-flow behaviour.
func TestApiCreateFlow_GenerateNewPassword_PopulatesOutput(t *testing.T) {
	rows := []model.CreateUser{{Username: "alice", Name: "Alice"}}
	cap := newAPICapture()
	srv := newAPIFakeServer(t, cap, apiFakeOpts{
		organizations: `[]`,
		users:         `[]`,
	})
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGW{}
	out, err := apiCreateFlow(context.Background(), c, gw, rows, "2", ImportOptions{})
	if err != nil {
		t.Fatalf("apiCreateFlow: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].Password == nil || *out[0].Password == "" {
		t.Errorf("out[0].Password should be populated by generation")
	}
}

// TestApiCreateFlow_ContextCancelled propagates a cancelled context
// from the very first API call, before any state mutates.
func TestApiCreateFlow_ContextCancelled(t *testing.T) {
	rows := []model.CreateUser{{Username: "alice", Name: "Alice", Password: stringPtr("pw")}}
	cap := newAPICapture()
	srv := newAPIFakeServer(t, cap, apiFakeOpts{
		organizations: `[]`,
		users:         `[]`,
	})
	t.Cleanup(srv.Close)

	c := apiv4.New(apiv4.Options{BaseURL: srv.URL})
	gw := &mockGW{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := apiCreateFlow(ctx, c, gw, rows, "2", ImportOptions{})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// --- helpers -----------------------------------------------------------

// apiFakeOpts configures the responses returned by newAPIFakeServer.
// Empty strings mean "respond with empty array".
type apiFakeOpts struct {
	contests      string // GET /contests
	organizations string // GET /contests/{cid}/organizations
	users         string // GET /users
}

// newAPIFakeServer spins up a single httptest.Server that dispatches on
// (method, path) to the right capture slot. The capture returned to the
// caller exposes thread-safe accessors for inspection.
func newAPIFakeServer(t *testing.T, cap *apiCapture, opts apiFakeOpts) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/contests":
			respondJSON(w, opts.contests, "[]")
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v4/contests/") && strings.HasSuffix(r.URL.Path, "/organizations"):
			respondJSON(w, opts.organizations, "[]")
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/users":
			respondJSON(w, opts.users, "[]")
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/users/groups":
			cap.captureMultipart(t, r, "tsv", &cap.groupsBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["1"]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/users/organizations":
			cap.captureMultipartJSON(t, r, "json", &cap.orgsPayload)
			cap.incrOrgsCalls()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["acme"]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/users/teams":
			cap.captureMultipart(t, r, "tsv", &cap.teamsBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["alice","bob","carol"]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/users/accounts":
			cap.captureMultipart(t, r, "tsv", &cap.accountsBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["alice","bob","carol"]`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

// contestsListServer returns a server that only handles GET /contests.
// Used by resolveContestID tests where one endpoint suffices.
func contestsListServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/contests" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func respondJSON(w http.ResponseWriter, body, fallback string) {
	w.Header().Set("Content-Type", "application/json")
	if body == "" {
		_, _ = w.Write([]byte(fallback))
		return
	}
	_, _ = w.Write([]byte(body))
}

// apiCapture records the multipart bodies of POST endpoints.
type apiCapture struct {
	mu             sync.Mutex
	groupsBody     string
	teamsBody      string
	accountsBody   string
	orgsPayload    []model.Organization
	orgsCallsCount int
}

func newAPICapture() *apiCapture { return &apiCapture{} }

func (c *apiCapture) groupsTSV() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.groupsBody
}

func (c *apiCapture) teamsTSV() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.teamsBody
}

func (c *apiCapture) accountsTSV() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accountsBody
}

func (c *apiCapture) orgsBody() []model.Organization {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]model.Organization, len(c.orgsPayload))
	copy(out, c.orgsPayload)
	return out
}

func (c *apiCapture) orgsCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.orgsCallsCount
}

func (c *apiCapture) incrOrgsCalls() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.orgsCallsCount++
}

func (c *apiCapture) captureMultipart(t *testing.T, r *http.Request, field string, dst *string) {
	t.Helper()
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}
	f, _, err := r.FormFile(field)
	if err != nil {
		t.Fatalf("FormFile(%s): %v", field, err)
	}
	defer func() { _ = f.Close() }()
	body, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll(%s): %v", field, err)
	}
	c.mu.Lock()
	*dst = string(body)
	c.mu.Unlock()
}

func (c *apiCapture) captureMultipartJSON(t *testing.T, r *http.Request, field string, dst *[]model.Organization) {
	t.Helper()
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}
	f, _, err := r.FormFile(field)
	if err != nil {
		t.Fatalf("FormFile(%s): %v", field, err)
	}
	defer func() { _ = f.Close() }()
	var payload []model.Organization
	if err := json.NewDecoder(f).Decode(&payload); err != nil {
		t.Fatalf("Decode(%s): %v", field, err)
	}
	c.mu.Lock()
	*dst = payload
	c.mu.Unlock()
}

// mockGW is a minimal web.Gateway test double scoped to the api_create
// tests: only Login and UpdateUserEmail need real behaviour; the other
// methods satisfy the interface with no-ops.
type mockGW struct {
	emailCalls []emailCall
}

type emailCall struct {
	userID string
	email  string
}

var _ web.Gateway = (*mockGW)(nil)

func (m *mockGW) Login(context.Context) error { return nil }

func (m *mockGW) GetAffiliations(context.Context) ([]model.Affiliation, error) { return nil, nil }

func (m *mockGW) GetAffiliation(context.Context, string) (*model.Affiliation, error) {
	return nil, nil
}

func (m *mockGW) CreateAffiliation(context.Context, string, string, string) (*model.Affiliation, error) {
	return nil, nil
}

func (m *mockGW) GetProblems(context.Context, web.ProblemFilter) ([]model.ProblemItem, error) {
	return nil, nil
}

func (m *mockGW) SetUserPassword(context.Context, string, string, []int, bool) error { return nil }

func (m *mockGW) UpdateUserEmail(_ context.Context, userID, email string) error {
	m.emailCalls = append(m.emailCalls, emailCall{userID: userID, email: email})
	return nil
}

func (m *mockGW) DeleteUsers(context.Context, []string, []string) error { return nil }

func (m *mockGW) CreateTeamAndUser(context.Context, model.CreateUser, int, int, bool) (string, string, error) {
	return "", "", nil
}

func (m *mockGW) UpdateTeam(context.Context, model.User, int, int, bool) (string, string, error) {
	return "", "", nil
}

func (m *mockGW) DeleteTeams(context.Context, []string, []string) error { return nil }

func (m *mockGW) Download(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}
