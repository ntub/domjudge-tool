package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/handler"
)

const cliTestHost = "127.0.0.1:9900"

func TestCLISettingsUnavailable(t *testing.T) {
	h, _ := newSettingsEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	req.Host = cliTestHost
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	for _, text := range []string{"Command-Line Tool", "unavailable", "disabled in preview", "PATH"} {
		if !strings.Contains(rec.Body.String(), text) {
			t.Errorf("settings lacks %q", text)
		}
	}
	if strings.Contains(rec.Body.String(), `action="/settings/cli/`) {
		t.Error("unavailable installer exposes a mutation form")
	}
	assertCLIResponseHeaders(t, rec)
}

func TestCLISettingsRequestBoundary(t *testing.T) {
	h, _ := newSettingsEnv(t)
	cases := []struct {
		name   string
		host   string
		header http.Header
		want   int
	}{
		{name: "native navigation", host: cliTestHost, want: http.StatusOK},
		{name: "same origin", host: cliTestHost, header: http.Header{"Origin": {"http://" + cliTestHost}, "Sec-Fetch-Site": {"same-origin"}}, want: http.StatusOK},
		{name: "direct navigation", host: cliTestHost, header: http.Header{"Sec-Fetch-Site": {"none"}, "Sec-Fetch-Dest": {"document"}}, want: http.StatusOK},
		{name: "dns rebinding", host: "attacker.example.org:9900", want: http.StatusForbidden},
		{name: "other loopback name", host: "localhost:9900", want: http.StatusForbidden},
		{name: "other listener", host: "127.0.0.1:9901", want: http.StatusForbidden},
		{name: "null origin", host: cliTestHost, header: http.Header{"Origin": {"null"}}, want: http.StatusForbidden},
		{name: "foreign origin", host: cliTestHost, header: http.Header{"Origin": {"https://attacker.example.org"}}, want: http.StatusForbidden},
		{name: "same host wrong scheme", host: cliTestHost, header: http.Header{"Origin": {"https://" + cliTestHost}}, want: http.StatusForbidden},
		{name: "origin path", host: cliTestHost, header: http.Header{"Origin": {"http://" + cliTestHost + "/"}}, want: http.StatusForbidden},
		{name: "multiple origins", host: cliTestHost, header: http.Header{"Origin": {"http://" + cliTestHost, "https://attacker.example.org"}}, want: http.StatusForbidden},
		{name: "cross site", host: cliTestHost, header: http.Header{"Sec-Fetch-Site": {"cross-site"}}, want: http.StatusForbidden},
		{name: "same site different origin", host: cliTestHost, header: http.Header{"Sec-Fetch-Site": {"same-site"}}, want: http.StatusForbidden},
		{name: "unknown fetch site", host: cliTestHost, header: http.Header{"Sec-Fetch-Site": {"unexpected"}}, want: http.StatusForbidden},
		{name: "multiple fetch sites", host: cliTestHost, header: http.Header{"Sec-Fetch-Site": {"same-origin", "cross-site"}}, want: http.StatusForbidden},
		{name: "framed navigation", host: cliTestHost, header: http.Header{"Sec-Fetch-Site": {"same-origin"}, "Sec-Fetch-Dest": {"iframe"}}, want: http.StatusForbidden},
		{name: "spoofed native agent", host: "attacker.example.org:9900", header: http.Header{"User-Agent": {"domjudge-tool-app"}}, want: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/settings", nil)
			req.Host = tc.host
			if tc.header != nil {
				req.Header = tc.header
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			assertCLIResponseHeaders(t, rec)
			if tc.want != http.StatusOK && strings.Contains(rec.Body.String(), `name="csrf_token"`) {
				t.Error("denied request leaked a CSRF token")
			}
		})
	}
}

func assertCLIResponseHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for name, want := range map[string]string{
		"Cache-Control":           "no-store",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "frame-ancestors 'none'",
		"Referrer-Policy":         "same-origin",
	} {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestCLISettingsRejectInvalidListener(t *testing.T) {
	for _, host := range []string{"", "localhost:9900", "attacker.example.org:9900", "192.0.2.1:9900", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:invalid"} {
		t.Run(host, func(t *testing.T) {
			mux := http.NewServeMux()
			if err := handler.RegisterSettings(mux, nil, nil, handler.CLIInstallOptions{Host: host}); err == nil {
				t.Fatal("settings accepted an ambiguous or non-loopback origin")
			}
		})
	}
}

func TestCLISettingsActionsRequirePOST(t *testing.T) {
	h, _ := newSettingsEnv(t)
	for _, action := range []string{"install", "relink", "uninstall"} {
		req := httptest.NewRequest(http.MethodGet, "/settings/cli/"+action, nil)
		req.Host = cliTestHost
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost {
			t.Errorf("%s GET = %d, Allow = %q", action, rec.Code, rec.Header().Get("Allow"))
		}
		assertCLIResponseHeaders(t, rec)
	}
}
