package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/model"
)

func newFragmentHandler(t *testing.T, fetchContests handler.ContestsFetchFunc, fetchUsers handler.UsersFetchFunc, fetchProblems handler.ProblemsFetchFunc, fetchTeams handler.TeamsFetchFunc) http.Handler {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB, _ := gdb.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })

	pStore, err := profile.NewStore(gdb)
	if err != nil {
		t.Fatalf("profile.NewStore: %v", err)
	}
	cid := "1"
	_, err = pStore.Create(context.Background(), profile.ServerProfile{
		Name:      "test-server",
		Host:      "https://test-station.example.org",
		Username:  "admin",
		Password:  "secret",
		ContestID: &cid,
	})
	if err != nil {
		t.Fatalf("create test profile: %v", err)
	}

	mux := handler.New(pStore)
	handler.RegisterAPIFragment(mux, pStore, fetchContests, fetchUsers, fetchProblems, fetchTeams)
	return mux
}

func getContestFragment(t *testing.T, h http.Handler, profileID string) *httptest.ResponseRecorder {
	t.Helper()
	return getContestFragmentOpts(t, h, profileID, false)
}

func getContestFragmentOpts(t *testing.T, h http.Handler, profileID string, followProblems bool) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/fragment/contests"
	var params []string
	if profileID != "" {
		params = append(params, "profile_id="+profileID)
	}
	if followProblems {
		params = append(params, "follow_problems=true")
	}
	if len(params) > 0 {
		target += "?" + strings.Join(params, "&")
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getUserFragment(t *testing.T, h http.Handler, profileID string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/fragment/users"
	if profileID != "" {
		target += "?profile_id=" + profileID
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getUserFragmentWithField(t *testing.T, h http.Handler, profileID, field string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/fragment/users"
	var params []string
	if profileID != "" {
		params = append(params, "profile_id="+profileID)
	}
	if field != "" {
		params = append(params, "field="+field)
	}
	if len(params) > 0 {
		target += "?" + strings.Join(params, "&")
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getTeamFragment(t *testing.T, h http.Handler, profileID string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/fragment/teams"
	if profileID != "" {
		target += "?profile_id=" + profileID
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestFragmentContests_NoProfileID_ReturnsInputFallback(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil, nil)
	rec := getContestFragment(t, h, "")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "contest-fragment") {
		t.Error("body should contain contest-fragment id")
	}
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallback")
	}
}

func TestFragmentContests_InvalidProfileID_ReturnsInputFallback(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil, nil)
	rec := getContestFragment(t, h, "notanumber")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `type="text"`) {
		t.Error("body should contain text input fallback")
	}
}

func TestFragmentContests_APIError_ReturnsInputFallbackWithWarning(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.Contest, error) {
		return nil, errors.New("connection refused")
	}
	h := newFragmentHandler(t, fetch, nil, nil, nil)
	rec := getContestFragment(t, h, "1")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallback")
	}
	if !strings.Contains(body, "Could not load contests") {
		t.Error("body should contain error hint")
	}
}

func TestFragmentContests_Success_ReturnsSelectWithOptions(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.Contest, error) {
		return []model.Contest{
			{ID: "1", ShortName: "icpc2024", Name: "ICPC 2024"},
			{ID: "2", ShortName: "test2025", Name: "Test Contest"},
		}, nil
	}
	h := newFragmentHandler(t, fetch, nil, nil, nil)
	rec := getContestFragment(t, h, "1")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "contest-fragment") {
		t.Error("body should contain contest-fragment id")
	}
	if !strings.Contains(body, "<select") {
		t.Error("body should contain select element")
	}
	if !strings.Contains(body, "ICPC 2024") {
		t.Error("body should contain first contest name")
	}
	if !strings.Contains(body, "Test Contest") {
		t.Error("body should contain second contest name")
	}
}

func TestFragmentContests_DefaultContestPreselected(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.Contest, error) {
		return []model.Contest{
			{ID: "1", ShortName: "icpc2024", Name: "ICPC 2024"},
			{ID: "2", ShortName: "test2025", Name: "Test Contest"},
		}, nil
	}
	h := newFragmentHandler(t, fetch, nil, nil, nil)
	// Profile has ContestID="1" set (from newFragmentHandler seed).
	rec := getContestFragment(t, h, "1")

	body := rec.Body.String()
	// The option for ID "1" should have selected attribute.
	if !strings.Contains(body, `selected`) {
		t.Error("body should contain selected option for profile default contest")
	}
}

func TestFragmentContests_WithoutFollowProblems_NoProblemsHandlers(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.Contest, error) {
		return []model.Contest{{ID: "1", ShortName: "icpc2024", Name: "ICPC 2024"}}, nil
	}
	h := newFragmentHandler(t, fetch, nil, nil, nil)
	rec := getContestFragment(t, h, "1")

	body := rec.Body.String()
	if strings.Contains(body, "api/fragment/problems") {
		t.Error("body should NOT contain problems URL when follow_problems is false")
	}
}

func TestFragmentContests_WithFollowProblems_HasProblemsHandlers(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.Contest, error) {
		return []model.Contest{{ID: "1", ShortName: "icpc2024", Name: "ICPC 2024"}}, nil
	}
	h := newFragmentHandler(t, fetch, nil, nil, nil)
	rec := getContestFragmentOpts(t, h, "1", true)

	body := rec.Body.String()
	if !strings.Contains(body, "api/fragment/problems") {
		t.Error("body should contain problems URL when follow_problems=true")
	}
	if !strings.Contains(body, "data-on:load") {
		t.Error("body should contain data-on:load for auto-load on inject (BUG-4 fix)")
	}
	if !strings.Contains(body, "data-on:change") {
		t.Error("body should contain data-on:change for subsequent contest changes")
	}
}

func TestFragmentUsers_NoProfileID_ReturnsInputFallback(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil, nil)
	rec := getUserFragment(t, h, "")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "username-fragment") {
		t.Error("body should contain username-fragment id")
	}
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallback")
	}
}

func TestFragmentUsers_InvalidProfileID_ReturnsInputFallback(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil, nil)
	rec := getUserFragment(t, h, "notanumber")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `type="text"`) {
		t.Error("body should contain text input fallback")
	}
}

func TestFragmentUsers_APIError_ReturnsInputFallbackWithWarning(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.User, error) {
		return nil, errors.New("connection refused")
	}
	h := newFragmentHandler(t, nil, fetch, nil, nil)
	rec := getUserFragment(t, h, "1")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallback")
	}
	if !strings.Contains(body, "Could not load users") {
		t.Error("body should contain error hint")
	}
}

func TestFragmentUsers_Success_ReturnsSelectWithOptions(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.User, error) {
		return []model.User{
			{Username: "admin", Name: "Administrator"},
			{Username: "user1", Name: "Demo User 1"},
		}, nil
	}
	h := newFragmentHandler(t, nil, fetch, nil, nil)
	rec := getUserFragment(t, h, "1")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "username-fragment") {
		t.Error("body should contain username-fragment id")
	}
	if !strings.Contains(body, "<select") {
		t.Error("body should contain select element")
	}
	if !strings.Contains(body, "Administrator") {
		t.Error("body should contain first user name")
	}
	if !strings.Contains(body, "Demo User 1") {
		t.Error("body should contain second user name")
	}
}

func TestFragmentUsers_MultiSelect_Include(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.User, error) {
		return []model.User{
			{ID: "1", Username: "userA", Name: "User Alice"},
			{ID: "2", Username: "userB", Name: "User Bob"},
		}, nil
	}
	h := newFragmentHandler(t, nil, fetch, nil, nil)
	rec := getUserFragmentWithField(t, h, "1", "include")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="users-include-fragment"`) {
		t.Error("body should contain users-include-fragment id")
	}
	if !strings.Contains(body, `name="include"`) {
		t.Error("body should contain name include")
	}
	if !strings.Contains(body, "window.syncTags") {
		t.Error("body should contain syncTags script reference")
	}
	if !strings.Contains(body, "userA (User Alice)") {
		t.Error("body should contain options label userA (User Alice)")
	}
}

func TestFragmentUsers_MultiSelect_Exclude(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.User, error) {
		return []model.User{
			{ID: "1", Username: "userA", Name: "User Alice"},
		}, nil
	}
	h := newFragmentHandler(t, nil, fetch, nil, nil)
	rec := getUserFragmentWithField(t, h, "1", "exclude")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="users-exclude-fragment"`) {
		t.Error("body should contain users-exclude-fragment id")
	}
	if !strings.Contains(body, `name="exclude"`) {
		t.Error("body should contain name exclude")
	}
}

func TestFragmentUsers_MultiSelect_IDs(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.User, error) {
		return []model.User{
			{ID: "123", Username: "userA", Name: "User Alice"},
		}, nil
	}
	h := newFragmentHandler(t, nil, fetch, nil, nil)
	rec := getUserFragmentWithField(t, h, "1", "ids")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="users-ids-fragment"`) {
		t.Error("body should contain users-ids-fragment id")
	}
	if !strings.Contains(body, `name="ids"`) {
		t.Error("body should contain name ids")
	}
	if !strings.Contains(body, `value="123"`) {
		t.Error("body should contain option value 123")
	}
	if !strings.Contains(body, "userA (123)") {
		t.Error("body should contain option label userA (123)")
	}
}

func TestFragmentUsers_MultiSelect_ErrorFallback(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.User, error) {
		return nil, errors.New("offline")
	}
	h := newFragmentHandler(t, nil, fetch, nil, nil)
	rec := getUserFragmentWithField(t, h, "1", "include")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="users-include-fragment"`) {
		t.Error("body should contain users-include-fragment id")
	}
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should fallback to text input")
	}
	if !strings.Contains(body, "Could not load users") {
		t.Error("body should contain error string")
	}
}

func TestFragmentTeams_NoProfile_ReturnsInputFallback(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil, nil)
	rec := getTeamFragment(t, h, "")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "team-id-fragment") {
		t.Error("body should contain team-id-fragment id")
	}
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input")
	}
}

func TestFragmentTeams_Success(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.Team, error) {
		return []model.Team{
			{ID: "10", Name: "Team Alice"},
			{ID: "20", Name: "Team Bob"},
		}, nil
	}
	h := newFragmentHandler(t, nil, nil, nil, fetch)
	rec := getTeamFragment(t, h, "1")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "team-id-fragment") {
		t.Error("body should contain team-id-fragment id")
	}
	if !strings.Contains(body, "<select") {
		t.Error("body should contain select element")
	}
	if !strings.Contains(body, "Team Alice (10)") {
		t.Error("body should contain option Team Alice (10)")
	}
}

func TestFragmentTeams_ErrorFallback(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.Team, error) {
		return nil, errors.New("offline")
	}
	h := newFragmentHandler(t, nil, nil, nil, fetch)
	rec := getTeamFragment(t, h, "1")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "team-id-fragment") {
		t.Error("body should contain team-id-fragment id")
	}
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallback")
	}
}

func TestFragmentUsers_CombinedIncludeExclude_PatchesBothFragments(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile) ([]model.User, error) {
		return []model.User{
			{ID: "1", Username: "userA", Name: "User Alice"},
		}, nil
	}
	h := newFragmentHandler(t, nil, fetch, nil, nil)
	rec := getUserFragmentWithField(t, h, "1", "include,exclude")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="users-include-fragment"`) {
		t.Error("body should contain users-include-fragment")
	}
	if !strings.Contains(body, `id="users-exclude-fragment"`) {
		t.Error("body should contain users-exclude-fragment")
	}
	if !strings.Contains(body, "userA (User Alice)") {
		t.Error("body should contain option label")
	}
}

func getExportUsersInitFragment(t *testing.T, h http.Handler, profileID string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/fragment/export-users-init"
	if profileID != "" {
		target += "?profile_id=" + profileID
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestFragmentExportUsersInit_Success_PatchesBothFragments(t *testing.T) {
	fetchUsers := func(_ context.Context, _ profile.ServerProfile) ([]model.User, error) {
		return []model.User{{ID: "1", Username: "admin", Name: "Admin"}}, nil
	}
	fetchTeams := func(_ context.Context, _ profile.ServerProfile) ([]model.Team, error) {
		return []model.Team{{ID: "10", Name: "Team A"}}, nil
	}
	h := newFragmentHandler(t, nil, fetchUsers, nil, fetchTeams)
	rec := getExportUsersInitFragment(t, h, "1")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="users-ids-fragment"`) {
		t.Error("body should contain users-ids-fragment")
	}
	if !strings.Contains(body, `id="team-id-fragment"`) {
		t.Error("body should contain team-id-fragment")
	}
	if !strings.Contains(body, "admin (1)") {
		t.Error("body should contain user option label")
	}
	if !strings.Contains(body, "Team A (10)") {
		t.Error("body should contain team option label")
	}
}

func TestFragmentExportUsersInit_NoProfile_ReturnsBothFallbacks(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil, nil)
	rec := getExportUsersInitFragment(t, h, "")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="users-ids-fragment"`) {
		t.Error("body should contain users-ids-fragment fallback")
	}
	if !strings.Contains(body, `id="team-id-fragment"`) {
		t.Error("body should contain team-id-fragment fallback")
	}
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallbacks")
	}
}
