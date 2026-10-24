package templates_test

import (
	"bytes"
	"context"
	"html"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"

	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
	"github.com/ntub/domjudge-tool/internal/model"
)

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestViewsEscapeUntrustedData(t *testing.T) {
	const attack = `"><img src=x onerror="bad()">&`
	cid := "1"
	p := profile.ServerProfile{Name: attack, Host: attack, Username: attack, Password: attack, Version: attack, ContestID: &cid, IsDefault: true, DisableSSL: true}
	p.ID = 1
	other := p
	other.ID = 2
	other.IsDefault = false
	profiles := []profile.ServerProfile{p, other}
	finished := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	entry := oplog.OperationLog{ProfileID: 1, Type: "import-users-teams", Status: oplog.StatusDone, StartedAt: finished.Add(-time.Minute), FinishedAt: &finished, ParamsJSON: attack, SummaryJSON: attack}
	entry.ID = 1
	failed := entry
	failed.ID = 2
	failed.Status = oplog.StatusError
	failed.ErrorMsg = attack
	cur := settings.AppSettings{Theme: "light", DefaultProfileID: 1, SMTPHost: attack, SMTPUsername: attack, SMTPPassword: attack, SMTPFromEmail: attack, SMTPPort: 587, SMTPUseTLS: true}
	cases := []struct {
		name string
		view templ.Component
	}{
		{"title", templates.Layout(attack)},
		{"flash", templates.Flash("error", attack)},
		{"profile list", templates.ProfileList(profiles, "", "")},
		{"profile edit", templates.ProfileForm(p, "/profiles/1", "Edit", "", "")},
		{"import users", templates.ImportUsersForm(profiles, "", "")},
		{"remove users", templates.RemoveUsersForm(profiles, "", "")},
		{"export users", templates.ExportUsersForm(profiles)},
		{"password form", templates.SetPasswordForm(profiles, "", "")},
		{"generated password", templates.SetPasswordDone(attack, attack, true)},
		{"manual password", templates.SetPasswordDone(attack, attack, false)},
		{"download problems", templates.DownloadProblemsForm(profiles, attack, "", "")},
		{"import problems", templates.ImportProblemsForm(profiles, "", "")},
		{"scoreboard", templates.DownloadScoreboardForm(profiles)},
		{"submissions", templates.DownloadSubmissionsForm(profiles, attack, "", "")},
		{"email tools", templates.EmailToolsIndex(attack, "error")},
		{"email scaffold", templates.EmailScaffoldForm(attack, "error")},
		{"email validate", templates.EmailValidateForm(attack, "error")},
		{"email render", templates.EmailRenderForm(attack, "error")},
		{"email send", templates.EmailSendForm(attack, "error")},
		{"history list", templates.HistoryList([]oplog.OperationLog{entry, failed}, map[uint]string{1: attack}, templates.HistoryFilter{Type: "import-users-teams", ProfileID: 1})},
		{"completed history", templates.HistoryDetail(entry, attack)},
		{"failed history", templates.HistoryDetail(failed, attack)},
		{"settings", templates.SettingsForm(cur, profiles, templates.CLIInstallView{}, "", "")},
		{"contest options", templates.ContestSelectFragment([]model.Contest{{ID: "1", Name: attack, ShortName: attack}}, &cid, true, true)},
		{"contest fallback", templates.ContestInputFragment(attack, attack, attack, true)},
		{"username options", templates.UsernameSelectFragment([]model.User{{Username: attack, Name: attack}}, true)},
		{"username fallback", templates.UsernameInputFragment(attack, attack, attack, true)},
		{"problem filter", templates.ProblemsTextInputFragment("only", attack, attack, attack)},
		{"user filter", templates.UsersTextInputFragment("include", attack, attack, attack)},
		{"team options", templates.TeamSelectFragment([]model.Team{{ID: "1", Name: attack}}, false)},
		{"team fallback", templates.TeamInputFragment(false, attack)},
		{"tag options", templates.TagMultiSelect("tags", "only", "Problems", []templates.SelectOption{{Value: attack, Label: attack}}, "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := render(t, tc.view)
			if strings.Contains(got, attack) || strings.Contains(got, "<img src=x") {
				t.Fatal("untrusted data became executable markup")
			}
			if !strings.Contains(got, html.EscapeString(attack)) {
				t.Fatal("escaped user data was lost")
			}
		})
	}
}

func TestProgressViewsAnnounceResults(t *testing.T) {
	cases := []struct {
		name string
		view templ.Component
	}{
		{"import users", templates.ImportUsersRun(17)},
		{"remove users", templates.RemoveUsersRun(17)},
		{"download problems", templates.DownloadProblemsRun(17)},
		{"import problems", templates.ImportProblemsRun(17)},
		{"download submissions", templates.DownloadSubmissionsRun(17)},
		{"email send", templates.EmailSendRun(17)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := render(t, tc.view)
			for _, semantic := range []string{`aria-live="polite"`, `role="status"`, `<progress `} {
				if !strings.Contains(got, semantic) {
					t.Errorf("progress view lacks accessible result semantics %s", semantic)
				}
			}
		})
	}
}

func TestLayoutAndFlashAccessibility(t *testing.T) {
	got := render(t, templates.Layout("Page"))
	if !strings.Contains(got, `href="#dj-main"`) || !strings.Contains(got, `<main class="dj-content" id="dj-main" tabindex="-1">`) {
		t.Fatal("skip link has no focusable main destination")
	}
	if got := render(t, templates.Flash("error", "request failed")); !strings.Contains(got, `role="alert"`) {
		t.Fatal("error feedback is not announced")
	}
	if got := render(t, templates.Flash("", "")); got != "" {
		t.Fatalf("absent feedback still renders an alert: %q", got)
	}
}

func TestFormsPreserveSelections(t *testing.T) {
	cid := "2"
	contests := []model.Contest{{ID: "1", Name: "First"}, {ID: "2", Name: "Second"}}
	got := render(t, templates.ContestSelectFragment(contests, &cid, true, true))
	if !regexp.MustCompile(`<option[^>]*value="2"[^>]*selected`).MatchString(got) {
		t.Fatal("saved contest is not selected")
	}
	if regexp.MustCompile(`<option[^>]*value="1"[^>]*selected`).MatchString(got) {
		t.Fatal("unrelated contest overrides the saved selection")
	}
	p := profile.ServerProfile{Name: "Saved"}
	p.ID = 2
	got = render(t, templates.SettingsForm(settings.AppSettings{Theme: "light", DefaultProfileID: 2}, []profile.ServerProfile{p}, templates.CLIInstallView{}, "", ""))
	for _, value := range []string{"light", "2"} {
		if !regexp.MustCompile(`<option[^>]*value="` + value + `"[^>]*selected`).MatchString(got) {
			t.Errorf("saved setting %s is not selected", value)
		}
	}
	got = render(t, templates.RemoveUsersForm([]profile.ServerProfile{p}, "", ""))
	if !regexp.MustCompile(`<input[^>]*id="dry_run"[^>]*checked`).MatchString(got) {
		t.Fatal("initial deletion form permits mutation without opting out of dry-run")
	}
}

func TestEmptyViewsOfferRecovery(t *testing.T) {
	got := render(t, templates.ProfileList(nil, "", ""))
	if !strings.Contains(got, `href="/profiles/new"`) || strings.Contains(got, `<tbody>`) {
		t.Fatal("empty server list offers no create action or renders a phantom row")
	}
	got = render(t, templates.ProfileForm(profile.ServerProfile{}, "/profiles/new", "Add", "", ""))
	if !regexp.MustCompile(`<input[^>]*name="host"[^>]*required`).MatchString(got) {
		t.Fatal("profile creation allows an absent server URL")
	}
	got = render(t, templates.HistoryList(nil, nil, templates.HistoryFilter{}))
	if strings.Contains(got, `href="/history/0"`) {
		t.Fatal("empty history links to a nonexistent operation")
	}
}
