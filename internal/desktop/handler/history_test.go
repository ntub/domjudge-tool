package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
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
