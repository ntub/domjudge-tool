package templates_test

import (
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/cliinstall"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
)

func TestCLISettingsActionsAreIndependent(t *testing.T) {
	cases := []struct {
		state     string
		install   bool
		relink    bool
		uninstall bool
	}{
		{state: "absent", install: true},
		{state: "current", uninstall: true},
		{state: "other", relink: true, uninstall: true},
		{state: "broken", relink: true, uninstall: true},
		{state: "conflict"},
		{state: "unavailable"},
		{state: "error"},
		{state: "unavailable", uninstall: true},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			view := templates.CLIInstallView{
				Status: cliinstall.Status{
					State: tc.state, CanInstall: tc.install, CanRelink: tc.relink, CanUninstall: tc.uninstall,
					CommandPath: "/path/to/user/bin/domjudge-tool", CLIPath: "/path/to/app/cli/domjudge-tool",
					TargetPath: "/path/to/previous/app/cli/domjudge-tool", BundlePath: "/path/to/app",
					Version: "v1.2.3", Revision: "inspected-revision", Reason: "Visible state explanation", PathNote: "Terminal PATH has not been checked.",
				},
				Token: "test-form-token", AppVersion: "v1.2.3",
			}
			got := render(t, templates.SettingsForm(settings.AppSettings{}, nil, view, "", ""))
			for _, text := range []string{tc.state, view.Status.CommandPath, view.Status.CLIPath, view.Status.TargetPath, view.Status.BundlePath, view.Status.Version, view.Status.Reason, view.Status.PathNote} {
				if !strings.Contains(got, html.EscapeString(text)) {
					t.Errorf("settings lacks %q", text)
				}
			}
			for action, available := range map[string]bool{"install": tc.install, "relink": tc.relink, "uninstall": tc.uninstall} {
				form := regexp.MustCompile(`<form[^>]*action="/settings/cli/` + action + `"[^>]*>(.*?)</form>`).FindString(got)
				if (form != "") != available {
					t.Errorf("%s form available = %v, want %v", action, form != "", available)
				}
				if !available {
					continue
				}
				for _, field := range []string{`name="csrf_token" value="test-form-token"`, `name="revision" value="inspected-revision"`, `name="confirm" value="` + action + `"`, "required", view.Status.CommandPath, "data-dj-busy"} {
					if !strings.Contains(form, field) {
						t.Errorf("%s form lacks explicit confirmation or credential %q", action, field)
					}
				}
			}
			depth := 0
			for _, tag := range regexp.MustCompile(`<form\b[^>]*>|</form>`).FindAllString(got, -1) {
				if tag == "</form>" {
					depth--
				} else {
					depth++
				}
				if depth < 0 || depth > 1 {
					t.Fatal("CLI actions overlap the general/SMTP settings form")
				}
			}
			if depth != 0 {
				t.Fatal("unclosed settings form")
			}
			if regexp.MustCompile(`<input[^>]*name="(?:path|source|target|destination|executable)"`).MatchString(got) {
				t.Fatal("filesystem path is submitted by the view")
			}
		})
	}
}

func TestCLISettingsEscapesPathsAndReasons(t *testing.T) {
	const attack = `'"><img src=x onerror="bad()">&`
	view := templates.CLIInstallView{
		Status: cliinstall.Status{State: "absent", BundlePath: attack, CLIPath: attack, CommandPath: attack, TargetPath: attack, Version: attack, Revision: attack, Reason: attack, PathNote: attack, CanInstall: true},
		Token:  attack, AppVersion: attack,
	}
	got := render(t, templates.SettingsForm(settings.AppSettings{}, nil, view, "", ""))
	if strings.Contains(got, "<img src=x") || strings.Contains(got, attack) {
		t.Fatal("untrusted installer status became executable markup")
	}
	if !strings.Contains(got, html.EscapeString(attack)) {
		t.Fatal("escaped status was lost")
	}
}

func TestCLISettingsManualPATHGuidance(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    []string
	}{
		{name: "unix", command: "/path/to/User's tools/bin/domjudge-tool", want: []string{"export PATH=", "command -v domjudge-tool", "hash -r", "shell startup", "User", "--help"}},
		{name: "windows", command: `C:\Users\Operator's tools\cli-install\bin\domjudge-tool.exe`, want: []string{"$env:Path", "Get-Command domjudge-tool", "User variables", "PowerShell", "--help"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view := templates.CLIInstallView{Status: cliinstall.Status{State: "current", CommandPath: tc.command}}
			got := render(t, templates.SettingsForm(settings.AppSettings{}, nil, view, "", ""))
			for _, text := range append(tc.want, "does not change PATH", "another terminal", "registry", "administrator", "profiles") {
				if !strings.Contains(got, html.EscapeString(text)) {
					t.Errorf("manual setup guidance lacks %q", text)
				}
			}
		})
	}
}

func TestCLISettingsWithoutTokenHasNoMutationForms(t *testing.T) {
	view := templates.CLIInstallView{Status: cliinstall.Status{State: "absent", CanInstall: true, CanRelink: true, CanUninstall: true}}
	got := render(t, templates.SettingsForm(settings.AppSettings{}, nil, view, "", ""))
	if strings.Contains(got, `action="/settings/cli/`) {
		t.Fatal("tokenless response rendered actionable CLI forms")
	}
}
