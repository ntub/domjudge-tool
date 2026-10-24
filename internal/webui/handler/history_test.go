package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

type historyEnv struct {
	mux    http.Handler
	oStore *oplog.Store
	pStore *profile.Store
}

func newHistoryEnv(t *testing.T) historyEnv {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB, _ := gdb.DB()
	t.Cleanup(func() { sqlDB.Close() })

	pStore, err := profile.NewStore(gdb)
	if err != nil {
		t.Fatalf("profile.NewStore: %v", err)
	}
	oStore, err := oplog.NewStore(gdb)
	if err != nil {
		t.Fatalf("oplog.NewStore: %v", err)
	}
	_, err = pStore.Create(context.Background(), profile.ServerProfile{
		Name: "test-server",
		Host: "https://test-station.example.org",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	mux := handler.New(pStore)
	handler.RegisterHistory(mux, pStore, oStore)
	return historyEnv{mux: mux, oStore: oStore, pStore: pStore}
}

func TestHistory_EmptyList(t *testing.T) {
	env := newHistoryEnv(t)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "History") {
		t.Error("body should contain 'History'")
	}
}

func TestHistory_ListShowsEntries(t *testing.T) {
	env := newHistoryEnv(t)
	ctx := context.Background()
	_, err := env.oStore.Create(ctx, oplog.OperationLog{
		Type:      oplog.TypeImportUsersTeams,
		ProfileID: 1,
		Status:    oplog.StatusDone,
		RowCount:  5,
	})
	if err != nil {
		t.Fatalf("create oplog: %v", err)
	}

	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "import-users-teams") {
		t.Error("body should contain operation type")
	}
}

func TestHistory_FilterByType(t *testing.T) {
	env := newHistoryEnv(t)
	ctx := context.Background()
	_, _ = env.oStore.Create(ctx, oplog.OperationLog{Type: oplog.TypeImportUsersTeams, ProfileID: 1, Status: oplog.StatusDone})
	_, _ = env.oStore.Create(ctx, oplog.OperationLog{Type: oplog.TypeDownloadProblems, ProfileID: 1, Status: oplog.StatusDone})

	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history?type=download-problems", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// Check for the badge span that only appears in table rows, not in filter <select> options.
	if !strings.Contains(body, `font-mono">download-problems`) {
		t.Error("body should contain download-problems table row")
	}
	if strings.Contains(body, `font-mono">import-users-teams`) {
		t.Error("body should not contain import-users-teams table row after type filter")
	}
}

func TestHistory_Detail(t *testing.T) {
	env := newHistoryEnv(t)
	ctx := context.Background()
	entry, err := env.oStore.Create(ctx, oplog.OperationLog{
		Type:        oplog.TypeImportUsersTeams,
		ProfileID:   1,
		Status:      oplog.StatusDone,
		RowCount:    3,
		SummaryJSON: `{"imported":3}`,
	})
	if err != nil {
		t.Fatalf("create oplog: %v", err)
	}

	path := "/history/" + strconv.Itoa(int(entry.ID))
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "import-users-teams") {
		t.Error("body should contain operation type")
	}
}

func TestHistory_DetailNotFound(t *testing.T) {
	env := newHistoryEnv(t)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history/9999", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestHistory_FilterByProfileAndBadID(t *testing.T) {
	env := newHistoryEnv(t)
	ctx := context.Background()
	secondID, err := env.pStore.Create(ctx, profile.ServerProfile{
		Name: "other-server", Host: "https://other.example.org",
	})
	if err != nil {
		t.Fatalf("create second profile: %v", err)
	}
	var entries []oplog.OperationLog
	for _, row := range []oplog.OperationLog{
		{Type: oplog.TypeDownloadProblems, ProfileID: 1, Status: oplog.StatusDone, RowCount: 3},
		{Type: oplog.TypeDownloadProblems, ProfileID: uint(secondID), Status: oplog.StatusError},
		{Type: oplog.TypeImportUsersTeams, ProfileID: 1, Status: oplog.StatusDone, RowCount: 7},
		{Type: oplog.TypeDownloadProblems, ProfileID: 1, Status: oplog.StatusDone, RowCount: 5},
	} {
		entry, err := env.oStore.Create(ctx, row)
		if err != nil {
			t.Fatalf("seed history: %v", err)
		}
		entries = append(entries, entry)
	}
	for _, tc := range []struct {
		name    string
		query   string
		indices []int
	}{
		{name: "first profile", query: "?profile_id=1", indices: []int{3, 2, 0}},
		{name: "second profile", query: fmt.Sprintf("?profile_id=%d", secondID), indices: []int{1}},
		{name: "profile and type intersect", query: "?profile_id=1&type=download-problems", indices: []int{3, 0}},
		{name: "unfiltered", indices: []int{3, 2, 1, 0}},
		{name: "unknown profile", query: "?profile_id=9999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history"+tc.query, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("history status = %d, want 200", rec.Code)
			}
			body := rec.Body.String()
			wanted := make(map[int]bool, len(tc.indices))
			lastPosition := -1
			for _, index := range tc.indices {
				wanted[index] = true
				link := fmt.Sprintf(`href="/history/%d"`, entries[index].ID)
				position := strings.Index(body, link)
				if position < 0 {
					t.Errorf("history omitted operation %d", entries[index].ID)
				} else if position <= lastPosition {
					t.Errorf("operation %d rendered out of newest-first order", entries[index].ID)
				}
				lastPosition = position
			}
			for index, entry := range entries {
				if !wanted[index] && strings.Contains(body, fmt.Sprintf(`href="/history/%d"`, entry.ID)) {
					t.Errorf("history included excluded operation %d from profile %d/type %s", entry.ID, entry.ProfileID, entry.Type)
				}
			}
		})
	}
	for _, id := range []string{"bad", "0", "-1"} {
		t.Run("invalid detail "+id, func(t *testing.T) {
			rec := httptest.NewRecorder()
			env.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history/"+id, nil))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("history detail status = %d, want 400", rec.Code)
			}
		})
	}
}
