// Package users implements the "User & Team Commands:" subtree of the
// domjudge-tool CLI: import-users-teams, import-users-teams-example,
// export-users, rm-teams-and-users, set-password.
//
// Every command except import-users-teams-example needs a loaded
// config and live HTTP clients. The shared Deps struct injects the
// API and gateway constructors so tests can swap in stubs without
// hitting the network.
package users

import (
	"context"
	"errors"
	"io"
	"time"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/httpx"
	"github.com/ntub/domjudge-tool/internal/client/web"
	v7 "github.com/ntub/domjudge-tool/internal/client/web/v7"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/importer"
)

// groupID matches the "users" group registered in cli.NewRootCmd. The
// literal is duplicated to avoid a cli ↔ users import cycle (cli
// imports users via the registry pattern; users imports cli for ctx
// helpers). docs/phase1/README.md §"Command tree shape" is the
// canonical source of truth.
const groupID = "users"

// AccountsImporter is the ImportsResource subset set-password needs.
type AccountsImporter interface {
	Accounts(ctx context.Context, tsv io.Reader) ([]string, error)
}

// Deps groups the factories users-subtree commands need. The default
// implementation builds real apiv4 and web/v7 clients; tests provide
// stubs returning the same interfaces.
type Deps struct {
	// NewAPI constructs an apiv4 client for cfg. Returning a *Client
	// (rather than a sub-resource pointer) lets each command pick the
	// resource it needs.
	NewAPI func(cfg *config.DomServerClient) *apiv4.Client
	// NewGateway constructs a web.Gateway for cfg. ctx is forwarded so
	// future implementations can short-circuit on cancellation; the
	// Phase-1 v7 implementation ignores it.
	NewGateway func(ctx context.Context, cfg *config.DomServerClient) (web.Gateway, error)
	// NewAccountsImporter selects the API bulk accounts importer from
	// a top-level apiv4 client.
	NewAccountsImporter func(api *apiv4.Client) AccountsImporter
	// NewRouter constructs the Phase 1.5 hybrid operation router.
	NewRouter func(cfg *config.DomServerClient) importer.Router
}

// DefaultDeps returns a Deps wired against the production apiv4 and
// web/v7 clients. cmd/domjudge-tool/main.go's blank import calls
// register(), which wires every command with these defaults.
func DefaultDeps() Deps {
	return Deps{
		NewAPI:              defaultNewAPI,
		NewGateway:          defaultNewGateway,
		NewAccountsImporter: defaultNewAccountsImporter,
		NewRouter:           defaultNewRouter,
	}
}

func defaultNewAPI(cfg *config.DomServerClient) *apiv4.Client {
	return apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
}

func defaultNewGateway(_ context.Context, cfg *config.DomServerClient) (web.Gateway, error) {
	session, err := httpx.NewSession(httpx.SessionOptions{
		BaseURL:    cfg.Host,
		DisableSSL: cfg.DisableSSL,
		Timeout:    derefSecondsDuration(cfg.Timeout),
	})
	if err != nil {
		return nil, err
	}
	return v7.New(v7.Options{
		Session:  session,
		Username: cfg.Username,
		Password: cfg.Password,
	})
}

func defaultNewAccountsImporter(api *apiv4.Client) AccountsImporter {
	if api == nil || api.Imports == nil {
		return missingAccountsImporter{}
	}
	return api.Imports
}

func defaultNewRouter(cfg *config.DomServerClient) importer.Router {
	return importer.NewDefaultRouter(cfg)
}

func (d Deps) accountsImporterFor(api *apiv4.Client) AccountsImporter {
	if d.NewAccountsImporter != nil {
		return d.NewAccountsImporter(api)
	}
	return defaultNewAccountsImporter(api)
}

func (d Deps) routerFor(cfg *config.DomServerClient) importer.Router {
	if d.NewRouter != nil {
		return d.NewRouter(cfg)
	}
	return defaultNewRouter(cfg)
}

var errAccountsImporterUnavailable = errors.New("users: api imports accounts resource unavailable")

type missingAccountsImporter struct{}

func (missingAccountsImporter) Accounts(context.Context, io.Reader) ([]string, error) {
	return nil, errAccountsImporterUnavailable
}

// derefSecondsDuration converts the optional float-seconds field on
// the config (Pydantic timeout) to a time.Duration. Zero/nil means
// "let the client apply its own default".
func derefSecondsDuration(p *float64) time.Duration {
	if p == nil {
		return 0
	}
	return time.Duration(*p * float64(time.Second))
}

// derefString returns the pointed-to value, or empty string for nil.
// Used by import-users-teams to pass cfg.contest_id into the importer
// without exposing pointer plumbing in the call site.
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
