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

func newFragmentHandler(t *testing.T, fetchContests handler.ContestsFetchFunc, fetchUsers handler.UsersFetchFunc, fetchProblems handler.ProblemsFetchFunc) http.Handler {
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
	handler.RegisterAPIFragment(mux, pStore, fetchContests, fetchUsers, fetchProblems)
	return mux
}

func getContestFragment(t *testing.T, h http.Handler, profileID string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/fragment/contests"
	if profileID != "" {
		target += "?profile_id=" + profileID
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

func TestFragmentContests_NoProfileID_ReturnsInputFallback(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil)
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
	h := newFragmentHandler(t, nil, nil, nil)
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
	h := newFragmentHandler(t, fetch, nil, nil)
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
	h := newFragmentHandler(t, fetch, nil, nil)
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
	h := newFragmentHandler(t, fetch, nil, nil)
	// Profile has ContestID="1" set (from newFragmentHandler seed).
	rec := getContestFragment(t, h, "1")

	body := rec.Body.String()
	// The option for ID "1" should have selected attribute.
	if !strings.Contains(body, `selected`) {
		t.Error("body should contain selected option for profile default contest")
	}
}

func TestFragmentUsers_NoProfileID_ReturnsInputFallback(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil)
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
	h := newFragmentHandler(t, nil, nil, nil)
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
	h := newFragmentHandler(t, nil, fetch, nil)
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
	h := newFragmentHandler(t, nil, fetch, nil)
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
