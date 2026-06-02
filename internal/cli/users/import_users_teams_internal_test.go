package users

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/importer"
)

// stubImporter swaps importUsersTeamsFn for the duration of the test
// and captures the ImportOptions argument.
func stubImporter(t *testing.T, capture *importer.ImportOptions) {
	t.Helper()
	prev := importUsersTeamsFn
	importUsersTeamsFn = func(
		_ context.Context,
		_ web.Gateway,
		_ *apiv4.UsersResource,
		_ io.Reader,
		opts importer.ImportOptions,
	) (importer.ImportUsersTeamsResult, error) {
		*capture = opts
		return importer.ImportUsersTeamsResult{}, nil
	}
	t.Cleanup(func() { importUsersTeamsFn = prev })
}

func TestImportUsersTeams_FlagPropagation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want func(opts importer.ImportOptions) error
	}{
		{
			name: "category-id",
			args: []string{"--category-id", "21"},
			want: func(o importer.ImportOptions) error {
				if o.CategoryID == nil || *o.CategoryID != 21 {
					return fmt.Errorf("CategoryID = %v, want 21", o.CategoryID)
				}
				return nil
			},
		},
		{
			name: "affiliation-id",
			args: []string{"--affiliation-id", "285"},
			want: func(o importer.ImportOptions) error {
				if o.AffiliationID == nil || *o.AffiliationID != 285 {
					return fmt.Errorf("AffiliationID = %v, want 285", o.AffiliationID)
				}
				return nil
			},
		},
		{
			name: "user-roles",
			args: []string{"--user-roles", "3,4"},
			want: func(o importer.ImportOptions) error {
				if len(o.UserRoles) != 2 || o.UserRoles[0] != 3 || o.UserRoles[1] != 4 {
					return fmt.Errorf("UserRoles = %v, want [3 4]", o.UserRoles)
				}
				return nil
			},
		},
		{
			name: "enabled false",
			args: []string{"--enabled=false"},
			want: func(o importer.ImportOptions) error {
				if o.Enabled {
					return fmt.Errorf("Enabled = true, want false")
				}
				return nil
			},
		},
		{
			name: "ignore-existing",
			args: []string{"--ignore-existing"},
			want: func(o importer.ImportOptions) error {
				if !o.IgnoreExisting {
					return fmt.Errorf("IgnoreExisting = false, want true")
				}
				return nil
			},
		},
		{
			name: "delete-existing",
			args: []string{"--delete-existing"},
			want: func(o importer.ImportOptions) error {
				if !o.DeleteExisting {
					return fmt.Errorf("DeleteExisting = false, want true")
				}
				return nil
			},
		},
		{
			name: "password-length",
			args: []string{"--password-length", "16"},
			want: func(o importer.ImportOptions) error {
				if o.PasswordLength != 16 {
					return fmt.Errorf("PasswordLength = %d, want 16", o.PasswordLength)
				}
				return nil
			},
		},
		{
			name: "password-pattern",
			args: []string{"--password-pattern", "0123456789"},
			want: func(o importer.ImportOptions) error {
				if o.PasswordCharset != "0123456789" {
					return fmt.Errorf("PasswordCharset = %q, want 0123456789", o.PasswordCharset)
				}
				return nil
			},
		},
		{
			name: "new-password",
			args: []string{"--new-password"},
			want: func(o importer.ImportOptions) error {
				if !o.NewPassword {
					return fmt.Errorf("NewPassword = false, want true")
				}
				return nil
			},
		},
		{
			name: "affiliation-country",
			args: []string{"--affiliation-country", "USA"},
			want: func(o importer.ImportOptions) error {
				if o.AffiliationCountry != "USA" {
					return fmt.Errorf("AffiliationCountry = %q, want USA", o.AffiliationCountry)
				}
				return nil
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			csv := filepath.Join(tmpDir, "in.csv")
			if err := os.WriteFile(csv, []byte("username,name,email,affiliation,password\nalice,Alice,,,p\n"), 0o600); err != nil {
				t.Fatalf("seed: %v", err)
			}

			var captured importer.ImportOptions
			stubImporter(t, &captured)

			deps := Deps{
				NewAPI:     func(*config.DomServerClient) *apiv4.Client { return apiv4.New(apiv4.Options{BaseURL: "http://x"}) },
				NewGateway: func(context.Context, *config.DomServerClient) (web.Gateway, error) { return nil, nil },
			}
			cmd := NewImportUsersTeamsCmd(deps)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			args := append([]string{csv}, tc.args...)
			cmd.SetArgs(args)

			prev, _ := os.Getwd()
			_ = os.Chdir(tmpDir)
			t.Cleanup(func() { _ = os.Chdir(prev) })

			cmd.SetContext(cli.WithConfig(context.Background(), &config.DomServerClient{Host: "http://x"}))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if err := tc.want(captured); err != nil {
				t.Errorf("flag did not propagate: %v\noptions=%+v", err, captured)
			}
		})
	}
}

func TestImportUsersTeams_ConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	csv := filepath.Join(tmpDir, "in.csv")
	if err := os.WriteFile(csv, []byte("username,name,email,affiliation,password\nalice,Alice,,,p\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var captured importer.ImportOptions
	stubImporter(t, &captured)

	deps := Deps{
		NewAPI:     func(*config.DomServerClient) *apiv4.Client { return apiv4.New(apiv4.Options{BaseURL: "http://x"}) },
		NewGateway: func(context.Context, *config.DomServerClient) (web.Gateway, error) { return nil, nil },
	}
	cmd := NewImportUsersTeamsCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{csv})

	prev, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	t.Cleanup(func() { _ = os.Chdir(prev) })

	categoryID := 21
	affiliationID := 285
	country := "USA"
	cfg := &config.DomServerClient{
		Host:               "http://x",
		CategoryID:         &categoryID,
		AffiliationID:      &affiliationID,
		AffiliationCountry: &country,
		UserRoles:          []int{3, 4},
	}
	cmd.SetContext(cli.WithConfig(context.Background(), cfg))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if captured.CategoryID == nil || *captured.CategoryID != categoryID {
		t.Errorf("CategoryID = %v, want %d", captured.CategoryID, categoryID)
	}
	if captured.AffiliationID == nil || *captured.AffiliationID != affiliationID {
		t.Errorf("AffiliationID = %v, want %d", captured.AffiliationID, affiliationID)
	}
	if captured.AffiliationCountry != country {
		t.Errorf("AffiliationCountry = %q, want %q", captured.AffiliationCountry, country)
	}
	if len(captured.UserRoles) != 2 || captured.UserRoles[0] != 3 || captured.UserRoles[1] != 4 {
		t.Errorf("UserRoles = %v, want [3 4]", captured.UserRoles)
	}
}
