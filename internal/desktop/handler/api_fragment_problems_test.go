package handler_test

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/model"
)

func getProblemsFragment(t *testing.T, h http.Handler, profileID, contestID, field string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/fragment/problems?field=" + field
	if profileID != "" {
		target += "&profile_id=" + profileID
	}
	if contestID != "" {
		target += "&contest_id=" + contestID
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestFragmentProblems_NoProfileID_ReturnsInputFallback(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil, nil)
	rec := getProblemsFragment(t, h, "", "", "problem_ids")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "problems-problem_ids-fragment") {
		t.Error("body should contain correct fragment ID")
	}
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallback")
	}
}

func TestFragmentProblems_InvalidProfileID_ReturnsInputFallback(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil, nil)
	rec := getProblemsFragment(t, h, "9999", "", "only")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "problems-only-fragment") {
		t.Error("body should contain correct fragment ID")
	}
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallback")
	}
}

func TestFragmentProblems_NoContestID_ReturnsInputFallback(t *testing.T) {
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB, _ := gdb.DB()
	defer func() { _ = sqlDB.Close() }()

	pStore, err := profile.NewStore(gdb)
	if err != nil {
		t.Fatalf("profile.NewStore: %v", err)
	}
	pid, err := pStore.Create(context.Background(), profile.ServerProfile{
		Name:      "no-contest-server",
		Host:      "https://test-station.example.org",
		Username:  "admin",
		Password:  "secret",
		ContestID: nil,
	})
	if err != nil {
		t.Fatalf("create test profile: %v", err)
	}

	mux := handler.New(pStore)
	handler.RegisterAPIFragment(mux, pStore, nil, nil, func(_ context.Context, _ profile.ServerProfile, _ string) ([]model.Problem, error) {
		return []model.Problem{{ID: "1", Label: "A", Name: "HelloWorld"}}, nil
	}, nil)

	rec := getProblemsFragment(t, mux, fmt.Sprintf("%d", pid), "", "exclude")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "problems-exclude-fragment") {
		t.Error("body should contain exclude fragment ID")
	}
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallback when no contest ID")
	}
}

func TestFragmentProblems_APIError_ReturnsInputFallbackWithWarning(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile, _ string) ([]model.Problem, error) {
		return nil, errors.New("connection refused")
	}
	h := newFragmentHandler(t, nil, nil, fetch, nil)
	rec := getProblemsFragment(t, h, "1", "2", "problem_ids")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `type="text"`) {
		t.Error("body should contain text input fallback")
	}
	if !strings.Contains(body, "Could not load problems") {
		t.Error("body should contain error warning message")
	}
}

func TestFragmentProblems_Success_ReturnsTagMultiSelect(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile, _ string) ([]model.Problem, error) {
		return []model.Problem{
			{ID: "1", Label: "A", Name: "HelloWorld"},
			{ID: "2", Label: "B", Name: "AplusB"},
		}, nil
	}
	h := newFragmentHandler(t, nil, nil, fetch, nil)
	rec := getProblemsFragment(t, h, "1", "2", "only")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "problems-only-fragment") {
		t.Error("body should contain only fragment ID")
	}
	if !strings.Contains(body, "<select") {
		t.Error("body should contain select element")
	}
	if !strings.Contains(body, "A - HelloWorld") {
		t.Error("body should format first option Label correctly")
	}
	if !strings.Contains(body, "B - AplusB") {
		t.Error("body should format second option Label correctly")
	}
	if !strings.Contains(body, "window.syncTags") {
		t.Error("body should contain helper script reference")
	}
}

func TestFragmentProblems_InvalidField_Returns400(t *testing.T) {
	h := newFragmentHandler(t, nil, nil, nil, nil)
	rec := getProblemsFragment(t, h, "", "", "bogus")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestFragmentProblems_CombinedFields_PatchesBothFragments(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile, _ string) ([]model.Problem, error) {
		return []model.Problem{
			{ID: "1", Label: "A", Name: "HelloWorld"},
		}, nil
	}
	h := newFragmentHandler(t, nil, nil, fetch, nil)
	rec := getProblemsFragment(t, h, "1", "2", "only,exclude")

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "problems-only-fragment") {
		t.Error("body should contain only fragment ID")
	}
	if !strings.Contains(body, "problems-exclude-fragment") {
		t.Error("body should contain exclude fragment ID")
	}
}

func TestFragmentProblems_LabelFormats(t *testing.T) {
	fetch := func(_ context.Context, _ profile.ServerProfile, _ string) ([]model.Problem, error) {
		return []model.Problem{
			{ID: "p1", ShortName: "S1", Name: "Prob1"},
			{ID: "p2", Name: "Prob2Only"},
			{ID: "p3"},
		}, nil
	}
	h := newFragmentHandler(t, nil, nil, fetch, nil)
	rec := getProblemsFragment(t, h, "1", "2", "only")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	handlerTag(t, body, "select", "id", "problems-only-fragment-options")
	input := handlerTag(t, body, "input", "name", "only")
	if input["type"] != "hidden" {
		t.Errorf("problem selection submission type = %q, want hidden selected IDs", input["type"])
	}
	for _, tc := range []struct {
		id    string
		parts []string
	}{
		{id: "p1", parts: []string{"S1", "Prob1"}},
		{id: "p2", parts: []string{"Prob2Only"}},
		{id: "p3", parts: []string{"p3"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			handlerTag(t, body, "option", "value", tc.id)
			pattern := regexp.MustCompile(`(?s)<option\b[^>]*\bvalue="` + regexp.QuoteMeta(tc.id) + `"[^>]*>(.*?)</option>`)
			match := pattern.FindStringSubmatch(body)
			if len(match) != 2 {
				t.Fatalf("problem %s has no selectable label", tc.id)
			}
			label := html.UnescapeString(match[1])
			for _, part := range tc.parts {
				if !strings.Contains(label, part) {
					t.Errorf("problem %s label = %q, missing identifying data %q", tc.id, label, part)
				}
			}
		})
	}
}
