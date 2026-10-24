// Package profile provides CRUD for DOMjudge server profiles
// stored in the web UI's SQLite database via GORM.
package profile

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ErrNotFound is returned by Get when no profile matches the given id.
var ErrNotFound = errors.New("profile: not found")

// ServerProfile is the GORM model for the server_profiles table.
// Pointer fields reflect nullable columns.
type ServerProfile struct {
	gorm.Model
	Name                    string `gorm:"uniqueIndex;not null"`
	Host                    string `gorm:"not null"`
	Username                string `gorm:"not null"`
	Password                string `gorm:"not null"`
	DisableSSL              bool   `gorm:"default:false"`
	Timeout                 *float64
	MaxConnections          *int
	MaxKeepaliveConnections *int
	CategoryID              *int
	AffiliationID           *int
	AffiliationCountry      *string
	UserRoles               string `gorm:"default:'[]'"` // JSON array
	Version                 string `gorm:"default:'7.3.2'"`
	APIVersion              string `gorm:"column:api_version;default:'v4'"`
	PreferScraper           *bool
	ContestID               *string
	IsDefault               bool `gorm:"default:false"`
}

// Store wraps a *gorm.DB with profile-specific operations.
type Store struct {
	db *gorm.DB
}

// NewStore constructs a Store and applies AutoMigrate for server_profiles.
func NewStore(gdb *gorm.DB) (*Store, error) {
	if err := gdb.AutoMigrate(&ServerProfile{}); err != nil {
		return nil, fmt.Errorf("profile: auto-migrate: %w", err)
	}
	return &Store{db: gdb}, nil
}

// List returns all profiles ordered by name.
func (s *Store) List(ctx context.Context) ([]ServerProfile, error) {
	var profiles []ServerProfile
	if err := s.db.WithContext(ctx).Order("name").Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("profile: list: %w", err)
	}
	return profiles, nil
}

// Get returns the profile with the given id, or ErrNotFound.
func (s *Store) Get(ctx context.Context, id int64) (ServerProfile, error) {
	var p ServerProfile
	err := s.db.WithContext(ctx).First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ServerProfile{}, fmt.Errorf("%w: id=%d", ErrNotFound, id)
	}
	if err != nil {
		return ServerProfile{}, fmt.Errorf("profile: get id=%d: %w", id, err)
	}
	return p, nil
}

// Create inserts a new profile and returns its assigned id.
func (s *Store) Create(ctx context.Context, p ServerProfile) (int64, error) {
	p.ID = 0 // let GORM assign
	if err := s.db.WithContext(ctx).Create(&p).Error; err != nil {
		return 0, fmt.Errorf("profile: create: %w", err)
	}
	return int64(p.ID), nil
}

// Update replaces all mutable fields of the profile identified by p.ID.
func (s *Store) Update(ctx context.Context, p ServerProfile) error {
	if err := s.db.WithContext(ctx).Save(&p).Error; err != nil {
		return fmt.Errorf("profile: update id=%d: %w", p.ID, err)
	}
	return nil
}

// Delete hard-deletes the profile with the given id.
func (s *Store) Delete(ctx context.Context, id int64) error {
	if err := s.db.WithContext(ctx).Unscoped().Delete(&ServerProfile{}, id).Error; err != nil {
		return fmt.Errorf("profile: delete id=%d: %w", id, err)
	}
	return nil
}

// SetDefault marks exactly one profile as the default, clearing all others.
func (s *Store) SetDefault(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		session := tx.Session(&gorm.Session{AllowGlobalUpdate: true})
		if err := session.Model(&ServerProfile{}).Update("is_default", false).Error; err != nil {
			return fmt.Errorf("profile: set-default clear: %w", err)
		}
		if err := tx.Model(&ServerProfile{}).Where("id = ?", id).Update("is_default", true).Error; err != nil {
			return fmt.Errorf("profile: set-default id=%d: %w", id, err)
		}
		return nil
	})
}

// SeedFromLegacy creates a profile from a legacy domserver.json field set.
// It is a no-op (returns existing id) if a profile with the same host already exists.
func (s *Store) SeedFromLegacy(
	ctx context.Context,
	name, host, username, password, version string,
	affiliationCountry *string,
) (int64, error) {
	var existing ServerProfile
	err := s.db.WithContext(ctx).Where("host = ?", host).First(&existing).Error
	if err == nil {
		return int64(existing.ID), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("profile: seed check: %w", err)
	}
	return s.Create(ctx, ServerProfile{
		Name:               name,
		Host:               host,
		Username:           username,
		Password:           password,
		Version:            version,
		APIVersion:         "v4",
		AffiliationCountry: affiliationCountry,
		IsDefault:          true,
	})
}
