// Package db opens the desktop app's SQLite database via GORM.
package db

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open opens (or creates) the SQLite file at path.
// Use ":memory:" for in-process tests.
func Open(path string) (*gorm.DB, error) {
	cfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
	db, err := gorm.Open(sqlite.Open(path), cfg)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", path, err)
	}
	sql, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("db: raw db: %w", err)
	}
	sql.SetMaxOpenConns(1) // SQLite WAL supports one writer
	if _, err := sql.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("db: foreign_keys: %w", err)
	}
	if path != ":memory:" {
		if _, err := sql.Exec("PRAGMA journal_mode = WAL"); err != nil {
			return nil, fmt.Errorf("db: journal_mode: %w", err)
		}
	}
	return db, nil
}
