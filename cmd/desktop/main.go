//go:build desktop

// Command desktop is the Wails v3 desktop app for domjudge-tool.
// Build with: CGO_ENABLED=1 go build -tags desktop ./cmd/desktop
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ntub/domjudge-tool/internal/desktop/assets"
	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const port = ":9900"

func main() {
	dbPath := desktopDBPath()
	gdb, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	store, err := profile.NewStore(gdb)
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
	mux := handler.New(store)
	handler.RegisterImportUsersTeams(mux, store, oStore, desktopRunImport)
	handler.RegisterRemoveUsersTeams(mux, store, oStore, desktopRunDelete)
	handler.RegisterExportUsers(mux, store, desktopRunExport)
	handler.RegisterDownloadProblems(mux, store, oStore, desktopRunDownload, defaultDownloadOutDir())
	handler.RegisterImportProblems(mux, store, oStore, desktopRunImportProblems)
	handler.RegisterDownloadScoreboard(mux, store, desktopRunDownloadScoreboard)
	handler.RegisterDownloadSubmissions(mux, store, oStore, desktopRunDownloadSubmissions, defaultDownloadOutDir())
	handler.RegisterHistory(mux, store, oStore)
	handler.RegisterSetPassword(mux, store, desktopRunSetPassword)
	handler.RegisterSettings(mux, sStore, store)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets.FS))))

	go func() {
		if err := http.ListenAndServe("127.0.0.1"+port, mux); err != nil {
			log.Fatalf("http server: %v", err)
		}
	}()
	time.Sleep(80 * time.Millisecond)

	app := application.New(application.Options{
		Name: "domjudge-tool",
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "domjudge-tool",
		Width:  1280,
		Height: 800,
		URL:    "http://127.0.0.1" + port + "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// desktopDBPath returns the SQLite file path in the OS user config dir.
func desktopDBPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	appDir := filepath.Join(dir, "domjudge-tool")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return "domjudge-tool.db"
	}
	return filepath.Join(appDir, "domjudge-tool.db")
}
