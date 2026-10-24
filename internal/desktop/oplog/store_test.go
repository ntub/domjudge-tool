package oplog_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
)

func newStore(t *testing.T) *oplog.Store {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("raw DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	s, err := oplog.NewStore(gdb)
	if err != nil {
		t.Fatalf("oplog.NewStore: %v", err)
	}
	return s
}

func TestCreateAndGet(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	entry, err := s.Create(ctx, oplog.OperationLog{
		Type:      oplog.TypeImportUsersTeams,
		ProfileID: 1,
		Status:    oplog.StatusRunning,
		CsvPath:   "/tmp/test.csv",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if entry.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := s.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Type != oplog.TypeImportUsersTeams {
		t.Errorf("Type = %q, want %q", got.Type, oplog.TypeImportUsersTeams)
	}
	if got.Status != oplog.StatusRunning {
		t.Errorf("Status = %q, want %q", got.Status, oplog.StatusRunning)
	}
}

func TestUpdateStatus(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	entry, err := s.Create(ctx, oplog.OperationLog{
		Type:       oplog.TypeImportUsersTeams,
		Status:     oplog.StatusRunning,
		CsvPath:    "pending.csv",
		ParamsJSON: `{"profile_id":1}`,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Now()
	err = s.FinishOK(ctx, entry.ID, 10, `{"InputRows":10}`)
	if err != nil {
		t.Fatalf("FinishOK: %v", err)
	}

	got, err := s.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != oplog.StatusDone {
		t.Errorf("Status = %q, want %q", got.Status, oplog.StatusDone)
	}
	if got.RowCount != 10 {
		t.Errorf("RowCount = %d, want 10", got.RowCount)
	}
	if got.SummaryJSON != `{"InputRows":10}` {
		t.Errorf("SummaryJSON = %q", got.SummaryJSON)
	}
	if got.FinishedAt == nil || got.FinishedAt.Before(now) {
		t.Error("FinishedAt not set correctly")
	}
	if got.CsvPath != "" {
		t.Errorf("CsvPath after completion = %q, want cleared", got.CsvPath)
	}
	if got.ParamsJSON != entry.ParamsJSON || !got.StartedAt.Equal(entry.StartedAt) {
		t.Errorf("completion changed original operation metadata: %+v", got)
	}
}

func TestFinishError(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	entry, err := s.Create(ctx, oplog.OperationLog{
		Type:       oplog.TypeImportUsersTeams,
		Status:     oplog.StatusRunning,
		CsvPath:    "pending.csv",
		ParamsJSON: `{"profile_id":1}`,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = s.FinishError(ctx, entry.ID, "gateway timeout")
	if err != nil {
		t.Fatalf("FinishError: %v", err)
	}

	got, err := s.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != oplog.StatusError {
		t.Errorf("Status = %q, want %q", got.Status, oplog.StatusError)
	}
	if got.ErrorMsg != "gateway timeout" {
		t.Errorf("ErrorMsg = %q", got.ErrorMsg)
	}
	if got.CsvPath != "" {
		t.Errorf("CsvPath after failure = %q, want cleared", got.CsvPath)
	}
	if got.FinishedAt == nil || got.FinishedAt.Before(entry.StartedAt) {
		t.Errorf("FinishedAt after failure = %v, want at or after %v", got.FinishedAt, entry.StartedAt)
	}
	if got.ParamsJSON != entry.ParamsJSON || !got.StartedAt.Equal(entry.StartedAt) {
		t.Errorf("failure changed original operation metadata: %+v", got)
	}
}

func TestList(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	for i := range 3 {
		_, err := s.Create(ctx, oplog.OperationLog{
			Type:      oplog.TypeImportUsersTeams,
			ProfileID: uint(i + 1),
			Status:    oplog.StatusDone,
		})
		if err != nil {
			t.Fatalf("Create[%d]: %v", i, err)
		}
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("List len = %d, want 3", len(list))
	}
}

func TestGetNotFound(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	_, err := s.Get(ctx, 999)
	if err == nil {
		t.Fatal("expected error for non-existent ID")
	}
}

func TestListFiltered(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	entries := []oplog.OperationLog{
		{Type: oplog.TypeImportUsersTeams, ProfileID: 1, Status: oplog.StatusDone},
		{Type: oplog.TypeImportUsersTeams, ProfileID: 1, Status: oplog.StatusRunning},
		{Type: oplog.TypeRemoveUsersTeams, ProfileID: 1, Status: oplog.StatusDone},
		{Type: oplog.TypeImportUsersTeams, ProfileID: 2, Status: oplog.StatusError},
	}
	for i, entry := range entries {
		created, err := s.Create(ctx, entry)
		if err != nil {
			t.Fatalf("Create[%d]: %v", i, err)
		}
		entries[i] = created
	}

	tests := []struct {
		name      string
		opType    string
		profileID uint
		want      []oplog.OperationLog
	}{
		{"type only", oplog.TypeRemoveUsersTeams, 0, []oplog.OperationLog{entries[2]}},
		{"profile only", "", 2, []oplog.OperationLog{entries[3]}},
		{"type and profile", oplog.TypeImportUsersTeams, 1, []oplog.OperationLog{entries[1], entries[0]}},
		{"disjoint filters", oplog.TypeRemoveUsersTeams, 2, nil},
		{"no matching profile", "", 99, nil},
		{"all newest first", "", 0, []oplog.OperationLog{entries[3], entries[2], entries[1], entries[0]}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ListFiltered(ctx, tc.opType, tc.profileID)
			if err != nil {
				t.Fatalf("ListFiltered: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("rows = %+v, want %+v", got, tc.want)
			}
			for i, want := range tc.want {
				if got[i].ID != want.ID || got[i].Type != want.Type || got[i].ProfileID != want.ProfileID || got[i].Status != want.Status {
					t.Errorf("row %d = %+v, want operation ID %d, type %s, profile %d, status %s", i, got[i], want.ID, want.Type, want.ProfileID, want.Status)
				}
			}
		})
	}
}

func TestStore_ClosedDBErrors(t *testing.T) {
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("raw DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	s, err := oplog.NewStore(gdb)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close DB: %v", err)
	}

	ctx := context.Background()
	if _, err := s.Create(ctx, oplog.OperationLog{Type: oplog.TypeEmailSend}); err == nil {
		t.Error("Create on closed DB succeeded")
	}
	if _, err := s.Get(ctx, 1); err == nil || errors.Is(err, oplog.ErrNotFound) {
		t.Errorf("Get error = %v, want database failure distinct from not found", err)
	}
	if _, err := s.List(ctx); err == nil {
		t.Error("List on closed DB succeeded")
	}
	if _, err := s.ListFiltered(ctx, oplog.TypeEmailSend, 1); err == nil {
		t.Error("ListFiltered on closed DB succeeded")
	}
	if err := s.FinishOK(ctx, 1, 1, `{"sent":1}`); err == nil {
		t.Error("FinishOK on closed DB succeeded")
	}
	if err := s.FinishError(ctx, 1, "send failed"); err == nil {
		t.Error("FinishError on closed DB succeeded")
	}
	if _, err := oplog.NewStore(gdb); err == nil {
		t.Error("NewStore on closed DB succeeded")
	}
}
