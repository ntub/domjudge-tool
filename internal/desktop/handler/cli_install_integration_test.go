package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/cliinstall"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
)

func newCLIManager(t *testing.T) (*cliinstall.Manager, cliinstall.Config, string) {
	t.Helper()
	root := t.TempDir()
	bundle := filepath.Join(root, "App space 日本語.app")
	base := bundle
	executable := filepath.Join(bundle, "domjudge-tool-app")
	name := "domjudge-tool"
	if runtime.GOOS == "darwin" {
		executable = filepath.Join(bundle, "Contents", "MacOS", "domjudge-tool-app")
		base = filepath.Join(bundle, "Contents", "Resources")
	}
	if runtime.GOOS == "windows" {
		executable += ".exe"
		name += ".exe"
		writeCLITestFile(t, filepath.Join(bundle, "resources", "cli-launcher.exe"), []byte("console launcher fixture"))
	}
	writeCLITestFile(t, executable, []byte("desktop fixture"))
	writeCLITestFile(t, filepath.Join(base, "cli", name), []byte("CLI fixture"))
	manifest, err := json.Marshal(map[string]any{
		"schema": 1, "version": "v1.2.3", "revision": "fixture-revision", "goos": runtime.GOOS, "goarch": runtime.GOARCH,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeCLITestFile(t, filepath.Join(base, "bundle.json"), manifest)
	cfg := cliinstall.Config{ExecutablePath: executable, HomeDir: filepath.Join(root, "home"), StateDir: filepath.Join(root, "state")}
	manager, err := cliinstall.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return manager, cfg, bundle
}

func writeCLITestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
}

func readCLIForm(t *testing.T, h http.Handler, action string) url.Values {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	req.Host = cliTestHost
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET settings = %d: %s", rec.Code, rec.Body.String())
	}
	assertCLIResponseHeaders(t, rec)
	form := regexp.MustCompile(`<form[^>]*action="/settings/cli/` + action + `"[^>]*>(.*?)</form>`).FindString(rec.Body.String())
	if form == "" {
		t.Fatalf("settings has no %s form: %s", action, rec.Body.String())
	}
	values := url.Values{"confirm": {action}}
	for _, key := range []string{"csrf_token", "revision"} {
		match := regexp.MustCompile(`name="` + key + `" value="([^"]+)"`).FindStringSubmatch(form)
		if len(match) != 2 {
			t.Fatalf("%s form has no %s", action, key)
		}
		values.Set(key, html.UnescapeString(match[1]))
	}
	return values
}

func postCLIForm(h http.Handler, action string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/settings/cli/"+action, strings.NewReader(form.Encode()))
	req.Host = cliTestHost
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func inspectCLI(t *testing.T, manager *cliinstall.Manager, want string) cliinstall.Status {
	t.Helper()
	status, err := manager.Inspect(context.Background())
	if err != nil || status.State != want {
		t.Fatalf("Inspect = %+v, %v; want %s", status, err, want)
	}
	return status
}

func assertNoCLIState(t *testing.T, cfg cliinstall.Config) {
	t.Helper()
	for _, path := range []string{cfg.HomeDir, cfg.StateDir} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("read-only or denied request wrote %s: %v", path, err)
		}
	}
}

func TestCLISettingsLifecycle_Integration(t *testing.T) {
	manager, cfg, bundle := newCLIManager(t)
	h, store := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager, AppVersion: "v1.2.3"})
	before := inspectCLI(t, manager, "absent")
	install := readCLIForm(t, h, "install")
	if install.Get("revision") != before.Revision {
		t.Fatal("form revision does not describe its displayed paths")
	}
	assertNoCLIState(t, cfg)

	// General/SMTP saving is independent and must not install a command.
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader("theme=light&smtp_host=smtp.example.org"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	saved := httptest.NewRecorder()
	h.ServeHTTP(saved, req)
	if saved.Code != http.StatusSeeOther {
		t.Fatalf("save settings = %d", saved.Code)
	}
	assertNoCLIState(t, cfg)

	installed := postCLIForm(h, "install", install)
	if installed.Code != http.StatusSeeOther || !strings.HasSuffix(installed.Header().Get("Location"), "#cli-install") {
		t.Fatalf("install = %d: %s", installed.Code, installed.Body.String())
	}
	assertCLIResponseHeaders(t, installed)
	current := inspectCLI(t, manager, "current")
	if current.TargetPath != current.CLIPath {
		t.Fatal("HTTP install did not link to its displayed bundle")
	}
	publicBefore, err := os.Lstat(current.CommandPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		link, err := os.Readlink(current.CommandPath)
		if err != nil || link != filepath.Join(cfg.StateDir, "target") {
			t.Fatalf("public indirection = %q, %v", link, err)
		}
	}
	stale := postCLIForm(h, "install", install)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "changed since this form was opened") {
		t.Fatalf("stale form = %d: %s", stale.Code, stale.Body.String())
	}
	if after := inspectCLI(t, manager, "current"); after.Revision != current.Revision {
		t.Fatal("stale request changed installed state")
	}

	movedBundle := bundle + " moved"
	if err := os.Rename(bundle, movedBundle); err != nil {
		t.Fatal(err)
	}
	relativeExecutable, err := filepath.Rel(bundle, cfg.ExecutablePath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ExecutablePath = filepath.Join(movedBundle, relativeExecutable)
	moved, err := cliinstall.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	movedHTTP, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: moved})
	relink := readCLIForm(t, movedHTTP, "relink")
	relinked := postCLIForm(movedHTTP, "relink", relink)
	if relinked.Code != http.StatusSeeOther {
		t.Fatalf("relink = %d: %s", relinked.Code, relinked.Body.String())
	}
	current = inspectCLI(t, moved, "current")
	publicAfter, err := os.Lstat(current.CommandPath)
	if err != nil || !os.SameFile(publicBefore, publicAfter) {
		t.Fatalf("relink replaced the public command: %v", err)
	}
	resolvedBundle, err := filepath.EvalSymlinks(movedBundle)
	if err != nil {
		t.Fatal(err)
	}
	if current.TargetPath != current.CLIPath || current.BundlePath != resolvedBundle {
		t.Fatalf("relink did not update the private target to the moved app: %+v", current)
	}
	uninstalled := postCLIForm(movedHTTP, "uninstall", readCLIForm(t, movedHTTP, "uninstall"))
	if uninstalled.Code != http.StatusSeeOther {
		t.Fatalf("uninstall = %d: %s", uninstalled.Code, uninstalled.Body.String())
	}
	inspectCLI(t, moved, "absent")
	if _, err := os.Lstat(current.CommandPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("uninstall retained public command: %v", err)
	}
	for _, path := range []string{cfg.ExecutablePath, current.CLIPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("uninstall removed app binary %s: %v", path, err)
		}
	}
	settings, err := store.Get(context.Background())
	if err != nil || settings.Theme != "light" || settings.SMTPHost != "smtp.example.org" {
		t.Fatalf("CLI lifecycle changed saved settings: %+v, %v", settings, err)
	}
}

func TestCLISettingsMutationBoundary_Integration(t *testing.T) {
	cases := []struct {
		name   string
		change func(*http.Request, url.Values)
		want   int
	}{
		{name: "missing token", change: func(_ *http.Request, v url.Values) { v.Del("csrf_token") }, want: http.StatusForbidden},
		{name: "incorrect token", change: func(_ *http.Request, v url.Values) { v.Set("csrf_token", strings.Repeat("0", 64)) }, want: http.StatusForbidden},
		{name: "repeated token", change: func(_ *http.Request, v url.Values) { v.Add("csrf_token", v.Get("csrf_token")) }, want: http.StatusBadRequest},
		{name: "missing revision", change: func(_ *http.Request, v url.Values) { v.Del("revision") }, want: http.StatusBadRequest},
		{name: "missing confirmation", change: func(_ *http.Request, v url.Values) { v.Del("confirm") }, want: http.StatusBadRequest},
		{name: "other action confirmation", change: func(_ *http.Request, v url.Values) { v.Set("confirm", "uninstall") }, want: http.StatusBadRequest},
		{name: "repeated revision", change: func(_ *http.Request, v url.Values) { v.Add("revision", v.Get("revision")) }, want: http.StatusBadRequest},
		{name: "source path", change: func(_ *http.Request, v url.Values) { v.Set("source", "/path/to/foreign") }, want: http.StatusBadRequest},
		{name: "destination path", change: func(_ *http.Request, v url.Values) { v.Set("destination", "/path/to/foreign") }, want: http.StatusBadRequest},
		{name: "executable path", change: func(_ *http.Request, v url.Values) { v.Set("executable", "/path/to/foreign") }, want: http.StatusBadRequest},
		{name: "unknown field", change: func(_ *http.Request, v url.Values) { v.Set("force", "true") }, want: http.StatusBadRequest},
		{name: "query token", change: func(r *http.Request, v url.Values) {
			r.URL.RawQuery = "csrf_token=" + v.Get("csrf_token")
			v.Del("csrf_token")
		}, want: http.StatusBadRequest},
		{name: "query path", change: func(r *http.Request, _ url.Values) { r.URL.RawQuery = "path=/path/to/foreign" }, want: http.StatusBadRequest},
		{name: "oversized form", change: func(_ *http.Request, v url.Values) { v.Set("revision", strings.Repeat("x", 8192)) }, want: http.StatusBadRequest},
		{name: "multipart", change: func(r *http.Request, _ url.Values) {
			r.Header.Set("Content-Type", "multipart/form-data; boundary=fixture")
		}, want: http.StatusUnsupportedMediaType},
		{name: "json", change: func(r *http.Request, _ url.Values) { r.Header.Set("Content-Type", "application/json") }, want: http.StatusUnsupportedMediaType},
		{name: "host", change: func(r *http.Request, _ url.Values) { r.Host = "attacker.example.org:9900" }, want: http.StatusForbidden},
		{name: "origin", change: func(r *http.Request, _ url.Values) { r.Header.Set("Origin", "https://attacker.example.org") }, want: http.StatusForbidden},
		{name: "null origin", change: func(r *http.Request, _ url.Values) { r.Header.Set("Origin", "null") }, want: http.StatusForbidden},
		{name: "cross site metadata", change: func(r *http.Request, _ url.Values) { r.Header.Set("Sec-Fetch-Site", "cross-site") }, want: http.StatusForbidden},
		{name: "same site metadata", change: func(r *http.Request, _ url.Values) { r.Header.Set("Sec-Fetch-Site", "same-site") }, want: http.StatusForbidden},
		{name: "native user agent is not authority", change: func(r *http.Request, v url.Values) {
			r.Header.Set("User-Agent", "domjudge-tool-app")
			v.Del("csrf_token")
		}, want: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager, cfg, _ := newCLIManager(t)
			h, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager})
			form := readCLIForm(t, h, "install")
			req := httptest.NewRequest(http.MethodPost, "/settings/cli/install", nil)
			req.Host = cliTestHost
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			tc.change(req, form)
			body := strings.NewReader(form.Encode())
			req.Body = io.NopCloser(body)
			req.ContentLength = int64(body.Len())
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			assertCLIResponseHeaders(t, rec)
			inspectCLI(t, manager, "absent")
			assertNoCLIState(t, cfg)
		})
	}
}

func TestCLISettingsProcessLocalTokens_Integration(t *testing.T) {
	manager, cfg, _ := newCLIManager(t)
	h, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager})
	other, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager})
	form := readCLIForm(t, h, "install")
	otherForm := readCLIForm(t, other, "install")
	if form.Get("csrf_token") == otherForm.Get("csrf_token") {
		t.Fatal("separate handler instances share a CSRF token")
	}
	if rec := postCLIForm(other, "install", form); rec.Code != http.StatusForbidden {
		t.Fatalf("old process token = %d", rec.Code)
	}
	assertNoCLIState(t, cfg)
}

func TestCLISettingsForeignCommandIsPreserved_Integration(t *testing.T) {
	manager, _, _ := newCLIManager(t)
	h, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager})
	form := readCLIForm(t, h, "install")
	before := inspectCLI(t, manager, "absent")
	writeCLITestFile(t, before.CommandPath, []byte("unrelated command"))
	rec := postCLIForm(h, "install", form)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "conflict") {
		t.Fatalf("foreign command response = %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `action="/settings/cli/`) {
		t.Fatal("foreign conflict offers a mutation")
	}
	got, err := os.ReadFile(before.CommandPath)
	if err != nil || string(got) != "unrelated command" {
		t.Fatalf("foreign command overwritten: %q, %v", got, err)
	}
}

func TestCLISettingsMissingBundleCanUninstall_Integration(t *testing.T) {
	manager, _, _ := newCLIManager(t)
	h, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager})
	if rec := postCLIForm(h, "install", readCLIForm(t, h, "install")); rec.Code != http.StatusSeeOther {
		t.Fatalf("install = %d: %s", rec.Code, rec.Body.String())
	}
	current := inspectCLI(t, manager, "current")
	if err := os.Remove(current.CLIPath); err != nil {
		t.Fatal(err)
	}
	rec := postCLIForm(h, "uninstall", readCLIForm(t, h, "uninstall"))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("missing-bundle uninstall = %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Lstat(current.CommandPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("broken command was not removed: %v", err)
	}
}

func TestCLISettingsCorruptStateIsVisible_Integration(t *testing.T) {
	manager, cfg, _ := newCLIManager(t)
	h, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager})
	form := readCLIForm(t, h, "install")
	writeCLITestFile(t, filepath.Join(cfg.StateDir, "receipt.json"), []byte("invalid receipt"))
	rec := postCLIForm(h, "install", form)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "receipt") {
		t.Fatalf("state error = %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `action="/settings/cli/`) {
		t.Fatal("corrupt state offers a mutation")
	}
	got, err := os.ReadFile(filepath.Join(cfg.StateDir, "receipt.json"))
	if err != nil || string(got) != "invalid receipt" {
		t.Fatalf("corrupt receipt silently replaced: %q, %v", got, err)
	}
}

func TestCLISettingsPreviewFailsClosed_Integration(t *testing.T) {
	manager, cfg, _ := newCLIManager(t)
	h, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager, UnavailableReason: "CLI integration is disabled in preview mode."})
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	req.Host = cliTestHost
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "disabled in preview") || strings.Contains(rec.Body.String(), cfg.HomeDir) {
		t.Fatalf("preview discovered native paths: %d: %s", rec.Code, rec.Body.String())
	}
	for _, action := range []string{"install", "relink", "uninstall"} {
		rejected := postCLIForm(h, action, url.Values{"csrf_token": {"not-issued"}, "revision": {"not-inspected"}, "confirm": {action}})
		if rejected.Code != http.StatusForbidden {
			t.Errorf("preview %s = %d", action, rejected.Code)
		}
		assertCLIResponseHeaders(t, rejected)
	}
	assertNoCLIState(t, cfg)
}

func TestCLISettingsEveryActionRejectsForeignOrigin_Integration(t *testing.T) {
	manager, cfg, _ := newCLIManager(t)
	h, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager})
	form := readCLIForm(t, h, "install")
	for _, action := range []string{"install", "relink", "uninstall"} {
		t.Run(action, func(t *testing.T) {
			form.Set("confirm", action)
			req := httptest.NewRequest(http.MethodPost, "/settings/cli/"+action, strings.NewReader(form.Encode()))
			req.Host = cliTestHost
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", "https://attacker.example.org")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s accepted foreign origin: %d", action, rec.Code)
			}
			assertCLIResponseHeaders(t, rec)
		})
	}
	assertNoCLIState(t, cfg)
}

func TestCLISettingsSameOriginMutation_Integration(t *testing.T) {
	manager, _, _ := newCLIManager(t)
	h, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager})
	form := readCLIForm(t, h, "install")
	req := httptest.NewRequest(http.MethodPost, "/settings/cli/install", strings.NewReader(form.Encode()))
	req.Host = cliTestHost
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+cliTestHost)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Dest", "document")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("same-origin install = %d: %s", rec.Code, rec.Body.String())
	}
	inspectCLI(t, manager, "current")
}

func TestCLISettingsUnavailableActionsNeverWrite_Integration(t *testing.T) {
	manager, cfg, _ := newCLIManager(t)
	h, _ := newSettingsCLIEnv(t, handler.CLIInstallOptions{Host: cliTestHost, Installer: manager})
	form := readCLIForm(t, h, "install")
	for _, action := range []string{"relink", "uninstall"} {
		form.Set("confirm", action)
		rec := postCLIForm(h, action, form)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "action is unavailable") {
			t.Errorf("disabled %s = %d: %s", action, rec.Code, rec.Body.String())
		}
		assertCLIResponseHeaders(t, rec)
	}
	assertNoCLIState(t, cfg)
}
