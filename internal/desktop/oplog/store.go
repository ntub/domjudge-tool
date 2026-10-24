// Package oplog records desktop batch operations in SQLite.
package oplog

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	TypeImportUsersTeams    = "import-users-teams"
	TypeRemoveUsersTeams    = "remove-users-teams"
	TypeDownloadProblems    = "download-problems"
	TypeImportProblems      = "import-problems"
	TypeDownloadSubmissions = "download-submissions"
	TypeDownloadScoreboard  = "download-scoreboard"
	TypeEmailSend           = "email-send"

	StatusRunning = "running"
	StatusDone    = "done"
	StatusError   = "error"
)

// OperationLog is one row in the operation history table.
type OperationLog struct {
	gorm.Model
	Type        string
	ProfileID   uint
	Status      string
	RowCount    int
	ErrorMsg    string
	SummaryJSON string
	ParamsJSON  string // JSON-encoded input params for the operation
	CsvPath     string // temp file path; cleared after streaming finishes
	StartedAt   time.Time
	FinishedAt  *time.Time
}

// Store provides CRUD for OperationLog rows.
type Store struct {
	db *gorm.DB
}

// ErrNotFound is returned by Get when no row matches the given ID.
var ErrNotFound = errors.New("oplog: not found")

// NewStore migrates the schema and returns a Store.
func NewStore(gdb *gorm.DB) (*Store, error) {
	if err := gdb.AutoMigrate(&OperationLog{}); err != nil {
		return nil, err
	}
	return &Store{db: gdb}, nil
}

// Create inserts a new OperationLog row with StartedAt = now.
func (s *Store) Create(ctx context.Context, entry OperationLog) (OperationLog, error) {
	entry.StartedAt = time.Now()
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		return OperationLog{}, err
	}
	return entry, nil
}

// Get returns a single row by primary key.
func (s *Store) Get(ctx context.Context, id uint) (OperationLog, error) {
	var entry OperationLog
	err := s.db.WithContext(ctx).First(&entry, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OperationLog{}, ErrNotFound
	}
	return entry, err
}

// List returns all rows ordered by creation time, newest first.
func (s *Store) List(ctx context.Context) ([]OperationLog, error) {
	var rows []OperationLog
	err := s.db.WithContext(ctx).Order("id desc").Find(&rows).Error
	return rows, err
}

// ListFiltered returns rows filtered by optional opType (empty = all) and
// profileID (0 = all), ordered newest first.
func (s *Store) ListFiltered(ctx context.Context, opType string, profileID uint) ([]OperationLog, error) {
	q := s.db.WithContext(ctx).Order("id desc")
	if opType != "" {
		q = q.Where("type = ?", opType)
	}
	if profileID > 0 {
		q = q.Where("profile_id = ?", profileID)
	}
	var rows []OperationLog
	return rows, q.Find(&rows).Error
}

// FinishOK marks a row as done with a row count and summary JSON.
func (s *Store) FinishOK(ctx context.Context, id uint, rowCount int, summaryJSON string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&OperationLog{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       StatusDone,
			"row_count":    rowCount,
			"summary_json": summaryJSON,
			"csv_path":     "",
			"finished_at":  now,
		}).Error
}

// FinishError marks a row as errored with a message.
func (s *Store) FinishError(ctx context.Context, id uint, msg string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&OperationLog{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      StatusError,
			"error_msg":   msg,
			"csv_path":    "",
			"finished_at": now,
		}).Error
}
