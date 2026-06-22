package handler

import (
	"context"
	"net/http"
	"regexp"

	"github.com/starfederation/datastar-go/datastar"
)

// DirectoryPickerFunc defines the signature for picking a directory path.
type DirectoryPickerFunc func(ctx context.Context) (string, error)

// signalName matches simple camelCase identifiers (e.g. "outDir").
var signalName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`)

// RegisterPickDirectory registers the GET /api/pick-directory?signal=<name> endpoint.
func RegisterPickDirectory(mux *http.ServeMux, picker DirectoryPickerFunc) {
	mux.HandleFunc("GET /api/pick-directory", func(w http.ResponseWriter, r *http.Request) {
		signal := r.URL.Query().Get("signal")
		if signal == "" || !signalName.MatchString(signal) {
			signal = "outDir"
		}

		path, err := picker(r.Context())
		if err != nil || path == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		sse := datastar.NewSSE(w, r)
		_ = sse.MarshalAndPatchSignals(map[string]any{signal: path})
	})
}
