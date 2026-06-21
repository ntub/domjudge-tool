//go:build desktop

package main

import (
	"context"
	"encoding/json"
	"io"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web/gatewayfactory"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// desktopRunImport is the production ImportFunc wired into the desktop app.
// It converts a ServerProfile to the legacy config format, builds real API
// and web-gateway clients, then calls the existing CLI importer.
func desktopRunImport(
	ctx context.Context,
	p profile.ServerProfile,
	r io.Reader,
	rep progress.Reporter,
) (importer.ImportUsersTeamsSummary, error) {
	cfg := profileToConfig(p)

	api := apiv4.New(apiv4.Options{
		BaseURL:    cfg.Host,
		Username:   cfg.Username,
		Password:   cfg.Password,
		DisableSSL: cfg.DisableSSL,
	})

	gw, err := gatewayfactory.New(ctx, cfg)
	if err != nil {
		return importer.ImportUsersTeamsSummary{}, err
	}

	opts := importer.ImportOptions{
		CategoryID:    cfg.CategoryID,
		AffiliationID: cfg.AffiliationID,
		Enabled:       true,
		Router:        importer.NewDefaultRouter(cfg),
		API:           api,
		Reporter:      rep,
	}
	if cfg.AffiliationCountry != nil {
		opts.AffiliationCountry = *cfg.AffiliationCountry
	}
	if cfg.ContestID != nil {
		opts.ContestHint = *cfg.ContestID
	}
	if len(cfg.UserRoles) > 0 {
		opts.UserRoles = cfg.UserRoles
	}

	result, err := importer.ImportUsersTeamsWithReport(ctx, gw, api.Users, r, opts)
	return result.Summary, err
}

// profileToConfig converts a desktop ServerProfile to the legacy DomServerClient
// format that the CLI gatewayfactory and apiv4 package expect.
func profileToConfig(p profile.ServerProfile) *config.DomServerClient {
	cfg := config.Default()
	cfg.Host = p.Host
	cfg.Username = p.Username
	cfg.Password = p.Password
	cfg.DisableSSL = p.DisableSSL
	cfg.Timeout = p.Timeout
	cfg.MaxConnections = p.MaxConnections
	cfg.MaxKeepaliveConnections = p.MaxKeepaliveConnections
	cfg.CategoryID = p.CategoryID
	cfg.AffiliationID = p.AffiliationID
	cfg.AffiliationCountry = p.AffiliationCountry
	cfg.ContestID = p.ContestID
	cfg.PreferScraper = p.PreferScraper
	if p.Version != "" {
		cfg.Version = p.Version
	}
	if p.APIVersion != "" {
		cfg.APIVersion = p.APIVersion
	}
	// UserRoles is stored as a JSON array string in the profile
	if p.UserRoles != "" && p.UserRoles != "[]" {
		var roles []int
		if err := json.Unmarshal([]byte(p.UserRoles), &roles); err == nil {
			cfg.UserRoles = roles
		}
	}
	return cfg
}
