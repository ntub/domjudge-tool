// Package settings persists user preferences (theme, SMTP, default profile)
// in a single SQLite row for the desktop app.
package settings

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// AppSettings is the single-row preferences table.
type AppSettings struct {
	gorm.Model
	Theme            string // "dark" | "light" | "system"; default "dark"
	DefaultProfileID uint
	SMTPHost         string
	SMTPPort         int
	SMTPFromEmail    string
	SMTPUseTLS       bool
	SMTPUsername     string
	SMTPPassword     string
}

// Store provides Get/Save for the single AppSettings row.
type Store struct {
	db *gorm.DB
}

// NewStore migrates the schema and returns a Store.
func NewStore(gdb *gorm.DB) (*Store, error) {
	if err := gdb.AutoMigrate(&AppSettings{}); err != nil {
		return nil, err
	}
	return &Store{db: gdb}, nil
}

// Get returns the current settings, or a zero-value struct with defaults if
// no row exists yet.
func (s *Store) Get(ctx context.Context) (AppSettings, error) {
	var row AppSettings
	err := s.db.WithContext(ctx).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AppSettings{Theme: "dark", SMTPPort: 25}, nil
	}
	if err != nil {
		return AppSettings{}, err
	}
	if row.Theme == "" {
		row.Theme = "dark"
	}
	if row.SMTPPort == 0 {
		row.SMTPPort = 25
	}
	return row, nil
}

// Save upserts the settings (always uses ID=1).
func (s *Store) Save(ctx context.Context, row AppSettings) error {
	row.ID = 1
	return s.db.WithContext(ctx).Save(&row).Error
}
