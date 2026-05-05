package cli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ntub/domjudge-tool/internal/cli"
)

func TestLoadConfigPreRunE_LoadsAndStashesOnContext(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "domserver.json")
	if err := os.WriteFile(path, []byte(`{"host":"https://x","username":"u","password":"p"}`), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("config", path, "")
	cmd.PreRunE = cli.LoadConfigPreRunE
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		cfg, ok := cli.ConfigFromContext(c.Context())
		if !ok {
			t.Errorf("config not on ctx")
			return nil
		}
		if cfg.Host != "https://x" {
			t.Errorf("Host = %q, want https://x", cfg.Host)
		}
		return nil
	}
	cmd.SetContext(context.Background())
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestLoadConfigPreRunE_MissingFile(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("config", filepath.Join(t.TempDir(), "missing.json"), "")
	cmd.PreRunE = cli.LoadConfigPreRunE
	cmd.RunE = func(*cobra.Command, []string) error { return nil }
	cmd.SetContext(context.Background())

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Execute should fail when config missing")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want wraps os.ErrNotExist", err)
	}
}

func TestLoadConfigPreRunE_EnvVarFallback(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "from-env.json")
	if err := os.WriteFile(path, []byte(`{"host":"https://env","username":"u","password":"p"}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Setenv("DOMSERVER_CONFIG", path)

	cmd := &cobra.Command{Use: "x"}
	// Default unchanged: --config is not Changed, env should win.
	cmd.Flags().String("config", "domserver.json", "")
	cmd.PreRunE = cli.LoadConfigPreRunE
	var loaded string
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		cfg, _ := cli.ConfigFromContext(c.Context())
		loaded = cfg.Host
		return nil
	}
	cmd.SetContext(context.Background())
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if loaded != "https://env" {
		t.Errorf("Host = %q, want https://env (env override missed)", loaded)
	}
}

func TestLoadConfigPreRunE_DisableSSLFlagOverridesConfig(t *testing.T) {
	cases := []struct {
		name string
		json string
		args []string
		want bool
	}{
		{
			name: "true overrides config false",
			json: `{"host":"https://x","username":"u","password":"p","disable_ssl":false}`,
			args: []string{"--disable-ssl"},
			want: true,
		},
		{
			name: "explicit false overrides config true",
			json: `{"host":"https://x","username":"u","password":"p","disable_ssl":true}`,
			args: []string{"--disable-ssl=false"},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "domserver.json")
			if err := os.WriteFile(path, []byte(tc.json), 0o600); err != nil {
				t.Fatalf("seed: %v", err)
			}

			root := cli.NewRootCmd(cli.Deps{})
			var got bool
			probe := &cobra.Command{
				Use:     "probe",
				PreRunE: cli.LoadConfigPreRunE,
				RunE: func(c *cobra.Command, _ []string) error {
					cfg, ok := cli.ConfigFromContext(c.Context())
					if !ok {
						t.Fatalf("config not on ctx")
					}
					got = cfg.DisableSSL
					return nil
				},
			}
			root.AddCommand(probe)
			args := append([]string{"--config", path}, tc.args...)
			args = append(args, "probe")
			root.SetArgs(args)

			if err := root.ExecuteContext(context.Background()); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if got != tc.want {
				t.Errorf("DisableSSL = %v, want %v", got, tc.want)
			}
		})
	}
}
