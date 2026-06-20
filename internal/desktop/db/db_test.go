package db_test

import (
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
)

func TestOpen_InMemory(t *testing.T) {
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("raw DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
