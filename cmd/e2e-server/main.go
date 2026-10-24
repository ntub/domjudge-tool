// Command e2e-server starts the desktop UI on :9901 with in-memory SQLite
// and nop run-funcs for browser-based end-to-end testing.
//
// Usage:
//
//	go run ./cmd/e2e-server
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/desktop/assets"
	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

const addr = "127.0.0.1:9901"

func main() {
	gdb, err := db.Open(":memory:")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	pStore, err := profile.NewStore(gdb)
	if err != nil {
		log.Fatalf("profile store: %v", err)
	}
	oStore, err := oplog.NewStore(gdb)
	if err != nil {
		log.Fatalf("oplog store: %v", err)
	}
	sStore, err := settings.NewStore(gdb)
	if err != nil {
		log.Fatalf("settings store: %v", err)
	}

	// Seed one demo profile so dropdowns are non-empty.
	cid := "1"
	_, err = pStore.Create(context.Background(), profile.ServerProfile{
		Name:      "Demo Server",
		Host:      "test-station.example.org",
		Username:  "admin",
		Password:  "secret",
		ContestID: &cid,
	})
	if err != nil {
		log.Fatalf("seed profile: %v", err)
	}

	mux := handler.New(pStore)

	// Screen 1: Import Users+Teams (SSE).
	handler.RegisterImportUsersTeams(mux, pStore, oStore, func(
		_ context.Context, _ profile.ServerProfile, _ io.Reader, rep progress.Reporter,
	) (importer.ImportUsersTeamsSummary, error) {
		rep.Start(1, "import")
		rep.Tick()
		rep.Finish()
		return importer.ImportUsersTeamsSummary{CreatedUsers: 1}, nil
	})

	// Screen 2: Remove Users+Teams (SSE).
	handler.RegisterRemoveUsersTeams(mux, pStore, oStore, func(
		_ context.Context, _ profile.ServerProfile, _ handler.DeleteOpts, rep progress.Reporter,
	) (handler.DeleteResult, error) {
		rep.Start(1, "delete")
		rep.Tick()
		rep.Finish()
		return handler.DeleteResult{DeletedUsers: 1}, nil
	})

	// Screen 3: Export Users (sync download).
	handler.RegisterExportUsers(mux, pStore, func(
		_ context.Context, _ profile.ServerProfile, _ handler.ExportOpts, w io.Writer,
	) error {
		_, _ = io.WriteString(w, "username,name,password\nuser1,User One,pw\n")
		return nil
	})

	// Screen 4: Download Problems (SSE).
	handler.RegisterDownloadProblems(mux, pStore, oStore, func(
		_ context.Context, _ profile.ServerProfile, _ handler.DownloadOpts, rep progress.Reporter,
	) (handler.DownloadResult, error) {
		rep.Start(1, "download")
		rep.Tick()
		rep.Finish()
		return handler.DownloadResult{Downloaded: 1, OutDir: "/tmp/e2e-problems"}, nil
	}, "/tmp/e2e-problems")

	// Screen 5: Import Problems (SSE).
	handler.RegisterImportProblems(mux, pStore, oStore, func(
		_ context.Context, _ profile.ServerProfile, _ handler.ImportProblemsOpts, rep progress.Reporter,
	) (handler.ImportProblemsResult, error) {
		rep.Start(1, "import")
		rep.Tick()
		rep.Finish()
		return handler.ImportProblemsResult{Imported: 1}, nil
	})

	// Screen 6: Download Scoreboard (sync download).
	handler.RegisterDownloadScoreboard(mux, pStore, oStore, func(
		_ context.Context, _ profile.ServerProfile, _ handler.DownloadScoreboardOpts, w io.Writer,
	) error {
		_, _ = io.WriteString(w, "Rank,Team,Score\r\n1,Team A,100\r\n")
		return nil
	})

	// Screen 7: Download Submissions (SSE).
	handler.RegisterDownloadSubmissions(mux, pStore, oStore, func(
		_ context.Context, _ profile.ServerProfile, _ handler.DownloadSubmissionsOpts, rep progress.Reporter,
	) (handler.DownloadSubmissionsResult, error) {
		rep.Start(1, "download")
		rep.Tick()
		rep.Finish()
		return handler.DownloadSubmissionsResult{Downloaded: 1}, nil
	}, "/tmp/e2e-submissions")

	// Screen 11: Operation History (read-only).
	handler.RegisterHistory(mux, pStore, oStore)

	// Screen 9: Set Password (sync PRG).
	handler.RegisterSetPassword(mux, pStore, func(
		_ context.Context, _ profile.ServerProfile, opts handler.SetPasswordOpts,
	) (handler.SetPasswordResult, error) {
		pw := opts.Password
		gen := pw == ""
		if gen {
			pw = "Generated-Pw-123!"
		}
		return handler.SetPasswordResult{Username: opts.Username, Password: pw, Generated: gen}, nil
	})

	// Screen 12: Settings (read/write via sStore).
	if err := handler.RegisterSettings(mux, sStore, pStore, handler.CLIInstallOptions{
		Host: addr, AppVersion: "preview",
		UnavailableReason: "CLI integration is disabled in preview mode. Open the packaged desktop app to install its command.",
	}); err != nil {
		log.Fatalf("settings routes: %v", err)
	}

	// Screen 10: Email Tools (nop funcs).
	handler.RegisterEmailTools(mux, sStore, oStore,
		func(_ context.Context, _ string, _ bool) error { return nil },
		func(_ context.Context, _ string) error { return nil },
		func(_ context.Context, _ handler.EmailRenderOpts) (int, error) { return 3, nil },
		func(_ context.Context, _ handler.EmailSendOpts, _ settings.AppSettings, rep progress.Reporter) (int, error) {
			rep.Start(2, "send")
			rep.Tick()
			rep.Tick()
			rep.Finish()
			return 2, nil
		},
	)

	handler.RegisterAPIFragment(mux, pStore, func(ctx context.Context, p profile.ServerProfile) ([]model.Contest, error) {
		if p.Host == "test-station.example.org" {
			return []model.Contest{
				{ID: "1", ShortName: "demo2024", Name: "Demo Contest 2024"},
				{ID: "2", ShortName: "test2025", Name: "Test Contest 2025"},
			}, nil
		}
		cfg := profileToConfig(p)
		api := apiv4.New(apiv4.Options{
			BaseURL:    cfg.Host,
			Username:   cfg.Username,
			Password:   cfg.Password,
			DisableSSL: cfg.DisableSSL,
			Timeout:    derefSecondsDuration(cfg.Timeout),
		})
		return api.Contests.List(ctx)
	}, func(ctx context.Context, p profile.ServerProfile) ([]model.User, error) {
		if p.Host == "test-station.example.org" {
			return []model.User{
				{ID: "1", Username: "admin", Name: "Administrator"},
				{ID: "2", Username: "user1", Name: "Demo User 1"},
			}, nil
		}
		cfg := profileToConfig(p)
		api := apiv4.New(apiv4.Options{
			BaseURL:    cfg.Host,
			Username:   cfg.Username,
			Password:   cfg.Password,
			DisableSSL: cfg.DisableSSL,
			Timeout:    derefSecondsDuration(cfg.Timeout),
		})
		return api.Users.List(ctx, apiv4.UsersListOpts{})
	}, func(ctx context.Context, p profile.ServerProfile, contestID string) ([]model.Problem, error) {
		if p.Host == "test-station.example.org" {
			return []model.Problem{
				{ID: "1", Label: "A", Name: "HelloWorld"},
				{ID: "2", Label: "B", Name: "AplusB"},
				{ID: "3", Label: "C", Name: "EasyMath"},
			}, nil
		}
		cfg := profileToConfig(p)
		api := apiv4.New(apiv4.Options{
			BaseURL:    cfg.Host,
			Username:   cfg.Username,
			Password:   cfg.Password,
			DisableSSL: cfg.DisableSSL,
			Timeout:    derefSecondsDuration(cfg.Timeout),
		})
		return api.Contests.ListProblems(ctx, contestID)
	}, func(ctx context.Context, p profile.ServerProfile) ([]model.Team, error) {
		if p.Host == "test-station.example.org" {
			return []model.Team{
				{ID: "10", Name: "Team A"},
				{ID: "20", Name: "Team B"},
			}, nil
		}
		cfg := profileToConfig(p)
		api := apiv4.New(apiv4.Options{
			BaseURL:    cfg.Host,
			Username:   cfg.Username,
			Password:   cfg.Password,
			DisableSSL: cfg.DisableSSL,
			Timeout:    derefSecondsDuration(cfg.Timeout),
		})
		cid := ""
		if p.ContestID != nil {
			cid = *p.ContestID
		}
		if cid == "" {
			contests, err := api.Contests.List(ctx)
			if err != nil {
				return nil, fmt.Errorf("list contests: %w", err)
			}
			if len(contests) == 0 {
				return nil, fmt.Errorf("no contests found on server")
			}
			cid = contests[0].ID
		}
		return api.Teams.List(ctx, cid)
	})

	handler.RegisterPickDirectory(mux, handler.DesktopPickDirectory)

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets.FS))))

	log.Printf("E2E server listening → http://%s/", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// profileToConfig converts a desktop ServerProfile to the legacy DomServerClient.
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
	if p.UserRoles != "" && p.UserRoles != "[]" {
		var roles []int
		if err := json.Unmarshal([]byte(p.UserRoles), &roles); err == nil {
			cfg.UserRoles = roles
		}
	}
	return cfg
}

func derefSecondsDuration(p *float64) time.Duration {
	if p == nil {
		return 0
	}
	return time.Duration(*p * float64(time.Second))
}
