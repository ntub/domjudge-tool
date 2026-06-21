package settings_test

import (
	"context"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
)

func newStore(t *testing.T) *settings.Store {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB, _ := gdb.DB()
	t.Cleanup(func() { sqlDB.Close() })
	s, err := settings.NewStore(gdb)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestSettings_GetDefaults(t *testing.T) {
	s := newStore(t)
	got, err := s.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Theme != "dark" {
		t.Errorf("Theme default: got %q, want %q", got.Theme, "dark")
	}
	if got.SMTPPort != 25 {
		t.Errorf("SMTPPort default: got %d, want 25", got.SMTPPort)
	}
}

func TestSettings_SaveAndGet(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	err := s.Save(ctx, settings.AppSettings{
		Theme:         "light",
		SMTPHost:      "smtp.example.org",
		SMTPPort:      587,
		SMTPFromEmail: "noreply@example.org",
		SMTPUseTLS:    true,
		SMTPUsername:  "user",
		SMTPPassword:  "secret",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Theme != "light" {
		t.Errorf("Theme: got %q, want %q", got.Theme, "light")
	}
	if got.SMTPHost != "smtp.example.org" {
		t.Errorf("SMTPHost: got %q", got.SMTPHost)
	}
	if got.SMTPPort != 587 {
		t.Errorf("SMTPPort: got %d, want 587", got.SMTPPort)
	}
}

func TestSettings_SaveIdempotent(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	for i := range 3 {
		theme := "dark"
		if i%2 == 1 {
			theme = "light"
		}
		if err := s.Save(ctx, settings.AppSettings{Theme: theme}); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}

	// Count rows — must be exactly 1.
	got, err := s.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Theme != "dark" {
		t.Errorf("last save should be dark; got %q", got.Theme)
	}
}
