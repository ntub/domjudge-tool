// Package handler wires the HTTP routes of the local web UI.
// Each handler follows the PRG (Post/Redirect/Get) pattern; long-running
// operations stream progress over SSE.
package handler

import (
	"net/http"

	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

// New builds and returns the root ServeMux for the web UI.
// The caller (internal/webui/server) registers the remaining routes, such
// as /static/, on the returned mux before serving.
func New(store *profile.Store) *http.ServeMux {
	mux := http.NewServeMux()
	registerProfiles(mux, store)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/profiles", http.StatusSeeOther)
	})
	return mux
}
