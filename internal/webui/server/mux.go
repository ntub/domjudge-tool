package server

import (
	"net/http"

	"github.com/ntub/domjudge-tool/internal/webui/assets"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/oplog"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
	"github.com/ntub/domjudge-tool/internal/webui/settings"
)

// newMux registers every web UI route with the production operation
// functions. Security middleware is applied separately by protect.
func newMux(profiles *profile.Store, history *oplog.Store, appSettings *settings.Store) *http.ServeMux {
	mux := handler.New(profiles)
	handler.RegisterImportUsersTeams(mux, profiles, history, runImport)
	handler.RegisterRemoveUsersTeams(mux, profiles, history, runDelete)
	handler.RegisterExportUsers(mux, profiles, runExport)
	handler.RegisterDownloadProblems(mux, profiles, history, runDownloadProblems, defaultDownloadOutDir())
	handler.RegisterImportProblems(mux, profiles, history, runImportProblems)
	handler.RegisterDownloadScoreboard(mux, profiles, history, runDownloadScoreboard)
	handler.RegisterDownloadSubmissions(mux, profiles, history, runDownloadSubmissions, defaultDownloadOutDir())
	handler.RegisterHistory(mux, profiles, history)
	handler.RegisterSetPassword(mux, profiles, runSetPassword)
	handler.RegisterSettings(mux, appSettings, profiles)
	handler.RegisterEmailTools(mux, appSettings, history, emailScaffold, emailValidate, emailRender, emailSend)
	handler.RegisterAPIFragment(mux, profiles, fetchContests, fetchUsers, fetchProblems, fetchTeams)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets.FS))))
	return mux
}
