package profile_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

func newTestStore(t *testing.T) *profile.Store {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	store, err := profile.NewStore(gdb)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func sampleProfile(name string) profile.ServerProfile {
	return profile.ServerProfile{
		Name:     name,
		Host:     "https://test-station.example.org",
		Username: "admin",
		Password: "secret",
		Version:  "7.3.2",
	}
}

func TestStore_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	p := sampleProfile("prod")
	id, err := s.Create(ctx, p)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != p.Name {
		t.Errorf("Name: got %q, want %q", got.Name, p.Name)
	}
	if got.Host != p.Host {
		t.Errorf("Host: got %q, want %q", got.Host, p.Host)
	}
}

func TestStore_List(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if _, err := s.Create(ctx, sampleProfile(name)); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(list))
	}
}

func TestStore_Update(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id, err := s.Create(ctx, sampleProfile("original"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, _ := s.Get(ctx, id)
	got.Name = "renamed"
	got.Host = "https://prod-station.example.org"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	after, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if after.Name != "renamed" {
		t.Errorf("Name: got %q, want %q", after.Name, "renamed")
	}
	if after.Host != "https://prod-station.example.org" {
		t.Errorf("Host: got %q, want %q", after.Host, "https://prod-station.example.org")
	}
}

func TestStore_Delete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id, err := s.Create(ctx, sampleProfile("to-delete"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = s.Get(ctx, id)
	if !errors.Is(err, profile.ErrNotFound) {
		t.Fatalf("Get deleted profile error = %v, want ErrNotFound", err)
	}
}

func TestStore_SetDefault(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id1, _ := s.Create(ctx, sampleProfile("first"))
	id2, _ := s.Create(ctx, sampleProfile("second"))

	if err := s.SetDefault(ctx, id1); err != nil {
		t.Fatalf("SetDefault id1: %v", err)
	}
	if err := s.SetDefault(ctx, id2); err != nil {
		t.Fatalf("SetDefault id2: %v", err)
	}

	list, _ := s.List(ctx)
	for _, p := range list {
		if int64(p.ID) == id2 && !p.IsDefault {
			t.Errorf("id2 should be default")
		}
		if int64(p.ID) == id1 && p.IsDefault {
			t.Errorf("id1 should no longer be default")
		}
	}
}

func TestStore_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, err := s.Get(ctx, 9999)
	if !errors.Is(err, profile.ErrNotFound) {
		t.Fatalf("Get missing profile error = %v, want ErrNotFound", err)
	}
}

func TestStore_SeedFromConfig(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	country := "TWN"
	id, err := s.SeedFromLegacy(ctx, "imported", "https://test-station.example.org", "admin", "pass", "7.3.2", &country)
	if err != nil {
		t.Fatalf("SeedFromLegacy: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AffiliationCountry == nil || *got.AffiliationCountry != "TWN" {
		t.Errorf("AffiliationCountry: got %v", got.AffiliationCountry)
	}
}

func TestStore_SeedFromLegacy_Idempotent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id1, err := s.SeedFromLegacy(ctx, "first", "https://test-station.example.org", "admin", "pass", "7.3.2", nil)
	if err != nil {
		t.Fatalf("first seed: %v", err)
	}

	id2, err := s.SeedFromLegacy(ctx, "second", "https://test-station.example.org", "admin2", "pass2", "8.3.1", nil)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if id1 != id2 {
		t.Errorf("expected idempotent seed to return id %d, got %d", id1, id2)
	}
	got, err := s.Get(ctx, id1)
	if err != nil {
		t.Fatalf("Get original profile: %v", err)
	}
	if got.Name != "first" || got.Username != "admin" || got.Password != "pass" || got.Version != "7.3.2" || !got.IsDefault {
		t.Errorf("second seed changed existing profile: %+v", got)
	}
	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List after second seed: %v", err)
	}
	if len(list) != 1 || int64(list[0].ID) != id1 {
		t.Errorf("profiles after second seed = %+v, want only ID %d", list, id1)
	}
}

func TestStore_ClosedDBErrors(t *testing.T) {
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s, err := profile.NewStore(gdb)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("raw DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close DB: %v", err)
	}

	ctx := context.Background()
	if _, err := s.List(ctx); err == nil {
		t.Error("expected error on List with closed db")
	}
	if _, err := s.Get(ctx, 1); err == nil {
		t.Error("expected error on Get with closed db")
	}
	if _, err := s.Create(ctx, sampleProfile("p")); err == nil {
		t.Error("expected error on Create with closed db")
	}
	if err := s.Update(ctx, sampleProfile("p")); err == nil {
		t.Error("expected error on Update with closed db")
	}
	if err := s.Delete(ctx, 1); err == nil {
		t.Error("expected error on Delete with closed db")
	}
	if err := s.SetDefault(ctx, 1); err == nil {
		t.Error("expected error on SetDefault with closed db")
	}
	if _, err := s.SeedFromLegacy(ctx, "x", "https://closed.example.org", "u", "p", "v", nil); err == nil {
		t.Error("expected error on SeedFromLegacy with closed db")
	}
	if _, err := profile.NewStore(gdb); err == nil {
		t.Error("expected migration error on NewStore with closed db")
	}
}
