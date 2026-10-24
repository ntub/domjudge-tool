package db_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/webui/db"
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
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec("CREATE TABLE parents (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create parents: %v", err)
	}
	if _, err := sqlDB.Exec("CREATE TABLE children (parent_id INTEGER REFERENCES parents(id))"); err != nil {
		t.Fatalf("create children: %v", err)
	}
	if _, err := sqlDB.Exec("INSERT INTO parents (id) VALUES (1)"); err != nil {
		t.Fatalf("insert parent: %v", err)
	}
	if _, err := sqlDB.Exec("INSERT INTO children (parent_id) VALUES (1)"); err != nil {
		t.Fatalf("insert valid child: %v", err)
	}
	if _, err := sqlDB.Exec("INSERT INTO children (parent_id) VALUES (2)"); err == nil {
		t.Fatal("insert child with missing parent succeeded; want foreign-key rejection")
	}
	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM children").Scan(&count); err != nil {
		t.Fatalf("count children: %v", err)
	}
	if count != 1 {
		t.Fatalf("child rows = %d, want only the valid child", count)
	}
}

func TestOpen_File(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.db")
	gdb, err := db.Open(filePath)
	if err != nil {
		t.Fatalf("Open file: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("raw DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec("CREATE TABLE saved_values (id INTEGER PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := sqlDB.Exec("INSERT INTO saved_values (id, value) VALUES (?, ?)", 7, "persisted value"); err != nil {
		t.Fatalf("insert value: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close before reopen: %v", err)
	}

	reopened, err := db.Open(filePath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	reopenedSQL, err := reopened.DB()
	if err != nil {
		t.Fatalf("reopened raw DB: %v", err)
	}
	t.Cleanup(func() { _ = reopenedSQL.Close() })
	var got string
	if err := reopenedSQL.QueryRow("SELECT value FROM saved_values WHERE id = ?", 7).Scan(&got); err != nil {
		t.Fatalf("read persisted value: %v", err)
	}
	if got != "persisted value" {
		t.Errorf("reopened value = %q, want persisted value", got)
	}
	var journalMode string
	if err := reopenedSQL.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("file database journal mode = %q, want wal", journalMode)
	}
	var foreignKeys int
	if err := reopenedSQL.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("foreign key setting: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("reopened foreign_keys = %d, want 1", foreignKeys)
	}
}

func TestOpen_Error(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("keep this file"), 0o600); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}
	_, err := db.Open(filepath.Join(blocked, "test.db"))
	if err == nil {
		t.Fatal("expected error opening impossible path, got nil")
	}
}

func TestOpen_CorruptFile(t *testing.T) {
	tmp := t.TempDir()
	filePath := tmp + "/corrupt.db"
	if err := os.WriteFile(filePath, []byte("NOT A VALID SQLITE DATABASE FILE HEADER"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := db.Open(filePath)
	if err == nil {
		t.Fatal("expected error on corrupt database file, got nil")
	}
}

func TestOpen_ReadOnlyDatabaseRejectsWAL_Integration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "readonly.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.Exec("CREATE TABLE saved (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = db.Open("file:" + path + "?mode=ro")
	var sqliteErr interface{ Code() int }
	if !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != 8 {
		t.Fatalf("opening read-only non-WAL database: %v, want SQLITE_READONLY", err)
	}
}
