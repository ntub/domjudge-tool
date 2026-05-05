package config_test

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/config"
)

// TestDomServerClient_RoundTrip_Golden is the named TDD anchor for
// backward compatibility with the Python serializer's canonical output.
// See docs/PHASE1.md for the reinterpretation note: the golden is the
// canonical 4-space-indent shape, not the user's hand-edited
// production file (which contains real credentials and predates the
// `affiliation_country` field).
func TestDomServerClient_RoundTrip_Golden(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "config", "domserver_canonical.json")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := config.MarshalCanonical(cfg)
	if err != nil {
		t.Fatalf("MarshalCanonical: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("byte drift\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestLoad_FillsDefaultsForMinimal(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "config", "domserver_minimal.json")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Version != "7.3.2" {
		t.Errorf("Version default not applied, got %q", cfg.Version)
	}
	if cfg.APIVersion != "v4" {
		t.Errorf("APIVersion default not applied, got %q", cfg.APIVersion)
	}
	if cfg.AffiliationCountry == nil || *cfg.AffiliationCountry != "TWN" {
		t.Errorf("AffiliationCountry default not applied, got %v", cfg.AffiliationCountry)
	}
	if cfg.DisableSSL {
		t.Errorf("DisableSSL should default to false")
	}
	if cfg.PreferScraper != nil {
		t.Errorf("PreferScraper = %v, want nil for an omitted legacy field", *cfg.PreferScraper)
	}
}

func TestLoad_PreferScraperTrue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "domserver.json")
	src := []byte(`{
    "host": "https://example.com",
    "username": "user",
    "password": "pass",
    "prefer_scraper": true
}`)
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PreferScraper == nil || !*cfg.PreferScraper {
		t.Fatalf("PreferScraper = %v, want true", cfg.PreferScraper)
	}
}

func TestLoad_AcceptsUnknownFieldAndWarns(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	path := filepath.Join("..", "..", "testdata", "config", "domserver_unknown_field.json")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Host == "" {
		t.Fatal("Host not parsed")
	}
	out := buf.String()
	if !strings.Contains(out, "unknown field") || !strings.Contains(out, "future_field") {
		t.Fatalf("expected slog warning naming the unknown field, got: %s", out)
	}
}

func TestLoad_ReturnsErrorForMalformedJSON(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "config", "domserver_invalid.json")
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoad_ReturnsErrorForMissingFile(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "does_not_exist.json"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist match via errors.Is", err)
	}
}

func TestSave_WritesCanonicalAndRoundTrips(t *testing.T) {
	src := filepath.Join("..", "..", "testdata", "config", "domserver_canonical.json")
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	cfg, err := config.Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "domserver.json")
	if err := config.Save(cfg, dst); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read written: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("byte drift after Save\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestMarshalCanonical_OmitsNilPreferScraper(t *testing.T) {
	cfg := config.Default()
	cfg.Host = "https://example.com"
	cfg.Username = "user"
	cfg.Password = "pass"

	got, err := config.MarshalCanonical(cfg)
	if err != nil {
		t.Fatalf("MarshalCanonical: %v", err)
	}
	if bytes.Contains(got, []byte("prefer_scraper")) {
		t.Fatalf("prefer_scraper should be omitted when nil:\n%s", got)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     config.DomServerClient
		wantErr bool
	}{
		{name: "all required filled", cfg: config.DomServerClient{Host: "h", Username: "u", Password: "p"}, wantErr: false},
		{name: "missing host", cfg: config.DomServerClient{Username: "u", Password: "p"}, wantErr: true},
		{name: "missing username", cfg: config.DomServerClient{Host: "h", Password: "p"}, wantErr: true},
		{name: "missing password", cfg: config.DomServerClient{Host: "h", Username: "u"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr && !errors.Is(err, config.ErrMissingRequiredField) {
				t.Fatalf("err = %v, want ErrMissingRequiredField match via errors.Is", err)
			}
		})
	}
}

func TestSave_RejectsNil(t *testing.T) {
	if err := config.Save(nil, filepath.Join(t.TempDir(), "out.json")); err == nil {
		t.Fatal("expected error on nil cfg")
	}
}
