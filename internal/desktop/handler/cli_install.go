package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ntub/domjudge-tool/internal/desktop/cliinstall"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
	"github.com/ntub/domjudge-tool/internal/desktop/templates"
)

// CLIInstaller manages only the integration paths selected by native composition.
// HTTP clients supply an inspected revision, never a filesystem path.
type CLIInstaller interface {
	Inspect(context.Context) (cliinstall.Status, error)
	Install(context.Context, string) (cliinstall.Status, error)
	Relink(context.Context, string) (cliinstall.Status, error)
	Uninstall(context.Context, string) (cliinstall.Status, error)
}

// CLIInstallOptions configures the Settings boundary for one loopback listener.
// A nil Installer or nonempty UnavailableReason disables every CLI mutation.
type CLIInstallOptions struct {
	Installer         CLIInstaller
	Host              string
	UnavailableReason string
	AppVersion        string
}

type cliInstallHandler struct {
	installer         CLIInstaller
	host              string
	token             string
	unavailableReason string
	appVersion        string
	settingsStore     *settings.Store
	profileStore      *profile.Store
}

func newCLIInstallHandler(opts CLIInstallOptions, settingsStore *settings.Store, profileStore *profile.Store) (*cliInstallHandler, error) {
	host, port, err := net.SplitHostPort(opts.Host)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("CLI settings require an exact loopback listener host: %q", opts.Host)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, fmt.Errorf("CLI settings require a valid listener port: %q", port)
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, fmt.Errorf("create CLI settings CSRF token: %w", err)
	}
	if opts.UnavailableReason != "" {
		opts.Installer = nil
	}
	if opts.Installer == nil && opts.UnavailableReason == "" {
		opts.UnavailableReason = "CLI integration is unavailable. Start the packaged desktop app to manage its command."
	}
	return &cliInstallHandler{
		installer: opts.Installer, host: opts.Host, token: hex.EncodeToString(token[:]),
		unavailableReason: opts.UnavailableReason, appVersion: opts.AppVersion,
		settingsStore: settingsStore, profileStore: profileStore,
	}, nil
}

func cliResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
	// no-referrer makes native form POSTs send Origin: null.
	w.Header().Set("Referrer-Policy", "same-origin")
}

func (h *cliInstallHandler) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cliResponseHeaders(w)
		if r.Host != h.host || (r.URL.Host != "" && r.URL.Host != h.host) {
			http.Error(w, "CLI settings require the desktop listener host", http.StatusForbidden)
			return
		}
		if origins, present := r.Header["Origin"]; present && (len(origins) != 1 || origins[0] != "http://"+h.host) {
			http.Error(w, "CLI settings require the desktop origin", http.StatusForbidden)
			return
		}
		if sites, present := r.Header["Sec-Fetch-Site"]; present && (len(sites) != 1 || (sites[0] != "same-origin" && sites[0] != "none")) {
			http.Error(w, "cross-origin CLI settings requests are not allowed", http.StatusForbidden)
			return
		}
		if destinations, present := r.Header["Sec-Fetch-Dest"]; present && (len(destinations) != 1 || (destinations[0] != "document" && destinations[0] != "empty")) {
			http.Error(w, "framed CLI settings requests are not allowed", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (h *cliInstallHandler) inspect(ctx context.Context) (cliinstall.Status, error) {
	if h.installer == nil {
		return cliinstall.Status{State: "unavailable", Reason: h.unavailableReason}, nil
	}
	status, err := h.installer.Inspect(ctx)
	if err != nil {
		status.State = "error"
		status.CanInstall, status.CanRelink, status.CanUninstall = false, false, false
		if status.Reason == "" {
			status.Reason = err.Error()
		}
	}
	return status, err
}

func (h *cliInstallHandler) view(status cliinstall.Status) templates.CLIInstallView {
	return templates.CLIInstallView{Status: status, Token: h.token, AppVersion: h.appVersion}
}

func (h *cliInstallHandler) mutation(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || contentType != "application/x-www-form-urlencoded" {
			http.Error(w, "CLI actions require a URL-encoded form", http.StatusUnsupportedMediaType)
			return
		}
		if r.URL.RawQuery != "" {
			http.Error(w, "CLI actions do not accept query fields", http.StatusBadRequest)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid CLI action form", http.StatusBadRequest)
			return
		}
		for key, values := range r.PostForm {
			if (key != "csrf_token" && key != "revision" && key != "confirm") || len(values) != 1 {
				http.Error(w, "unexpected or repeated CLI action field", http.StatusBadRequest)
				return
			}
		}
		if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(h.token)) != 1 {
			http.Error(w, "CLI action token is invalid; reload Settings and try again", http.StatusForbidden)
			return
		}
		revision := r.PostForm.Get("revision")
		if revision == "" || r.PostForm.Get("confirm") != action {
			http.Error(w, "confirm the displayed CLI action and reload Settings if its revision is missing", http.StatusBadRequest)
			return
		}

		status, err := h.inspect(r.Context())
		if h.installer == nil {
			renderSettings(w, r, h.settingsStore, h.profileStore, h.view(status), status.Reason, "error", http.StatusServiceUnavailable)
			return
		}
		if err != nil {
			renderSettings(w, r, h.settingsStore, h.profileStore, h.view(status), err.Error(), "error", http.StatusInternalServerError)
			return
		}
		if revision != status.Revision {
			renderSettings(w, r, h.settingsStore, h.profileStore, h.view(status), "CLI integration changed since this form was opened. Review the current paths and confirm again.", "error", http.StatusConflict)
			return
		}
		allowed := (action == "install" && status.CanInstall) || (action == "relink" && status.CanRelink) || (action == "uninstall" && status.CanUninstall)
		if !allowed {
			renderSettings(w, r, h.settingsStore, h.profileStore, h.view(status), "This CLI action is unavailable in the current state. Review the reason below.", "error", http.StatusConflict)
			return
		}

		var message string
		switch action {
		case "install":
			status, err = h.installer.Install(r.Context(), revision)
			message = "CLI command installed. Review the manual PATH instructions below."
		case "relink":
			status, err = h.installer.Relink(r.Context(), revision)
			message = "CLI command relinked to this app."
		case "uninstall":
			status, err = h.installer.Uninstall(r.Context(), revision)
			message = "Managed CLI integration removed. App binaries, profiles, and configuration were kept."
		}
		if err != nil {
			if status.State == "" {
				status = cliinstall.Status{State: "error", Reason: err.Error()}
			}
			renderSettings(w, r, h.settingsStore, h.profileStore, h.view(status), "CLI action failed: "+err.Error(), "error", http.StatusConflict)
			return
		}
		values := url.Values{"flash": {message}, "kind": {"success"}}
		http.Redirect(w, r, "/settings?"+values.Encode()+"#cli-install", http.StatusSeeOther)
	}
}

func registerCLIActions(mux *http.ServeMux, h *cliInstallHandler) {
	for _, action := range []string{"install", "relink", "uninstall"} {
		path := "/settings/cli/" + action
		mux.HandleFunc("POST "+path, h.guard(h.mutation(action)))
		mux.HandleFunc("GET "+path, h.guard(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "CLI actions require an explicit POST confirmation", http.StatusMethodNotAllowed)
		}))
	}
}

// cliUnavailableView carries no token: legacy Settings save errors must not
// expose CLI credentials through a route outside the new request boundary.
func cliUnavailableView() templates.CLIInstallView {
	return templates.CLIInstallView{Status: cliinstall.Status{
		State: "unavailable", Reason: "Reload Settings to inspect the command-line tool and confirm its paths.",
	}}
}

func cliFlash(r *http.Request) (message, kind string) {
	kind = r.URL.Query().Get("kind")
	if strings.TrimSpace(kind) == "" {
		kind = "info"
	}
	return r.URL.Query().Get("flash"), kind
}
