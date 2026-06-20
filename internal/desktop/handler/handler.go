// Package handler wires the in-process HTTP mux for the desktop app.
// Each handler follows the PRG (Post/Redirect/Get) pattern; SSE endpoints
// are added per-screen as Phase 3 progresses.
package handler

import (
	"net/http"

	"github.com/ntub/domjudge-tool/internal/desktop/profile"
)

// New builds and returns the root ServeMux for the desktop app.
// The caller (cmd/desktop) may register additional routes (e.g. /static/)
// on the returned mux before starting the server.
func New(store *profile.Store) *http.ServeMux {
	mux := http.NewServeMux()
	registerProfiles(mux, store)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/profiles", http.StatusSeeOther)
	})
	return mux
}
