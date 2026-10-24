package general_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli/general"
	"github.com/ntub/domjudge-tool/internal/config"
)

const goldenFixturePath = "../../../testdata/cli/general/gen_config_golden.json"

// TestGenConfig_GoldenDomserverJSON locks Phase 1 DoD #4: gen-config
// with the canonical flag set writes a domserver.json byte-identical
// to the Python serializer's canonical output.
func TestGenConfig_GoldenDomserverJSON(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "domserver.json")

	cmd := general.NewGenConfigCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--host", "https://example.com",
		"--username", "user",
		"--password", "pass",
		"--output", out,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	want, err := os.ReadFile(goldenFixturePath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("DoD #4 byte-identical FAILED\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGenConfig_RequiredFlagsMissing(t *testing.T) {
	t.Parallel()
	cases := []string{"--host", "--username", "--password"}
	for _, missing := range cases {
		t.Run("missing "+missing, func(t *testing.T) {
			t.Parallel()
			args := []string{
				"--host", "https://example.com",
				"--username", "user",
				"--password", "pass",
				"--output", filepath.Join(t.TempDir(), "domserver.json"),
			}
			// Drop the missing flag and its value.
			pruned := make([]string, 0, len(args))
			for i := 0; i < len(args); i += 2 {
				if args[i] == missing {
					continue
				}
				pruned = append(pruned, args[i], args[i+1])
			}
			cmd := general.NewGenConfigCmd()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(pruned)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("Execute should fail when %s is missing", missing)
			}
		})
	}
}

func TestGenConfig_DefaultsForOptionals(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "domserver.json")
	cmd := general.NewGenConfigCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--host", "https://example.com",
		"--username", "user",
		"--password", "pass",
		"--output", out,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	cfg, err := config.Load(out)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AffiliationCountry == nil || *cfg.AffiliationCountry != "TWN" {
		t.Errorf("AffiliationCountry = %v, want TWN", cfg.AffiliationCountry)
	}
	if cfg.Version != "7.3.2" {
		t.Errorf("Version = %q, want 7.3.2", cfg.Version)
	}
	if cfg.APIVersion != "v4" {
		t.Errorf("APIVersion = %q, want v4", cfg.APIVersion)
	}
	if cfg.Timeout != nil {
		t.Errorf("Timeout should be nil when --timeout is unset, got %v", *cfg.Timeout)
	}
}

func TestGenConfig_OutputFlagWritesElsewhere(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "custom.json")
	cmd := general.NewGenConfigCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--host", "https://example.com",
		"--username", "user",
		"--password", "pass",
		"--output", out,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected file at %s, got: %v", out, err)
	}
}

func TestGenConfig_OptionalFlagsPropagate(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "domserver.json")
	cmd := general.NewGenConfigCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--host", "https://example.com",
		"--username", "user",
		"--password", "pass",
		"--output", out,
		"--version", "7.3.4",
		"--api-version", "v4",
		"--disable-ssl",
		"--timeout", "30",
		"--max-connections", "8",
		"--max-keepalive-connections", "4",
		"--category-id", "21",
		"--affiliation-id", "285",
		"--affiliation-country", "USA",
		"--user-roles", "3,4",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	cfg, err := config.Load(out)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Version != "7.3.4" {
		t.Errorf("Version = %q, want 7.3.4", cfg.Version)
	}
	if !cfg.DisableSSL {
		t.Errorf("DisableSSL should be true")
	}
	if cfg.Timeout == nil || *cfg.Timeout != 30 {
		t.Errorf("Timeout = %v, want 30", cfg.Timeout)
	}
	if cfg.MaxConnections == nil || *cfg.MaxConnections != 8 {
		t.Errorf("MaxConnections = %v, want 8", cfg.MaxConnections)
	}
	if cfg.CategoryID == nil || *cfg.CategoryID != 21 {
		t.Errorf("CategoryID = %v, want 21", cfg.CategoryID)
	}
	if cfg.AffiliationID == nil || *cfg.AffiliationID != 285 {
		t.Errorf("AffiliationID = %v, want 285", cfg.AffiliationID)
	}
	if cfg.AffiliationCountry == nil || *cfg.AffiliationCountry != "USA" {
		t.Errorf("AffiliationCountry = %v, want USA", cfg.AffiliationCountry)
	}
	if len(cfg.UserRoles) != 2 || cfg.UserRoles[0] != 3 || cfg.UserRoles[1] != 4 {
		t.Errorf("UserRoles = %v, want [3 4]", cfg.UserRoles)
	}
}

// TestGenConfig_ContestIDFlagPropagates verifies that --contest-id is stored
// in cfg.ContestID for multi-contest import resolution.
func TestGenConfig_ContestIDFlagPropagates(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "domserver.json")
	cmd := general.NewGenConfigCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--host", "https://example.com",
		"--username", "user",
		"--password", "pass",
		"--output", out,
		"--contest-id", "2",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	cfg, err := config.Load(out)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ContestID == nil || *cfg.ContestID != "2" {
		t.Errorf("ContestID = %v, want \"2\"", cfg.ContestID)
	}
}

// TestGenConfig_PrintsWrotePath locks the UX fix from #8: gen-config
// must echo the destination so the operator sees confirmation.
// Mirrors download-scoreboard's `Wrote %s` line.
func TestGenConfig_PrintsWrotePath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "domserver.json")
	cmd := general.NewGenConfigCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--host", "https://example.com",
		"--username", "user",
		"--password", "pass",
		"--output", out,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Wrote") || !strings.Contains(got, out) {
		t.Errorf("stdout %q should mention 'Wrote' and the output path %q", got, out)
	}
}

func TestGenConfig_DoesNotLoadConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "domserver.json")

	cmd := general.NewGenConfigCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	// Point --config at a non-existent path; gen-config must not try
	// to load it (no PreRunE attached).
	cmd.Flags().StringP("config", "c", "/non/existent/domserver.json", "")
	cmd.SetArgs([]string{
		"--host", "https://example.com",
		"--username", "user",
		"--password", "pass",
		"--output", out,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen-config should ignore --config: got err %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output file not written: %v", err)
	}
}

func TestGenConfig_GroupedUnderGeneral(t *testing.T) {
	t.Parallel()
	cmd := general.NewGenConfigCmd()
	if cmd.GroupID != "general" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "general")
	}
}

func TestGenConfig_ParentMissingError(t *testing.T) {
	t.Parallel()
	cmd := general.NewGenConfigCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--host", "https://example.com",
		"--username", "user",
		"--password", "pass",
		"--output", filepath.Join(t.TempDir(), "no-such-dir", "domserver.json"),
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Execute should fail when parent dir is missing")
	}
	if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("err = %v, want wraps os.ErrNotExist", err)
	}
}
