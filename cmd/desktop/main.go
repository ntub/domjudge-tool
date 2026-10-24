//go:build desktop

// Command desktop is the Wails v3 desktop app for domjudge-tool.
// Build with: CGO_ENABLED=1 go build -tags desktop ./cmd/desktop
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ntub/domjudge-tool/internal/desktop/assets"
	"github.com/ntub/domjudge-tool/internal/desktop/db"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/oplog"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
	"github.com/ntub/domjudge-tool/internal/desktop/settings"
)

const port = ":9900"

// appVersion is injected by the native bundle packager.
var appVersion = "dev"

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
	handler.RegisterDownloadScoreboard(mux, store, oStore, desktopRunDownloadScoreboard)
	handler.RegisterDownloadSubmissions(mux, store, oStore, desktopRunDownloadSubmissions, defaultDownloadOutDir())
	handler.RegisterHistory(mux, store, oStore)
	handler.RegisterSetPassword(mux, store, desktopRunSetPassword)
	if err := handler.RegisterSettings(mux, sStore, store, desktopCLIInstallOptions("127.0.0.1"+port, appVersion)); err != nil {
		log.Fatalf("settings routes: %v", err)
	}
	handler.RegisterEmailTools(mux, sStore, oStore,
		desktopEmailScaffold, desktopEmailValidate, desktopEmailRender, desktopEmailSend)
	handler.RegisterAPIFragment(mux, store, desktopFetchContests, desktopFetchUsers, desktopFetchProblems, desktopFetchTeams)
	handler.RegisterPickDirectory(mux, handler.DesktopPickDirectory)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets.FS))))

	// Reserve the exact origin before Wails can open a window. A bind failure
	// must never navigate the WebView to a different process on the same port.
	listener, err := net.Listen("tcp", "127.0.0.1"+port)
	if err != nil {
		log.Fatalf("bind desktop listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	app := application.New(application.Options{
		Name: "domjudge-tool",
		Icon: assets.Icon,
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	downloads := nativeDownloads(mux, func(ctx context.Context, name string) (io.WriteCloser, error) {
		return pickDownloadFile(ctx, app, name)
	})
	go func() {
		if err := http.Serve(listener, downloads); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "domjudge-tool",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 560,
		URL:       "http://127.0.0.1" + port + "/",
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{
				ApplicationNameForUserAgent: desktopUserAgent,
			},
		},
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
