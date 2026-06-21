package oplog_test

import (
	"context"
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
		Type:   oplog.TypeImportUsersTeams,
		Status: oplog.StatusRunning,
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
}

func TestFinishError(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	entry, err := s.Create(ctx, oplog.OperationLog{
		Type:   oplog.TypeImportUsersTeams,
		Status: oplog.StatusRunning,
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
